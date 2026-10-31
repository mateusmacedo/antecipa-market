//go:build integration

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
)

// serveAPI starts the api role through RunWith, the entry the process takes
// after the telemetry, and returns once it reports SERVING.
func serveAPI(t *testing.T) *grpc.ClientConn {
	t.Helper()
	appkit.OpenPool(t)
	dsn, err := withSessionTimeouts(pg.DSN(t, appkit.PoolOptions.Project))
	if err != nil {
		t.Fatalf("withSessionTimeouts() = %v", err)
	}
	cfg := Defaults(RoleAPI)
	cfg.DSN = dsn
	cfg.API.GRPCAddr, cfg.API.GRPCInsecure = obstest.UnusedLoopbackAddr(t), true
	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Disabled:   true,
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "gateway", ServiceVersion: "dev", ServiceInstanceID: "gateway-api-1", Role: string(RoleAPI)},
	})
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	ctx, cancel := context.WithCancel(context.Background())
	var served error
	done := make(chan struct{})
	go func() {
		defer close(done)
		served = RunWith(ctx, cfg, runtime)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		if served != nil && !errors.Is(served, context.Canceled) {
			t.Errorf("RunWith = %v, want a clean stop", served)
		}
	})

	conn, err := grpc.NewClient(cfg.API.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	health := healthpb.NewHealthClient(conn)
	for deadline := time.Now().Add(15 * time.Second); ; {
		probe, stop := context.WithTimeout(context.Background(), time.Second)
		resp, err := health.Check(probe, &healthpb.HealthCheckRequest{Service: rpc.ServiceName})
		stop()
		if err == nil && resp.GetStatus() == healthpb.HealthCheckResponse_SERVING {
			return conn
		}
		select {
		case <-done:
			t.Fatalf("RunWith = %v before the service reported SERVING", served)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the service reported no SERVING within 15s: last %v, %v", resp, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func asTenant(t *testing.T, metadataPairs ...string) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return metadata.AppendToOutgoingContext(ctx, append([]string{kernelgrpc.TenantKey, "acme"}, metadataPairs...)...)
}

func TestTheAPIRoleServesTheQueriesWithoutAnIdempotencyKey(t *testing.T) {
	conn := serveAPI(t)
	var registered servicev1.RegisterConnectorResponse
	err := conn.Invoke(asTenant(t, kernelgrpc.IdempotencyKey, "wiring-k-1"), rpc.FullMethod("RegisterConnector"), &servicev1.RegisterConnectorRequest{
		PartnerId: "wiring-prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, CredentialRef: "vault://wiring-prt-1", IntegrationVersion: 1,
	}, &registered)
	if err != nil || registered.GetRegistered().GetConnectorKey() != "wiring-prt-1:production" {
		t.Fatalf("setup: RegisterConnector() = %v, %v; want Registered wiring-prt-1:production", &registered, err)
	}

	var found servicev1.FindConnectorResponse
	err = conn.Invoke(asTenant(t), rpc.FullMethod("FindConnector"), &servicev1.FindConnectorRequest{ConnectorKey: "wiring-prt-1:production"}, &found)
	if err != nil || found.GetConnector().GetPartnerId() != "wiring-prt-1" || found.GetConnector().GetIntegrationVersion() != 1 {
		t.Fatalf("FindConnector() = %v, %v; want the connector just registered", &found, err)
	}
	var effective servicev1.FindEffectiveCapabilityProfileResponse
	err = conn.Invoke(asTenant(t), rpc.FullMethod("FindEffectiveCapabilityProfile"), &servicev1.FindEffectiveCapabilityProfileRequest{
		ConnectorId: "wiring-prt-1:production", EffectiveAt: 1,
	}, &effective)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindEffectiveCapabilityProfile() = %v, %v; want NotFound for a connector that declared no profile", &effective, err)
	}
}

func TestTheAPIRoleRefusesEveryCommandWithoutAnIdempotencyKey(t *testing.T) {
	conn := serveAPI(t)

	for _, method := range rpc.Commands() {
		t.Run(method, func(t *testing.T) {
			// The interceptor answers before the handler reads the request,
			// so an empty message stands for the request of any command.
			err := conn.Invoke(asTenant(t), rpc.FullMethod(method), &emptypb.Empty{}, &emptypb.Empty{})

			if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
				t.Fatalf("%s without a key = %v, want InvalidArgument with %s", method, err, kernelgrpc.ReasonMissingIdempotencyKey)
			}
		})
	}
}
