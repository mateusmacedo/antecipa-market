//go:build integration

package app

import (
	"context"
	"errors"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

type funders struct{}

func (funders) FindOrganization(context.Context, port.TenantID, domain.OrganizationID) ([]domain.OrganizationKind, error) {
	return []domain.OrganizationKind{domain.KindFunder}, nil
}

func serveAPI(t *testing.T, serve func(context.Context, Config, *otelboot.Runtime) error) *grpc.ClientConn {
	t.Helper()
	appkit.OpenPool(t)
	cfg := Defaults(RoleAPI)
	cfg.DSN = pg.DSN(t, appkit.PoolOptions.Project)
	cfg.API.GRPCAddr, cfg.API.GRPCInsecure = obstest.UnusedLoopbackAddr(t), true
	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Disabled:   true,
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "partner", ServiceVersion: "dev", ServiceInstanceID: "partner-api-1", Role: string(RoleAPI)},
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
		served = serve(ctx, cfg, runtime)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		if served != nil && !errors.Is(served, context.Canceled) {
			t.Errorf("serve = %v, want a clean stop", served)
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
			t.Fatalf("serve = %v before the service reported SERVING", served)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the service reported no SERVING within 15s: last %v, %v", resp, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func registerOver(t *testing.T, conn *grpc.ClientConn) (*servicev1.RegisterPartnerResponse, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, "acme", kernelgrpc.IdempotencyKey, "wiring-k-1")
	var resp servicev1.RegisterPartnerResponse
	err := conn.Invoke(ctx, rpc.FullMethod("RegisterPartner"), &servicev1.RegisterPartnerRequest{
		PartnerId: "wiring-prt-1", OrganizationId: "org-1", Role: servicev1.PartnerRole_PARTNER_ROLE_FUNDER,
	}, &resp)
	return &resp, err
}

func TestTheAPIRoleRefusesToRegisterAPartnerWithTheDirectoryOfProduction(t *testing.T) {
	conn := serveAPI(t, RunWith)

	resp, err := registerOver(t, conn)

	if status.Code(err) != codes.Internal {
		t.Fatalf("RegisterPartner() = %v (%v), want the technical failure of the directory that fails closed", err, resp)
	}
}

func TestTheAPIRoleRegistersAPartnerThroughTheDirectoryItIsHanded(t *testing.T) {
	conn := serveAPI(t, func(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
		var directory ports.OrganizationDirectory = funders{}
		return runAPI(ctx, cfg, rt, directory)
	})

	resp, err := registerOver(t, conn)

	if err != nil || resp.GetRegistered().GetPartnerId() != "wiring-prt-1" {
		t.Fatalf("RegisterPartner() = %v, %v; want Registered wiring-prt-1", resp, err)
	}
}

func TestTheAPIRoleServesTheQueriesWithoutAnIdempotencyKey(t *testing.T) {
	conn := serveAPI(t, func(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
		return runAPI(ctx, cfg, rt, funders{})
	})
	if _, err := registerOver(t, conn); err != nil {
		t.Fatalf("setup: RegisterPartner() = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, "acme")

	var found servicev1.FindPartnerResponse
	err := conn.Invoke(ctx, rpc.FullMethod("FindPartner"), &servicev1.FindPartnerRequest{PartnerId: "wiring-prt-1"}, &found)
	if err != nil || found.GetPartner().GetStatus() != servicev1.PartnerStatus_PARTNER_STATUS_PROSPECT {
		t.Fatalf("FindPartner() = %v, %v; want the prospect just registered", &found, err)
	}
	var effective servicev1.FindEffectiveCapabilityResponse
	err = conn.Invoke(ctx, rpc.FullMethod("FindEffectiveCapability"), &servicev1.FindEffectiveCapabilityRequest{
		PartnerId: "wiring-prt-1", Operation: "register-receivable", AssetType: "duplicata",
		Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, TenantId: "acme", AsOf: 1,
	}, &effective)
	if err != nil || effective.GetAllowed() ||
		effective.GetFailedCondition() != servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_CAPABILITY_NOT_FOUND {
		t.Fatalf("FindEffectiveCapability() = %v, %v; want denied with capability_not_found", &effective, err)
	}
}
