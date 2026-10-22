package rpc_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

var (
	partnersTable = memory.Table[domain.PartnerID, domain.PartnerSnapshot]{Name: "partners", Unique: []memory.Unique[domain.PartnerSnapshot]{{
		Name: "partners_tenant_id_organization_id_role_key",
		Key: func(s domain.PartnerSnapshot) ([]string, bool) {
			return []string{string(s.OrganizationID), string(s.Role)}, true
		},
		Rejection: domain.RejectPartnerAlreadyRegistered,
	}}}
	agreementsTable = memory.Table[domain.AgreementID, domain.PartnerAgreementSnapshot]{Name: "partner_agreements", Unique: seriesUniques("partner_agreements",
		func(s domain.PartnerAgreementSnapshot) (string, string) {
			return string(s.SupersedesID), string(s.PartnerID)
		},
		domain.RejectPartnerAgreementSupersedesInvalid)}
	profilesTable = memory.Table[domain.ProfileID, domain.IntegrationProfileSnapshot]{Name: "integration_profiles", Unique: seriesUniques("integration_profiles",
		func(s domain.IntegrationProfileSnapshot) (string, string) {
			return string(s.SupersedesID), seriesKey(string(s.PartnerID), string(s.Environment))
		},
		domain.RejectIntegrationProfileSupersedesInvalid)}
	policiesTable = memory.Table[domain.PolicyID, domain.OperationalPolicySnapshot]{Name: "operational_policies", Unique: seriesUniques("operational_policies",
		func(s domain.OperationalPolicySnapshot) (string, string) {
			return string(s.SupersedesID), string(s.PartnerID)
		},
		domain.RejectOperationalPolicySupersedesInvalid)}
	capsTable = memory.Table[domain.CapabilityID, domain.CapabilitySnapshot]{Name: "capabilities", Unique: seriesUniques("capabilities",
		func(s domain.CapabilitySnapshot) (string, string) {
			return string(s.SupersedesID), seriesKey(string(s.PartnerID), s.Operation, s.AssetType, string(s.Environment))
		},
		domain.RejectCapabilitySupersedesInvalid)}
	productsTable = memory.Table[domain.ProductID, domain.FinancialProductSnapshot]{Name: "financial_products", Unique: seriesUniques("financial_products",
		func(s domain.FinancialProductSnapshot) (string, string) {
			return string(s.SupersedesID), seriesKey(string(s.PartnerID), s.Code)
		},
		domain.RejectFinancialProductSupersedesInvalid)}
)

func seriesKey(parts ...string) string { return strings.Join(parts, "\x1f") }

// seriesUniques mirrors the two constraints of a versioned table: supersedes
// names one version once, and a series holds one first version, whose
// series_origin is the series key; a later version has none.
func seriesUniques[S any](table string, keys func(S) (supersedes, series string), rejection *kernel.Rejection) []memory.Unique[S] {
	return []memory.Unique[S]{
		{
			Name: table + "_tenant_id_supersedes_id_key",
			Key: func(s S) ([]string, bool) {
				supersedes, _ := keys(s)
				return []string{supersedes}, supersedes != ""
			},
			Rejection: rejection,
		},
		{
			Name: table + "_tenant_id_series_origin_key",
			Key: func(s S) ([]string, bool) {
				supersedes, series := keys(s)
				return []string{series}, supersedes == ""
			},
			Rejection: rejection,
		},
	}
}

const (
	occurred   = port.Instant(1_755_432_000_000_000_000)
	testTenant = port.TenantID("acme")
)

var unlimited = admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}

type directory map[domain.OrganizationID][]domain.OrganizationKind

func (d directory) FindOrganization(_ context.Context, _ port.TenantID, id domain.OrganizationID) ([]domain.OrganizationKind, error) {
	kinds, found := d[id]
	if !found {
		return nil, port.ErrNotFound
	}
	return kinds, nil
}

type partnersByOrganization map[domain.OrganizationID][]domain.PartnerSnapshot

func (r partnersByOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.PartnerSnapshot, error) {
	return r[id], nil
}

type agreementsByPartner map[domain.PartnerID][]domain.PartnerAgreementSnapshot

func (r agreementsByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	return r[id], nil
}

type profilesByPartner map[domain.PartnerID][]domain.IntegrationProfileSnapshot

func (r profilesByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error) {
	return r[id], nil
}

type policiesByPartner map[domain.PartnerID][]domain.OperationalPolicySnapshot

func (r policiesByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.OperationalPolicySnapshot, error) {
	return r[id], nil
}

type capabilitiesByPartner map[domain.PartnerID][]domain.CapabilitySnapshot

func (r capabilitiesByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.CapabilitySnapshot, error) {
	return r[id], nil
}

type productsByPartner map[domain.PartnerID][]domain.FinancialProductSnapshot

func (r productsByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.FinancialProductSnapshot, error) {
	return r[id], nil
}

type partnersByRole map[domain.PartnerRole][]domain.PartnerSnapshot

func (r partnersByRole) LoadByRole(_ context.Context, role domain.PartnerRole) ([]domain.PartnerSnapshot, error) {
	return r[role], nil
}

type profilesByEnvironment map[domain.Environment][]domain.IntegrationProfileSnapshot

func (r profilesByEnvironment) LoadByEnvironment(_ context.Context, environment domain.Environment) ([]domain.IntegrationProfileSnapshot, error) {
	return r[environment], nil
}

type bucketOf int

func (b bucketOf) Bucket(domain.CapabilityID, port.TenantID) int { return int(b) }

// The kernel memory store serves reads and writes by ID only (memory v1.0.0-rc.5
// store.go:27-33), so a test plants every relation keyed otherwise here.
type relations struct {
	organizations directory
	partners      partnersByOrganization
	agreements    agreementsByPartner
	profiles      profilesByPartner
	policies      policiesByPartner
	capabilities  capabilitiesByPartner
	products      productsByPartner
	byRole        partnersByRole
	byEnvironment profilesByEnvironment
	bucket        bucketOf
}

type harness struct {
	store *memory.Store
	conn  *grpc.ClientConn
}

func newHarness(t *testing.T, planted relations) *harness {
	t.Helper()

	if planted.organizations == nil {
		planted.organizations = directory{"org-1": {domain.KindFunder}}
	}
	store := memory.New()
	policy, err := kernelapp.IdempotencyPolicy(time.Second, time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{
				Partners:   partnersTable.Repository(tx),
				Agreements: agreementsTable.Repository(tx),

				IntegrationProfiles: profilesTable.Repository(tx),
				OperationalPolicies: policiesTable.Repository(tx),
				Capabilities:        capsTable.Repository(tx),
				FinancialProducts:   productsTable.Repository(tx),

				AgreementsByPartner:   planted.agreements,
				ProfilesByPartner:     planted.profiles,
				CapabilitiesByPartner: planted.capabilities,

				Outbox:   tx.Outbox(),
				Commands: tx.CommandInbox(application.CommandConsumer),
			}
		}),
		Directory:              planted.organizations,
		PartnersByOrganization: planted.partners,
		AgreementsByPartner:    planted.agreements,
		ProfilesByPartner:      planted.profiles,
		PoliciesByPartner:      planted.policies,
		CapabilitiesByPartner:  planted.capabilities,
		ProductsByPartner:      planted.products,
		PartnerReader:          partnersTable.Reader(store),
		PartnersByRole:         planted.byRole,
		ProfilesByEnvironment:  planted.byEnvironment,
		Rollout:                planted.bucket,
		Clock:                  memory.FixedClock{At: occurred},
		IDs:                    &memory.SequenceIDs{Prefix: "m-"},
		Authorize:              usecase.AllowAll[application.Operation](),
		Idempotency:            policy,
	}

	ctrl, err := admission.New(admission.Config{Limits: kernelgrpc.MethodLimits(rpc.ServiceName, rpc.Methods(), unlimited), MaxKeys: 16, Clock: obsclock.System()})
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

	return &harness{store: store, conn: conn}
}

func (h *harness) invoke(ctx context.Context, method string, req, resp proto.Message, opts ...grpc.CallOption) error {
	return h.conn.Invoke(ctx, rpc.FullMethod(method), req, resp, opts...)
}

func (h *harness) seedAgreement(t *testing.T, s domain.PartnerAgreementSnapshot) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) port.Repository[domain.AgreementID, domain.PartnerAgreementSnapshot] {
		return agreementsTable.Repository(tx)
	})
	ctx := execctx.WithExecution(t, context.Background())
	if err := uow.Within(ctx, func(ctx context.Context, repo port.Repository[domain.AgreementID, domain.PartnerAgreementSnapshot]) error {
		return repo.Save(ctx, s.ID, s, 0)
	}); err != nil {
		t.Fatalf("seed agreement %s: %v", s.ID, err)
	}
}

var keys atomic.Int64

func withTenant(t *testing.T) context.Context {
	t.Helper()
	return withKey(t, fmt.Sprintf("k-%d", keys.Add(1)))
}

func withKey(t *testing.T, key string) context.Context {
	t.Helper()
	return metadata.AppendToOutgoingContext(withoutKey(t), kernelgrpc.IdempotencyKey, key)
}

func withoutKey(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, string(testTenant))
}

func (h *harness) seedProfile(t *testing.T, s domain.IntegrationProfileSnapshot) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) port.Repository[domain.ProfileID, domain.IntegrationProfileSnapshot] {
		return profilesTable.Repository(tx)
	})
	ctx := execctx.WithExecution(t, context.Background())
	if err := uow.Within(ctx, func(ctx context.Context, repo port.Repository[domain.ProfileID, domain.IntegrationProfileSnapshot]) error {
		return repo.Save(ctx, s.ID, s, 0)
	}); err != nil {
		t.Fatalf("seed profile %s: %v", s.ID, err)
	}
}

func seedIn[ID comparable, S any](t *testing.T, h *harness, table memory.Table[ID, S], id ID, s S) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) port.Repository[ID, S] { return table.Repository(tx) })
	ctx := execctx.WithExecution(t, context.Background())
	if err := uow.Within(ctx, func(ctx context.Context, repo port.Repository[ID, S]) error {
		return repo.Save(ctx, id, s, 0)
	}); err != nil {
		t.Fatalf("seed %s %v: %v", table.Name, id, err)
	}
}
