//go:build integration

package app_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
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
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	waits := app.Waits{Message: time.Second, Command: time.Second}
	// A prefix of its own: the sequence of the setup service already issued "e2e-".
	service := app.NewService(pool, memory.FixedClock{At: occurred}, &memory.SequenceIDs{Prefix: "guard-"}, waits)
	service.Idempotency = policy
	bind := app.Bind(waits)
	service.UoW = app.UnitOfWork(pool, func(tx *postgres.Tx) application.Resources {
		res := bind(tx)
		edit(&res)
		return res
	})
	return serveOver(t, service)
}

func call(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, string(testTenant), kernelgrpc.IdempotencyKey, fmt.Sprintf("e2e-%d", e2eKeys.Add(1)))
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
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
func race(t *testing.T, pool *pgxpool.Pool, p *pause, writer, reader func() <-chan answer) (wrote, read answer) {
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

func createAsync(t *testing.T, conn *grpc.ClientConn, key string, version int32) func() <-chan answer {
	return func() <-chan answer {
		return async(t, conn, "CreateExternalCommand", create(key, version), &servicev1.CreateExternalCommandResponse{})
	}
}

// A profile that withdraws submitEffect, effective at the instant of the setup
// profile: as the higher version it is the one in force.
func withdrawnProfile(version int32) *servicev1.DeclareCapabilityProfileRequest {
	return &servicev1.DeclareCapabilityProfileRequest{
		ConnectorId: "prt-1:production", Version: version, EffectiveFrom: int64(occurred),
		Operations: []*servicev1.OperationCapability{{
			Operation: "submitEffect", Supported: false, IdempotencySupport: servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_KEY_HEADER,
			Efficacy: servicev1.Efficacy_EFFICACY_ON_SUBMIT, Quiescence: 1, MaxQueries: 3, KeyAlphabet: "0123456789abcdef",
			KeyMaxLength: 32, AmbiguityDeadline: 3600, Protocol: servicev1.Protocol_PROTOCOL_SYNC_HTTP,
			EncumbranceCoverage: servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_UNAVAILABLE,
		}},
	}
}

func TestCreateExternalCommandWaitsForACommandOfTheSeriesAndDecidesOverIt(t *testing.T) {
	setup := serve(t)
	setup.ready(t)
	p := newPause()
	conn := dialWith(t, setup.pool, func(res *application.Resources) { res.ExternalCommands = paused(res.ExternalCommands, p) })

	wrote, read := race(t, setup.pool, p, createAsync(t, conn, "race-1", 1), createAsync(t, conn, "race-2", 2))

	if wrote.err != nil || wrote.resp.(*servicev1.CreateExternalCommandResponse).GetCreated() == nil {
		t.Fatalf("CreateExternalCommand(v1) = %v (%v), want created", wrote.err, wrote.resp)
	}
	if got := read.resp.(*servicev1.CreateExternalCommandResponse).GetRejection().GetCode(); read.err != nil || got != string(domain.CodeExternalCommandContentChangedAfterSend) {
		t.Fatalf("CreateExternalCommand(v2) = %v (%v), want %s: the series it read held the v1 the writer committed", read.err, read.resp, domain.CodeExternalCommandContentChangedAfterSend)
	}
	if got := count(t, setup.pool, "SELECT count(*) FROM external_commands WHERE external_command_id = 'race-1'"); got != 1 {
		t.Fatalf("external_commands race-1 = %d, want the committed v1", got)
	}
	if got := count(t, setup.pool, "SELECT count(*) FROM external_commands"); got != 1 {
		t.Fatalf("external_commands = %d, want the v2 left out of a series that is alive", got)
	}
	if got := setup.destinations(t); len(got) != 1 || got[0] != application.DispatchDestination {
		t.Fatalf("outbox destinations = %v, want only the dispatch intent of the v1", got)
	}
}

func TestCreateExternalCommandWaitsForANewProfileVersionAndDecidesOverIt(t *testing.T) {
	setup := serve(t)
	setup.ready(t)
	p := newPause()
	conn := dialWith(t, setup.pool, func(res *application.Resources) { res.Profiles = paused(res.Profiles, p) })

	wrote, read := race(t, setup.pool, p,
		func() <-chan answer {
			return async(t, conn, "DeclareCapabilityProfile", withdrawnProfile(2), &servicev1.DeclareCapabilityProfileResponse{})
		},
		createAsync(t, conn, "race-1", 1))

	if wrote.err != nil || wrote.resp.(*servicev1.DeclareCapabilityProfileResponse).GetDeclared() == nil {
		t.Fatalf("DeclareCapabilityProfile(v2) = %v (%v), want declared", wrote.err, wrote.resp)
	}
	if got := read.resp.(*servicev1.CreateExternalCommandResponse).GetRejection().GetCode(); read.err != nil || got != string(domain.CodeExternalCommandCapabilityNotEnabled) {
		t.Fatalf("CreateExternalCommand() = %v (%v), want %s: the profile in force it read was the v2 the declaration committed", read.err, read.resp, domain.CodeExternalCommandCapabilityNotEnabled)
	}
	if got := count(t, setup.pool, "SELECT count(*) FROM capability_profiles"); got != 2 {
		t.Fatalf("capability_profiles = %d, want the v1 of the setup and the committed v2", got)
	}
	if got := count(t, setup.pool, "SELECT count(*) FROM external_commands"); got != 0 {
		t.Fatalf("external_commands = %d, want none created under a profile that was superseded", got)
	}
	if got := setup.destinations(t); len(got) != 0 {
		t.Fatalf("outbox destinations = %v, want no dispatch intent", got)
	}
}

type conflictingProfiles struct{}

func (conflictingProfiles) LoadByConnector(context.Context, domain.ConnectorKey) ([]domain.CapabilityProfileSnapshot, error) {
	return nil, fmt.Errorf("postgres: guarded read: %w", port.ErrVersionConflict)
}

type conflictingSeries struct{}

func (conflictingSeries) LoadBySeries(context.Context, string) ([]domain.ExternalCommandSnapshot, error) {
	return nil, fmt.Errorf("postgres: guarded read: %w", port.ErrVersionConflict)
}

// A deadlock between two guarded transactions reaches the reader as the version
// conflict of the kernel; the edge answers it as Aborted, a call to replay.
func TestAConflictOnTheGuardIsAbortedAtTheEdge(t *testing.T) {
	setup := serve(t)
	setup.ready(t)
	for _, c := range []struct {
		guard string
		edit  func(*application.Resources)
	}{
		{"the series", func(res *application.Resources) { res.CommandsBySeries = conflictingSeries{} }},
		{"the connector", func(res *application.Resources) { res.ProfilesByConnector = conflictingProfiles{} }},
	} {
		t.Run(c.guard, func(t *testing.T) {
			conn := dialWith(t, setup.pool, c.edit)

			var resp servicev1.CreateExternalCommandResponse
			err := conn.Invoke(call(t), rpc.FullMethod("CreateExternalCommand"), create("conflict-1", 1), &resp)

			if status.Code(err) != codes.Aborted {
				t.Fatalf("CreateExternalCommand() = %v (%v), want Aborted", err, &resp)
			}
			if got := count(t, setup.pool, "SELECT count(*) FROM external_commands"); got != 0 {
				t.Fatalf("external_commands = %d, want nothing written by a call that was aborted", got)
			}
		})
	}
}

// A reader the guard keeps waiting past the lock_timeout of the session ends as
// the same conflict: the edge answers Aborted, never Internal.
func TestALockWaitPastTheTimeoutIsAbortedAtTheEdge(t *testing.T) {
	setup := serve(t)
	setup.ready(t)
	p := newPause()
	conn := dialWith(t, setup.pool, func(res *application.Resources) { res.ExternalCommands = paused(res.ExternalCommands, p) })
	t.Cleanup(p.free)
	writing := createAsync(t, conn, "race-1", 1)()
	select {
	case <-p.saved:
	case a := <-writing:
		t.Fatalf("the writer ended before its Save held the guard: %v (%v)", a.err, a.resp)
	case <-time.After(5 * time.Second):
		t.Fatal("the writer never saved")
	}

	var resp servicev1.CreateExternalCommandResponse
	err := conn.Invoke(call(t), rpc.FullMethod("CreateExternalCommand"), create("race-2", 2), &resp)

	if status.Code(err) != codes.Aborted {
		t.Fatalf("CreateExternalCommand(v2) = %v (%v), want Aborted after the lock_timeout", err, &resp)
	}
	p.free()
	if wrote := <-writing; wrote.err != nil || wrote.resp.(*servicev1.CreateExternalCommandResponse).GetCreated() == nil {
		t.Fatalf("CreateExternalCommand(v1) = %v (%v), want created once released", wrote.err, wrote.resp)
	}
	if got := count(t, setup.pool, "SELECT count(*) FROM external_commands"); got != 1 {
		t.Fatalf("external_commands = %d, want only the v1 of the writer", got)
	}
}
