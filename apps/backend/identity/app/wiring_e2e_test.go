//go:build integration

package app_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func serveAPI(t *testing.T, cfg app.Config) *grpc.ClientConn {
	t.Helper()
	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Disabled:   true,
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "identity", ServiceVersion: "dev", ServiceInstanceID: "identity-api-1", Role: string(app.RoleAPI)},
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
		served = app.RunWith(ctx, cfg, runtime)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		if served != nil && !errors.Is(served, context.Canceled) {
			t.Errorf("RunWith() = %v, want a clean stop", served)
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
			t.Fatalf("RunWith() = %v before the service reported SERVING", served)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the service reported no SERVING within 15s: last %v, %v", resp, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func servedOver(t *testing.T, dsn string, concurrency int) *grpc.ClientConn {
	t.Helper()
	cfg := apiConfig(testIdentifierKey)
	cfg.DSN = dsn
	cfg.API.GRPCAddr, cfg.API.GRPCInsecure = obstest.UnusedLoopbackAddr(t), true
	cfg.Admission.Concurrency, cfg.Admission.Burst = concurrency, max(cfg.Admission.Burst, concurrency)
	return serveAPI(t, cfg)
}

func defaultMaxConns(t *testing.T, dsn string) int {
	t.Helper()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() = %v", err)
	}
	return int(config.MaxConns)
}

func registerOfTenantsAtOnce(conn *grpc.ClientConn, tenants []string, perTenant int) []error {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		refused []error
	)
	for n, tenant := range tenants {
		for i := range perTenant {
			wg.Go(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, tenant, kernelgrpc.IdempotencyKey, fmt.Sprintf("pool-k-%d-%d", n, i))
				req := registration(fmt.Sprintf("pool-org-%d-%03d", n, i))
				req.TaxId = fmt.Sprintf("11222333%06d", n*perTenant+i)
				var resp servicev1.RegisterOrganizationResponse
				err := conn.Invoke(ctx, rpc.FullMethod("RegisterOrganization"), req, &resp)
				if err == nil && resp.GetRegistered() == nil {
					err = fmt.Errorf("answered %v", &resp)
				}
				if err != nil {
					mu.Lock()
					defer mu.Unlock()
					refused = append(refused, fmt.Errorf("%s/%s: %w", tenant, req.GetOrganizationId(), err))
				}
			})
		}
	}
	wg.Wait()
	return refused
}

func TestTheAPIRoleServesMoreConsultativeCommandsAtOnceThanTheDefaultPoolHolds(t *testing.T) {
	appkit.OpenPool(t)
	dsn := pg.DSN(t, appkit.PoolOptions.Project)
	commands := 2 * defaultMaxConns(t, dsn)
	conn := servedOver(t, dsn, commands)

	if refused := registerOfTenantsAtOnce(conn, []string{tenantA}, commands); len(refused) != 0 {
		t.Fatalf("%d of %d registrations at once over a default pool of %d failed, want every one registered: %v",
			len(refused), commands, commands/2, refused)
	}
}

func TestTheAPIRoleServesTheConsultativeCommandsOfManyTenantsAtOnce(t *testing.T) {
	appkit.OpenPool(t)
	dsn := pg.DSN(t, appkit.PoolOptions.Project)
	perTenant := 2 * defaultMaxConns(t, dsn)
	tenants := []string{tenantA, tenantB, "initech", "umbrella"}
	conn := servedOver(t, dsn, perTenant)

	if refused := registerOfTenantsAtOnce(conn, tenants, perTenant); len(refused) != 0 {
		t.Fatalf("%d of %d registrations at once failed, want every one registered: admission bounds each (method, tenant), so %d tenants hold %d commands in flight: %v",
			len(refused), len(tenants)*perTenant, len(tenants), len(tenants)*perTenant, refused)
	}
}

func TestTheAPIRoleRegistersWhileTheTransactionHoldsTheOnlyConnectionOfItsPool(t *testing.T) {
	appkit.OpenPool(t)
	dsn, err := url.Parse(pg.DSN(t, appkit.PoolOptions.Project))
	if err != nil {
		t.Fatalf("url.Parse() = %v", err)
	}
	query := dsn.Query()
	query.Set("pool_max_conns", "1")
	dsn.RawQuery = query.Encode()
	conn := servedOver(t, dsn.String(), 4)

	if refused := registerOfTenantsAtOnce(conn, []string{tenantA}, 4); len(refused) != 0 {
		t.Fatalf("%d of 4 registrations over pool_max_conns=1 failed, want the read of note †† served apart from the connection the transaction holds: %v",
			len(refused), refused)
	}
}
