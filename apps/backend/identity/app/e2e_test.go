//go:build integration

package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

const (
	tenantA          = "acme"
	tenantB          = "globex"
	e2eCNPJ          = "12ABC34501DE35"
	e2eCNPJFormatted = "12.abc.345/01de-35"
	e2ePlatformCNPJ  = "11222333000181"
)

type fixedClock struct{}

func (fixedClock) Now() port.Instant { return 1_755_432_000_000_000_000 }

// messages numbers across every service a test dials: the kernel outbox holds
// message_id unique (outbox.sql, outbox_message_id_key), and a sequence per
// service would hand a second one an id the first already wrote.
var messages atomic.Int64

type sequenceIDs struct{}

func (sequenceIDs) NewMessageID() port.MessageID {
	return port.MessageID(fmt.Sprintf("m-%06d", messages.Add(1)))
}

// barrier holds each Save until every party arrived, so the transactions are
// all open before any insert runs and the unique constraint decides the race.
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

func protector(t *testing.T) *provider.IdentifierProtector {
	t.Helper()
	p, err := provider.NewIdentifierProtector([]byte(testIdentifierKey), 1)
	if err != nil {
		t.Fatalf("NewIdentifierProtector() = %v", err)
	}
	return p
}

func dial(t *testing.T, pool *pgxpool.Pool, saves *barrier) *grpc.ClientConn {
	t.Helper()
	return dialAt(t, pool, saves, fixedClock{})
}

func dialAt(t *testing.T, pool *pgxpool.Pool, saves *barrier, clock port.Clock) *grpc.ClientConn {
	t.Helper()
	policy, err := app.IdempotencyPolicy(apiConfig(testIdentifierKey))
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := app.NewService(pool, pool, protector(t), clock, sequenceIDs{}, app.Waits{Command: time.Second})
	service.Idempotency = policy
	if saves != nil {
		service.UoW = postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) application.Resources {
			return application.Resources{
				Organizations:   held(provider.NewOrganizationRepository(tx), saves),
				Establishments:  held(provider.NewEstablishmentRepository(tx), saves),
				Persons:         held(provider.NewPersonRepository(tx), saves),
				Representations: provider.NewRepresentationRepository(tx),
				UserAccounts:    held(provider.NewUserAccountRepository(tx), saves),
				Memberships:     provider.NewMembershipRepository(tx),
				Outbox:          tx.Outbox(provider.Mapper{}),
				Commands:        tx.CommandInbox(application.CommandConsumer, time.Second),

				BankAccountChanges: provider.NewBankAccountChangeRepository(tx),
			}
		})
	}

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

func call(t *testing.T, tenant string) context.Context {
	t.Helper()
	return callWithKey(t, tenant, fmt.Sprintf("e2e-k-%d", keys.Add(1)))
}

func callWithKey(t *testing.T, tenant, key string) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if key != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.IdempotencyKey, key)
	}
	if tenant == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, tenant)
}

func registration(organizationID string) *servicev1.RegisterOrganizationRequest {
	return &servicev1.RegisterOrganizationRequest{
		OrganizationId: organizationID,
		Kinds:          []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT},
		Jurisdiction:   "BR",
		TaxIdType:      servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxId:          e2eCNPJ,
		LegalName:      "Recebiveis Sinteticos Ltda",
		TradeName:      "Sintetica",
	}
}

func registerOver(t *testing.T, conn *grpc.ClientConn, ctx context.Context, req *servicev1.RegisterOrganizationRequest) *servicev1.RegisterOrganizationResponse {
	t.Helper()
	var resp servicev1.RegisterOrganizationResponse
	if err := conn.Invoke(ctx, rpc.FullMethod("RegisterOrganization"), req, &resp); err != nil {
		t.Fatalf("RegisterOrganization(%s) = %v", req.GetOrganizationId(), err)
	}
	return &resp
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func outboxRows(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return count(t, pool, "SELECT count(*) FROM outbox")
}

func organizationRows(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	return count(t, pool, "SELECT count(*) FROM organizations")
}

type enqueued struct {
	messageType, destination, partitionKey string
	payload                                []byte
}

func outbox(t *testing.T, pool *pgxpool.Pool) []enqueued {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT message_type, destination, partition_key, payload FROM outbox ORDER BY id")
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()
	var out []enqueued
	for rows.Next() {
		var e enqueued
		if err := rows.Scan(&e.messageType, &e.destination, &e.partitionKey, &e.payload); err != nil {
			t.Fatalf("scan outbox: %v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	return out
}

func TestRegisterOrganizationEndToEndOverGRPC(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	registered := registerOver(t, conn, call(t, tenantA), registration("e2e-org-001"))

	if registered.GetRegistered().GetOrganizationId() != "e2e-org-001" {
		t.Fatalf("RegisterOrganization() = %v, want Registered e2e-org-001", registered)
	}
	held, err := protector(t).Protect(context.Background(), tenantA, domain.TaxIDTypeCNPJ, e2eCNPJ)
	if err != nil {
		t.Fatalf("Protect() = %v", err)
	}
	records := outbox(t, pool)
	if len(records) != 1 {
		t.Fatalf("outbox rows = %d, want 1", len(records))
	}
	record := records[0]
	if record.messageType != "com.company.identity.organization-registered.v1" || record.destination != application.Destination || record.partitionKey != "e2e-org-001" {
		t.Fatalf("outbox record = %s to %s keyed %s, want the organization-registered type to %s keyed by the organization",
			record.messageType, record.destination, record.partitionKey, application.Destination)
	}
	var event eventv1.OrganizationRegistered
	if err := proto.Unmarshal(record.payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetTaxIdHash() != held.Hash || event.GetTaxIdMasked() != "**.***.***/**DE-35" || event.GetOrganizationId() != "e2e-org-001" {
		t.Fatalf("event = %v, want the hash of the tenant and the mask", &event)
	}
	if bytes.Contains(record.payload, []byte(e2eCNPJ)) || bytes.Contains(record.payload, []byte(e2eCNPJFormatted)) {
		t.Fatalf("the event payload carries the tax id in clear: %x", record.payload)
	}

	var found servicev1.FindOrganizationResponse
	if err := conn.Invoke(call(t, tenantA), rpc.FullMethod("FindOrganization"), &servicev1.FindOrganizationRequest{OrganizationId: "e2e-org-001"}, &found); err != nil {
		t.Fatalf("FindOrganization() = %v", err)
	}
	if o := found.GetOrganization(); o.GetStatus() != servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE || o.GetTaxId() != e2eCNPJ || o.GetTaxIdHash() != held.Hash {
		t.Fatalf("FindOrganization() = %v, want ACTIVE with the tax id and its hash", o)
	}

	var byTaxID servicev1.FindOrganizationByTaxIdResponse
	if err := conn.Invoke(call(t, tenantA), rpc.FullMethod("FindOrganizationByTaxId"),
		&servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ, TaxId: e2eCNPJFormatted, Jurisdiction: "BR"}, &byTaxID); err != nil {
		t.Fatalf("FindOrganizationByTaxId() = %v", err)
	}
	if byTaxID.GetOrganization().GetOrganizationId() != "e2e-org-001" {
		t.Fatalf("FindOrganizationByTaxId() = %v, want e2e-org-001 found by the tax id as a person types it", byTaxID.GetOrganization())
	}
}

func TestEachRefusalOfRegisterOrganizationWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-held"))

	cases := []struct {
		name string
		edit func(*servicev1.RegisterOrganizationRequest)
		code string
	}{
		{"kinds empty", func(r *servicev1.RegisterOrganizationRequest) { r.Kinds = nil }, "identity-organization/organization/kinds-empty"},
		{"jurisdiction of three letters", func(r *servicev1.RegisterOrganizationRequest) { r.Jurisdiction = "BRA" }, "identity-organization/organization/jurisdiction-invalid"},
		{"tax id empty", func(r *servicev1.RegisterOrganizationRequest) { r.TaxId = "" }, "identity-organization/organization/tax-id-empty"},
		{"legal name blank", func(r *servicev1.RegisterOrganizationRequest) { r.LegalName = " " }, "identity-organization/organization/legal-name-empty"},
		{"tax id held in the jurisdiction of the tenant", func(r *servicev1.RegisterOrganizationRequest) { r.TaxId = e2eCNPJFormatted },
			"identity-organization/organization/duplicate-tax-id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := registration("e2e-org-refused")
			c.edit(req)

			resp := registerOver(t, conn, call(t, tenantA), req)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q (response %v)", got, c.code, resp)
			}
			if o, e := organizationRows(t, pool), outboxRows(t, pool); o != 1 || e != 1 {
				t.Fatalf("organizations = %d, outbox = %d after a refusal, want only the held organization and its event", o, e)
			}
		})
	}
}

func TestARepeatedRegisterOverGRPCReplaysAndAnotherKeyAlreadyExists(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	req := registration("e2e-org-004")

	var first, again servicev1.RegisterOrganizationResponse
	var header metadata.MD
	if err := conn.Invoke(callWithKey(t, tenantA, "e2e-replay"), rpc.FullMethod("RegisterOrganization"), req, &first); err != nil {
		t.Fatalf("RegisterOrganization() = %v", err)
	}
	if err := conn.Invoke(callWithKey(t, tenantA, "e2e-replay"), rpc.FullMethod("RegisterOrganization"), req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated RegisterOrganization() = %v, want the stored response", err)
	}
	if again.GetRegistered().GetOrganizationId() != "e2e-org-004" || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want Registered e2e-org-004 marked as replay", &again, header)
	}

	var other servicev1.RegisterOrganizationResponse
	err := conn.Invoke(call(t, tenantA), rpc.FullMethod("RegisterOrganization"), req, &other)
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("RegisterOrganization(another key) = %v, want AlreadyExists", err)
	}
	if o, e := organizationRows(t, pool), outboxRows(t, pool); o != 1 || e != 1 {
		t.Fatalf("organizations = %d, outbox = %d, want one effect and one event per organization", o, e)
	}
}

func TestFindOrganizationByTaxIDFromAnotherTenantIsNotFound(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-005"))

	var resp servicev1.FindOrganizationByTaxIdResponse
	err := conn.Invoke(call(t, tenantB), rpc.FullMethod("FindOrganizationByTaxId"),
		&servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ, TaxId: e2eCNPJ, Jurisdiction: "BR"}, &resp)

	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindOrganizationByTaxId() from %s = %v (%v), want NotFound", tenantB, err, resp.GetOrganization())
	}
}

type attempt struct {
	resp *servicev1.RegisterOrganizationResponse
	err  error
}

func registerAtOnce(t *testing.T, conn *grpc.ClientConn, calls map[string]*servicev1.RegisterOrganizationRequest, tenants map[string]string) map[string]attempt {
	t.Helper()
	var mu sync.Mutex
	var group sync.WaitGroup
	out := map[string]attempt{}
	for name, req := range calls {
		ctx := call(t, tenants[name])
		group.Go(func() {
			var resp servicev1.RegisterOrganizationResponse
			err := conn.Invoke(ctx, rpc.FullMethod("RegisterOrganization"), req, &resp)
			mu.Lock()
			defer mu.Unlock()
			out[name] = attempt{resp: &resp, err: err}
		})
	}
	group.Wait()
	return out
}

func TestTwoConcurrentRegistrationsOfOneTaxIDInOneTenantLeaveOneOrganization(t *testing.T) {
	pool := appkit.OpenPool(t)
	saves := newBarrier(2)
	conn := dial(t, pool, saves)

	got := registerAtOnce(t, conn,
		map[string]*servicev1.RegisterOrganizationRequest{"first": registration("e2e-org-a1"), "second": registration("e2e-org-a2")},
		map[string]string{"first": tenantA, "second": tenantA})

	accepted, rejected := 0, 0
	for name, a := range got {
		switch {
		case a.err == nil && a.resp.GetRegistered() != nil:
			accepted++
		case a.err == nil && a.resp.GetRejection().GetCode() == string(domain.CodeOrganizationDuplicateTaxID):
			rejected++
		default:
			t.Errorf("%s = %v (%v), want either Registered or the duplicate-tax-id rejection of the loser", name, a.err, a.resp)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("accepted %d and rejected %d, want exactly one of each", accepted, rejected)
	}
	refusedByUniqueness(t, saves, "organizations_tenant_id_tax_id_hash_jurisdiction_key")
	if o, e := organizationRows(t, pool), outboxRows(t, pool); o != 1 || e != 1 {
		t.Fatalf("organizations = %d, outbox = %d, want exactly one row and one OrganizationRegistered", o, e)
	}
}

func TestTheSameTaxIDInTwoTenantsAtOnceIsRegisteredInEach(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, newBarrier(2))

	got := registerAtOnce(t, conn,
		map[string]*servicev1.RegisterOrganizationRequest{tenantA: registration("e2e-org-b1"), tenantB: registration("e2e-org-b2")},
		map[string]string{tenantA: tenantA, tenantB: tenantB})

	for tenant, a := range got {
		if a.err != nil || a.resp.GetRegistered() == nil {
			t.Fatalf("RegisterOrganization() in %s = %v (%v), want Registered: uniqueness is per tenant", tenant, a.err, a.resp)
		}
		rows := count(t, pool, "SELECT count(*) FROM organizations WHERE tenant_id = $1", tenant)
		events := count(t, pool, "SELECT count(*) FROM outbox WHERE metadata->>'tenantid' = $1", tenant)
		if rows != 1 || events != 1 {
			t.Fatalf("tenant %s holds %d organizations and %d events, want one of each", tenant, rows, events)
		}
	}
}

func TestACommandWithoutAKeyIsRefusedAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	var resp servicev1.RegisterOrganizationResponse
	err := conn.Invoke(callWithKey(t, tenantA, ""), rpc.FullMethod("RegisterOrganization"), registration("e2e-org-006"), &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
		t.Fatalf("RegisterOrganization() without a key = %v, want InvalidArgument with %s", err, kernelgrpc.ReasonMissingIdempotencyKey)
	}
	if o, e := organizationRows(t, pool), outboxRows(t, pool); o != 0 || e != 0 {
		t.Fatalf("organizations = %d, outbox = %d, want nothing written", o, e)
	}
}

func TestACallWithoutATenantIsRefusedAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	var resp servicev1.RegisterOrganizationResponse
	err := conn.Invoke(call(t, ""), rpc.FullMethod("RegisterOrganization"), registration("e2e-org-007"), &resp)

	if err == nil {
		t.Fatalf("RegisterOrganization() without a tenant = %v, want the call refused", &resp)
	}
	if o, e := organizationRows(t, pool), outboxRows(t, pool); o != 0 || e != 0 {
		t.Fatalf("organizations = %d, outbox = %d, want nothing written", o, e)
	}
}

func TestANULInFreeTextIsRefusedOverGRPCAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	organization := registration("e2e-org-nul")
	organization.LegalName = "Recebiveis\x00Sinteticos"
	person := personOver("e2e-per-nul", e2eCPF)
	person.FullName = "Ana\x00Sintetica"

	calls := []struct {
		method    string
		req, resp proto.Message
	}{
		{"RegisterOrganization", organization, &servicev1.RegisterOrganizationResponse{}},
		{"RegisterPerson", person, &servicev1.RegisterPersonResponse{}},
	}
	for _, c := range calls {
		err := conn.Invoke(call(t, tenantA), rpc.FullMethod(c.method), c.req, c.resp)

		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("%s() = %v, want InvalidArgument before the jsonb of the snapshot refuses U+0000", c.method, err)
		}
	}
	for _, table := range []string{"organizations", "persons", "outbox", "inbox"} {
		if n := rows(t, pool, table); n != 0 {
			t.Fatalf("%s holds %d rows, want nothing written", table, n)
		}
	}
}

const (
	e2eCPF       = "123.456.789-09"
	e2eCPFDigits = "12345678909"
	e2eFrom      = int64(1_755_432_000_000_000_000)
	e2eYear      = int64(365 * 24 * 3600 * 1_000_000_000)
)

func invokeOver(t *testing.T, conn *grpc.ClientConn, ctx context.Context, method string, req, resp proto.Message) {
	t.Helper()
	if err := conn.Invoke(ctx, rpc.FullMethod(method), req, resp); err != nil {
		t.Fatalf("%s() = %v", method, err)
	}
}

func rows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	return count(t, pool, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize())
}

func establishmentOver(establishmentID, organizationID string) *servicev1.AddEstablishmentRequest {
	return &servicev1.AddEstablishmentRequest{
		EstablishmentId: establishmentID,
		OrganizationId:  organizationID,
		TaxIdType:       servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxId:           e2eCNPJ,
		TradeName:       "Loja Sintetica",
		Receiver:        true,
		ExternalReferences: []*servicev1.ExternalReference{{
			ProviderId:      "partner-registry",
			System:          servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY,
			ReferenceType:   servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_ASSET,
			Reference:       "AST-0001/a",
			AsOf:            e2eFrom - 1,
			ObservedVersion: "v3",
		}},
	}
}

func personOver(personID, document string) *servicev1.RegisterPersonRequest {
	return &servicev1.RegisterPersonRequest{PersonId: personID, FullName: "Ana Sintetica", Document: document}
}

func representationOver(representationID string, from, until int64) *servicev1.RegisterRepresentationRequest {
	return &servicev1.RegisterRepresentationRequest{
		RepresentationId:    representationID,
		OrganizationId:      "e2e-org-r",
		PersonId:            "e2e-per-r",
		PowerType:           servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT,
		SourceType:          servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY,
		SourceEvidenceId:    "evd-0001",
		ScopeCurrency:       "BRL",
		ScopeMaxAmountMinor: 10_000_000,
		ScopeActTypes:       []string{"assignment", "consent"},
		ValidFrom:           from,
		ValidUntil:          until,
		ProposedByUserId:    "usr-1",
	}
}

func withRepresentationParties(t *testing.T, conn *grpc.ClientConn) {
	t.Helper()
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-r"))
	invokeOver(t, conn, call(t, tenantA), "RegisterPerson", personOver("e2e-per-r", e2eCPF), &servicev1.RegisterPersonResponse{})
}

func tablesHolding(t *testing.T, pool *pgxpool.Pool, values ...string) []string {
	t.Helper()
	var holding []string
	for _, table := range pg.Tables(t, pool) {
		if tableHolds(t, pool, table, values) {
			holding = append(holding, table)
		}
	}
	return holding
}

func tableHolds(t *testing.T, pool *pgxpool.Pool, table string, values []string) bool {
	t.Helper()
	found, err := pool.Query(context.Background(), "SELECT * FROM "+pgx.Identifier{table}.Sanitize())
	if err != nil {
		t.Fatalf("read %s: %v", table, err)
	}
	defer found.Close()
	for found.Next() {
		columns, err := found.Values()
		if err != nil {
			t.Fatalf("scan %s: %v", table, err)
		}
		for _, column := range columns {
			raw := rendered(t, column)
			for _, value := range values {
				if bytes.Contains(raw, []byte(value)) {
					return true
				}
			}
		}
	}
	if err := found.Err(); err != nil {
		t.Fatalf("read %s: %v", table, err)
	}
	return false
}

func rendered(t *testing.T, column any) []byte {
	t.Helper()
	switch v := column.(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("render %T: %v", v, err)
		}
		return raw
	}
}

func TestAddEstablishmentEndToEndOverGRPC(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-e1"))

	var added servicev1.AddEstablishmentResponse
	invokeOver(t, conn, call(t, tenantA), "AddEstablishment", establishmentOver("e2e-est-001", "e2e-org-e1"), &added)

	if added.GetAdded().GetEstablishmentId() != "e2e-est-001" {
		t.Fatalf("AddEstablishment() = %v, want Added e2e-est-001", &added)
	}
	protected, err := protector(t).Protect(context.Background(), tenantA, domain.TaxIDTypeCNPJ, e2eCNPJ)
	if err != nil {
		t.Fatalf("Protect() = %v", err)
	}
	records := outbox(t, pool)
	if len(records) != 2 {
		t.Fatalf("outbox rows = %d, want the organization's and the establishment's", len(records))
	}
	record := records[1]
	if record.messageType != "com.company.identity.establishment-added.v1" || record.destination != application.Destination || record.partitionKey != "e2e-est-001" {
		t.Fatalf("outbox record = %s to %s keyed %s, want the establishment-added type to %s keyed by the establishment",
			record.messageType, record.destination, record.partitionKey, application.Destination)
	}
	var event eventv1.EstablishmentAdded
	if err := proto.Unmarshal(record.payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetEstablishmentId() != "e2e-est-001" || event.GetOrganizationId() != "e2e-org-e1" || event.GetTaxIdHash() != protected.Hash ||
		event.GetTaxIdType() != eventv1.TaxIdType_TAX_ID_TYPE_CNPJ || !event.GetReceiver() || event.GetAddedAt().AsTime().UnixNano() != e2eFrom {
		t.Fatalf("event = %v, want the establishment, its organization, the hash of the tenant, a receiver, added at the clock", &event)
	}
	for _, clear := range []string{e2eCNPJ, e2eCNPJFormatted, "**.***.***/**DE-35"} {
		if bytes.Contains(record.payload, []byte(clear)) {
			t.Fatalf("the event payload carries %q: only the hash of the tax id crosses the context", clear)
		}
	}

	var found servicev1.FindEstablishmentsByOrganizationResponse
	invokeOver(t, conn, call(t, tenantA), "FindEstablishmentsByOrganization", &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "e2e-org-e1"}, &found)
	want := &servicev1.Establishment{
		EstablishmentId: "e2e-est-001", OrganizationId: "e2e-org-e1", TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ, TaxId: e2eCNPJ,
		TaxIdHash: protected.Hash, TaxIdHashKeyVersion: 1, TaxIdMasked: "**.***.***/**DE-35", TradeName: "Loja Sintetica", Receiver: true,
		ExternalReferences: establishmentOver("", "").GetExternalReferences(), CreatedAt: e2eFrom,
	}
	if len(found.GetEstablishments()) != 1 || !proto.Equal(found.GetEstablishments()[0], want) {
		t.Fatalf("FindEstablishmentsByOrganization() = %v, want [%v]", found.GetEstablishments(), want)
	}

	var elsewhere servicev1.FindEstablishmentsByOrganizationResponse
	invokeOver(t, conn, call(t, tenantB), "FindEstablishmentsByOrganization", &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "e2e-org-e1"}, &elsewhere)
	if len(elsewhere.GetEstablishments()) != 0 {
		t.Fatalf("FindEstablishmentsByOrganization() from %s = %v, want nothing of another tenant", tenantB, elsewhere.GetEstablishments())
	}
}

func TestEachRefusalOfAddEstablishmentWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-e2"))
	invokeOver(t, conn, call(t, tenantA), "AddEstablishment", establishmentOver("e2e-est-held", "e2e-org-e2"), &servicev1.AddEstablishmentResponse{})

	cases := []struct {
		name string
		edit func(*servicev1.AddEstablishmentRequest)
		code string
	}{
		{"organization not found", func(r *servicev1.AddEstablishmentRequest) { r.OrganizationId = "e2e-org-none" },
			"identity-organization/establishment/organization-not-active"},
		{"organization of another tenant", func(r *servicev1.AddEstablishmentRequest) { r.OrganizationId = "e2e-org-b-only" },
			"identity-organization/establishment/organization-not-active"},
		{"tax id empty", func(r *servicev1.AddEstablishmentRequest) { r.TaxId = "" }, "identity-organization/establishment/tax-id-empty"},
		{"tax id held in the organization", func(r *servicev1.AddEstablishmentRequest) { r.TaxId = e2eCNPJFormatted },
			"identity-organization/establishment/duplicate-tax-id"},
	}
	registerOver(t, conn, call(t, tenantB), registration("e2e-org-b-only"))
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := establishmentOver("e2e-est-refused", "e2e-org-e2")
			c.edit(req)

			var resp servicev1.AddEstablishmentResponse
			invokeOver(t, conn, call(t, tenantA), "AddEstablishment", req, &resp)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q (response %v)", got, c.code, &resp)
			}
			if e, o := rows(t, pool, "establishments"), outboxRows(t, pool); e != 1 || o != 3 {
				t.Fatalf("establishments = %d, outbox = %d after a refusal, want only the held establishment and the three events before it", e, o)
			}
		})
	}
}

func TestRegisterPersonEndToEndKeepsOnlyTheHashAndTheMaskOfTheDocument(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	var registered servicev1.RegisterPersonResponse
	invokeOver(t, conn, call(t, tenantA), "RegisterPerson", personOver("e2e-per-001", e2eCPF), &registered)

	if registered.GetRegistered().GetPersonId() != "e2e-per-001" {
		t.Fatalf("RegisterPerson() = %v, want Registered e2e-per-001", &registered)
	}
	if n := outboxRows(t, pool); n != 0 {
		t.Fatalf("outbox rows = %d, want none: RegisterPerson emits no event", n)
	}
	var snapshot map[string]any
	if err := pool.QueryRow(context.Background(), "SELECT snapshot FROM persons WHERE person_id = $1", "e2e-per-001").Scan(&snapshot); err != nil {
		t.Fatalf("read the person: %v", err)
	}
	for key := range snapshot {
		if strings.EqualFold(key, "document") {
			t.Fatalf("snapshot %v holds the document, want only its hash and mask (spec note ‡‡)", snapshot)
		}
	}
	if holding := tablesHolding(t, pool, e2eCPF, e2eCPFDigits); len(holding) != 0 {
		t.Fatalf("tables %v hold the document in clear, want none", holding)
	}

	protected, err := protector(t).Protect(context.Background(), tenantA, domain.TaxIDTypePersonDocument, e2eCPF)
	if err != nil {
		t.Fatalf("Protect() = %v", err)
	}
	var found servicev1.FindPersonResponse
	invokeOver(t, conn, call(t, tenantA), "FindPerson", &servicev1.FindPersonRequest{PersonId: "e2e-per-001"}, &found)
	want := &servicev1.Person{
		PersonId: "e2e-per-001", FullName: "Ana Sintetica", DocumentHash: protected.Hash, DocumentHashKeyVersion: 1,
		DocumentMasked: "***.***.*89-09", CreatedAt: e2eFrom,
	}
	if !proto.Equal(found.GetPerson(), want) {
		t.Fatalf("FindPerson() = %v, want %v", found.GetPerson(), want)
	}
}

func TestEachRefusalOfRegisterPersonWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invokeOver(t, conn, call(t, tenantA), "RegisterPerson", personOver("e2e-per-held", e2eCPF), &servicev1.RegisterPersonResponse{})

	cases := []struct {
		name string
		edit func(*servicev1.RegisterPersonRequest)
		code string
	}{
		{"full name blank", func(r *servicev1.RegisterPersonRequest) { r.FullName = " " }, "identity-organization/person/full-name-empty"},
		{"document empty", func(r *servicev1.RegisterPersonRequest) { r.Document = "" }, "identity-organization/person/document-empty"},
		{"document held in the tenant, typed as digits", func(r *servicev1.RegisterPersonRequest) { r.Document = e2eCPFDigits },
			"identity-organization/person/duplicate-document"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := personOver("e2e-per-refused", "987.654.321-00")
			c.edit(req)

			var resp servicev1.RegisterPersonResponse
			invokeOver(t, conn, call(t, tenantA), "RegisterPerson", req, &resp)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q (response %v)", got, c.code, &resp)
			}
			if p, o := rows(t, pool, "persons"), outboxRows(t, pool); p != 1 || o != 0 {
				t.Fatalf("persons = %d, outbox = %d after a refusal, want only the held person and no event", p, o)
			}
		})
	}
}

func TestRegisterRepresentationEndToEndAndItsValidity(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	withRepresentationParties(t, conn)

	var first servicev1.RegisterRepresentationResponse
	invokeOver(t, conn, call(t, tenantA), "RegisterRepresentation", representationOver("e2e-rep-001", e2eFrom, e2eFrom+e2eYear), &first)
	later := representationOver("e2e-rep-002", e2eFrom+2*e2eYear, 0)
	later.PowerType, later.SourceType = servicev1.PowerType_POWER_TYPE_LEGAL_REPRESENTATIVE, servicev1.SourceType_SOURCE_TYPE_ARTICLES_OF_ASSOCIATION
	later.ScopeCurrency, later.ScopeMaxAmountMinor, later.ScopeActTypes = "", 0, nil
	invokeOver(t, conn, call(t, tenantA), "RegisterRepresentation", later, &servicev1.RegisterRepresentationResponse{})

	if first.GetRegistered().GetRepresentationId() != "e2e-rep-001" {
		t.Fatalf("RegisterRepresentation() = %v, want Registered e2e-rep-001", &first)
	}
	if r, o := rows(t, pool, "representations"), outboxRows(t, pool); r != 2 || o != 1 {
		t.Fatalf("representations = %d, outbox = %d, want two rows and only the organization's event", r, o)
	}
	var found servicev1.FindRepresentationResponse
	invokeOver(t, conn, call(t, tenantA), "FindRepresentation", &servicev1.FindRepresentationRequest{RepresentationId: "e2e-rep-001"}, &found)
	want := &servicev1.Representation{
		RepresentationId: "e2e-rep-001", OrganizationId: "e2e-org-r", PersonId: "e2e-per-r",
		PowerType: servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT, SourceType: servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY,
		SourceEvidenceId: "evd-0001", ScopeCurrency: "BRL", ScopeMaxAmountMinor: 10_000_000, ScopeActTypes: []string{"assignment", "consent"},
		ValidFrom: e2eFrom, ValidUntil: e2eFrom + e2eYear,
		VerificationStatus: servicev1.RepresentationVerificationStatus_REPRESENTATION_VERIFICATION_STATUS_DECLARED, ProposedByUserId: "usr-1",
	}
	if !proto.Equal(found.GetRepresentation(), want) {
		t.Fatalf("FindRepresentation() = %v, want %v", found.GetRepresentation(), want)
	}

	for _, c := range []struct {
		name    string
		validAt int64
		want    []string
	}{
		{"inside the first validity", e2eFrom + e2eYear/2, []string{"e2e-rep-001"}},
		{"at the end of the first, which is exclusive", e2eFrom + e2eYear, []string{}},
		{"before any validity", e2eFrom - 1, []string{}},
		{"inside the indefinite one", e2eFrom + 3*e2eYear, []string{"e2e-rep-002"}},
		{"with no instant, every one", 0, []string{"e2e-rep-001", "e2e-rep-002"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var listed servicev1.FindRepresentationsByOrganizationResponse
			invokeOver(t, conn, call(t, tenantA), "FindRepresentationsByOrganization",
				&servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: "e2e-org-r", ValidAt: c.validAt}, &listed)
			got := []string{}
			for _, r := range listed.GetRepresentations() {
				got = append(got, r.GetRepresentationId())
			}
			slices.Sort(got)
			if !slices.Equal(got, c.want) {
				t.Fatalf("FindRepresentationsByOrganization(valid at %d) = %v, want %v", c.validAt, got, c.want)
			}
		})
	}
}

func TestEachRefusalOfRegisterRepresentationWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	withRepresentationParties(t, conn)

	cases := []struct {
		name string
		edit func(*servicev1.RegisterRepresentationRequest)
		code string
	}{
		{"evidence missing", func(r *servicev1.RegisterRepresentationRequest) { r.SourceEvidenceId = "" },
			"identity-organization/representation/source-evidence-missing"},
		{"valid until before valid from", func(r *servicev1.RegisterRepresentationRequest) { r.ValidUntil = r.ValidFrom - 1 },
			"identity-organization/representation/validity-invalid"},
		{"amount without currency", func(r *servicev1.RegisterRepresentationRequest) { r.ScopeCurrency = "" },
			"identity-organization/representation/scope-invalid"},
		{"organization not found", func(r *servicev1.RegisterRepresentationRequest) { r.OrganizationId = "e2e-org-none" },
			"identity-organization/representation/organization-not-active"},
		{"person not found", func(r *servicev1.RegisterRepresentationRequest) { r.PersonId = "e2e-per-none" },
			"identity-organization/representation/person-not-found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := representationOver("e2e-rep-refused", e2eFrom, e2eFrom+e2eYear)
			c.edit(req)

			var resp servicev1.RegisterRepresentationResponse
			invokeOver(t, conn, call(t, tenantA), "RegisterRepresentation", req, &resp)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q (response %v)", got, c.code, &resp)
			}
			if r := rows(t, pool, "representations"); r != 0 {
				t.Fatalf("representations = %d after a refusal, want none", r)
			}
		})
	}
}

// The inbox keeps the digest of the fingerprint, which holds the document in
// clear: it must be the HMAC under the identifier key, never the bare SHA-256.
func TestRegisterPersonStoresTheKeyedDigestAndReplaysUnderTheSameKey(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	req := personOver("e2e-per-002", e2eCPF)

	var first, again servicev1.RegisterPersonResponse
	var header metadata.MD
	invokeOver(t, conn, callWithKey(t, tenantA, "e2e-person-replay"), "RegisterPerson", req, &first)
	if err := conn.Invoke(callWithKey(t, tenantA, "e2e-person-replay"), rpc.FullMethod("RegisterPerson"), req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated RegisterPerson() = %v, want the stored response", err)
	}

	if again.GetRegistered().GetPersonId() != "e2e-per-002" || !proto.Equal(&again, &first) || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want %v marked as replay", &again, header, &first)
	}
	if p := rows(t, pool, "persons"); p != 1 {
		t.Fatalf("persons = %d, want one effect per key", p)
	}
	var stored string
	if err := pool.QueryRow(context.Background(), "SELECT payload_hash FROM inbox WHERE consumer_name = $1 AND message_id = $2",
		application.CommandConsumer, tenantA+"/e2e-person-replay").Scan(&stored); err != nil {
		t.Fatalf("read the inbox entry: %v", err)
	}
	canonical := append(usecase.NewFingerprint(application.OperationRegisterPerson).Canonical(),
		usecase.NewFingerprint(application.OperationRegisterPerson).String("e2e-per-002").String("Ana Sintetica").String(e2eCPF).Canonical()...)
	unkeyed := sha256.Sum256(canonical)
	if stored == hex.EncodeToString(unkeyed[:]) {
		t.Fatalf("payload_hash = %s, the bare SHA-256 a brute force over the document recomputes", stored)
	}
	policy, err := app.IdempotencyPolicy(apiConfig(testIdentifierKey))
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	if keyed := policy.Digest(canonical); stored != hex.EncodeToString(keyed[:]) {
		t.Fatalf("payload_hash = %s, want the HMAC of the canonical under the identifier key %x", stored, keyed)
	}
}

func TestARepeatedCommandOfTheRelationsReplaysWithoutASecondEffect(t *testing.T) {
	for _, c := range []struct {
		method string
		req    proto.Message
		resp   func() proto.Message
		table  string
	}{
		{"AddEstablishment", establishmentOver("e2e-est-replay", "e2e-org-r"), func() proto.Message { return &servicev1.AddEstablishmentResponse{} }, "establishments"},
		{"RegisterRepresentation", representationOver("e2e-rep-replay", e2eFrom, 0), func() proto.Message { return &servicev1.RegisterRepresentationResponse{} }, "representations"},
	} {
		t.Run(c.method, func(t *testing.T) {
			pool := appkit.OpenPool(t)
			conn := dial(t, pool, nil)
			withRepresentationParties(t, conn)
			first, again := c.resp(), c.resp()
			var header metadata.MD

			invokeOver(t, conn, callWithKey(t, tenantA, "e2e-replay-"+c.method), c.method, c.req, first)
			before := outboxRows(t, pool)
			if err := conn.Invoke(callWithKey(t, tenantA, "e2e-replay-"+c.method), rpc.FullMethod(c.method), c.req, again, grpc.Header(&header)); err != nil {
				t.Fatalf("repeated %s() = %v, want the stored response", c.method, err)
			}

			if !proto.Equal(again, first) || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
				t.Fatalf("replay = %v with header %v, want %v marked as replay", again, header, first)
			}
			if n, o := rows(t, pool, c.table), outboxRows(t, pool); n != 1 || o != before {
				t.Fatalf("%s = %d, outbox %d after %d, want one effect and no second event", c.table, n, o, before)
			}
		})
	}
}

type racer struct {
	tenant string
	req    proto.Message
	resp   proto.Message
}

func invokeAtOnce(t *testing.T, conn *grpc.ClientConn, method string, calls map[string]racer) map[string]error {
	t.Helper()
	var mu sync.Mutex
	var group sync.WaitGroup
	out := map[string]error{}
	for name, c := range calls {
		ctx := call(t, c.tenant)
		group.Go(func() {
			err := conn.Invoke(ctx, rpc.FullMethod(method), c.req, c.resp)
			mu.Lock()
			defer mu.Unlock()
			out[name] = err
		})
	}
	group.Wait()
	return out
}

// oneAcceptedOneRejected holds the race to its single legal ending: the unique
// constraint decides, and the loser answers the rejection it declares (ADR-066).
func oneAcceptedOneRejected(t *testing.T, calls map[string]racer, errs map[string]error, accepted func(proto.Message) bool, code string) {
	t.Helper()
	won, lost := 0, 0
	for name, err := range errs {
		resp := calls[name].resp
		switch {
		case err == nil && accepted(resp):
			won++
		case err == nil && resp.(refusable).GetRejection().GetCode() == code:
			lost++
		default:
			t.Errorf("%s = %v (%v), want either the acceptance or the rejection %s", name, err, resp, code)
		}
	}
	if won != 1 || lost != 1 {
		t.Fatalf("accepted %d and rejected %d, want exactly one of each", won, lost)
	}
}

// refusedByUniqueness pins the loser to the unique constraint the race is
// about, so a refusal from any other write cannot pass for it.
func refusedByUniqueness(t *testing.T, saves *barrier, constraint string) {
	t.Helper()
	refused := saves.refusals()
	var violation port.UniqueViolation
	if len(refused) != 1 || !errors.As(refused[0], &violation) || violation.Constraint != constraint {
		t.Fatalf("refused Saves = %v, want exactly one, the loser's, at %s", refused, constraint)
	}
}

func TestTwoConcurrentAdditionsOfOneTaxIDToOneOrganizationLeaveOneEstablishment(t *testing.T) {
	pool := appkit.OpenPool(t)
	registerOver(t, dial(t, pool, nil), call(t, tenantA), registration("e2e-org-c1"))
	saves := newBarrier(2)
	conn := dial(t, pool, saves)
	calls := map[string]racer{
		"first":  {tenantA, establishmentOver("e2e-est-a1", "e2e-org-c1"), &servicev1.AddEstablishmentResponse{}},
		"second": {tenantA, establishmentOver("e2e-est-a2", "e2e-org-c1"), &servicev1.AddEstablishmentResponse{}},
	}

	errs := invokeAtOnce(t, conn, "AddEstablishment", calls)

	added := func(m proto.Message) bool { return m.(*servicev1.AddEstablishmentResponse).GetAdded() != nil }
	oneAcceptedOneRejected(t, calls, errs, added, string(domain.CodeEstablishmentDuplicateTaxID))
	refusedByUniqueness(t, saves, "establishments_tenant_id_organization_id_tax_id_hash_key")
	var winner string
	for _, c := range calls {
		if added(c.resp) {
			winner = c.req.(*servicev1.AddEstablishmentRequest).GetEstablishmentId()
		}
	}
	kept := count(t, pool, "SELECT count(*) FROM establishments WHERE establishment_id = $1", winner)
	events := count(t, pool, "SELECT count(*) FROM outbox WHERE message_type = $1", "com.company.identity.establishment-added.v1")
	announced := count(t, pool, "SELECT count(*) FROM outbox WHERE message_type = $1 AND aggregate_id = $2", "com.company.identity.establishment-added.v1", winner)
	if e := rows(t, pool, "establishments"); e != 1 || kept != 1 || events != 1 || announced != 1 {
		t.Fatalf("establishments = %d (%d of %s), EstablishmentAdded = %d (%d of %s), want exactly one row and one event, both of the accepted one", e, kept, winner, events, announced, winner)
	}
}

func TestTwoConcurrentRegistrationsOfOneDocumentInOneTenantLeaveOnePerson(t *testing.T) {
	pool := appkit.OpenPool(t)
	saves := newBarrier(2)
	conn := dial(t, pool, saves)
	calls := map[string]racer{
		"first":  {tenantA, personOver("e2e-per-a1", e2eCPF), &servicev1.RegisterPersonResponse{}},
		"second": {tenantA, personOver("e2e-per-a2", e2eCPFDigits), &servicev1.RegisterPersonResponse{}},
	}

	errs := invokeAtOnce(t, conn, "RegisterPerson", calls)

	oneAcceptedOneRejected(t, calls, errs, func(m proto.Message) bool { return m.(*servicev1.RegisterPersonResponse).GetRegistered() != nil }, string(domain.CodePersonDuplicateDocument))
	refusedByUniqueness(t, saves, "persons_tenant_id_document_hash_key")
	if p, o := rows(t, pool, "persons"), outboxRows(t, pool); p != 1 || o != 0 {
		t.Fatalf("persons = %d, outbox = %d, want exactly one row and no event", p, o)
	}
}

func TestTheSameDocumentInTwoTenantsAtOnceIsRegisteredInEach(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, newBarrier(2))
	calls := map[string]racer{
		tenantA: {tenantA, personOver("e2e-per-b1", e2eCPF), &servicev1.RegisterPersonResponse{}},
		tenantB: {tenantB, personOver("e2e-per-b2", e2eCPF), &servicev1.RegisterPersonResponse{}},
	}

	errs := invokeAtOnce(t, conn, "RegisterPerson", calls)

	for tenant, err := range errs {
		if err != nil || calls[tenant].resp.(*servicev1.RegisterPersonResponse).GetRegistered() == nil {
			t.Fatalf("RegisterPerson() in %s = %v (%v), want Registered: uniqueness is per tenant", tenant, err, calls[tenant].resp)
		}
		if n := count(t, pool, "SELECT count(*) FROM persons WHERE tenant_id = $1", tenant); n != 1 {
			t.Fatalf("tenant %s holds %d persons, want one", tenant, n)
		}
	}
	if o := outboxRows(t, pool); o != 0 {
		t.Fatalf("outbox = %d, want no event", o)
	}
}

const (
	e2eWeek  = int64(7 * 24 * 3600 * 1_000_000_000)
	e2eEmail = "@example.test"
)

type clockAt port.Instant

func (c clockAt) Now() port.Instant { return port.Instant(c) }

var (
	operatorPermissions = []string{
		"eligibility:request-eligibility-evaluation",
		"identity:find-bank-account-change",
		"identity:find-bank-account-changes-by-organization",
		"identity:request-bank-account-change",
		"kyb:find-business-profile",
		"kyb:find-disclosures-by-case",
		"kyb:find-ownership-structure",
		"kyb:find-verification-case",
		"kyb:find-verification-cases-by-organization",
		"marketplace:open-funding-request",
		"portfolio:find-availability-by-snapshot",
		"portfolio:find-latest-published-snapshot",
		"portfolio:find-portfolio",
		"portfolio:find-snapshots-by-portfolio",
		"portfolio:group-assets-by-snapshot",
		"portfolio:list-assets-by-snapshot",
		"portfolio:request-discovery",
	}
	auditorPermissions = []string{
		"kyb:find-business-profile",
		"kyb:find-disclosures-by-case",
		"kyb:find-ownership-structure",
		"kyb:find-verification-case",
		"kyb:find-verification-cases-by-organization",
		"ledger:access-evidence-bundle",
		"ledger:find-evidence-bundle",
		"ledger:request-evidence-bundle",
		"portfolio:find-availability-by-snapshot",
		"portfolio:find-latest-published-snapshot",
		"portfolio:find-portfolio",
		"portfolio:find-snapshots-by-portfolio",
		"portfolio:group-assets-by-snapshot",
		"portfolio:list-assets-by-snapshot",
	}
	platformOpsPermissions = []string{
		"partner:activate-agreement",
		"partner:change-partner-stage",
		"partner:change-rollout",
		"partner:declare-capability",
		"partner:define-operational-policy",
		"partner:define-product",
		"partner:enable-capability",
		"partner:find-agreements-by-partner",
		"partner:find-capabilities-by-partner",
		"partner:find-certificates-expiring",
		"partner:find-effective-capability",
		"partner:find-eligible-partners",
		"partner:find-integration-profile",
		"partner:find-operational-policy",
		"partner:find-partner",
		"partner:find-partners-by-organization",
		"partner:find-products-by-partner",
		"partner:raise-certificate-expiry-alert",
		"partner:record-homologation",
		"partner:register-agreement",
		"partner:register-integration-profile",
		"partner:register-partner",
		"partner:retire-product",
		"partner:suspend-capability",
	}
)

func provisionOver(subject string) *servicev1.ProvisionUserAccountRequest {
	return &servicev1.ProvisionUserAccountRequest{Subject: subject, PersonId: "e2e-per-m"}
}

func inviteOver(membershipID, organizationID string, role servicev1.MembershipRole) *servicev1.InviteUserRequest {
	return &servicev1.InviteUserRequest{
		MembershipId:        membershipID,
		OrganizationId:      organizationID,
		InviteeEmail:        membershipID + e2eEmail,
		Role:                role,
		InvitedByUserId:     "sub-admin",
		InvitationExpiresAt: e2eFrom + e2eWeek,
	}
}

func grantOver(membershipID, userID string, level servicev1.VerificationLevel) *servicev1.GrantMembershipRequest {
	return &servicev1.GrantMembershipRequest{MembershipId: membershipID, UserId: userID, PersonId: "e2e-per-m", ChannelConfirmed: true, VerificationLevel: level}
}

func admit(t *testing.T, conn *grpc.ClientConn, membershipID, organizationID, userID string, role servicev1.MembershipRole) {
	t.Helper()
	level := servicev1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED
	if role == servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR {
		level = servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP
	}
	var invited servicev1.InviteUserResponse
	invokeOver(t, conn, call(t, tenantA), "InviteUser", inviteOver(membershipID, organizationID, role), &invited)
	var granted servicev1.GrantMembershipResponse
	invokeOver(t, conn, call(t, tenantA), "GrantMembership", grantOver(membershipID, userID, level), &granted)
	if invited.GetInvited().GetMembershipId() != membershipID || granted.GetGranted().GetMembershipId() != membershipID {
		t.Fatalf("InviteUser() = %v and GrantMembership() = %v, want %s invited and granted", &invited, &granted, membershipID)
	}
}

func permissionsOf(t *testing.T, conn *grpc.ClientConn, tenant, userID string) []string {
	t.Helper()
	var resp servicev1.FindEffectivePermissionsResponse
	invokeOver(t, conn, call(t, tenant), "FindEffectivePermissions", &servicev1.FindEffectivePermissionsRequest{UserId: userID}, &resp)
	return resp.GetPermissions()
}

func TestEffectivePermissionsFollowTheMembershipsEndToEndOverGRPC(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-m"))
	for _, subject := range []string{"sub-op", "sub-au", "sub-both"} {
		invokeOver(t, conn, call(t, tenantA), "ProvisionUserAccount", provisionOver(subject), &servicev1.ProvisionUserAccountResponse{})
	}
	admit(t, conn, "e2e-mbr-op", "e2e-org-m", "sub-op", servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR)
	admit(t, conn, "e2e-mbr-au", "e2e-org-m", "sub-au", servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR)
	admit(t, conn, "e2e-mbr-b1", "e2e-org-m", "sub-both", servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR)
	admit(t, conn, "e2e-mbr-b2", "e2e-org-m", "sub-both", servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR)
	union := slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(operatorPermissions), auditorPermissions...))))

	if got := permissionsOf(t, conn, tenantA, "sub-op"); !slices.Equal(got, operatorPermissions) {
		t.Fatalf("FindEffectivePermissions(operator) = %v, want the 17 of the table in order", got)
	}
	if got := permissionsOf(t, conn, tenantA, "sub-au"); !slices.Equal(got, auditorPermissions) {
		t.Fatalf("FindEffectivePermissions(auditor) = %v, want the 14 of the table in order", got)
	}
	if got := permissionsOf(t, conn, tenantA, "sub-both"); len(union) != 20 || !slices.Equal(got, union) {
		t.Fatalf("FindEffectivePermissions(both) = %v, want the union of 20 %v", got, union)
	}
	if got := permissionsOf(t, conn, tenantA, "sub-ghost"); len(got) != 0 {
		t.Fatalf("FindEffectivePermissions(sub without account) = %v, want none", got)
	}
	if got := permissionsOf(t, conn, tenantB, "sub-op"); len(got) != 0 {
		t.Fatalf("FindEffectivePermissions(operator) from %s = %v, want none: the account is of %s", tenantB, got, tenantA)
	}

	var byUser servicev1.FindMembershipsByUserResponse
	invokeOver(t, conn, call(t, tenantA), "FindMembershipsByUser", &servicev1.FindMembershipsByUserRequest{UserId: "sub-both"}, &byUser)
	var byOrganization servicev1.FindMembershipsByOrganizationResponse
	invokeOver(t, conn, call(t, tenantA), "FindMembershipsByOrganization", &servicev1.FindMembershipsByOrganizationRequest{OrganizationId: "e2e-org-m"}, &byOrganization)
	if len(byUser.GetMemberships()) != 2 || len(byOrganization.GetMemberships()) != 4 {
		t.Fatalf("FindMembershipsByUser() = %d and FindMembershipsByOrganization() = %d, want 2 and 4", len(byUser.GetMemberships()), len(byOrganization.GetMemberships()))
	}
	for _, m := range byUser.GetMemberships() {
		if m.GetStatus() != servicev1.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE || m.GetUserId() != "sub-both" || m.GetGrantedAt() != e2eFrom {
			t.Fatalf("membership %v, want ACTIVE for sub-both granted at the clock", m)
		}
	}

	invokeOver(t, conn, call(t, tenantA), "RevokeMembership", &servicev1.RevokeMembershipRequest{MembershipId: "e2e-mbr-au", Reason: "audit concluded"}, &servicev1.RevokeMembershipResponse{})
	invokeOver(t, conn, call(t, tenantA), "RevokeMembership", &servicev1.RevokeMembershipRequest{MembershipId: "e2e-mbr-b2", Reason: "role changed"}, &servicev1.RevokeMembershipResponse{})
	invokeOver(t, conn, call(t, tenantA), "SuspendUser", &servicev1.SuspendUserRequest{UserId: "sub-op", Reason: "fraud review"}, &servicev1.SuspendUserResponse{})

	if got := permissionsOf(t, conn, tenantA, "sub-au"); len(got) != 0 {
		t.Fatalf("FindEffectivePermissions(auditor) after the revocation = %v, want none", got)
	}
	if got := permissionsOf(t, conn, tenantA, "sub-both"); !slices.Equal(got, operatorPermissions) {
		t.Fatalf("FindEffectivePermissions(both) after revoking the auditor = %v, want only the 17 of operator", got)
	}
	if got := permissionsOf(t, conn, tenantA, "sub-op"); len(got) != 0 {
		t.Fatalf("FindEffectivePermissions(operator) after the suspension = %v, want none", got)
	}

	types := map[string]int{}
	for _, record := range outbox(t, pool) {
		types[record.messageType]++
		if bytes.Contains(record.payload, []byte(e2eEmail)) {
			t.Fatalf("the %s payload carries an invitee email: %x", record.messageType, record.payload)
		}
		if record.messageType == "com.company.identity.membership-granted.v1" && record.partitionKey == "e2e-mbr-au" {
			var event eventv1.MembershipGranted
			if err := proto.Unmarshal(record.payload, &event); err != nil {
				t.Fatalf("payload does not decode: %v", err)
			}
			if event.GetUserId() != "sub-au" || event.GetRole() != eventv1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR ||
				event.GetVerificationLevel() != eventv1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP || event.GetGrantedAt().AsTime().UnixNano() != e2eFrom {
				t.Fatalf("event = %v, want sub-au granted auditor at channel ownership at the clock", &event)
			}
		}
		if record.messageType == "com.company.identity.user-suspended.v1" {
			var event eventv1.UserSuspended
			if err := proto.Unmarshal(record.payload, &event); err != nil {
				t.Fatalf("payload does not decode: %v", err)
			}
			if record.partitionKey != "sub-op" || event.GetUserId() != "sub-op" || event.GetReason() != "fraud review" {
				t.Fatalf("event = %v keyed %s, want sub-op suspended for fraud review, keyed by the subject", &event, record.partitionKey)
			}
		}
	}
	want := map[string]int{
		"com.company.identity.organization-registered.v1": 1,
		"com.company.identity.membership-granted.v1":      4,
		"com.company.identity.membership-revoked.v1":      2,
		"com.company.identity.user-suspended.v1":          1,
	}
	if !maps.Equal(types, want) {
		t.Fatalf("outbox types = %v, want %v", types, want)
	}
}

func TestPlatformOpsHoldsThePartnerPermissionsEndToEndOverGRPC(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-client"))
	platform := &servicev1.RegisterOrganizationRequest{
		OrganizationId: "e2e-org-platform",
		Kinds:          []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_PLATFORM},
		Jurisdiction:   "BR",
		TaxIdType:      servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxId:          e2ePlatformCNPJ,
		LegalName:      "Plataforma Sintetica Ltda",
		TradeName:      "Plataforma",
	}
	if resp := registerOver(t, conn, call(t, tenantA), platform); resp.GetRejection() != nil {
		t.Fatalf("RegisterOrganization(platform) = %v, want registered", resp)
	}
	for _, subject := range []string{"sub-ops", "sub-op-ops"} {
		invokeOver(t, conn, call(t, tenantA), "ProvisionUserAccount", provisionOver(subject), &servicev1.ProvisionUserAccountResponse{})
	}
	admit(t, conn, "e2e-mbr-ops", "e2e-org-platform", "sub-ops", servicev1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS)
	admit(t, conn, "e2e-mbr-oo1", "e2e-org-client", "sub-op-ops", servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR)
	admit(t, conn, "e2e-mbr-oo2", "e2e-org-platform", "sub-op-ops", servicev1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS)
	both := slices.Sorted(slices.Values(append(slices.Clone(operatorPermissions), platformOpsPermissions...)))

	if got := permissionsOf(t, conn, tenantA, "sub-ops"); !slices.Equal(got, platformOpsPermissions) {
		t.Fatalf("FindEffectivePermissions(platform_ops) = %v, want the 24 of the table in order", got)
	}
	if got := permissionsOf(t, conn, tenantA, "sub-op-ops"); len(both) != 41 || !slices.Equal(got, both) {
		t.Fatalf("FindEffectivePermissions(operator and platform_ops) = %v, want the union of 41 %v", got, both)
	}

	var misplaced servicev1.InviteUserResponse
	invokeOver(t, conn, call(t, tenantA), "InviteUser",
		inviteOver("e2e-mbr-misplaced", "e2e-org-client", servicev1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS), &misplaced)
	if got := misplaced.GetRejection().GetCode(); got != "identity-organization/membership/role-kind-mismatch" {
		t.Fatalf("InviteUser(platform_ops into a client) rejection = %q, want role-kind-mismatch (response %v)", got, &misplaced)
	}

	granted := 0
	for _, record := range outbox(t, pool) {
		if record.messageType != "com.company.identity.membership-granted.v1" || record.partitionKey != "e2e-mbr-ops" {
			continue
		}
		var event eventv1.MembershipGranted
		if err := proto.Unmarshal(record.payload, &event); err != nil {
			t.Fatalf("payload does not decode: %v", err)
		}
		if event.GetUserId() != "sub-ops" || event.GetRole() != eventv1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS ||
			event.GetVerificationLevel() != eventv1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED {
			t.Fatalf("MembershipGranted of e2e-mbr-ops = %v, want sub-ops as MEMBERSHIP_ROLE_PLATFORM_OPS at VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED", &event)
		}
		granted++
	}
	if granted != 1 {
		t.Fatalf("outbox holds %d grants of e2e-mbr-ops, want 1", granted)
	}
}

func userAccountVersion(t *testing.T, pool *pgxpool.Pool, subject string) (int, string) {
	t.Helper()
	var version int
	var snapshot string
	if err := pool.QueryRow(context.Background(), "SELECT version, snapshot::text FROM user_accounts WHERE user_account_id = $1", subject).Scan(&version, &snapshot); err != nil {
		t.Fatalf("read the account %s: %v", subject, err)
	}
	return version, snapshot
}

func TestProvisionUserAccountReplaysTheKeyAndAcceptsANewKeyWithoutAnEvent(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	var first, again, renewed servicev1.ProvisionUserAccountResponse
	var header metadata.MD
	invokeOver(t, conn, callWithKey(t, tenantA, "e2e-provision"), "ProvisionUserAccount", provisionOver("sub-p"), &first)
	version, snapshot := userAccountVersion(t, pool, "sub-p")
	if err := conn.Invoke(callWithKey(t, tenantA, "e2e-provision"), rpc.FullMethod("ProvisionUserAccount"), provisionOver("sub-p"), &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated ProvisionUserAccount() = %v, want the stored response", err)
	}
	replayed, _ := userAccountVersion(t, pool, "sub-p")
	invokeOver(t, conn, callWithKey(t, tenantA, "e2e-provision-again"), "ProvisionUserAccount", provisionOver("sub-p"), &renewed)
	bumped, unchanged := userAccountVersion(t, pool, "sub-p")

	if first.GetProvisioned().GetUserId() != "sub-p" || !proto.Equal(&again, &first) || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want %v marked as replay", &again, header, &first)
	}
	if renewed.GetProvisioned().GetUserId() != "sub-p" {
		t.Fatalf("ProvisionUserAccount(new key) = %v, want accepted", &renewed)
	}
	if replayed != version || bumped != version+1 || unchanged != snapshot {
		t.Fatalf("versions %d, %d, %d; want the replay untouched and the new key to raise only the technical version over %s", version, replayed, bumped, snapshot)
	}
	if a, o := rows(t, pool, "user_accounts"), outboxRows(t, pool); a != 1 || o != 0 {
		t.Fatalf("user_accounts = %d, outbox = %d, want one account and no event", a, o)
	}
}

type provisioned struct {
	resp *servicev1.ProvisionUserAccountResponse
	err  error
}

// provisionAtOnce keeps the key of each racer, which invokeAtOnce draws and
// drops, so the loser can be retried under the key it was refused with.
func provisionAtOnce(t *testing.T, conn *grpc.ClientConn, subject string, keys ...string) map[string]provisioned {
	t.Helper()
	var mu sync.Mutex
	var group sync.WaitGroup
	out := map[string]provisioned{}
	for _, key := range keys {
		ctx := callWithKey(t, tenantA, key)
		group.Go(func() {
			var resp servicev1.ProvisionUserAccountResponse
			err := conn.Invoke(ctx, rpc.FullMethod("ProvisionUserAccount"), provisionOver(subject), &resp)
			mu.Lock()
			defer mu.Unlock()
			out[key] = provisioned{resp: &resp, err: err}
		})
	}
	group.Wait()
	return out
}

func TestTwoConcurrentFirstProvisionsOfOneSubjectLeaveOneActiveAccount(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, newBarrier(2))
	retry := dial(t, pool, nil)

	got := provisionAtOnce(t, conn, "sub-race", "e2e-race-first", "e2e-race-second")

	accepted := 0
	for key, p := range got {
		switch {
		case p.err == nil && p.resp.GetProvisioned().GetUserId() == "sub-race":
			accepted++
		case status.Code(p.err) == codes.Aborted:
			var again servicev1.ProvisionUserAccountResponse
			invokeOver(t, retry, callWithKey(t, tenantA, key), "ProvisionUserAccount", provisionOver("sub-race"), &again)
			if again.GetProvisioned().GetUserId() != "sub-race" {
				t.Fatalf("retry of %s = %v, want the version conflict retryable under its own key", key, &again)
			}
		default:
			t.Errorf("%s = %v (%v), want accepted or the retryable version conflict", key, p.err, p.resp)
		}
	}
	if accepted == 0 {
		t.Fatal("no provision was accepted, want at least one")
	}
	ctx := execctx.WithExecution(t, context.Background(), execctx.WithTenant(tenantA))
	account, _, err := provider.NewUserAccountReader(postgres.NewReadPool(pool)).Load(ctx, "sub-race")
	if err != nil || account.Status != domain.UserAccountActive || account.PersonID != "e2e-per-m" {
		t.Fatalf("Load(sub-race) = %+v, %v; want the account active for e2e-per-m", account, err)
	}
	if a, o := rows(t, pool, "user_accounts"), outboxRows(t, pool); a != 1 || o != 0 {
		t.Fatalf("user_accounts = %d, outbox = %d, want exactly one account and no event", a, o)
	}
}

type refusable interface {
	proto.Message
	GetRejection() *servicev1.Rejection
}

func accessState(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	return fmt.Sprintf("accounts=%d/%d memberships=%d/%d outbox=%d",
		rows(t, pool, "user_accounts"), count(t, pool, "SELECT coalesce(sum(version), 0)::bigint FROM user_accounts"),
		rows(t, pool, "memberships"), count(t, pool, "SELECT coalesce(sum(version), 0)::bigint FROM memberships"),
		outboxRows(t, pool))
}

func TestEachRefusalOfTheAccessWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	later := dialAt(t, pool, nil, clockAt(e2eFrom+e2eWeek+1))
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-a"))
	funder := registration("e2e-org-f")
	funder.Kinds = []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER}
	funder.TaxIdType, funder.TaxId = servicev1.TaxIdType_TAX_ID_TYPE_CPF, e2eCPF
	registerOver(t, conn, call(t, tenantA), funder)
	for _, subject := range []string{"sub-a", "sub-s"} {
		invokeOver(t, conn, call(t, tenantA), "ProvisionUserAccount", provisionOver(subject), &servicev1.ProvisionUserAccountResponse{})
	}
	invokeOver(t, conn, call(t, tenantA), "SuspendUser", &servicev1.SuspendUserRequest{UserId: "sub-s", Reason: "fraud review"}, &servicev1.SuspendUserResponse{})
	invokeOver(t, conn, call(t, tenantA), "InviteUser", inviteOver("e2e-mbr-inv", "e2e-org-a", servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR), &servicev1.InviteUserResponse{})
	admit(t, conn, "e2e-mbr-act", "e2e-org-a", "sub-a", servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR)

	invite := func(edit func(*servicev1.InviteUserRequest)) proto.Message {
		r := inviteOver("e2e-mbr-new", "e2e-org-a", servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR)
		edit(r)
		return r
	}
	grant := func(edit func(*servicev1.GrantMembershipRequest)) proto.Message {
		r := grantOver("e2e-mbr-inv", "sub-a", servicev1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED)
		edit(r)
		return r
	}
	cases := []struct {
		name   string
		conn   *grpc.ClientConn
		method string
		req    proto.Message
		resp   refusable
		code   string
	}{
		{"subject empty", conn, "ProvisionUserAccount", provisionOver(""), &servicev1.ProvisionUserAccountResponse{},
			"identity-organization/user-account/subject-empty"},
		{"suspend a suspended account", conn, "SuspendUser", &servicev1.SuspendUserRequest{UserId: "sub-s", Reason: "again"}, &servicev1.SuspendUserResponse{},
			"identity-organization/user-account/not-active"},
		{"suspend without a reason", conn, "SuspendUser", &servicev1.SuspendUserRequest{UserId: "sub-a", Reason: " "}, &servicev1.SuspendUserResponse{},
			"identity-organization/user-account/reason-empty"},
		{"invite without an email", conn, "InviteUser", invite(func(r *servicev1.InviteUserRequest) { r.InviteeEmail = " " }), &servicev1.InviteUserResponse{},
			"identity-organization/membership/invitee-email-empty"},
		{"invite expiring at the request", conn, "InviteUser", invite(func(r *servicev1.InviteUserRequest) { r.InvitationExpiresAt = e2eFrom }), &servicev1.InviteUserResponse{},
			"identity-organization/membership/invitation-expiry-invalid"},
		{"invite into an unknown organization", conn, "InviteUser", invite(func(r *servicev1.InviteUserRequest) { r.OrganizationId = "e2e-org-none" }), &servicev1.InviteUserResponse{},
			"identity-organization/membership/organization-not-active"},
		{"invite an operator into a funder", conn, "InviteUser", invite(func(r *servicev1.InviteUserRequest) { r.OrganizationId = "e2e-org-f" }), &servicev1.InviteUserResponse{},
			"identity-organization/membership/role-kind-mismatch"},
		{"grant an active membership", conn, "GrantMembership", grant(func(r *servicev1.GrantMembershipRequest) { r.MembershipId = "e2e-mbr-act" }), &servicev1.GrantMembershipResponse{},
			"identity-organization/membership/not-invited"},
		{"grant an expired invitation", later, "GrantMembership", grant(func(*servicev1.GrantMembershipRequest) {}), &servicev1.GrantMembershipResponse{},
			"identity-organization/membership/invitation-expired"},
		{"grant without the channel", conn, "GrantMembership", grant(func(r *servicev1.GrantMembershipRequest) { r.ChannelConfirmed = false }), &servicev1.GrantMembershipResponse{},
			"identity-organization/membership/channel-not-confirmed"},
		{"grant channel ownership to an operator", conn, "GrantMembership", grant(func(r *servicev1.GrantMembershipRequest) {
			r.VerificationLevel = servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP
		}), &servicev1.GrantMembershipResponse{}, "identity-organization/membership/verification-insufficient"},
		{"grant to a suspended user", conn, "GrantMembership", grant(func(r *servicev1.GrantMembershipRequest) { r.UserId = "sub-s" }), &servicev1.GrantMembershipResponse{},
			"identity-organization/membership/user-not-active"},
		{"revoke an invitation", conn, "RevokeMembership", &servicev1.RevokeMembershipRequest{MembershipId: "e2e-mbr-inv", Reason: "left"}, &servicev1.RevokeMembershipResponse{},
			"identity-organization/membership/not-active"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := accessState(t, pool)

			invokeOver(t, c.conn, call(t, tenantA), c.method, c.req, c.resp)

			if got := c.resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q (response %v)", got, c.code, c.resp)
			}
			if after := accessState(t, pool); after != before {
				t.Fatalf("state %s after a refusal, want %s untouched", after, before)
			}
		})
	}
}

func changeRequestOver(bankAccountChangeID, organizationID, requestedBy string) *servicev1.RequestBankAccountChangeRequest {
	return &servicev1.RequestBankAccountChangeRequest{
		BankAccountChangeId:   bankAccountChangeID,
		OrganizationId:        organizationID,
		DestinationEvidenceId: "evd-" + bankAccountChangeID,
		RequestedByUserId:     requestedBy,
	}
}

func changeApprovalOver(bankAccountChangeID, approvedBy string) *servicev1.ApproveBankAccountChangeRequest {
	return &servicev1.ApproveBankAccountChangeRequest{BankAccountChangeId: bankAccountChangeID, ApprovedByUserId: approvedBy}
}

func changeState(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	return fmt.Sprintf("changes=%d/%d outbox=%d", rows(t, pool, "bank_account_changes"),
		count(t, pool, "SELECT coalesce(sum(version), 0)::bigint FROM bank_account_changes"), outboxRows(t, pool))
}

func TestBankAccountChangeEndToEndOverGRPC(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-k"))

	var requested servicev1.RequestBankAccountChangeResponse
	invokeOver(t, conn, call(t, tenantA), "RequestBankAccountChange", changeRequestOver("e2e-bac-1", "e2e-org-k", "sub-u1"), &requested)
	var approved servicev1.ApproveBankAccountChangeResponse
	invokeOver(t, conn, call(t, tenantA), "ApproveBankAccountChange", changeApprovalOver("e2e-bac-1", "sub-u2"), &approved)

	if requested.GetRequested().GetBankAccountChangeId() != "e2e-bac-1" || approved.GetApproved().GetBankAccountChangeId() != "e2e-bac-1" {
		t.Fatalf("RequestBankAccountChange() = %v and ApproveBankAccountChange() = %v, want e2e-bac-1 requested and approved", &requested, &approved)
	}
	records := outbox(t, pool)
	if len(records) != 3 {
		t.Fatalf("outbox rows = %d, want the organization's, the request and the approval", len(records))
	}
	request, approval := records[1], records[2]
	if request.messageType != "com.company.identity.bank-account-change-requested.v1" || approval.messageType != "com.company.identity.bank-account-change-approved.v1" {
		t.Fatalf("outbox types = %s and %s, want the requested and the approved of the change", request.messageType, approval.messageType)
	}
	for _, record := range []enqueued{request, approval} {
		if record.destination != application.Destination || record.partitionKey != "e2e-bac-1" {
			t.Fatalf("outbox record %s to %s keyed %s, want %s keyed by the change", record.messageType, record.destination, record.partitionKey, application.Destination)
		}
	}
	var requestedEvent eventv1.BankAccountChangeRequested
	if err := proto.Unmarshal(request.payload, &requestedEvent); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if requestedEvent.GetBankAccountChangeId() != "e2e-bac-1" || requestedEvent.GetOrganizationId() != "e2e-org-k" ||
		requestedEvent.GetDestinationEvidenceId() != "evd-e2e-bac-1" || requestedEvent.GetRequestedByUserId() != "sub-u1" ||
		requestedEvent.GetRequestedAt().AsTime().UnixNano() != e2eFrom {
		t.Fatalf("event = %v, want e2e-bac-1 of e2e-org-k with its evidence, requested by sub-u1 at the clock", &requestedEvent)
	}
	var approvedEvent eventv1.BankAccountChangeApproved
	if err := proto.Unmarshal(approval.payload, &approvedEvent); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if approvedEvent.GetBankAccountChangeId() != "e2e-bac-1" || approvedEvent.GetOrganizationId() != "e2e-org-k" ||
		approvedEvent.GetApprovedByUserId() != "sub-u2" || approvedEvent.GetApprovedAt().AsTime().UnixNano() != e2eFrom {
		t.Fatalf("event = %v, want e2e-bac-1 of e2e-org-k approved by sub-u2 at the clock", &approvedEvent)
	}

	var found servicev1.FindBankAccountChangeResponse
	invokeOver(t, conn, call(t, tenantA), "FindBankAccountChange", &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "e2e-bac-1"}, &found)
	want := &servicev1.BankAccountChange{
		BankAccountChangeId: "e2e-bac-1", OrganizationId: "e2e-org-k", DestinationEvidenceId: "evd-e2e-bac-1", RequestedByUserId: "sub-u1",
		ApprovedByUserId: "sub-u2", Status: servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED, RequestedAt: e2eFrom, ApprovedAt: e2eFrom,
	}
	if !proto.Equal(found.GetBankAccountChange(), want) {
		t.Fatalf("FindBankAccountChange() = %v, want %v", found.GetBankAccountChange(), want)
	}

	invokeOver(t, conn, call(t, tenantA), "RequestBankAccountChange", changeRequestOver("e2e-bac-2", "e2e-org-k", "sub-u2"), &servicev1.RequestBankAccountChangeResponse{})
	var byOrganization servicev1.FindBankAccountChangesByOrganizationResponse
	invokeOver(t, conn, call(t, tenantA), "FindBankAccountChangesByOrganization", &servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: "e2e-org-k"}, &byOrganization)
	statuses := map[string]servicev1.BankAccountChangeStatus{}
	for _, c := range byOrganization.GetBankAccountChanges() {
		statuses[c.GetBankAccountChangeId()] = c.GetStatus()
	}
	wantStatuses := map[string]servicev1.BankAccountChangeStatus{
		"e2e-bac-1": servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED,
		"e2e-bac-2": servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_REQUESTED,
	}
	if !maps.Equal(statuses, wantStatuses) {
		t.Fatalf("FindBankAccountChangesByOrganization() = %v, want %v", statuses, wantStatuses)
	}

	var elsewhere servicev1.FindBankAccountChangesByOrganizationResponse
	invokeOver(t, conn, call(t, tenantB), "FindBankAccountChangesByOrganization", &servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: "e2e-org-k"}, &elsewhere)
	err := conn.Invoke(call(t, tenantB), rpc.FullMethod("FindBankAccountChange"), &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "e2e-bac-1"}, &servicev1.FindBankAccountChangeResponse{})
	if len(elsewhere.GetBankAccountChanges()) != 0 || status.Code(err) != codes.NotFound {
		t.Fatalf("from %s: FindBankAccountChangesByOrganization() = %v and FindBankAccountChange() = %v, want nothing of another tenant", tenantB, elsewhere.GetBankAccountChanges(), err)
	}
}

func TestEachRefusalOfTheBankAccountChangeWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-k"))
	registerOver(t, conn, call(t, tenantB), registration("e2e-org-kb"))
	invokeOver(t, conn, call(t, tenantA), "RequestBankAccountChange", changeRequestOver("e2e-bac-req", "e2e-org-k", "sub-u1"), &servicev1.RequestBankAccountChangeResponse{})
	invokeOver(t, conn, call(t, tenantA), "RequestBankAccountChange", changeRequestOver("e2e-bac-app", "e2e-org-k", "sub-u1"), &servicev1.RequestBankAccountChangeResponse{})
	invokeOver(t, conn, call(t, tenantA), "ApproveBankAccountChange", changeApprovalOver("e2e-bac-app", "sub-u2"), &servicev1.ApproveBankAccountChangeResponse{})

	request := func(edit func(*servicev1.RequestBankAccountChangeRequest)) proto.Message {
		r := changeRequestOver("e2e-bac-new", "e2e-org-k", "sub-u1")
		edit(r)
		return r
	}
	cases := []struct {
		name   string
		method string
		req    proto.Message
		resp   refusable
		code   string
	}{
		{"request without the evidence", "RequestBankAccountChange", request(func(r *servicev1.RequestBankAccountChangeRequest) { r.DestinationEvidenceId = "" }),
			&servicev1.RequestBankAccountChangeResponse{}, "identity-organization/bank-account-change/destination-evidence-missing"},
		{"request for an unknown organization", "RequestBankAccountChange", request(func(r *servicev1.RequestBankAccountChangeRequest) { r.OrganizationId = "e2e-org-none" }),
			&servicev1.RequestBankAccountChangeResponse{}, "identity-organization/bank-account-change/organization-not-active"},
		{"request for an organization of another tenant", "RequestBankAccountChange", request(func(r *servicev1.RequestBankAccountChangeRequest) { r.OrganizationId = "e2e-org-kb" }),
			&servicev1.RequestBankAccountChangeResponse{}, "identity-organization/bank-account-change/organization-not-active"},
		{"approval by the requester", "ApproveBankAccountChange", changeApprovalOver("e2e-bac-req", "sub-u1"),
			&servicev1.ApproveBankAccountChangeResponse{}, "identity-organization/bank-account-change/same-actor-approval"},
		{"second approval", "ApproveBankAccountChange", changeApprovalOver("e2e-bac-app", "sub-u3"),
			&servicev1.ApproveBankAccountChangeResponse{}, "identity-organization/bank-account-change/not-requested"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := changeState(t, pool)

			invokeOver(t, conn, call(t, tenantA), c.method, c.req, c.resp)

			if got := c.resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q (response %v)", got, c.code, c.resp)
			}
			if after := changeState(t, pool); after != before {
				t.Fatalf("state %s after a refusal, want %s untouched", after, before)
			}
		})
	}
}

func TestABlankActorOfTheBankAccountChangeIsRefusedAndNothingIsWritten(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	registerOver(t, conn, call(t, tenantA), registration("e2e-org-k"))
	invokeOver(t, conn, call(t, tenantA), "RequestBankAccountChange", changeRequestOver("e2e-bac-1", "e2e-org-k", "sub-u1"), &servicev1.RequestBankAccountChangeResponse{})
	before := changeState(t, pool)

	for _, blank := range []string{"", "  "} {
		errRequest := conn.Invoke(call(t, tenantA), rpc.FullMethod("RequestBankAccountChange"), changeRequestOver("e2e-bac-2", "e2e-org-k", blank),
			&servicev1.RequestBankAccountChangeResponse{})
		errApprove := conn.Invoke(call(t, tenantA), rpc.FullMethod("ApproveBankAccountChange"), changeApprovalOver("e2e-bac-1", blank),
			&servicev1.ApproveBankAccountChangeResponse{})

		if status.Code(errRequest) != codes.InvalidArgument || status.Code(errApprove) != codes.InvalidArgument {
			t.Fatalf("actor %q: RequestBankAccountChange() = %v and ApproveBankAccountChange() = %v, want InvalidArgument for both", blank, errRequest, errApprove)
		}
	}
	if after := changeState(t, pool); after != before {
		t.Fatalf("state %s after the blank actors, want %s untouched", after, before)
	}
}

func TestARepeatedCommandOfTheBankAccountChangeReplaysWithoutASecondEffect(t *testing.T) {
	for _, c := range []struct {
		method string
		req    proto.Message
		resp   func() proto.Message
	}{
		{"RequestBankAccountChange", changeRequestOver("e2e-bac-replay", "e2e-org-k", "sub-u1"), func() proto.Message { return &servicev1.RequestBankAccountChangeResponse{} }},
		{"ApproveBankAccountChange", changeApprovalOver("e2e-bac-held", "sub-u2"), func() proto.Message { return &servicev1.ApproveBankAccountChangeResponse{} }},
	} {
		t.Run(c.method, func(t *testing.T) {
			pool := appkit.OpenPool(t)
			conn := dial(t, pool, nil)
			registerOver(t, conn, call(t, tenantA), registration("e2e-org-k"))
			invokeOver(t, conn, call(t, tenantA), "RequestBankAccountChange", changeRequestOver("e2e-bac-held", "e2e-org-k", "sub-u1"), &servicev1.RequestBankAccountChangeResponse{})
			first, again := c.resp(), c.resp()
			var header metadata.MD

			invokeOver(t, conn, callWithKey(t, tenantA, "e2e-replay-"+c.method), c.method, c.req, first)
			before := changeState(t, pool)
			if err := conn.Invoke(callWithKey(t, tenantA, "e2e-replay-"+c.method), rpc.FullMethod(c.method), c.req, again, grpc.Header(&header)); err != nil {
				t.Fatalf("repeated %s() = %v, want the stored response", c.method, err)
			}

			if refusal := again.(refusable).GetRejection(); refusal != nil || !proto.Equal(again, first) {
				t.Fatalf("replay = %v, want the stored acceptance %v, not a refusal of the state it left", again, first)
			}
			if !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
				t.Fatalf("replay header = %v, want [true]", header.Get(kernelgrpc.ReplayedHeader))
			}
			if after := changeState(t, pool); after != before {
				t.Fatalf("state %s after a replay, want %s: one effect and one event per key", after, before)
			}
		})
	}
}
