//go:build integration

package app_test

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

type e2e struct {
	pool    *pgxpool.Pool
	service application.Service
	conn    *grpc.ClientConn
}

// serve runs the use cases over Postgres behind the server chain the api
// mounts, so the suite crosses the same hop the BFF does.
func serve(t *testing.T) *e2e {
	t.Helper()
	pool := appkit.OpenPool(t)
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := app.NewService(pool, memory.FixedClock{At: occurred}, &memory.SequenceIDs{Prefix: "e2e-"}, app.Waits{Message: time.Second, Command: time.Second})
	service.Idempotency = policy

	conn := serveOver(t, service)
	return &e2e{pool: pool, service: service, conn: conn}
}

// serveOver mounts the server chain the api role mounts over a service the
// suite assembles, and dials it in memory.
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

var e2eKeys atomic.Int64

func (e *e2e) command(t *testing.T, method string, req, resp proto.Message) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, string(testTenant), kernelgrpc.IdempotencyKey, fmt.Sprintf("e2e-%d", e2eKeys.Add(1)))
	if err := e.conn.Invoke(ctx, rpc.FullMethod(method), req, resp); err != nil {
		t.Fatalf("%s() = %v", method, err)
	}
}

func (e *e2e) destinations(t *testing.T) []string {
	t.Helper()
	rows, err := e.pool.Query(context.Background(), "SELECT destination FROM outbox ORDER BY id")
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var destination string
		if err := rows.Scan(&destination); err != nil {
			t.Fatalf("scan outbox: %v", err)
		}
		out = append(out, destination)
	}
	return out
}

func (e *e2e) ready(t *testing.T) {
	t.Helper()
	e.command(t, "RegisterConnector", &servicev1.RegisterConnectorRequest{
		PartnerId: "prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, CredentialRef: "vault://prt-1", IntegrationVersion: 1,
	}, &servicev1.RegisterConnectorResponse{})
	e.command(t, "EnableOperation", &servicev1.EnableOperationRequest{
		ConnectorKey: "prt-1:production", CapabilityId: "cap-1", Operation: "submitEffect", StateQuerySupported: false, Version: 1,
	}, &servicev1.EnableOperationResponse{})
	var declared servicev1.DeclareCapabilityProfileResponse
	e.command(t, "DeclareCapabilityProfile", &servicev1.DeclareCapabilityProfileRequest{
		ConnectorId: "prt-1:production", Version: 1, EffectiveFrom: int64(occurred),
		Operations: []*servicev1.OperationCapability{{
			Operation: "submitEffect", Supported: true, IdempotencySupport: servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_KEY_HEADER,
			Efficacy: servicev1.Efficacy_EFFICACY_ON_SUBMIT, Quiescence: 1, MaxQueries: 3, KeyAlphabet: "0123456789abcdef",
			KeyMaxLength: 32, AmbiguityDeadline: 3600, Protocol: servicev1.Protocol_PROTOCOL_SYNC_HTTP,
			EncumbranceCoverage: servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_UNAVAILABLE,
		}},
	}, &declared)
	if declared.GetDeclared() == nil {
		t.Fatalf("DeclareCapabilityProfile() = %v", &declared)
	}
}

// commandOutboundKey reads the column the kernel's sweep finds a command by;
// the edge does not publish the outbound key, which only the partner sees.
func commandOutboundKey(t *testing.T, pool *pgxpool.Pool, key string) ports.OutboundKey {
	t.Helper()
	var outbound string
	if err := pool.QueryRow(context.Background(), "SELECT outbound_key FROM external_commands WHERE external_command_id = $1", key).Scan(&outbound); err != nil {
		t.Fatalf("read outbound key: %v", err)
	}
	return ports.OutboundKey(outbound)
}

func create(key string, version int32) *servicev1.CreateExternalCommandRequest {
	return &servicev1.CreateExternalCommandRequest{
		IdempotencyKey: key, TenantId: "tnt-1", ConnectorId: "prt-1:production", Operation: "submitEffect",
		BusinessKey: "eff-1", PayloadVersion: version, Payload: `{"amount":100}`, PayloadHash: "h-" + key,
	}
}

func TestTheExternalCommandIsDurableWithItsIntentOverPostgres(t *testing.T) {
	e := serve(t)
	e.ready(t)

	var created servicev1.CreateExternalCommandResponse
	e.command(t, "CreateExternalCommand", create("idem-1", 1), &created)
	if created.GetCreated().GetStatus() != servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_CREATED {
		t.Fatalf("CreateExternalCommand() = %v", &created)
	}
	if got := e.destinations(t); len(got) != 1 || got[0] != application.DispatchDestination {
		t.Fatalf("outbox destinations = %v, want only the dispatch intent", got)
	}

	var refused servicev1.CreateExternalCommandResponse
	e.command(t, "CreateExternalCommand", create("idem-2", 2), &refused)
	if refused.GetRejection().GetCode() != string(domain.CodeExternalCommandContentChangedAfterSend) {
		t.Fatalf("a second version of a live series = %v, want content-changed-after-send", &refused)
	}

	e.command(t, "CancelExternalCommand", &servicev1.CancelExternalCommandRequest{IdempotencyKey: "idem-1"}, &servicev1.CancelExternalCommandResponse{})
	var second servicev1.CreateExternalCommandResponse
	e.command(t, "CreateExternalCommand", create("idem-2", 2), &second)
	if second.GetCreated() == nil {
		t.Fatalf("the version after a cancellation = %v, want created", &second)
	}
}

// The human resolution runs the kernel's Resolve inside the unit of work of the
// command, so the inbox entry, the observation and the step commit together.
func TestAHumanResolutionCommitsWithTheObservationOverPostgres(t *testing.T) {
	e := serve(t)
	e.ready(t)
	e.command(t, "CreateExternalCommand", create("idem-1", 1), &servicev1.CreateExternalCommandResponse{})
	var found servicev1.FindExternalCommandResponse
	e.command(t, "FindExternalCommand", &servicev1.FindExternalCommandRequest{IdempotencyKey: "idem-1"}, &found)
	command := found.GetCommand()
	if command.GetStatus() != servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_CREATED {
		t.Fatalf("FindExternalCommand() = %v", command)
	}
	outbound := commandOutboundKey(t, e.pool, "idem-1")
	ledger := postgres.NewAttemptStore(e.pool)
	if _, err := ledger.Begin(withExecution(t), outbound, "m-1", occurred); err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	if _, err := e.service.AbandonExternalAttempt(withExecution(t), ports.Attempt{Tenant: testTenant, Key: outbound, N: 1, MessageID: "m-1", StartedAt: occurred}); err != nil {
		t.Fatalf("AbandonExternalAttempt() = %v", err)
	}

	var resolved servicev1.ResolveUnknownOutcomeResponse
	e.command(t, "ResolveUnknownOutcome", &servicev1.ResolveUnknownOutcomeRequest{
		IdempotencyKey: "idem-1", Resolution: servicev1.Resolution_RESOLUTION_NOT_RECEIVED, QueryInteractionId: "int-9", Justification: "registry has no record",
	}, &resolved)

	if resolved.GetResolved().GetStatus() != servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_RETRYABLE_FAILURE {
		t.Fatalf("ResolveUnknownOutcome() = %v, want a retryable failure", &resolved)
	}
	var observations int
	if err := e.pool.QueryRow(context.Background(), "SELECT queries FROM external_attempt WHERE outbound_key = $1", string(outbound)).Scan(&observations); err != nil || observations != 1 {
		t.Fatalf("ledger observations = %d, %v, want the resolution recorded once", observations, err)
	}
}
