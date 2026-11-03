package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
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

	// WHY: an attempt is stale only past the budget's cut, so a sweep every
	// interval closes an orphan within the cut plus the interval; the
	// redelivery of its intent waits for that anyway.
	attemptSweepInterval = 15 * time.Second
	attemptSweepBatch    = 100

	// consumerLocale answers a mandatory field of CTX-01 that a consumption has
	// no source for: there is no caller stating a preference on this side.
	consumerLocale = "en"
)

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

// ConsumerTelemetry is the telemetry every consumer of the role reports to.
type ConsumerTelemetry struct {
	Tracer         trace.Tracer
	MeterProvider  metric.MeterProvider
	LoggerProvider log.LoggerProvider
	System         string
}

// ConsumerParts is what every subscription of the consumer role shares: the
// service, the budget of a submission, the quarantine, the clock and whether
// the transport is verified.
type ConsumerParts struct {
	Service     application.Service
	Budget      kernelapp.ExternalBudget
	Containment ports.Containment
	Clock       ports.Clock

	// Authenticated is a transport whose TLS checks the broker and whose client
	// authenticates to it (IDN-03, IDN-04).
	Authenticated bool
	Telemetry     ConsumerTelemetry
}

// Subscription is one channel the consumer role reads and the kernel consumer
// that takes its deliveries; ProcessingDeadline bounds each at the broker.
type Subscription struct {
	Channel            channel.Channel
	Consumer           kernelapp.Consumer
	ProcessingDeadline time.Duration
}

// Subscriptions is every channel the consumer role reads (decision 21): the
// capability events of the Partner, the mismatches of this context's own
// gateway.events and the three internal channels. Each consumer accepts only
// the types its handler takes and names the message inbox it registers in;
// the dispatch runs under the budget of one submission, the others under the
// consumer's own time policy.
func Subscriptions(cfg Config, parts ConsumerParts) []Subscription {
	read := consumedChannels(cfg)
	partner, own := PartnerBoundary(cfg, parts.Authenticated), OwnBoundary(cfg, parts.Authenticated)
	service := parts.Service
	readings := []reading{
		{read.partner, ConsumerName, Handler(service), partner, cfg.ConsumerTimeout, processingDeadline,
			[]channel.Event{capabilityEnabled, capabilitySuspended}},
		{read.events, application.EventsConsumer, SchemaMismatchHandler(service), own, cfg.ConsumerTimeout, processingDeadline,
			[]channel.Event{externalSchemaMismatchDetected}},
		{read.dispatch, application.DispatchConsumer, DispatchHandler(service), own, parts.Budget.Processing, parts.Budget.Processing,
			[]channel.Event{externalCommandDispatchRequested}},
		{read.interactions, application.InteractionConsumer, InteractionHandler(service), own, cfg.ConsumerTimeout, processingDeadline,
			[]channel.Event{externalResponseCaptured}},
		{read.normalizations, application.NormalizationConsumer, NormalizationHandler(service), own, cfg.ConsumerTimeout, processingDeadline,
			[]channel.Event{externalInteractionStored}},
	}
	subscriptions := make([]Subscription, len(readings))
	for i, r := range readings {
		subscriptions[i] = parts.subscribe(r)
	}
	return subscriptions
}

// reading is how the role reads one channel: the inbox name, the handler, the
// trusted producer, the time policy and the subscription.
type reading struct {
	channel  channel.Channel
	name     string
	handle   kernelapp.Handler
	boundary kernelapp.Boundary
	timeout  time.Duration
	deadline time.Duration
	accepts  []channel.Event
}

func (p ConsumerParts) subscribe(r reading) Subscription {
	return Subscription{
		Channel:            r.channel,
		ProcessingDeadline: r.deadline,
		Consumer: kernelapp.Consumer{
			Name:           r.name,
			MaxAttempts:    r.channel.Retry.MaxAttempts,
			Handle:         r.handle,
			Accepts:        wireTypesOf(r.channel, r.accepts...),
			Containment:    p.Containment,
			Clock:          p.Clock,
			Timeout:        r.timeout,
			Boundary:       r.boundary,
			Locale:         consumerLocale,
			Tracer:         p.Telemetry.Tracer,
			MeterProvider:  p.Telemetry.MeterProvider,
			LoggerProvider: p.Telemetry.LoggerProvider,
			System:         p.Telemetry.System,
			Channel:        kernelapp.Channel{Address: r.channel.Address, Group: r.channel.Group, Carries: channel.WireTypes(r.channel)},
		},
	}
}

func wireTypesOf(ch channel.Channel, events ...channel.Event) []string {
	ch.Events = events
	return channel.WireTypes(ch)
}

// consumed is the channels the consumer role reads. Partner.events keeps the
// role's group; each channel of this context has a group of its own.
type consumed struct {
	partner        channel.Channel
	events         channel.Channel
	dispatch       channel.Channel
	interactions   channel.Channel
	normalizations channel.Channel
}

func consumedChannels(cfg Config) consumed {
	return consumed{
		partner:        PartnerChannel(cfg),
		events:         readBy(GatewayChannel(cfg), cfg.GatewayGroup),
		dispatch:       readBy(DispatchChannel(cfg), cfg.DispatchGroup),
		interactions:   readBy(InteractionChannel(cfg), cfg.InteractionGroup),
		normalizations: readBy(NormalizationChannel(cfg), cfg.NormalizationGroup),
	}
}

func (c consumed) all() []channel.Channel {
	return []channel.Channel{c.partner, c.events, c.dispatch, c.interactions, c.normalizations}
}

func readBy(ch channel.Channel, group string) channel.Channel {
	ch.Group = group
	return ch
}

// PartnerBoundary is the trust of partner.events: the Partner producer, and a
// verified transport only when TLS checks the broker and this client
// authenticates to it (IDN-03, IDN-04).
func PartnerBoundary(cfg Config, authenticated bool) kernelapp.Boundary {
	return kernelapp.Boundary{Transport: transportOf(authenticated), Sources: []string{cfg.PartnerSource}}
}

// OwnBoundary is the trust of the channels of this context: only its own relay
// publishes them, under the source it stamps on every envelope.
func OwnBoundary(cfg Config, authenticated bool) kernelapp.Boundary {
	return kernelapp.Boundary{Transport: transportOf(authenticated), Sources: []string{cfg.Relay.Source}}
}

func transportOf(authenticated bool) kernelapp.Transport {
	if authenticated {
		return kernelapp.TransportVerified
	}
	return kernelapp.TransportDevelopmentOnly
}
