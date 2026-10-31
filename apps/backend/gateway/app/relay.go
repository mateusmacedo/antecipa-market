package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// RelayCatalog is every destination the outbox of this context holds. The
// relay drains without filtering by destination, so one missing here would
// refuse the start (AssertOwnOutbox) or the publication.
func RelayCatalog(cfg Config) (channel.Catalog, error) {
	return channel.NewCatalog(GatewayChannel(cfg), DispatchChannel(cfg))
}

// WHY: serve.RunRelay of kernel rc.7 takes one channel, and this outbox holds
// two destinations; the steps are serve's, over a catalogue of both.
func runRelay(ctx context.Context, rt *otelboot.Runtime, cfg Config) error {
	if rt == nil {
		return serve.ErrNoRuntime
	}
	catalog, err := RelayCatalog(cfg)
	if err != nil {
		return err
	}
	ctx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	err = drainOutbox(ctx, abort, rt, cfg, catalog)
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func drainOutbox(ctx context.Context, abort context.CancelCauseFunc, rt *otelboot.Runtime, cfg Config, catalog channel.Catalog) error {
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := postgres.WaitForGrants(ctx, pool, time.Second, rt.LoggerProvider(), postgres.KernelGrants(postgres.Outbox)...); err != nil {
		return err
	}
	if err := postgres.AssertOwnOutbox(ctx, pool, slices.Collect(maps.Keys(catalog))); err != nil {
		return err
	}
	stopPurge, err := kernelapp.StartPurge(ctx, abort,
		kernelapp.PurgeConfig{Name: "outbox", Interval: cfg.Policies.PurgeInterval, Batch: cfg.Policies.PurgeBatch, Retention: cfg.Policies.OutboxRetention},
		idclock.SystemClock{}, rt.LoggerProvider(), kernelapp.PurgeOutbox(pool))
	if err != nil {
		return err
	}
	defer func() { _ = stopPurge() }()

	kafkaConfig, err := kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.KafkaInsecure, cfg.KafkaAuth)
	if err != nil {
		return err
	}
	publisher, err := kafka.NewPublisher(kafkaConfig, nil)
	if err != nil {
		return err
	}
	defer publisher.Close()

	drain, err := relay.NewOverPostgres(pool, publisher, "gateway", relay.Instrument(cfg.Relay, rt, catalog.AddressOf))
	if err != nil {
		return err
	}
	logger := rt.LoggerFor(reflect.TypeFor[Config]().PkgPath())
	for _, name := range slices.Sorted(maps.Keys(catalog)) {
		logger.LogAttrs(ctx, slog.LevelInfo, "relay draining",
			slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),
			slog.String(string(semconv.MessagingDestinationNameKey), catalog[name].Address),
			slog.String(logging.KeyChannelName, name))
	}
	if err := drain.Run(ctx); err != nil {
		return err
	}
	return stopPurge()
}
