package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

// loop is one long-running part of the role, which runs until ctx is done.
type loop func(ctx context.Context) error

// runConsumer reads the five channels of the role and runs the sweep of the
// kernel's attempts, with the same stale cut the dispatch takes (decision 21).
// The raw store is ready before any of them starts, or the role refuses to.
func runConsumer(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	ctx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	raw, err := OpenRawStore(ctx, cfg.RawStore, rt)
	if err != nil {
		return err
	}
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := postgres.WaitForGrants(ctx, pool, time.Second, rt.LoggerProvider(), consumerGrants()...); err != nil {
		return err
	}
	budget, err := DispatchBudget(PartnerRoute())
	if err != nil {
		return err
	}
	service, err := NewConsumerService(pool, rt, cfg, raw, budget)
	if err != nil {
		return err
	}
	loops, subscriptions, err := consumerLoops(ctx, cfg, rt, pool, service, budget)
	if err != nil {
		return err
	}
	stopPurges, err := startInboxPurges(ctx, abort, cfg, rt, pool, subscriptions)
	if err != nil {
		return err
	}
	defer func() { _ = stopPurges() }()
	if err := together(ctx, loops...); err != nil {
		return err
	}
	return stopPurges()
}

// consumerLoops is one broker consumer per subscription and the sweep of the
// attempts, each validated before any of them joins.
func consumerLoops(ctx context.Context, cfg Config, rt *otelboot.Runtime, pool *pgxpool.Pool, service application.Service, budget kernelapp.ExternalBudget) ([]loop, []Subscription, error) {
	catalog, err := channel.NewCatalog(consumedChannels(cfg).all()...)
	if err != nil {
		return nil, nil, err
	}
	kafkaConfig, err := kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.KafkaInsecure, cfg.KafkaAuth)
	if err != nil {
		return nil, nil, err
	}
	kafkaConfig.RevokeGrace = revokeGrace
	subscriptions := Subscriptions(cfg, ConsumerParts{
		Service: service, Budget: budget, Clock: idclock.SystemClock{}, Authenticated: kafkaConfig.ClientAuthenticated(),
		Containment: postgres.NewQuarantine(pool, postgres.WithQuarantineLoggerProvider(rt.LoggerProvider())),
		Telemetry: ConsumerTelemetry{
			Tracer: rt.Tracer(), MeterProvider: rt.MeterProvider(), LoggerProvider: rt.LoggerProvider(),
			System: semconv.MessagingSystemKafka.Value.AsString(),
		},
	})
	sweep, err := kernelapp.SweepExternalAttempts(kernelapp.ExternalSweep{
		Store: postgres.NewAttemptStore(pool), Abandon: service.AbandonExternalAttempt, Timeout: budget.Settle, Locale: consumerLocale, Tracer: rt.Tracer(),
	})
	if err != nil {
		return nil, nil, err
	}
	loops := []loop{func(ctx context.Context) error {
		return kernelapp.RunSweep(ctx, kernelapp.SweepConfig{Interval: attemptSweepInterval, Batch: attemptSweepBatch, StaleAfter: budget.StaleAfter()},
			idclock.SystemClock{}, rt.LoggerProvider(), sweep)
	}}
	logger := rt.LoggerFor(reflect.TypeFor[Config]().PkgPath())
	for _, s := range subscriptions {
		if err := s.Consumer.Validate(); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", s.Channel.Name, err)
		}
		consumer := &kafka.Consumer{
			Config:             kafkaConfig,
			Channel:            s.Channel,
			Sink:               kernelapp.Sink{Consumer: s.Consumer},
			Backoff:            retry.Backoff{Base: 10 * time.Millisecond, Factor: 2, Cap: time.Second},
			ProcessingDeadline: s.ProcessingDeadline,
			RebalanceTimeout:   rebalanceTimeout,
			QueuePerPartition:  queuePerPartition,
		}
		if err := consumer.Validate(); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", s.Channel.Name, err)
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "consumer joining",
			slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),
			slog.String(string(semconv.MessagingDestinationNameKey), s.Channel.Address),
			slog.String(string(semconv.MessagingConsumerGroupNameKey), s.Channel.Group),
			slog.String(logging.KeyChannelName, s.Channel.Name))
		loops = append(loops, consumer.Run)
	}
	return loops, subscriptions, nil
}

// startInboxPurges purges each message inbox of the role on its own, named
// after the consumer that registers in it (INB-01).
func startInboxPurges(ctx context.Context, abort context.CancelCauseFunc, cfg Config, rt *otelboot.Runtime, pool *pgxpool.Pool, subscriptions []Subscription) (func() error, error) {
	var stops []func() error
	stopAll := func() error {
		var errs []error
		for _, stop := range stops {
			errs = append(errs, stop())
		}
		return errors.Join(errs...)
	}
	for _, s := range subscriptions {
		stop, err := kernelapp.StartPurge(ctx, abort,
			kernelapp.PurgeConfig{Name: "message-inbox." + s.Consumer.Name, Interval: cfg.Policies.PurgeInterval, Batch: cfg.Policies.PurgeBatch, Retention: cfg.Policies.InboxRetention},
			idclock.SystemClock{}, rt.LoggerProvider(), kernelapp.PurgeMessageInbox(pool, s.Consumer.Name))
		if err != nil {
			return nil, errors.Join(err, stopAll())
		}
		stops = append(stops, stop)
	}
	return stopAll, nil
}

// together runs the loops until ctx is done; the first that ends stops the
// others. WHY: only the cancellation is a clean exit, so a broker failure that
// lands during the drain still reaches the caller.
func together(ctx context.Context, loops ...loop) error {
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	ended := make(chan error, len(loops))
	for _, run := range loops {
		go func() {
			err := run(ctx)
			stop(err)
			ended <- err
		}()
	}
	var first error
	for range loops {
		if err := <-ended; first == nil && err != nil && !errors.Is(err, context.Canceled) {
			first = err
		}
	}
	return first
}
