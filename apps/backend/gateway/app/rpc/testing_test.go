package rpc_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"slices"
	"sync"
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

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	occurred   = port.Instant(1_755_432_000_000_000_000)
	testTenant = port.TenantID("platform")
)

var unlimited = admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 64}

var (
	connectorsTable     = memory.Table[domain.ConnectorKey, domain.ConnectorSnapshot]{Name: "connectors"}
	profilesTable       = memory.Table[domain.ProfileKey, domain.CapabilityProfileSnapshot]{Name: "capability_profiles"}
	authorizationsTable = memory.Table[domain.ConsentID, domain.QueryAuthorizationSnapshot]{Name: "query_authorizations"}
	commandsTable       = memory.Table[domain.IdempotencyKey, domain.ExternalCommandSnapshot]{Name: "external_commands"}
)

// written remembers the ids a table received, so a relation reads them back
// through the store, which serves reads by id only.
type written[ID comparable] struct {
	mu  sync.Mutex
	ids []ID
}

func (w *written[ID]) add(id ID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !slices.Contains(w.ids, id) {
		w.ids = append(w.ids, id)
	}
}

func (w *written[ID]) scan(ctx context.Context, load func(context.Context, ID) (any, error)) []any {
	w.mu.Lock()
	ids := slices.Clone(w.ids)
	w.mu.Unlock()
	var out []any
	for _, id := range ids {
		if s, err := load(ctx, id); err == nil {
			out = append(out, s)
		}
	}
	return out
}

type tracked[ID comparable, S any] struct {
	port.Repository[ID, S]
	ids *written[ID]
}

func (r tracked[ID, S]) Save(ctx context.Context, id ID, state S, expected port.Version) error {
	if err := r.Repository.Save(ctx, id, state, expected); err != nil {
		return err
	}
	r.ids.add(id)
	return nil
}

func filtered[ID comparable, S any](ctx context.Context, ids *written[ID], reader port.Reader[ID, S], keep func(S) bool) []S {
	var out []S
	for _, s := range ids.scan(ctx, func(ctx context.Context, id ID) (any, error) {
		s, _, err := reader.Load(ctx, id)
		return s, err
	}) {
		if state := s.(S); keep(state) {
			out = append(out, state)
		}
	}
	return out
}

type relations struct {
	store          *memory.Store
	profiles       *written[domain.ProfileKey]
	authorizations *written[domain.ConsentID]
	commands       *written[domain.IdempotencyKey]
}

type profilesOf struct {
	r      relations
	reader port.Reader[domain.ProfileKey, domain.CapabilityProfileSnapshot]
}

func (p profilesOf) LoadByConnector(ctx context.Context, connector domain.ConnectorKey) ([]domain.CapabilityProfileSnapshot, error) {
	return filtered(ctx, p.r.profiles, p.reader, func(s domain.CapabilityProfileSnapshot) bool { return s.ConnectorKey == connector }), nil
}

type authorizationsOf struct{ r relations }

func (a authorizationsOf) LoadByOrganization(ctx context.Context, organization domain.OrganizationID) ([]domain.QueryAuthorizationSnapshot, error) {
	return filtered(ctx, a.r.authorizations, authorizationsTable.Reader(a.r.store), func(s domain.QueryAuthorizationSnapshot) bool { return s.OrganizationID == organization }), nil
}

type commandsOf struct {
	r      relations
	reader port.Reader[domain.IdempotencyKey, domain.ExternalCommandSnapshot]
}

func (c commandsOf) LoadBySeries(ctx context.Context, series string) ([]domain.ExternalCommandSnapshot, error) {
	return filtered(ctx, c.r.commands, c.reader, func(s domain.ExternalCommandSnapshot) bool { return s.SeriesKey == series }), nil
}

func (c commandsOf) LoadByBusinessKey(ctx context.Context, key string) ([]domain.ExternalCommandSnapshot, error) {
	return filtered(ctx, c.r.commands, c.reader, func(s domain.ExternalCommandSnapshot) bool { return s.BusinessKey == key }), nil
}

func (c commandsOf) LoadByStatus(ctx context.Context, status domain.CommandStatus) ([]domain.ExternalCommandSnapshot, error) {
	return filtered(ctx, c.r.commands, c.reader, func(s domain.ExternalCommandSnapshot) bool { return s.Status == status }), nil
}

func (c commandsOf) LoadByOutboundKey(ctx context.Context, key port.OutboundKey) ([]domain.ExternalCommandSnapshot, error) {
	return filtered(ctx, c.r.commands, c.reader, func(s domain.ExternalCommandSnapshot) bool { return s.OutboundKey == string(key) }), nil
}

type harness struct {
	store *memory.Store
	conn  *grpc.ClientConn
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store := memory.New()
	r := relations{store: store, profiles: &written[domain.ProfileKey]{}, authorizations: &written[domain.ConsentID]{}, commands: &written[domain.IdempotencyKey]{}}
	policy, err := kernelapp.IdempotencyPolicy(time.Second, time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	commandReader := commandsTable.Reader(store)
	service := application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			profiles := profilesTable.Repository(tx)
			commands := commandsTable.Repository(tx)
			return application.Resources{
				Connectors:          connectorsTable.Repository(tx),
				Profiles:            tracked[domain.ProfileKey, domain.CapabilityProfileSnapshot]{Repository: profiles, ids: r.profiles},
				Authorizations:      tracked[domain.ConsentID, domain.QueryAuthorizationSnapshot]{Repository: authorizationsTable.Repository(tx), ids: r.authorizations},
				ExternalCommands:    tracked[domain.IdempotencyKey, domain.ExternalCommandSnapshot]{Repository: commands, ids: r.commands},
				ProfilesByConnector: profilesOf{r: r, reader: profiles},
				CommandsBySeries:    commandsOf{r: r, reader: commands},
				Ledger:              tx.AttemptLedger(),
				Outbox:              tx.Outbox(),
				Commands:            tx.CommandInbox(application.CommandConsumer),
				Inbox:               tx.Inbox("gateway"),
			}
		}),
		ConnectorReader:              connectorsTable.Reader(store),
		ProfileReader:                profilesTable.Reader(store),
		ProfilesByConnector:          profilesOf{r: r, reader: profilesTable.Reader(store)},
		AuthorizationsByOrganization: authorizationsOf{r: r},
		CommandReader:                commandReader,
		CommandsByBusinessKey:        commandsOf{r: r, reader: commandReader},
		CommandsByStatus:             commandsOf{r: r, reader: commandReader},
		CommandsByOutboundKey:        commandsOf{r: r, reader: commandReader},
		Clock:                        memory.FixedClock{At: occurred},
		IDs:                          &memory.SequenceIDs{Prefix: "m-"},
		Authorize:                    usecase.AllowAll[application.Operation](),
		Idempotency:                  policy,
		OutboundKeys:                 usecase.OutboundKeyPolicy{Digest: func(c []byte) port.Fingerprint { return sha256.Sum256(c) }},
		Consumer:                     "gateway",
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

func (h *harness) invoke(ctx context.Context, method string, req, resp proto.Message) error {
	return h.conn.Invoke(ctx, rpc.FullMethod(method), req, resp)
}

var keys atomic.Int64

func withKey(t *testing.T) context.Context {
	t.Helper()
	return metadata.AppendToOutgoingContext(withoutKey(t), kernelgrpc.IdempotencyKey, fmt.Sprintf("k-%d", keys.Add(1)))
}

func withoutKey(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, string(testTenant))
}
