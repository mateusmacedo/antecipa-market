//go:build integration

package app_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

const (
	tenantA    = "acme"
	validFrom  = int64(1_755_432_000_000_000_000)
	validUntil = validFrom + 365*86_400_000_000_000
)

type fixedClock struct{}

func (fixedClock) Now() port.Instant { return port.Instant(validFrom) }

// messages numbers across every service a test dials: the kernel outbox holds
// message_id unique (outbox.sql, outbox_message_id_key), and a sequence per
// service would hand a second one an id the first already wrote.
var messages atomic.Int64

type sequenceIDs struct{}

func (sequenceIDs) NewMessageID() port.MessageID {
	return port.MessageID(fmt.Sprintf("m-%06d", messages.Add(1)))
}

type fakeDirectory map[domain.OrganizationID][]domain.OrganizationKind

func (d fakeDirectory) FindOrganization(_ context.Context, _ port.TenantID, id domain.OrganizationID) ([]domain.OrganizationKind, error) {
	kinds, found := d[id]
	if !found {
		return nil, port.ErrNotFound
	}
	return kinds, nil
}

var identity = fakeDirectory{
	"org-funder":   {domain.KindFunder},
	"org-funder-2": {domain.KindFunder},
	"org-partner":  {domain.KindPartner},
	"org-client":   {"client"},
}

// barrier holds each Save until every party arrived, so the transactions are
// all open, and every pre-read has run, before any insert does.
type barrier struct {
	arrivals sync.WaitGroup
	mu       sync.Mutex
	refused  []error
}

func newBarrier(parties int) *barrier {
	b := &barrier{}
	b.arrivals.Add(parties)
	return b
}

func (b *barrier) await() {
	b.arrivals.Done()
	released := make(chan struct{})
	go func() {
		b.arrivals.Wait()
		close(released)
	}()
	select {
	case <-released:
	case <-time.After(5 * time.Second):
	}
}

func (b *barrier) record(err error) error {
	if err != nil {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.refused = append(b.refused, err)
	}
	return err
}

func (b *barrier) refusals() []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.refused)
}

type barrierRepository[ID comparable, S any] struct {
	port.Repository[ID, S]
	barrier *barrier
}

func (r barrierRepository[ID, S]) Save(ctx context.Context, id ID, s S, expected port.Version) error {
	r.barrier.await()
	return r.barrier.record(r.Repository.Save(ctx, id, s, expected))
}

func held[ID comparable, S any](repository port.Repository[ID, S], b *barrier) port.Repository[ID, S] {
	if b == nil {
		return repository
	}
	return barrierRepository[ID, S]{Repository: repository, barrier: b}
}

func dial(t *testing.T, pool *pgxpool.Pool, saves *barrier) *grpc.ClientConn {
	t.Helper()
	policy, err := kernelapp.IdempotencyPolicy(time.Second, time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := app.NewService(pool, pool, identity, provider.RolloutBucket{}, fixedClock{}, sequenceIDs{}, app.Waits{Command: time.Second})
	service.Idempotency = policy
	if saves != nil {
		service.UoW = postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) application.Resources {
			return application.Resources{
				Partners:   held(provider.NewPartnerRepository(tx), saves),
				Agreements: held(provider.NewAgreementRepository(tx), saves),

				IntegrationProfiles: held(provider.NewIntegrationProfileRepository(tx), saves),
				OperationalPolicies: held(provider.NewOperationalPolicyRepository(tx), saves),
				Capabilities:        held(provider.NewCapabilityRepository(tx), saves),
				FinancialProducts:   held(provider.NewFinancialProductRepository(tx), saves),

				AgreementsByPartner:   provider.NewAgreementsByPartnerWithin(tx),
				ProfilesByPartner:     provider.NewIntegrationProfilesByPartnerWithin(tx),
				CapabilitiesByPartner: provider.NewCapabilitiesByPartnerWithin(tx),

				Outbox:   tx.Outbox(provider.Mapper{}),
				Commands: tx.CommandInbox(application.CommandConsumer, time.Second),
			}
		})
	}
	return serveOver(t, service)
}

// serveOver puts a service behind the gRPC edge of the context, with the
// interceptor chain of the kernel, and dials it.
func serveOver(t *testing.T, service application.Service) *grpc.ClientConn {
	t.Helper()
	limit := admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}
	ctrl, err := admission.New(admission.Config{Limits: kernelgrpc.MethodLimits(rpc.ServiceName, rpc.Methods(), limit), MaxKeys: 16, Clock: obsclock.System()})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	server, _, err := kernelgrpc.NewServer(kernelgrpc.ServerConfig{
		InsecureForDevelopmentOnly: true,
		Services:                   []string{rpc.ServiceName},
		UnaryInterceptors:          kernelgrpc.ServerInterceptors(rpc.ServiceName, ctrl, nil, nil, kernelgrpc.WithCommands(rpc.Commands()...)),
	})
	if err != nil {
		t.Fatalf("NewServer() = %v", err)
	}
	server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})
	listener := bufconn.Listen(1 << 20)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

var keys atomic.Int64

func call(t *testing.T) context.Context {
	t.Helper()
	return callWithKey(t, fmt.Sprintf("e2e-k-%d", keys.Add(1)))
}

func callWithKey(t *testing.T, key string) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if key != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.IdempotencyKey, key)
	}
	return metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, tenantA)
}

func registration(partnerID string) *servicev1.RegisterPartnerRequest {
	return &servicev1.RegisterPartnerRequest{PartnerId: partnerID, OrganizationId: "org-funder", Role: servicev1.PartnerRole_PARTNER_ROLE_FUNDER}
}

func agreementOf(agreementID, supersedes string) *servicev1.RegisterAgreementRequest {
	return &servicev1.RegisterAgreementRequest{
		AgreementId: agreementID, PartnerId: "e2e-prt-1", ValidFrom: validFrom, ValidUntil: validUntil, SupersedesId: supersedes,
	}
}

func activation(agreementID string) *servicev1.ActivateAgreementRequest {
	return &servicev1.ActivateAgreementRequest{
		AgreementId: agreementID, ContractEvidenceId: "evd-1", SatisfiedCriteria: []string{"H14", "H15", "H17", "H18"},
	}
}

func invoke[Resp proto.Message](t *testing.T, conn *grpc.ClientConn, ctx context.Context, method string, req proto.Message, resp Resp) Resp {
	t.Helper()
	if err := conn.Invoke(ctx, rpc.FullMethod(method), req, resp); err != nil {
		t.Fatalf("%s() = %v", method, err)
	}
	return resp
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func rows(t *testing.T, pool *pgxpool.Pool) (partners, agreements, events int) {
	t.Helper()
	return count(t, pool, "SELECT count(*) FROM partners"),
		count(t, pool, "SELECT count(*) FROM partner_agreements"),
		count(t, pool, "SELECT count(*) FROM outbox")
}

type enqueued struct {
	messageType, destination, partitionKey string
	payload                                []byte
}

func outbox(t *testing.T, pool *pgxpool.Pool) []enqueued {
	t.Helper()
	result, err := pool.Query(context.Background(), "SELECT message_type, destination, partition_key, payload FROM outbox ORDER BY id")
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer result.Close()
	var out []enqueued
	for result.Next() {
		var e enqueued
		if err := result.Scan(&e.messageType, &e.destination, &e.partitionKey, &e.payload); err != nil {
			t.Fatalf("scan outbox: %v", err)
		}
		out = append(out, e)
	}
	if err := result.Err(); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	return out
}

func TestRegisterPartnerEndToEndOverGRPC(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	resp := invoke(t, conn, call(t), "RegisterPartner", registration("e2e-prt-1"), &servicev1.RegisterPartnerResponse{})

	if resp.GetRegistered().GetPartnerId() != "e2e-prt-1" {
		t.Fatalf("RegisterPartner() = %v, want Registered e2e-prt-1", resp)
	}
	if got := count(t, pool, "SELECT count(*) FROM partners WHERE status = 'prospect' AND organization_id = 'org-funder' AND role = 'funder'"); got != 1 {
		t.Fatalf("prospect funders of org-funder = %d, want 1", got)
	}
	records := outbox(t, pool)
	if len(records) != 1 {
		t.Fatalf("outbox rows = %d, want 1", len(records))
	}
	record := records[0]
	if record.messageType != "com.company.partner.partner-registered.v1" || record.destination != application.Destination || record.partitionKey != "e2e-prt-1" {
		t.Fatalf("outbox record = %s to %s keyed %s, want the partner-registered type to %s keyed by the partner",
			record.messageType, record.destination, record.partitionKey, application.Destination)
	}
	var event eventv1.PartnerRegistered
	if err := proto.Unmarshal(record.payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetPartnerId() != "e2e-prt-1" || event.GetOrganizationId() != "org-funder" || event.GetRole() != eventv1.PartnerRole_PARTNER_ROLE_FUNDER ||
		event.GetAt().AsTime().UnixNano() != validFrom {
		t.Fatalf("event = %v, want the funder registered at the instant of the command", &event)
	}
}

func TestEachRefusalOfRegisterPartnerWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterPartner", registration("e2e-prt-held"), &servicev1.RegisterPartnerResponse{})

	for _, c := range []struct {
		name string
		edit func(*servicev1.RegisterPartnerRequest)
		code string
	}{
		{"organization unknown to identity", func(r *servicev1.RegisterPartnerRequest) { r.OrganizationId = "org-none" },
			"partner-management/partner/organization-not-found"},
		{"funder of a client organization", func(r *servicev1.RegisterPartnerRequest) { r.OrganizationId = "org-client" },
			"partner-management/partner/organization-kind-mismatch"},
		{"organization and role already registered", func(*servicev1.RegisterPartnerRequest) {},
			"partner-management/partner/already-registered"},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := registration("e2e-prt-refused")
			c.edit(req)

			resp := invoke(t, conn, call(t), "RegisterPartner", req, &servicev1.RegisterPartnerResponse{})

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q (response %v)", got, c.code, resp)
			}
			if p, _, e := rows(t, pool); p != 1 || e != 1 {
				t.Fatalf("partners = %d, outbox = %d after a refusal, want only the held partner and its event", p, e)
			}
		})
	}
}

type attempt struct {
	ok        bool
	rejection string
	err       error
}

type refusable interface {
	GetRejection() *servicev1.Rejection
}

func invokeAtOnce(t *testing.T, conn *grpc.ClientConn, method string, calls map[string]proto.Message, accepted func(proto.Message) bool, response func() proto.Message) map[string]attempt {
	t.Helper()
	var (
		mu    sync.Mutex
		group sync.WaitGroup
	)
	out := map[string]attempt{}
	for name, req := range calls {
		ctx := call(t)
		group.Go(func() {
			resp := response()
			err := conn.Invoke(ctx, rpc.FullMethod(method), req, resp)
			mu.Lock()
			defer mu.Unlock()
			a := attempt{ok: err == nil && accepted(resp), err: err}
			if r, refused := resp.(refusable); refused && err == nil {
				a.rejection = r.GetRejection().GetCode()
			}
			out[name] = a
		})
	}
	group.Wait()
	return out
}

// oneAcceptedOneRejected holds the race to its single legal ending: the unique
// constraint decides, and the loser answers the rejection it declares (ADR-066).
func oneAcceptedOneRejected(t *testing.T, got map[string]attempt, code string) {
	t.Helper()
	accepted, rejected := 0, 0
	for name, a := range got {
		switch {
		case a.ok:
			accepted++
		case a.err == nil && a.rejection == code:
			rejected++
		default:
			t.Errorf("%s = %v (rejection %q), want either accepted or the rejection %s", name, a.err, a.rejection, code)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("accepted %d and rejected %d, want exactly one of each", accepted, rejected)
	}
}

func refusedByUniqueness(t *testing.T, saves *barrier, constraint string) {
	t.Helper()
	refused := saves.refusals()
	var violation port.UniqueViolation
	if len(refused) != 1 || !errors.As(refused[0], &violation) || violation.Constraint != constraint {
		t.Fatalf("refused Saves = %v, want exactly one, the loser's, at %s", refused, constraint)
	}
}

func TestTwoConcurrentRegistrationsOfOneOrganizationAndRoleLeaveOnePartner(t *testing.T) {
	pool := appkit.OpenPool(t)
	saves := newBarrier(2)
	conn := dial(t, pool, saves)

	got := invokeAtOnce(t, conn, "RegisterPartner",
		map[string]proto.Message{"first": registration("e2e-prt-a1"), "second": registration("e2e-prt-a2")},
		func(m proto.Message) bool { return m.(*servicev1.RegisterPartnerResponse).GetRegistered() != nil },
		func() proto.Message { return &servicev1.RegisterPartnerResponse{} })

	oneAcceptedOneRejected(t, got, string(domain.CodePartnerAlreadyRegistered))
	refusedByUniqueness(t, saves, "partners_tenant_id_organization_id_role_key")
	if p, _, e := rows(t, pool); p != 1 || e != 1 {
		t.Fatalf("partners = %d, outbox = %d, want exactly one row and one PartnerRegistered", p, e)
	}
}

func TestRegisterAgreementEndToEndKeepsOneSeriesOfVersions(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	first := invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-1", ""), &servicev1.RegisterAgreementResponse{})
	second := invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-2", "e2e-agr-1"), &servicev1.RegisterAgreementResponse{})
	stale := invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-3", "e2e-agr-1"), &servicev1.RegisterAgreementResponse{})

	if first.GetRegistered().GetAgreementId() != "e2e-agr-1" || second.GetRegistered().GetAgreementId() != "e2e-agr-2" {
		t.Fatalf("RegisterAgreement() = %v and %v, want versions 1 and 2 registered", first, second)
	}
	if got := stale.GetRejection().GetCode(); got != "partner-management/partner-agreement/supersedes-invalid" {
		t.Fatalf("RegisterAgreement(stale supersedes) = %v, want supersedes-invalid", stale)
	}
	if got := count(t, pool, "SELECT count(*) FROM partner_agreements WHERE status = 'draft' AND (snapshot->>'version')::int = 2 AND supersedes_id = 'e2e-agr-1'"); got != 1 {
		t.Fatalf("drafts at version 2 superseding e2e-agr-1 = %d, want 1", got)
	}
	if _, a, e := rows(t, pool); a != 2 || e != 0 {
		t.Fatalf("partner_agreements = %d, outbox = %d, want two drafts and nothing emitted", a, e)
	}
}

func TestARepeatedRegisterAgreementReplaysWithoutANewRow(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	req := agreementOf("e2e-agr-1", "")

	var first, again servicev1.RegisterAgreementResponse
	var header metadata.MD
	if err := conn.Invoke(callWithKey(t, "e2e-replay"), rpc.FullMethod("RegisterAgreement"), req, &first); err != nil {
		t.Fatalf("RegisterAgreement() = %v", err)
	}
	if err := conn.Invoke(callWithKey(t, "e2e-replay"), rpc.FullMethod("RegisterAgreement"), req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated RegisterAgreement() = %v, want the stored response", err)
	}

	if again.GetRegistered().GetAgreementId() != "e2e-agr-1" || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want Registered e2e-agr-1 marked as replay", &again, header)
	}
	if _, a, _ := rows(t, pool); a != 1 {
		t.Fatalf("partner_agreements = %d, want the single row of the first call", a)
	}
}

func TestTwoConcurrentFirstVersionsOfOneSeriesLeaveOneAgreement(t *testing.T) {
	pool := appkit.OpenPool(t)
	saves := newBarrier(2)
	conn := dial(t, pool, saves)

	got := invokeAtOnce(t, conn, "RegisterAgreement",
		map[string]proto.Message{"first": agreementOf("e2e-agr-a1", ""), "second": agreementOf("e2e-agr-a2", "")},
		func(m proto.Message) bool { return m.(*servicev1.RegisterAgreementResponse).GetRegistered() != nil },
		func() proto.Message { return &servicev1.RegisterAgreementResponse{} })

	oneAcceptedOneRejected(t, got, string(domain.CodePartnerAgreementSupersedesInvalid))
	refusedByUniqueness(t, saves, "partner_agreements_tenant_id_series_origin_key")
	if _, a, _ := rows(t, pool); a != 1 {
		t.Fatalf("partner_agreements = %d, want exactly one first version", a)
	}
}

func TestActivateAgreementEndToEndEmitsTheActivation(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-1", ""), &servicev1.RegisterAgreementResponse{})

	resp := invoke(t, conn, call(t), "ActivateAgreement", activation("e2e-agr-1"), &servicev1.ActivateAgreementResponse{})

	if resp.GetActivated().GetAgreementId() != "e2e-agr-1" {
		t.Fatalf("ActivateAgreement() = %v, want Activated e2e-agr-1", resp)
	}
	records := outbox(t, pool)
	if len(records) != 1 || records[0].messageType != "com.company.partner.partner-agreement-activated.v1" || records[0].partitionKey != "e2e-agr-1" {
		t.Fatalf("outbox = %+v, want one partner-agreement-activated keyed by the agreement", records)
	}
	var event eventv1.PartnerAgreementActivated
	if err := proto.Unmarshal(records[0].payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetAgreementId() != "e2e-agr-1" || event.GetPartnerId() != "e2e-prt-1" || event.GetVersion() != 1 ||
		event.GetValidUntil().AsTime().UnixNano() != validUntil {
		t.Fatalf("event = %v, want version 1 of e2e-prt-1 with its validity", &event)
	}
}

func TestEachRefusalOfTheAgreementWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-active", ""), &servicev1.RegisterAgreementResponse{})
	invoke(t, conn, call(t), "ActivateAgreement", activation("e2e-agr-active"), &servicev1.ActivateAgreementResponse{})
	draft := agreementOf("e2e-agr-draft", "e2e-agr-active")
	invoke(t, conn, call(t), "RegisterAgreement", draft, &servicev1.RegisterAgreementResponse{})

	register := func(req *servicev1.RegisterAgreementRequest) string {
		return invoke(t, conn, call(t), "RegisterAgreement", req, &servicev1.RegisterAgreementResponse{}).GetRejection().GetCode()
	}
	activate := func(req *servicev1.ActivateAgreementRequest) string {
		return invoke(t, conn, call(t), "ActivateAgreement", req, &servicev1.ActivateAgreementResponse{}).GetRejection().GetCode()
	}
	for _, c := range []struct {
		name string
		got  func() string
		code string
	}{
		{"valid until before valid from", func() string {
			req := agreementOf("e2e-agr-x", "e2e-agr-draft")
			req.ValidUntil = validFrom - 1
			return register(req)
		}, "partner-management/partner-agreement/validity-invalid"},
		{"agreement already active", func() string { return activate(activation("e2e-agr-active")) },
			"partner-management/partner-agreement/not-draft"},
		{"evidence missing", func() string {
			req := activation("e2e-agr-draft")
			req.ContractEvidenceId = ""
			return activate(req)
		}, "partner-management/partner-agreement/contract-evidence-missing"},
		{"without H17", func() string {
			req := activation("e2e-agr-draft")
			req.SatisfiedCriteria = []string{"H14", "H15", "H18"}
			return activate(req)
		}, "partner-management/partner-agreement/contractual-criteria-incomplete"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.got(); got != c.code {
				t.Fatalf("rejection = %q, want %q", got, c.code)
			}
			if _, a, e := rows(t, pool); a != 2 || e != 1 {
				t.Fatalf("partner_agreements = %d, outbox = %d after a refusal, want the two held agreements and one activation", a, e)
			}
			if got := count(t, pool, "SELECT count(*) FROM partner_agreements WHERE partner_agreement_id = 'e2e-agr-draft' AND status = 'draft'"); got != 1 {
				t.Fatalf("e2e-agr-draft left draft after a refusal")
			}
		})
	}
}

func TestACommandWithoutAKeyIsRefusedAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	err := conn.Invoke(callWithKey(t, ""), rpc.FullMethod("RegisterPartner"), registration("e2e-prt-1"), &servicev1.RegisterPartnerResponse{})

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
		t.Fatalf("RegisterPartner() without a key = %v, want InvalidArgument with %s", err, kernelgrpc.ReasonMissingIdempotencyKey)
	}
	if p, a, e := rows(t, pool); p != 0 || a != 0 || e != 0 {
		t.Fatalf("partners = %d, agreements = %d, outbox = %d, want nothing written", p, a, e)
	}
}
