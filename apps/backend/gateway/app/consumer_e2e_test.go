//go:build integration

package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"go.opentelemetry.io/otel/propagation"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

const (
	roleTenant    = port.TenantID("acme")
	roleConnector = domain.ConnectorKey("prt-role:production")
	roleCommand   = domain.IdempotencyKey("idem-role-dispatch")
	roleOrphan    = domain.IdempotencyKey("idem-role-orphan")
)

// roleChannels gives the four channels of this context and partner.events
// topics of this test, and each channel a group of its own.
func roleChannels(t *testing.T, cfg *Config) {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	name := func(channel string) string { return "itest-gateway-" + suffix + "-" + channel }
	cfg.Brokers, cfg.KafkaInsecure, cfg.Group = strings.Split(tb.Env(t, dist.EnvBrokers), ","), true, name("partner-group")
	cfg.PartnerTopic, cfg.PartnerDLQ = name("partner"), name("partner-dlq")
	cfg.GatewayTopic, cfg.GatewayDLQ, cfg.GatewayGroup = name("events"), name("events-dlq"), name("events-group")
	cfg.DispatchTopic, cfg.DispatchDLQ, cfg.DispatchGroup = name("dispatch"), name("dispatch-dlq"), name("dispatch-group")
	cfg.InteractionTopic, cfg.InteractionDLQ, cfg.InteractionGroup = name("interactions"), name("interactions-dlq"), name("interactions-group")
	cfg.NormalizationTopic, cfg.NormalizationDLQ, cfg.NormalizationGroup = name("normalizations"), name("normalizations-dlq"), name("normalizations-group")
	dist.CreateTopics(t, cfg.Brokers, cfg.Group, cfg.PartnerTopic, cfg.PartnerDLQ, cfg.GatewayTopic, cfg.GatewayDLQ,
		cfg.DispatchTopic, cfg.DispatchDLQ, cfg.InteractionTopic, cfg.InteractionDLQ, cfg.NormalizationTopic, cfg.NormalizationDLQ)
}

// seeding is a call that leaves its messages in the outbox for the relay: the
// claim drains only rows whose metadata names the chain.
func seeding(t *testing.T, key string) context.Context {
	t.Helper()
	ctx := port.WithMessageContext(context.Background(), port.MessageContext{
		CorrelationID: "corr-role", CausationID: "caus-role", Traceparent: "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})
	return execctx.WithKey(t, ctx, key, execctx.WithoutSubject(), execctx.WithTenant(roleTenant))
}

type seeded[R any] struct {
	outcome usecase.Outcome[R]
	err     error
}

func seed[R any](outcome usecase.Outcome[R], err error) seeded[R] {
	return seeded[R]{outcome: outcome, err: err}
}

func (s seeded[R]) accepted(t *testing.T, step string) {
	t.Helper()
	if s.err != nil {
		t.Fatalf("%s() = %v", step, s.err)
	}
	if rejection, refused := s.outcome.Rejection(); refused {
		t.Fatalf("%s() refused: %v", step, rejection)
	}
}

// seedRole writes, through the use cases, one message for each channel of this
// context, and a begun attempt that no process will ever finish.
func seedRole(t *testing.T, pool *pgxpool.Pool) domain.ExternalCommandSnapshot {
	t.Helper()
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := NewService(pool, idclock.SystemClock{}, idclock.NewMessageIDs("seed"), Waits{Message: time.Second, Command: time.Second})
	service.Idempotency = policy
	seed(service.RegisterConnector(seeding(t, "role-register"), application.RegisterConnector{
		PartnerID: "prt-role", Environment: domain.EnvironmentProduction, CredentialRef: "vault://prt-role", IntegrationVersion: 1,
	})).accepted(t, "RegisterConnector")
	seed(service.EnableOperation(seeding(t, "role-enable"), application.EnableOperation{
		ConnectorKey: roleConnector, CapabilityID: "cap-role", Operation: "submitEffect", StateQuerySupported: true, Version: 1,
	})).accepted(t, "EnableOperation")
	seed(service.DeclareCapabilityProfile(seeding(t, "role-profile"), application.DeclareCapabilityProfile{
		ConnectorKey: roleConnector, Version: 1, EffectiveFrom: 1,
		Operations: []domain.OperationCapability{{
			Operation: "submitEffect", Supported: true, Protocol: domain.ProtocolSyncHTTP, StateQuerySupported: true,
			IdempotencySupport: domain.IdempotencyKeyHeader, AssetTypes: []string{"duplicata"}, EncumbranceCoverage: domain.CoverageComplete,
			Efficacy: domain.EfficacyOnSubmit, Quiescence: int64(30 * time.Second), MaxQueries: 3, KeyAlphabet: "0123456789abcdef",
			KeyMaxLength: 32, QueryInterval: 60, AmbiguityDeadline: 3600, MaxSubmissions: 3,
		}},
	})).accepted(t, "DeclareCapabilityProfile")
	for _, key := range []domain.IdempotencyKey{roleCommand, roleOrphan} {
		seed(service.CreateExternalCommand(seeding(t, string(key)), application.CreateExternalCommand{
			IdempotencyKey: key, TenantID: "tnt-role", ConnectorKey: roleConnector, Operation: "submitEffect", AssetType: "duplicata",
			BusinessKey: "eff-" + string(key), PayloadVersion: 1, Payload: `{"amount":100}`, PayloadHash: "h-" + string(key),
		})).accepted(t, "CreateExternalCommand")
	}
	orphan, _, err := service.CommandReader.Load(seeding(t, "role-read"), roleOrphan)
	if err != nil {
		t.Fatalf("load %s: %v", roleOrphan, err)
	}
	if _, err := postgres.NewAttemptStore(pool).Begin(seeding(t, "role-begin"), port.OutboundKey(orphan.OutboundKey), "m-role-orphan", 1); err != nil {
		t.Fatalf("Begin() = %v", err)
	}

	raw := domain.RawPayload{Ref: "interactions/prt-role:production/int-role/request", VersionID: "v1", Hash: "h"}
	seed(service.RecordExternalInteraction(seeding(t, "role-record-m"), application.RecordExternalInteraction{
		InteractionID: "int-role-mismatch", ConnectorKey: roleConnector, Operation: "revokeOptIn", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, ContentType: "application/json", PartnerEventID: "pe-role-m", Request: raw,
	})).accepted(t, "RecordExternalInteraction")
	seed(service.ConcludeInteraction(seeding(t, "role-conclude-m"), application.ConcludeInteraction{
		InteractionID: "int-role-mismatch", Revision: 1, Outcome: domain.InteractionSchemaMismatch,
		Mismatch: domain.Mismatch{Kind: domain.MismatchMissingField, Detail: "titular ausente"},
	})).accepted(t, "ConcludeInteraction")
	seed(service.RecordExternalInteraction(seeding(t, "role-record-cb"), application.RecordExternalInteraction{
		InteractionID: "int-role-callback", ConnectorKey: roleConnector, Operation: "submitEffect", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, ContentType: "application/json", PartnerEventID: "pe-role-cb", Request: raw,
	})).accepted(t, "RecordExternalInteraction")
	enqueueCapture(t, pool)
	return orphan
}

// enqueueCapture writes the capture of a submission the way the dispatch's
// unit of work does, since no partner adapter exists to produce one.
func enqueueCapture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	capture := domain.ExternalResponseCaptured{
		InteractionID: "int-role-capture", CommandID: roleCommand, ConnectorKey: roleConnector, Operation: "submitEffect",
		Request:     domain.RawPayload{Ref: "interactions/prt-role:production/int-role-capture/request", VersionID: "vq", Hash: "hq"},
		Response:    domain.RawPayload{Ref: "interactions/prt-role:production/int-role-capture/response", VersionID: "vs", Hash: "hs"},
		ContentType: "application/json", Step: domain.StepConfirm, At: 1,
	}
	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) port.Outbox { return tx.Outbox(provider.Mapper{}) })
	identity := usecase.ResolveIdentity(idclock.SystemClock{}, idclock.NewMessageIDs("seed"), 1)
	origin := usecase.Origin{Destination: application.InteractionDestination, AggregateType: application.AggregateTypeExternalInteraction, AggregateID: "int-role-capture"}
	if err := uow.Within(seeding(t, "role-capture"), func(ctx context.Context, outbox port.Outbox) error {
		return usecase.Enqueue(ctx, outbox, identity, origin, 0, []kernel.DomainEvent{capture})
	}); err != nil {
		t.Fatalf("enqueue capture: %v", err)
	}
}

// runRole runs one role in process until the test ends, and asks it to stop
// cleanly; the roles share the one telemetry runtime a process may start.
func runRole(t *testing.T, cfg Config, runtime *otelboot.Runtime) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunWith(ctx, cfg, runtime) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("RunWith(%s) = %v, want a clean stop", cfg.Role, err)
		}
	})
}

func eventually(t *testing.T, what string, done func() bool) {
	t.Helper()
	for deadline := time.Now().Add(90 * time.Second); !done(); time.Sleep(200 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen within 90s", what)
		}
	}
}

func countOf(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// Decision 21: the consumer role reads gateway.events for the mismatches, the
// three internal channels each under a group of its own, and sweeps the
// attempts with the stale cut of the dispatch; without a partner adapter the
// dispatch and the normalization fail closed into the quarantine.
func TestTheConsumerRoleReadsItsChannelsAndSweepsTheAttempts(t *testing.T) {
	pool := appkit.OpenPool(t)
	orphan := seedRole(t, pool)
	relay := Defaults(RoleRelay)
	roleChannels(t, &relay)
	relay.DSN = pg.DSN(t, appkit.PoolOptions.Project)
	consumer := relay
	consumer.Role, consumer.RawStore = RoleConsumer, testRawStoreEnv(t, createBucket(t, lockedBucket))
	if err := consumer.Validate(); err != nil {
		t.Fatalf("Validate(consumer) = %v", err)
	}

	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Disabled: true, Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: "gateway", ServiceVersion: "dev", ServiceInstanceID: "gateway-roles-1", Role: string(RoleConsumer)},
	})
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	runRole(t, consumer, runtime)
	runRole(t, relay, runtime)

	eventually(t, "the suspension of the mismatch from gateway.events", func() bool {
		return countOf(t, pool, `SELECT count(*) FROM connectors WHERE connector_id = $1 AND snapshot::text LIKE '%schema_mismatch%'`, string(roleConnector)) == 1
	})
	eventually(t, "the record of the capture from gateway.interactions", func() bool {
		return countOf(t, pool, `SELECT count(*) FROM external_interactions WHERE external_interaction_id = 'int-role-capture'`) == 1
	})
	eventually(t, "the dispatch failed closed into the quarantine of gateway.dispatch", func() bool {
		return countOf(t, pool, `SELECT count(*) FROM quarantine WHERE consumer_name = $1 AND reason = $2`,
			application.DispatchConsumer, string(port.ReasonAttemptsExhausted)) >= 1
	})
	eventually(t, "the normalization failed closed into the quarantine of gateway.normalizations", func() bool {
		return countOf(t, pool, `SELECT count(*) FROM quarantine WHERE consumer_name = $1 AND reason = $2`,
			application.NormalizationConsumer, string(port.ReasonAttemptsExhausted)) >= 1
	})
	eventually(t, "the sweep of the attempt begun and never finished", func() bool {
		return countOf(t, pool, `SELECT count(*) FROM external_attempt WHERE outbound_key = $1 AND state = 'finished'`, orphan.OutboundKey) == 1
	})
	if n := countOf(t, pool, `SELECT count(*) FROM quarantine WHERE reason <> $1`, string(port.ReasonAttemptsExhausted)); n != 0 {
		t.Fatalf("%d messages contained for another reason, want every channel admitting the relay of this context", n)
	}
	swept, _, err := provider.NewExternalCommandReader(postgres.NewReadPool(pool)).Load(seeding(t, "role-swept"), roleOrphan)
	if err != nil || swept.Status != domain.StatusUnknownOutcome || swept.FailureCategory != domain.CategoryUnknownSent {
		t.Fatalf("command = %+v, %v, want unknown_outcome presumed sent by the sweep", swept, err)
	}
}
