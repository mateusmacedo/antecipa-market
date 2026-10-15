package rpc_test

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

var (
	organizationsTable  = memory.Table[domain.OrganizationID, domain.OrganizationSnapshot]{Name: "organizations"}
	establishmentsTable = memory.Table[domain.EstablishmentID, domain.EstablishmentSnapshot]{
		Name: "establishments",
		Clone: func(s domain.EstablishmentSnapshot) domain.EstablishmentSnapshot {
			return domain.FromEstablishmentSnapshot(s).Snapshot()
		},
	}
	personsTable         = memory.Table[domain.PersonID, domain.PersonSnapshot]{Name: "persons"}
	userAccountsTable    = memory.Table[domain.UserID, domain.UserAccountSnapshot]{Name: "user_accounts"}
	membershipsTable     = memory.Table[domain.MembershipID, domain.MembershipSnapshot]{Name: "memberships"}
	representationsTable = memory.Table[domain.RepresentationID, domain.RepresentationSnapshot]{
		Name: "representations",
		Clone: func(s domain.RepresentationSnapshot) domain.RepresentationSnapshot {
			return domain.FromRepresentationSnapshot(s).Snapshot()
		},
	}
	bankAccountChangesTable = memory.Table[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]{Name: "bank_account_changes"}
)

const (
	occurred   = port.Instant(1_755_432_000_000_000_000)
	testTenant = port.TenantID("acme")
)

var unlimited = admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}

type fakeProtector struct{}

func (fakeProtector) Protect(_ context.Context, tenant port.TenantID, kind domain.TaxIDType, value string) (ports.ProtectedIdentifier, error) {
	return ports.ProtectedIdentifier{Hash: hashOf(tenant, kind, value), KeyVersion: 1}, nil
}

func hashOf(tenant port.TenantID, kind domain.TaxIDType, value string) string {
	return string(tenant) + "|" + string(kind) + "|" + domain.NormalizeTaxID(kind, value)
}

type byTaxIDHash map[string][]domain.OrganizationSnapshot

func (r byTaxIDHash) LoadByTaxIDHash(_ context.Context, taxIDHash string) ([]domain.OrganizationSnapshot, error) {
	return r[taxIDHash], nil
}

type establishmentsByTaxIDHash map[string][]domain.EstablishmentSnapshot

func (r establishmentsByTaxIDHash) LoadByTaxIDHash(_ context.Context, taxIDHash string) ([]domain.EstablishmentSnapshot, error) {
	return r[taxIDHash], nil
}

type establishmentsByOrganization map[domain.OrganizationID][]domain.EstablishmentSnapshot

func (r establishmentsByOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.EstablishmentSnapshot, error) {
	return r[id], nil
}

type personsByDocumentHash map[string][]domain.PersonSnapshot

func (r personsByDocumentHash) LoadByDocumentHash(_ context.Context, documentHash string) ([]domain.PersonSnapshot, error) {
	return r[documentHash], nil
}

type representationsByOrganization map[domain.OrganizationID][]domain.RepresentationSnapshot

func (r representationsByOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.RepresentationSnapshot, error) {
	return r[id], nil
}

type membershipsByUser map[domain.UserID][]domain.MembershipSnapshot

func (r membershipsByUser) LoadByUser(_ context.Context, id domain.UserID) ([]domain.MembershipSnapshot, error) {
	return r[id], nil
}

type membershipsByOrganization map[domain.OrganizationID][]domain.MembershipSnapshot

func (r membershipsByOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.MembershipSnapshot, error) {
	return r[id], nil
}

type bankAccountChangesByOrganization map[domain.OrganizationID][]domain.BankAccountChangeSnapshot

func (r bankAccountChangesByOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.BankAccountChangeSnapshot, error) {
	return r[id], nil
}

// The kernel memory store serves reads and writes by ID only (memory v1.0.0-rc.4
// store.go:27-33), so a test plants every relation keyed otherwise here.
type relations struct {
	organizations   byTaxIDHash
	establishments  establishmentsByTaxIDHash
	byOrganization  establishmentsByOrganization
	documents       personsByDocumentHash
	representations representationsByOrganization
	byUser          membershipsByUser
	members         membershipsByOrganization
	changes         bankAccountChangesByOrganization
}

type harness struct {
	store *memory.Store
	conn  *grpc.ClientConn
}

func newHarness(t *testing.T, planted byTaxIDHash) *harness {
	t.Helper()
	return newHarnessWith(t, relations{organizations: planted})
}

func newHarnessWith(t *testing.T, planted relations) *harness {
	t.Helper()

	store := memory.New()
	policy, err := kernelapp.IdempotencyPolicy(time.Second, time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{
				Organizations:   organizationsTable.Repository(tx),
				Establishments:  establishmentsTable.Repository(tx),
				Persons:         personsTable.Repository(tx),
				Representations: representationsTable.Repository(tx),
				UserAccounts:    userAccountsTable.Repository(tx),
				Memberships:     membershipsTable.Repository(tx),
				Outbox:          tx.Outbox(),
				Commands:        tx.CommandInbox(application.CommandConsumer),

				BankAccountChanges: bankAccountChangesTable.Repository(tx),
			}
		}),
		Reader:      organizationsTable.Reader(store),
		TaxIDReader: planted.organizations,
		Protector:   fakeProtector{},
		Clock:       memory.FixedClock{At: occurred},
		IDs:         &memory.SequenceIDs{Prefix: "m-"},
		Authorize:   usecase.AllowAll[application.Operation](),
		Idempotency: policy,

		EstablishmentTaxIDReader:      planted.establishments,
		EstablishmentsByOrganization:  planted.byOrganization,
		PersonReader:                  personsTable.Reader(store),
		DocumentReader:                planted.documents,
		RepresentationReader:          representationsTable.Reader(store),
		RepresentationsByOrganization: planted.representations,
		UserAccountReader:             userAccountsTable.Reader(store),
		MembershipsByUser:             planted.byUser,
		MembershipsByOrganization:     planted.members,

		BankAccountChangeReader:          bankAccountChangesTable.Reader(store),
		BankAccountChangesByOrganization: planted.changes,
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

var keys atomic.Int64

// Mirrors the bff call: a GRP-04 deadline, the IDN-15 tenant and an IDM-01 key.
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
