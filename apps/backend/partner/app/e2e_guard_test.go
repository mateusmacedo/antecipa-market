//go:build integration

package app_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

// dialWith serves the context over the Resources of the composition root's
// bind, edited by the test, on a pool opened with the DSN the api role
// derives, so the guard runs under the session timeouts of production.
func dialWith(t *testing.T, test *pgxpool.Pool, edit func(*application.Resources)) *grpc.ClientConn {
	t.Helper()
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", test.Config().ConnString(), "GRPC_INSECURE", "true"))
	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.DSN)
	if err != nil {
		t.Fatalf("pgxpool.New() = %v", err)
	}
	t.Cleanup(pool.Close)
	policy, err := kernelapp.IdempotencyPolicy(time.Second, time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	waits := app.Waits{Command: time.Second}
	service := app.NewService(pool, pool, identity, provider.RolloutBucket{}, fixedClock{}, sequenceIDs{}, waits)
	service.Idempotency = policy
	bind := app.Bind(waits)
	service.UoW = app.UnitOfWork(pool, func(tx *postgres.Tx) application.Resources {
		res := bind(tx)
		edit(&res)
		return res
	})
	return serveOver(t, service)
}

// pause holds the first Save of one table with its row written and its lock
// taken, the transaction still open, until the test releases it.
type pause struct {
	once    sync.Once
	freed   sync.Once
	saved   chan struct{}
	release chan struct{}
}

func newPause() *pause { return &pause{saved: make(chan struct{}), release: make(chan struct{})} }

// free lets the paused Save commit; a failing round frees it on cleanup, or
// the open transaction would hold the pool the test closes.
func (p *pause) free() { p.freed.Do(func() { close(p.release) }) }

type pausedRepository[ID comparable, S any] struct {
	port.Repository[ID, S]
	pause *pause
}

func (r pausedRepository[ID, S]) Save(ctx context.Context, id ID, s S, expected port.Version) error {
	err := r.Repository.Save(ctx, id, s, expected)
	if err == nil {
		first := false
		r.pause.once.Do(func() { first = true; close(r.pause.saved) })
		if first {
			<-r.pause.release
		}
	}
	return err
}

func paused[ID comparable, S any](repository port.Repository[ID, S], p *pause) port.Repository[ID, S] {
	return pausedRepository[ID, S]{Repository: repository, pause: p}
}

// awaitGuardWait returns once a session of this database waits on an advisory
// lock: the reader is provably blocked on the guard the writer holds.
func awaitGuardWait(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		err := pool.QueryRow(context.Background(), `
			SELECT EXISTS (
			  SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid
			   WHERE l.locktype = 'advisory' AND NOT l.granted AND a.datname = current_database()
			     AND a.wait_event_type = 'Lock')`).Scan(&waiting)
		if err != nil {
			t.Fatalf("pg_locks = %v", err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no session waited on the guard; the round would not exercise the race")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// seen keeps the ids a guarded read returned, so a race whose verdict no
// supersession flips still proves the reader decided over the committed series.
type seen struct {
	mu  sync.Mutex
	ids []string
}

func (s *seen) record(ids ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ids = append(s.ids, ids...)
}

func (s *seen) holds(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.ids, id)
}

type seenAgreements struct {
	ports.AgreementsByPartnerReader
	seen *seen
}

func (r seenAgreements) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	found, err := r.AgreementsByPartnerReader.LoadByPartner(ctx, partnerID)
	for _, a := range found {
		r.seen.record(string(a.ID))
	}
	return found, err
}

type seenProfiles struct {
	ports.IntegrationProfilesByPartnerReader
	seen *seen
}

func (r seenProfiles) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error) {
	found, err := r.IntegrationProfilesByPartnerReader.LoadByPartner(ctx, partnerID)
	for _, p := range found {
		r.seen.record(string(p.ID))
	}
	return found, err
}

type answer struct {
	resp proto.Message
	err  error
}

func async(t *testing.T, conn *grpc.ClientConn, method string, req, resp proto.Message) <-chan answer {
	t.Helper()
	ctx := call(t)
	done := make(chan answer, 1)
	go func() { done <- answer{resp: resp, err: conn.Invoke(ctx, rpc.FullMethod(method), req, resp)} }()
	return done
}

// race runs the writer until its Save holds the guard, then the reader until it
// waits on it, and only then lets the writer commit.
func race(t *testing.T, pool *pgxpool.Pool, conn *grpc.ClientConn, p *pause, writer, reader func() <-chan answer) (wrote, read answer) {
	t.Helper()
	t.Cleanup(p.free)
	writing := writer()
	select {
	case <-p.saved:
	case a := <-writing:
		t.Fatalf("the writer ended before its Save held the guard: %v (%v)", a.err, a.resp)
	case <-time.After(5 * time.Second):
		t.Fatal("the writer never saved")
	}
	reading := reader()
	awaitGuardWait(t, pool)
	p.free()
	return <-writing, <-reading
}

func TestChangePartnerStageWaitsForASuspensionAndDecidesOverIt(t *testing.T) {
	pool := appkit.OpenPool(t)
	setup := dial(t, pool, nil)
	homologatingPartner(t, setup, pool)
	homologatedCapability(t, setup, "e2e-cap-1")
	invoke(t, setup, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
	p := newPause()
	conn := dialWith(t, pool, func(res *application.Resources) { res.Capabilities = paused(res.Capabilities, p) })

	wrote, read := race(t, pool, conn, p,
		func() <-chan answer {
			return async(t, conn, "SuspendCapability", &servicev1.SuspendCapabilityRequest{CapabilityId: "e2e-cap-1", Reason: "partner incident"}, &servicev1.SuspendCapabilityResponse{})
		},
		func() <-chan answer {
			return async(t, conn, "ChangePartnerStage", stageTo(servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, ""), &servicev1.ChangePartnerStageResponse{})
		})

	if wrote.err != nil || wrote.resp.(*servicev1.SuspendCapabilityResponse).GetSuspended() == nil {
		t.Fatalf("SuspendCapability() = %v (%v), want suspended", wrote.err, wrote.resp)
	}
	if got := read.resp.(*servicev1.ChangePartnerStageResponse).GetRejection().GetCode(); read.err != nil || got != string(domain.CodePartnerNoEnabledCapability) {
		t.Fatalf("ChangePartnerStage(active) = %v (%v), want %s: the capability it read was the one the suspension committed", read.err, read.resp, domain.CodePartnerNoEnabledCapability)
	}
	if got := count(t, pool, "SELECT count(*) FROM partners WHERE status = 'homologating'"); got != 1 {
		t.Fatalf("homologating partners = %d, want the partner untouched", got)
	}
}

func TestEnableCapabilityWaitsForASupersessionOfTheAgreementAndDecidesOverIt(t *testing.T) {
	pool := appkit.OpenPool(t)
	setup := dial(t, pool, nil)
	homologatingPartner(t, setup, pool)
	homologatedCapability(t, setup, "e2e-cap-1")
	p, read := newPause(), &seen{}
	conn := dialWith(t, pool, func(res *application.Resources) {
		res.Agreements = paused(res.Agreements, p)
		res.AgreementsByPartner = seenAgreements{AgreementsByPartnerReader: res.AgreementsByPartner, seen: read}
	})

	wrote, enabled := race(t, pool, conn, p,
		func() <-chan answer {
			return async(t, conn, "RegisterAgreement", agreementOf("e2e-agr-2", "e2e-agr-1"), &servicev1.RegisterAgreementResponse{})
		},
		func() <-chan answer {
			return async(t, conn, "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
		})

	if wrote.err != nil || wrote.resp.(*servicev1.RegisterAgreementResponse).GetRegistered() == nil {
		t.Fatalf("RegisterAgreement(v2) = %v (%v), want registered", wrote.err, wrote.resp)
	}
	if enabled.err != nil || enabled.resp.(*servicev1.EnableCapabilityResponse).GetEnabled() == nil {
		t.Fatalf("EnableCapability() = %v (%v), want enabled over the series with the draft v2 and the active v1", enabled.err, enabled.resp)
	}
	if !read.holds("e2e-agr-2") {
		t.Fatalf("the guarded read returned %v, want the v2 the supersession committed", read.ids)
	}
}

func TestEnableCapabilityWaitsForASupersessionOfTheProfileAndDecidesOverIt(t *testing.T) {
	pool := appkit.OpenPool(t)
	setup := dial(t, pool, nil)
	homologatingPartner(t, setup, pool)
	homologatedCapability(t, setup, "e2e-cap-1")
	p, read := newPause(), &seen{}
	conn := dialWith(t, pool, func(res *application.Resources) {
		res.IntegrationProfiles = paused(res.IntegrationProfiles, p)
		res.ProfilesByPartner = seenProfiles{IntegrationProfilesByPartnerReader: res.ProfilesByPartner, seen: read}
	})
	second := profileOf("e2e-ipf-2", 90*day, "e2e-ipf-1")

	wrote, enabled := race(t, pool, conn, p,
		func() <-chan answer {
			return async(t, conn, "RegisterIntegrationProfile", second, &servicev1.RegisterIntegrationProfileResponse{})
		},
		func() <-chan answer {
			return async(t, conn, "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
		})

	if wrote.err != nil || wrote.resp.(*servicev1.RegisterIntegrationProfileResponse).GetRegistered() == nil {
		t.Fatalf("RegisterIntegrationProfile(v2) = %v (%v), want registered", wrote.err, wrote.resp)
	}
	if enabled.err != nil || enabled.resp.(*servicev1.EnableCapabilityResponse).GetEnabled() == nil {
		t.Fatalf("EnableCapability() = %v (%v), want enabled over the series with the v2 of the profile", enabled.err, enabled.resp)
	}
	if !read.holds("e2e-ipf-2") {
		t.Fatalf("the guarded read returned %v, want the v2 the supersession committed", read.ids)
	}
}

func TestDefineProductWaitsForTheActivationOfASuccessorAndDecidesOverIt(t *testing.T) {
	pool := appkit.OpenPool(t)
	setup := dial(t, pool, nil)
	withActiveAgreement(t, setup)
	invoke(t, setup, call(t), "RegisterAgreement", agreementOf("e2e-agr-2", "e2e-agr-1"), &servicev1.RegisterAgreementResponse{})
	p := newPause()
	conn := dialWith(t, pool, func(res *application.Resources) { res.Agreements = paused(res.Agreements, p) })

	wrote, read := race(t, pool, conn, p,
		func() <-chan answer {
			return async(t, conn, "ActivateAgreement", activation("e2e-agr-2"), &servicev1.ActivateAgreementResponse{})
		},
		func() <-chan answer {
			return async(t, conn, "DefineProduct", productOf("e2e-fpr-1", ""), &servicev1.DefineProductResponse{})
		})

	if wrote.err != nil || wrote.resp.(*servicev1.ActivateAgreementResponse).GetActivated() == nil {
		t.Fatalf("ActivateAgreement(v2) = %v (%v), want activated", wrote.err, wrote.resp)
	}
	if got := read.resp.(*servicev1.DefineProductResponse).GetRejection().GetCode(); read.err != nil || got != string(domain.CodeFinancialProductAgreementNotActive) {
		t.Fatalf("DefineProduct(over v1) = %v (%v), want %s: v2 is the agreement in effect once the activation commits", read.err, read.resp, domain.CodeFinancialProductAgreementNotActive)
	}
	if rows, _ := products(t, pool); rows != 0 {
		t.Fatalf("financial_products = %d, want none over a superseded agreement", rows)
	}
}

type conflictingCapabilities struct{}

func (conflictingCapabilities) LoadByPartner(context.Context, domain.PartnerID) ([]domain.CapabilitySnapshot, error) {
	return nil, fmt.Errorf("postgres: guarded read: %w", port.ErrVersionConflict)
}

// A deadlock between two guarded transactions reaches the reader as the version
// conflict of the kernel; the edge answers it as Aborted, a call to replay.
func TestAConflictOnTheGuardIsAbortedAtTheEdge(t *testing.T) {
	pool := appkit.OpenPool(t)
	homologatingPartner(t, dial(t, pool, nil), pool)
	conn := dialWith(t, pool, func(res *application.Resources) { res.CapabilitiesByPartner = conflictingCapabilities{} })

	var resp servicev1.ChangePartnerStageResponse
	err := conn.Invoke(call(t), rpc.FullMethod("ChangePartnerStage"), stageTo(servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, ""), &resp)

	if status.Code(err) != codes.Aborted {
		t.Fatalf("ChangePartnerStage() = %v (%v), want Aborted", err, &resp)
	}
}

// A reader the guard keeps waiting past the lock_timeout of the session ends as
// the same conflict: the edge answers Aborted, never Internal.
func TestALockWaitPastTheTimeoutIsAbortedAtTheEdge(t *testing.T) {
	pool := appkit.OpenPool(t)
	setup := dial(t, pool, nil)
	homologatingPartner(t, setup, pool)
	homologatedCapability(t, setup, "e2e-cap-1")
	invoke(t, setup, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
	p := newPause()
	conn := dialWith(t, pool, func(res *application.Resources) { res.Capabilities = paused(res.Capabilities, p) })
	t.Cleanup(p.free)
	writing := async(t, conn, "SuspendCapability", &servicev1.SuspendCapabilityRequest{CapabilityId: "e2e-cap-1", Reason: "partner incident"}, &servicev1.SuspendCapabilityResponse{})
	select {
	case <-p.saved:
	case a := <-writing:
		t.Fatalf("the writer ended before its Save held the guard: %v (%v)", a.err, a.resp)
	case <-time.After(5 * time.Second):
		t.Fatal("the writer never saved")
	}

	var resp servicev1.ChangePartnerStageResponse
	err := conn.Invoke(call(t), rpc.FullMethod("ChangePartnerStage"), stageTo(servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, ""), &resp)

	if status.Code(err) != codes.Aborted {
		t.Fatalf("ChangePartnerStage() = %v (%v), want Aborted after the lock_timeout", err, &resp)
	}
	p.free()
	if wrote := <-writing; wrote.err != nil || wrote.resp.(*servicev1.SuspendCapabilityResponse).GetSuspended() == nil {
		t.Fatalf("SuspendCapability() = %v (%v), want suspended once released", wrote.err, wrote.resp)
	}
}
