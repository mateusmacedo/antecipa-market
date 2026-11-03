//go:build integration

package app

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
)

// runtimePrivileges is what the runtime role holds on each table of the public
// schema, so a privilege nobody declared shows up as well as a missing one.
func runtimePrivileges(t *testing.T, admin *pgxpool.Pool) map[string][]string {
	t.Helper()
	rows, err := admin.Query(context.Background(), `
SELECT c.relname, p FROM pg_class c
CROSS JOIN unnest(ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER']) AS p
WHERE c.relnamespace = 'public'::regnamespace AND c.relkind IN ('r', 'p') AND has_table_privilege($1, c.oid, p)
ORDER BY c.relname, p`, pg.RuntimeRole)
	if err != nil {
		t.Fatalf("has_table_privilege() = %v", err)
	}
	held := map[string][]string{}
	type privilege struct{ Table, Name string }
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByPos[privilege])
	if err != nil {
		t.Fatalf("collect privileges: %v", err)
	}
	for _, p := range collected {
		held[p.Table] = append(held[p.Table], p.Name)
	}
	return held
}

// declaredPrivileges is the set the consumer waits for: the kernel's grants of
// the outbox, the inboxes and the attempt ledger, and provider.Grants.
func declaredPrivileges() map[string][]string {
	declared := map[string][]string{}
	for _, g := range consumerGrants() {
		for _, p := range g.Privileges {
			if !slices.Contains(declared[g.Table], string(p)) {
				declared[g.Table] = append(declared[g.Table], string(p))
			}
		}
	}
	for table := range declared {
		slices.Sort(declared[table])
	}
	return declared
}

func ownedBy(t *testing.T, admin *pgxpool.Pool, role string) []string {
	t.Helper()
	rows, err := admin.Query(context.Background(),
		`SELECT relname FROM pg_class WHERE relnamespace = 'public'::regnamespace AND relowner = to_regrole($1) ORDER BY relname`, role)
	if err != nil {
		t.Fatalf("read owners: %v", err)
	}
	owned, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect owners: %v", err)
	}
	return owned
}

// runStoppable runs one role until the test stops it or it returns on its own.
func runStoppable(t *testing.T, cfg Config, runtime *otelboot.Runtime) (stopped <-chan error, stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- RunWith(ctx, cfg, runtime) }()
	return done, func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(30 * time.Second):
			t.Fatalf("RunWith(%s) did not return within 30s of the cancellation", cfg.Role)
			return nil
		}
	}
}

// ADR-063: the api migrates as gateway_migrator and grants the runtime role,
// which owns nothing, exactly what the consumer waits for, the interactions,
// the references and the attempt ledger of this slice included.
func TestTheAPIWithAMigratorGrantsTheRuntimeRoleWhatTheConsumerWaitsFor(t *testing.T) {
	migrator := pg.MigratorDSN(t, appkit.PoolOptions.Project)
	admin := pg.OpenPool(t, pg.Options{Project: appkit.PoolOptions.Project})
	cfg := Defaults(RoleAPI)
	dsn, err := withSessionTimeouts(pg.RuntimeDSN(t, appkit.PoolOptions.Project))
	if err != nil {
		t.Fatalf("withSessionTimeouts() = %v", err)
	}
	cfg.DSN, cfg.MigrateDSN, cfg.Migrate = dsn, migrator, true
	cfg.API.GRPCAddr, cfg.API.GRPCInsecure = obstest.UnusedLoopbackAddr(t), true
	cfg.RawStore = testRawStoreEnv(t, createBucket(t, lockedBucket))
	runtime := disabledRuntime(t, RoleAPI)
	stopped, stop := runStoppable(t, cfg, runtime)

	want := declaredPrivileges()
	for _, table := range []string{"external_interactions", "external_references", "external_attempt"} {
		if _, declared := want[table]; !declared {
			t.Fatalf("declared grants %v leave %s out, want every table the consumer writes", slices.Sorted(maps.Keys(want)), table)
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for got := runtimePrivileges(t, admin); !maps.EqualFunc(got, want, slices.Equal); got = runtimePrivileges(t, admin) {
		select {
		case err := <-stopped:
			t.Fatalf("RunWith(api) = %v while the runtime role held %v, want %v", err, got, want)
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("privileges of the runtime role = %v after 30s, want %v", got, want)
		}
	}
	if owned := ownedBy(t, admin, pg.RuntimeRole); len(owned) != 0 {
		t.Fatalf("the runtime role owns %v, want nothing", owned)
	}

	if err := stop(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(api) = %v, want nil or the cancellation", err)
	}
}

func loggedRuntime(t *testing.T, logs *obstest.MemoryLogs) (*sdklog.LoggerProvider, *otelboot.Runtime) {
	t.Helper()
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "gateway", ServiceVersion: "dev", ServiceInstanceID: "gateway-consumer-1", Role: string(RoleConsumer)},
		TraceExporter: tracetest.NewInMemoryExporter(),
	}
	provider := otelboot.NewLoggerProvider(config, logs)
	config.LoggerProvider = provider
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return provider, runtime
}

func disabledRuntime(t *testing.T, role Role) *otelboot.Runtime {
	t.Helper()
	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Disabled: true, Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: "gateway", ServiceVersion: "dev", ServiceInstanceID: "gateway-" + string(role) + "-1", Role: string(role)},
	})
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return runtime
}

// awaitRecord waits for a log record of the role; the attributes of the latest
// one come back.
func awaitRecord(t *testing.T, provider *sdklog.LoggerProvider, logs *obstest.MemoryLogs, body string, stopped <-chan error, timeout time.Duration) map[string]string {
	t.Helper()
	for deadline := time.Now().Add(timeout); ; time.Sleep(50 * time.Millisecond) {
		if err := provider.ForceFlush(context.Background()); err != nil {
			t.Fatalf("ForceFlush() = %v", err)
		}
		if attributes, found := obstest.RecordAttributes(logs, body); found {
			return attributes
		}
		select {
		case err := <-stopped:
			t.Fatalf("RunWith() = %v before %q", err, body)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %q record within %v", body, timeout)
		}
	}
}

// grantExcept grants the runtime role every privilege the consumer waits for
// but those of one capability; ApplyGrants refuses a partial set by design.
func grantExcept(t *testing.T, admin *pgxpool.Pool, withheld postgres.Capability) {
	t.Helper()
	skipped := map[string]bool{}
	for _, g := range postgres.KernelGrants(withheld) {
		skipped[g.Table] = true
	}
	for _, g := range consumerGrants() {
		if skipped[g.Table] {
			continue
		}
		privileges := make([]string, len(g.Privileges))
		for i, p := range g.Privileges {
			privileges[i] = string(p)
		}
		statement := "GRANT " + strings.Join(privileges, ", ") + " ON " + pgx.Identifier{g.Table}.Sanitize() + " TO " + pgx.Identifier{pg.RuntimeRole}.Sanitize()
		if _, err := admin.Exec(context.Background(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

// The dispatch begins its attempts in the kernel's ledger, so a runtime role
// holding every other grant still keeps the consumer from joining any channel.
func TestTheConsumerWaitsForTheGrantOfTheAttemptLedgerBeforeJoining(t *testing.T) {
	admin := appkit.OpenPool(t)
	cfg := Defaults(RoleConsumer)
	roleChannels(t, &cfg)
	cfg.DSN = pg.RuntimeDSN(t, appkit.PoolOptions.Project)
	cfg.RawStore = testRawStoreEnv(t, createBucket(t, lockedBucket))
	grantExcept(t, admin, postgres.ExternalAttempt)
	logs := &obstest.MemoryLogs{}
	logger, runtime := loggedRuntime(t, logs)
	stopped, stop := runStoppable(t, cfg, runtime)

	waiting := awaitRecord(t, logger, logs, "waiting for table privilege", stopped, 30*time.Second)
	if waiting["db.collection.name"] != "external_attempt" {
		t.Fatalf("waiting for %v, want the attempt ledger", waiting)
	}
	if _, joined := obstest.RecordAttributes(logs, "consumer joining"); joined {
		t.Fatal("the consumer joined before the runtime role held the attempt ledger")
	}
	pg.GrantRuntime(t, admin, appkit.PoolOptions)
	awaitRecord(t, logger, logs, "consumer joining", stopped, 30*time.Second)

	if err := stop(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("RunWith(consumer) = %v, want nil or the cancellation", err)
	}
}
