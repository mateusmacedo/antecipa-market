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
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	partnerv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

// ConsumerName is the logical, stable identity of INB-01: shared by every
// replica, never a hostname or an instance id.
const ConsumerName = "gateway"

const (
	capabilityEnabledEvent   = "com.company.partner.capability-enabled.v1"
	capabilitySuspendedEvent = "com.company.partner.capability-suspended.v1"

	processingDeadline = 5 * time.Second
	revokeGrace        = 10 * time.Second
	rebalanceTimeout   = 30 * time.Second
	queuePerPartition  = 32

	// consumerLocale answers a mandatory field of CTX-01 that a consumption has
	// no source for: there is no caller stating a preference on this side.
	consumerLocale = "en"
)

// PartnerEventTypes is the closed set this consumer takes from partner.events;
// IntegrationProfileRegistered joins it when the Partner publishes it.
var PartnerEventTypes = []string{capabilityEnabledEvent, capabilitySuspendedEvent}

var errUnknownEnvironment = errors.New("gateway: the event names no environment this consumer knows")

// Handler unpacks the two capability events. A payload that is not their
// contract is terminal (Validation, ERR-11): nothing would be written under
// R1×D4, so no unit of work is opened for it.
func Handler(service application.Service) kernelapp.Handler {
	return func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (usecase.Disposition, error) {
		switch env.Type {
		case capabilityEnabledEvent:
			var enabled partnerv1.CapabilityEnabled
			if err := envelope.Unpack(env, &enabled); err != nil {
				return usecase.R1D4, usecase.NewFailure(usecase.Validation, false, err)
			}
			key, err := connectorOf(enabled.GetPartnerId(), enabled.GetEnvironment())
			if err != nil {
				return usecase.R1D4, err
			}
			return service.ConsumeCapabilityEnabled(ctx, application.ConsumeCapabilityEnabled{
				Receipt: receipt,
				Enable: application.EnableOperation{
					ConnectorKey: key, CapabilityID: domain.CapabilityID(enabled.GetCapabilityId()), Operation: enabled.GetOperation(),
					AssetType: enabled.GetAssetType(), StateQuerySupported: enabled.GetStateQuerySupported(), Version: int(enabled.GetVersion()),
				},
			})
		case capabilitySuspendedEvent:
			var suspended partnerv1.CapabilitySuspended
			if err := envelope.Unpack(env, &suspended); err != nil {
				return usecase.R1D4, usecase.NewFailure(usecase.Validation, false, err)
			}
			key, err := connectorOf(suspended.GetPartnerId(), suspended.GetEnvironment())
			if err != nil {
				return usecase.R1D4, err
			}
			return service.ConsumeCapabilitySuspended(ctx, application.ConsumeCapabilitySuspended{
				Receipt: receipt,
				Suspend: application.SuspendOperation{
					ConnectorKey: key, Operation: suspended.GetOperation(), AssetType: suspended.GetAssetType(), Cause: domain.CausePartner,
					CapabilityID: domain.CapabilityID(suspended.GetCapabilityId()), Version: int(suspended.GetVersion()),
				},
			})
		default:
			return usecase.R1D4, usecase.NewFailure(usecase.Validation, false, fmt.Errorf("gateway: event type %q is not consumed", env.Type))
		}
	}
}

// partnerEnvironments is closed: a value the Partner adds later is a contract
// this consumer does not know, not a connector that is still missing.
var partnerEnvironments = map[partnerv1.Environment]domain.Environment{
	partnerv1.Environment_ENVIRONMENT_DEVELOPMENT:  domain.EnvironmentDevelopment,
	partnerv1.Environment_ENVIRONMENT_INTEGRATION:  domain.EnvironmentIntegration,
	partnerv1.Environment_ENVIRONMENT_HOMOLOGATION: domain.EnvironmentHomologation,
	partnerv1.Environment_ENVIRONMENT_PRODUCTION:   domain.EnvironmentProduction,
}

func connectorOf(partner string, environment partnerv1.Environment) (domain.ConnectorKey, error) {
	name, known := partnerEnvironments[environment]
	if !known {
		return "", usecase.NewFailure(usecase.Validation, false, fmt.Errorf("%w: %d", errUnknownEnvironment, environment))
	}
	return domain.ConnectorKeyOf(domain.PartnerID(partner), name), nil
}

type ConsumerTelemetry struct {
	Tracer         trace.Tracer
	MeterProvider  metric.MeterProvider
	LoggerProvider log.LoggerProvider
	System         string
	Channel        kernelapp.Channel
}

// NewConsumer is the whole consumer: adapter, service and Postgres quarantine.
// maxAttempts <= 0 disables the attempt limit (GAR-08 fixes that one exists).
func NewConsumer(pool *pgxpool.Pool, clock ports.Clock, ids ports.IDGenerator, wait, timeout time.Duration, maxAttempts int, boundary kernelapp.Boundary, telemetry ConsumerTelemetry) kernelapp.Consumer {
	return kernelapp.Consumer{
		Name:           ConsumerName,
		MaxAttempts:    maxAttempts,
		Handle:         Handler(NewService(pool, clock, ids, Waits{Message: wait})),
		Containment:    postgres.NewQuarantine(pool, postgres.WithQuarantineLoggerProvider(telemetry.LoggerProvider)),
		Clock:          clock,
		Timeout:        timeout,
		Boundary:       boundary,
		Locale:         consumerLocale,
		Tracer:         telemetry.Tracer,
		MeterProvider:  telemetry.MeterProvider,
		LoggerProvider: telemetry.LoggerProvider,
		System:         telemetry.System,
		Channel:        telemetry.Channel,
	}
}

// PartnerBoundary is the one place the consumer's trust is declared: the
// Partner producer, and a verified transport only when TLS checks the broker
// and this client authenticates to it (IDN-03, IDN-04).
func PartnerBoundary(cfg Config, authenticated bool) kernelapp.Boundary {
	transport := kernelapp.TransportDevelopmentOnly
	if authenticated {
		transport = kernelapp.TransportVerified
	}
	return kernelapp.Boundary{Transport: transport, Sources: []string{cfg.PartnerSource}}
}

func runConsumer(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	ctx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	pool, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := postgres.WaitForGrants(ctx, pool, time.Second, rt.LoggerProvider(), append(postgres.KernelGrants(postgres.Outbox, postgres.Inbox), grants()...)...); err != nil {
		return err
	}
	stopPurge, err := kernelapp.StartPurge(ctx, abort,
		kernelapp.PurgeConfig{Name: "message-inbox", Interval: cfg.Policies.PurgeInterval, Batch: cfg.Policies.PurgeBatch, Retention: cfg.Policies.InboxRetention},
		idclock.SystemClock{}, rt.LoggerProvider(), kernelapp.PurgeMessageInbox(pool, ConsumerName))
	if err != nil {
		return err
	}
	defer func() { _ = stopPurge() }()

	ch := PartnerChannel(cfg)
	catalog, err := channel.NewCatalog(ch)
	if err != nil {
		return err
	}
	kafkaConfig, err := kafka.NewConfig(ctx, rt, catalog, cfg.Brokers, cfg.KafkaInsecure, cfg.KafkaAuth)
	if err != nil {
		return err
	}
	kafkaConfig.RevokeGrace = revokeGrace
	logger := rt.LoggerFor(reflect.TypeFor[Config]().PkgPath())
	consumer := &kafka.Consumer{
		Config:  kafkaConfig,
		Channel: ch,
		Sink: Sink{
			Consumer: NewConsumer(pool, idclock.SystemClock{}, idclock.NewMessageIDs("gateway"), cfg.Wait, cfg.ConsumerTimeout, ch.Retry.MaxAttempts,
				PartnerBoundary(cfg, kafkaConfig.ClientAuthenticated()), ConsumerTelemetry{
					Tracer: rt.Tracer(), MeterProvider: rt.MeterProvider(), LoggerProvider: rt.LoggerProvider(),
					System:  semconv.MessagingSystemKafka.Value.AsString(),
					Channel: kernelapp.Channel{Address: ch.Address, Group: ch.Group},
				}),
			EventTypes: PartnerEventTypes,
			Logger:     logger,
		},
		Backoff:            retry.Backoff{Base: 10 * time.Millisecond, Factor: 2, Cap: time.Second},
		ProcessingDeadline: processingDeadline,
		RebalanceTimeout:   rebalanceTimeout,
		QueuePerPartition:  queuePerPartition,
	}
	if err := consumer.Validate(); err != nil {
		return err
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "consumer joining",
		slog.String(string(semconv.MessagingSystemKey), semconv.MessagingSystemKafka.Value.AsString()),
		slog.String(string(semconv.MessagingDestinationNameKey), ch.Address),
		slog.String(string(semconv.MessagingConsumerGroupNameKey), ch.Group),
		slog.String(logging.KeyChannelName, ch.Name))
	// WHY: only the cancellation is a clean exit; a broker failure that lands
	// during the drain must still reach the caller.
	if err := consumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return stopPurge()
}
