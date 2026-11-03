//go:build integration

package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	gatewayports "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

const (
	cycleConnector = domain.ConnectorKey("prt-cycle:production")
	cycleCommand   = domain.IdempotencyKey("idem-cycle-1")
)

// cycle is the three roles of this context in process over Postgres, Redpanda
// and the S3 of floci, every one under the runtime role and the api migrating
// as gateway_migrator; the partner simulator stands for the adapters of the
// api and the consumer, writing its raw payloads to the same bucket.
type cycle struct {
	admin   *pgxpool.Pool
	runtime *otelboot.Runtime
	base    Config
	raw     gatewayports.RawStore
	sim     *partnersim.Simulator
	conn    *grpc.ClientConn
}

var cycleKeys atomic.Int64

// newCycle serves the api role with the simulator and waits until it reports
// SERVING; the relay and the consumer start when the test asks for them.
func newCycle(t *testing.T) *cycle {
	t.Helper()
	migrator := pg.MigratorDSN(t, appkit.PoolOptions.Project)
	c := &cycle{admin: pg.OpenPool(t, pg.Options{Project: appkit.PoolOptions.Project}), runtime: disabledRuntime(t, RoleConsumer)}
	c.base = Defaults(RoleRelay)
	roleChannels(t, &c.base)
	c.base.DSN = pg.RuntimeDSN(t, appkit.PoolOptions.Project)
	c.base.RawStore = testRawStoreEnv(t, createBucket(t, lockedBucket))
	raw, err := OpenRawStore(context.Background(), c.base.RawStore, c.runtime)
	if err != nil {
		t.Fatalf("OpenRawStore() = %v", err)
	}
	c.raw = raw
	if c.sim, err = partnersim.New(idclock.SystemClock{}, idclock.NewMessageIDs("sim"), raw); err != nil {
		t.Fatalf("partnersim.New() = %v", err)
	}

	api := c.base
	api.Role, api.MigrateDSN, api.Migrate, api.partners = RoleAPI, migrator, true, c.sim
	if api.DSN, err = withSessionTimeouts(c.base.DSN); err != nil {
		t.Fatalf("withSessionTimeouts() = %v", err)
	}
	api.API.GRPCAddr, api.API.GRPCInsecure = obstest.UnusedLoopbackAddr(t), true
	if err := api.Validate(); err != nil {
		t.Fatalf("Validate(api) = %v", err)
	}
	runRole(t, api, c.runtime)
	c.conn = dialServing(t, api.API.GRPCAddr)
	return c
}

func (c *cycle) start(t *testing.T, role Role) {
	t.Helper()
	cfg := c.base
	cfg.Role = role
	if role == RoleConsumer {
		cfg.partners = c.sim
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(%s) = %v", role, err)
	}
	runRole(t, cfg, c.runtime)
}

func dialServing(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("NewClient() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	health := healthpb.NewHealthClient(conn)
	eventually(t, "the api reporting SERVING", func() bool {
		probe, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		resp, err := health.Check(probe, &healthpb.HealthCheckRequest{Service: rpc.ServiceName})
		return err == nil && resp.GetStatus() == healthpb.HealthCheckResponse_SERVING
	})
	return conn
}

func (c *cycle) command(t *testing.T, method string, req, resp proto.Message) {
	t.Helper()
	ctx := asTenant(t, kernelgrpc.IdempotencyKey, fmt.Sprintf("cycle-%d", cycleKeys.Add(1)))
	if err := c.conn.Invoke(ctx, rpc.FullMethod(method), req, resp); err != nil {
		t.Fatalf("%s() = %v", method, err)
	}
}

// ready is the connector of the cycle: submitEffect with a state query due
// one second after the wait opens, queryAsset for the external read, and the
// authorization of query of org-cycle.
func (c *cycle) ready(t *testing.T) {
	t.Helper()
	c.command(t, "RegisterConnector", &servicev1.RegisterConnectorRequest{
		PartnerId: "prt-cycle", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, CredentialRef: "vault://prt-cycle", IntegrationVersion: 1,
	}, &servicev1.RegisterConnectorResponse{})
	for i, operation := range []string{"submitEffect", "queryAsset"} {
		c.command(t, "EnableOperation", &servicev1.EnableOperationRequest{
			ConnectorKey: string(cycleConnector), CapabilityId: fmt.Sprintf("cap-cycle-%d", i), Operation: operation,
			StateQuerySupported: operation == "submitEffect", Version: 1,
		}, &servicev1.EnableOperationResponse{})
	}
	submit := &servicev1.OperationCapability{
		Operation: "submitEffect", Supported: true, Protocol: servicev1.Protocol_PROTOCOL_SYNC_HTTP, StateQuerySupported: true,
		IdempotencySupport: servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_KEY_HEADER, AssetTypes: []string{"duplicata"},
		Efficacy: servicev1.Efficacy_EFFICACY_ON_SUBMIT, Quiescence: int64(time.Second), MaxQueries: 3,
		KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32, QueryInterval: 1, AmbiguityDeadline: 3600, MaxSubmissions: 3,
		MaxPayloadBytes: 1 << 12, EncumbranceCoverage: servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_UNAVAILABLE,
	}
	read := proto.Clone(submit).(*servicev1.OperationCapability)
	read.Operation, read.StateQuerySupported, read.QueryInterval = "queryAsset", false, 0
	var declared servicev1.DeclareCapabilityProfileResponse
	c.command(t, "DeclareCapabilityProfile", &servicev1.DeclareCapabilityProfileRequest{
		ConnectorId: string(cycleConnector), Version: 1, EffectiveFrom: 1, Operations: []*servicev1.OperationCapability{submit, read},
	}, &declared)
	if declared.GetDeclared() == nil {
		t.Fatalf("DeclareCapabilityProfile() = %v", &declared)
	}
	c.command(t, "GrantQueryAuthorization", &servicev1.GrantQueryAuthorizationRequest{
		ConsentId: "cns-cycle", OrganizationId: "org-cycle", ConsentVersion: 1, AssetTypes: []string{"duplicata"},
		PeriodFromDate: "2026-01-01", PeriodUntilDate: "2099-12-31",
	}, &servicev1.GrantQueryAuthorizationResponse{})
}

// create asks for the command the directive scripts on the simulator.
func (c *cycle) create(t *testing.T, directive partnersim.Directive) {
	t.Helper()
	var created servicev1.CreateExternalCommandResponse
	c.command(t, "CreateExternalCommand", &servicev1.CreateExternalCommandRequest{
		IdempotencyKey: string(cycleCommand), TenantId: "tnt-cycle", ConnectorId: string(cycleConnector), Operation: "submitEffect",
		AssetType: "duplicata", BusinessKey: "eff-cycle-1", PayloadVersion: 1, Payload: string(partnersim.Payload(directive)), PayloadHash: "h-cycle-1",
	}, &created)
	if created.GetCreated() == nil {
		t.Fatalf("CreateExternalCommand() = %v", &created)
	}
}

func (c *cycle) loaded(t *testing.T) domain.ExternalCommandSnapshot {
	t.Helper()
	command, _, err := provider.NewExternalCommandReader(postgres.NewReadPool(c.admin)).Load(seeding(t, "cycle-read"), cycleCommand)
	if err != nil {
		t.Fatalf("load %s: %v", cycleCommand, err)
	}
	return command
}

// interaction is the one interaction of the command from source, and whether
// it is there yet.
func (c *cycle) interaction(t *testing.T, source domain.InteractionSource) (domain.ExternalInteractionSnapshot, bool) {
	t.Helper()
	interactions, err := provider.NewExternalInteractionsReader(postgres.NewReadPool(c.admin)).LoadByCommand(seeding(t, "cycle-interactions"), cycleCommand)
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("interactions of %s: %v", cycleCommand, err)
	}
	var found []domain.ExternalInteractionSnapshot
	for _, i := range interactions {
		if i.Source == source {
			found = append(found, i)
		}
	}
	if len(found) > 1 {
		t.Fatalf("interactions %s of the command = %+v, want one", source, found)
	}
	if len(found) == 0 {
		return domain.ExternalInteractionSnapshot{}, false
	}
	return found[0], true
}

// sweepUntilQueried runs the sweep of the external scheduler through the api
// until it makes the state query whose time came.
func (c *cycle) sweepUntilQueried(t *testing.T) {
	t.Helper()
	eventually(t, "the state query of the sweep", func() bool {
		var swept servicev1.SweepExternalCommandsResponse
		c.command(t, "SweepExternalCommands", &servicev1.SweepExternalCommandsRequest{}, &swept)
		return swept.GetStateQueried() == 1
	})
}

// The scenarios of the timeout after the write, of the state query of the
// sweep and of ReadExternal, through the roles: the submission lands on the
// partner while its answer is a timeout, the capture is recorded and
// normalized, the sweep queries the state and its normalization confirms the
// command; the api answers an external read from the same simulator.
func TestACommandRunsTheWholeCycleThroughTheRolesWithTheSimulator(t *testing.T) {
	c := newCycle(t)
	c.ready(t)
	c.start(t, RoleConsumer)
	c.start(t, RoleRelay)

	c.create(t, partnersim.Directive{
		Replies: []partnersim.Reply{{Status: 504, Applied: true}},
		Queries: []partnersim.Answer{{FromEffect: true, ExternalRef: "ext-cycle-1"}},
	})

	eventually(t, "the dispatch through the simulator", func() bool { return c.loaded(t).Status == domain.StatusUnknownOutcome })
	if command := c.loaded(t); command.FailureCategory != domain.CategoryUnknownSent || command.Submissions != 1 {
		t.Fatalf("command = %+v, want unknown_outcome presumed sent after one submission", command)
	}
	eventually(t, "the normalization of the capture", func() bool {
		captured, recorded := c.interaction(t, domain.InteractionSubmitResponse)
		return recorded && captured.Status != domain.InteractionStored
	})
	captured, _ := c.interaction(t, domain.InteractionSubmitResponse)
	if captured.Request.VersionID == "" || captured.Response.VersionID == "" || captured.Status != domain.InteractionNormalized {
		t.Fatalf("capture = %+v, want both raw sides versioned in the bucket and normalized", captured)
	}

	c.sweepUntilQueried(t)
	eventually(t, "the confirmation by the normalization of the state query", func() bool { return c.loaded(t).Status == domain.StatusConfirmed })
	queried, recorded := c.interaction(t, domain.InteractionStateQuery)
	if !recorded || queried.Status != domain.InteractionNormalized || queried.Finding != domain.FindingApplied {
		t.Fatalf("state query = %+v, want normalized with the effect applied", queried)
	}
	if submitted, asked := c.sim.Partner().Submitted(), c.sim.Partner().Queried(); len(submitted) != 1 || len(asked) != 1 {
		t.Fatalf("the partner saw %d submissions and %d queries, want one of each", len(submitted), len(asked))
	}

	var read servicev1.ReadExternalResponse
	c.command(t, "ReadExternal", &servicev1.ReadExternalRequest{
		Operation: "queryAsset", ConnectorKey: string(cycleConnector), OrganizationId: "org-cycle", AssetType: "duplicata",
		Parameters: string(partnersim.Parameters(partnersim.Read{ExternalRef: "ast-cycle-1", Reading: []byte(`{"position":100}`)})),
	}, &read)
	if r := read.GetReading(); r.GetExternalReference() != "ast-cycle-1" || !strings.Contains(r.GetReading(), `"position":100`) || r.GetProviderId() != "prt-cycle" {
		t.Fatalf("ReadExternal() = %v, want the reading of the simulator with its provenance", &read)
	}
}

// dyingPartners is the simulator behind a process that dies with its partner
// call in flight: the partner applies the effect and the answer never leaves
// the call until the test released it, after the process is gone.
type dyingPartners struct {
	*partnersim.Simulator
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (d *dyingPartners) Command(connector domain.ConnectorKey, operation string, collector *gatewayports.CaptureCollector) (port.ExternalCommand, error) {
	command, err := d.Simulator.Command(connector, operation, collector)
	if err != nil {
		return nil, err
	}
	return dyingCommand{inner: command, partners: d}, nil
}

type dyingCommand struct {
	inner    port.ExternalCommand
	partners *dyingPartners
}

func (c dyingCommand) Submit(ctx context.Context, key port.OutboundKey, payload []byte) (port.ExternalClass, port.Observation, error) {
	class, observation, err := c.inner.Submit(ctx, key, payload)
	c.partners.once.Do(func() { close(c.partners.entered) })
	<-c.partners.release
	return class, observation, err
}

// dispatcher is the service of a consumer process over its own pool, under the
// clock it runs by.
func (c *cycle) dispatcher(t *testing.T, pool *pgxpool.Pool, clock port.Clock, partners gatewayports.PartnerAdapters) application.Service {
	t.Helper()
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	budget, err := DispatchBudget(PartnerRoute())
	if err != nil {
		t.Fatalf("DispatchBudget() = %v", err)
	}
	service := NewService(pool, clock, idclock.NewMessageIDs("dispatcher"), Waits{Message: time.Second, Command: time.Second})
	service.Idempotency, service.Raw, service.Partners = policy, c.raw, partners
	service.Attempts, service.Budget = postgres.NewAttemptStore(pool), DispatchBudgetOf(budget)
	return service
}

func runtimePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.NewWithConfig(context.Background(), pg.RuntimeConfig(t, appkit.PoolOptions.Project))
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig() = %v", err)
	}
	return pool
}

// intent is the dispatch intent of the command as the consumer receives it.
func (c *cycle) intent(t *testing.T) application.DispatchExternalCommand {
	t.Helper()
	var id string
	if err := c.admin.QueryRow(context.Background(), `SELECT message_id FROM outbox WHERE destination = $1`, application.DispatchDestination).Scan(&id); err != nil {
		t.Fatalf("read the dispatch intent: %v", err)
	}
	return application.DispatchExternalCommand{
		Receipt: port.Receipt{
			MessageID: port.MessageID(id), MessageType: dispatchRequestedEvent, PayloadHash: "h-intent", ReceivedAt: port.Instant(time.Now().UnixNano()),
		},
		IdempotencyKey: cycleCommand,
	}
}

type attemptRow struct {
	state string
	class port.ExternalClass
}

func (c *cycle) attempts(t *testing.T, key string) []attemptRow {
	t.Helper()
	rows, err := c.admin.Query(context.Background(), `SELECT state, coalesce(class, 0) FROM external_attempt WHERE outbound_key = $1 ORDER BY attempt`, key)
	if err != nil {
		t.Fatalf("read the attempts: %v", err)
	}
	defer rows.Close()
	var out []attemptRow
	for rows.Next() {
		var a attemptRow
		var class int16
		if err := rows.Scan(&a.state, &class); err != nil {
			t.Fatalf("scan attempt: %v", err)
		}
		a.class = port.ExternalClass(class)
		out = append(out, a)
	}
	return out
}

// RES-43 and the scenario of the attempt left begun by a crash, through the
// roles; the gateway's side of the kernel's CrashBetweenBeginAndFinishVector,
// which distkit runs over this database: a consumer dies between Begin and Finish
// after the partner applied the effect; the sweep of attempts of the consumer
// role closes the orphan as unknown_sent, the redelivery of the intent sends
// nothing, and only the state query of the sweep confirms the command.
func TestACrashBetweenBeginAndFinishIsSweptAsUnknownSentAndResolvedOnlyByTheQuery(t *testing.T) {
	c := newCycle(t)
	c.ready(t)
	c.create(t, partnersim.Directive{
		Replies: []partnersim.Reply{{Status: 202, ExternalRef: "ext-crash-1"}},
		Queries: []partnersim.Answer{{FromEffect: true, ExternalRef: "ext-crash-1"}},
	})
	outbound := c.loaded(t).OutboundKey

	dying := &dyingPartners{Simulator: c.sim, entered: make(chan struct{}), release: make(chan struct{})}
	dyingPool := runtimePool(t)
	stale := memory.FixedClock{At: port.Instant(time.Now().Add(-time.Hour).UnixNano())}
	dispatcher, intent, ctx := c.dispatcher(t, dyingPool, stale, dying), c.intent(t), seeding(t, "cycle-dying")
	died := make(chan error, 1)
	go func() {
		_, err := dispatcher.DispatchExternalCommand(ctx, intent)
		died <- err
	}()
	select {
	case <-dying.entered:
	case err := <-died:
		t.Fatalf("the dispatch returned %v before calling the partner", err)
	case <-time.After(30 * time.Second):
		t.Fatal("the dispatch never called the partner")
	}
	dyingPool.Close()
	close(dying.release)
	if err := <-died; err == nil {
		t.Fatal("the dying dispatch settled its outcome, want the process gone before Finish")
	}
	if rows := c.attempts(t, outbound); len(rows) != 1 || rows[0].state != "begun" {
		t.Fatalf("attempts after the crash = %+v, want one begun", rows)
	}

	c.start(t, RoleConsumer)
	eventually(t, "the sweep of the attempt the crash left begun", func() bool {
		rows := c.attempts(t, outbound)
		return len(rows) == 1 && rows[0].state == "finished"
	})
	if rows := c.attempts(t, outbound); rows[0].class != port.ClassUnknownSent {
		t.Fatalf("attempts = %+v, want the orphan finished as UnknownSent", rows)
	}
	if command := c.loaded(t); command.Status != domain.StatusUnknownOutcome || command.FailureCategory != domain.CategoryUnknownSent || command.Submissions != 1 {
		t.Fatalf("command = %+v, want unknown_outcome presumed sent once", command)
	}

	redelivered, err := c.dispatcher(t, runtimePoolOf(t), idclock.SystemClock{}, c.sim).DispatchExternalCommand(seeding(t, "cycle-redelivery"), c.intent(t))
	if err != nil || redelivered != usecase.R1D2 {
		t.Fatalf("redelivery = %v, %v, want acknowledged with no send", redelivered, err)
	}
	if submitted := c.sim.Partner().Submitted(); len(submitted) != 1 {
		t.Fatalf("the partner saw %d submissions, want only the one the crash interrupted", len(submitted))
	}

	c.start(t, RoleRelay)
	c.sweepUntilQueried(t)
	eventually(t, "the confirmation by the state query", func() bool { return c.loaded(t).Status == domain.StatusConfirmed })
	if submitted, asked := c.sim.Partner().Submitted(), c.sim.Partner().Queried(); len(submitted) != 1 || len(asked) != 1 {
		t.Fatalf("the partner saw %d submissions and %d queries, want the interrupted call and one query", len(submitted), len(asked))
	}
}

func runtimePoolOf(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := runtimePool(t)
	t.Cleanup(pool.Close)
	return pool
}
