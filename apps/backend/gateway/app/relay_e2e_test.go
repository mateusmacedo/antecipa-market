//go:build integration

package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

// capturingPublisher keeps every delivery by destination, so the test replays
// the bytes the relay put on the wire into the consumer.
type capturingPublisher struct {
	mu        sync.Mutex
	delivered map[string][][]byte
}

func (p *capturingPublisher) Publish(_ context.Context, destination string, message []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.delivered == nil {
		p.delivered = map[string][][]byte{}
	}
	p.delivered[destination] = append(p.delivered[destination], message)
	return nil
}

func (p *capturingPublisher) to(destination string) [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.delivered[destination]...)
}

// crashingStore is the window of FND-04 §5.4 made deliberate: the message
// reaches the broker and the transition never lands.
type crashingStore struct{ relay.Store }

func (crashingStore) MarkPublished(context.Context, int64, string) (int64, error) {
	return 0, context.Canceled
}

type acknowledger struct{ acks int }

func (a *acknowledger) Ack(context.Context) error     { a.acks++; return nil }
func (a *acknowledger) Release(context.Context) error { return nil }

func drainFor(t *testing.T, drain relay.Relay, within time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	if err := drain.Run(ctx); err != nil {
		t.Fatalf("Run() = %v", err)
	}
}

func shortRelay() relay.Config {
	cfg := Defaults(RoleRelay).Relay
	cfg.Interval, cfg.Lease, cfg.BackoffBase, cfg.BackoffCeiling, cfg.ShutdownGrace = 10*time.Millisecond, 150*time.Millisecond, time.Millisecond, 10*time.Millisecond, 2*time.Second
	return cfg
}

// V32 under the runtime role: a failure between publishing and marking
// republishes the capture under its id, and the inbox of gateway.interactions
// records the outbound interaction once.
func TestARepublishedCaptureIsRecordedOnceUnderTheRuntimeRole(t *testing.T) {
	pool := pg.OpenRuntimePool(t, appkit.PoolOptions)
	seedRole(t, pool)
	publisher := &capturingPublisher{}

	crashing, err := relay.NewOverPostgres(pool, publisher, "gateway", shortRelay())
	if err != nil {
		t.Fatalf("NewOverPostgres() = %v", err)
	}
	crashing.Store = crashingStore{Store: crashing.Store}
	drainFor(t, crashing, 150*time.Millisecond)
	if n := countOf(t, pool, `SELECT count(*) FROM outbox WHERE destination = $1 AND status = 'publishing'`, application.InteractionDestination); n != 1 {
		t.Fatalf("%d captures publishing after the crash, want the one the transition never marked", n)
	}
	time.Sleep(shortRelay().Lease + 50*time.Millisecond)
	healthy, err := relay.NewOverPostgres(pool, publisher, "gateway", shortRelay())
	if err != nil {
		t.Fatalf("NewOverPostgres() = %v", err)
	}
	drainFor(t, healthy, 400*time.Millisecond)

	captures := publisher.to(application.InteractionDestination)
	if len(captures) < 2 {
		t.Fatalf("published the capture %d times, want the window to republish it", len(captures))
	}
	first, err := envelope.Unmarshal(captures[0])
	if err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	last, err := envelope.Unmarshal(captures[len(captures)-1])
	if err != nil || last.ID != first.ID || last.Source != Defaults(RoleRelay).Relay.Source {
		t.Fatalf("republished %+v, %v; want the id %q under the source of this context's relay", last, err, first.ID)
	}

	consumer := interactionConsumer(t, pool)
	for delivery, raw := range [][]byte{captures[0], captures[len(captures)-1]} {
		ack := &acknowledger{}
		outcome, err := consumer.Consume(context.Background(), kernelapp.Delivery{Raw: raw, Attempt: 1}, ack)
		if err != nil || ack.acks != 1 {
			t.Fatalf("delivery %d = %+v, %v with %d acks, want acknowledged", delivery+1, outcome, err, ack.acks)
		}
		if delivery == 1 && outcome.Disposition != usecase.R2 {
			t.Fatalf("the republished capture was classified %v, want R2", outcome.Disposition)
		}
	}
	if n := countOf(t, pool, `SELECT count(*) FROM external_interactions WHERE external_interaction_id = 'int-role-capture'`); n != 1 {
		t.Fatalf("%d interactions after two deliveries of the capture, want 1", n)
	}
}

// interactionConsumer is the consumer of gateway.interactions the role mounts,
// over the pool of the runtime role.
func interactionConsumer(t *testing.T, pool *pgxpool.Pool) kernelapp.Consumer {
	t.Helper()
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := NewService(pool, idclock.SystemClock{}, idclock.NewMessageIDs("relay"), Waits{Message: time.Second, Command: time.Second})
	service.Idempotency = policy
	budget, err := DispatchBudget(PartnerRoute())
	if err != nil {
		t.Fatalf("DispatchBudget() = %v", err)
	}
	cfg := Defaults(RoleConsumer)
	cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group = "partner.events", "partner.events.dlq", ConsumerName
	cfg.GatewayTopic, cfg.GatewayDLQ, cfg.GatewayGroup = "gateway.events", "gateway.events.dlq", "gateway.events"
	cfg.DispatchTopic, cfg.DispatchDLQ, cfg.DispatchGroup = "gateway.dispatch", "gateway.dispatch.dlq", "gateway.dispatch"
	cfg.InteractionTopic, cfg.InteractionDLQ, cfg.InteractionGroup = "gateway.interactions", "gateway.interactions.dlq", "gateway.interactions"
	cfg.NormalizationTopic, cfg.NormalizationDLQ, cfg.NormalizationGroup = "gateway.normalizations", "gateway.normalizations.dlq", "gateway.normalizations"
	for _, s := range Subscriptions(cfg, ConsumerParts{
		Service: service, Budget: budget, Clock: idclock.SystemClock{}, Containment: postgres.NewQuarantine(pool),
		Telemetry: ConsumerTelemetry{Tracer: noop.NewTracerProvider().Tracer("")},
	}) {
		if s.Consumer.Name == application.InteractionConsumer {
			return s.Consumer
		}
	}
	t.Fatal("no subscription of gateway.interactions")
	return kernelapp.Consumer{}
}
