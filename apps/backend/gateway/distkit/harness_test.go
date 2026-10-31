//go:build integration && distributed

package distkit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/distkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const records = 6

// The name is fixed: dist.Harness re-executes the binary with
// -test.run=^TestDistkitRole$.
func TestDistkitRole(t *testing.T) { distkit.RunRole(t) }

func TestTwoDrainsPublishEveryDispatchIntentExactlyOnce(t *testing.T) {
	h := dist.New(t, distkit.Spec)
	events, eventsDLQ := h.Topic+"-events", h.Topic+"-events-dlq"
	dist.CreateTopics(t, h.Brokers, h.Group, events, eventsDLQ)
	t.Setenv(distkit.EnvEventsTopic, events)
	t.Setenv(distkit.EnvEventsDLQ, eventsDLQ)
	enqueue(t, h.Pool, records)

	tb.Require(t, dist.RelayVector(t, h, records))
}

// The message context is required: the outbox claim drains only records whose
// metadata names correlation, causation and traceparent.
func enqueue(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	service := compose(t, pool)
	ctx := withExecution(t, port.WithMessageContext(context.Background(), port.MessageContext{
		CorrelationID: "corr-distkit",
		CausationID:   "caus-distkit",
		Traceparent:   "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	}))
	accept(t, "RegisterConnector", must(service.RegisterConnector(ctx, application.RegisterConnector{
		PartnerID: "prt-distkit", Environment: domain.EnvironmentProduction, CredentialRef: "vault://distkit", IntegrationVersion: 1,
	})))
	accept(t, "EnableOperation", must(service.EnableOperation(withExecution(t, ctx), application.EnableOperation{
		ConnectorKey: "prt-distkit:production", CapabilityID: "cap-distkit", Operation: "submitEffect", Version: 1,
	})))
	accept(t, "DeclareCapabilityProfile", must(service.DeclareCapabilityProfile(withExecution(t, ctx), application.DeclareCapabilityProfile{
		ConnectorKey: "prt-distkit:production", Version: 1, EffectiveFrom: 1,
		Operations: []domain.OperationCapability{{
			Operation: "submitEffect", Supported: true, IdempotencySupport: domain.IdempotencyKeyHeader, Efficacy: domain.EfficacyOnSubmit,
			MaxQueries: 3, KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32, AmbiguityDeadline: 3600,
		}},
	})))
	for i := range n {
		key := domain.IdempotencyKey(fmt.Sprintf("idem-distkit-%d", i))
		accept(t, "CreateExternalCommand", must(service.CreateExternalCommand(withExecution(t, ctx), application.CreateExternalCommand{
			IdempotencyKey: key, TenantID: "tnt-distkit", ConnectorKey: "prt-distkit:production", Operation: "submitEffect",
			BusinessKey: fmt.Sprintf("eff-distkit-%d", i), PayloadVersion: 1, Payload: "{}", PayloadHash: string(key),
		})))
	}
}

type outcome struct {
	refused bool
	err     error
	reason  string
}

func must[R any](o usecase.Outcome[R], err error) outcome {
	rejection, refused := o.Rejection()
	if refused {
		return outcome{refused: true, reason: string(rejection.Code())}
	}
	return outcome{err: err}
}

func accept(t *testing.T, operation string, o outcome) {
	t.Helper()
	if o.err != nil || o.refused {
		t.Fatalf("%s = %v %s, want accepted", operation, o.err, o.reason)
	}
}

func compose(t *testing.T, pool *pgxpool.Pool) application.Service {
	t.Helper()
	cfg := app.Defaults(app.RoleAPI)
	policy, err := kernelapp.IdempotencyPolicy(cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := app.NewService(pool, clock.New(port.Instant(1)), &ids.Sequence{Prefix: "m-"}, app.Waits{Command: time.Second})
	service.Idempotency = policy
	return service
}
