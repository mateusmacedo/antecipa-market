package app

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

const (
	// partnerDestination is the flow the Partner publishes. A context never
	// imports another context's blocks: the channel name is the contract (ASY-01).
	partnerDestination = "partner.events"

	// The catalogue entry carries one event type; the channels of gateway.events
	// and partner.events carry a closed set, which the Sink filters on consumption.
	submittedType          = "com.company.gateway.external-command-submitted"
	submittedContract      = "company/gateway/event/v1/external_command_submitted.proto#ExternalCommandSubmitted"
	dispatchRequestedType  = "com.company.gateway.external-command-dispatch-requested"
	dispatchContract       = "company/gateway/event/v1/external_command_dispatch_requested.proto#ExternalCommandDispatchRequested"
	capabilityEnabledType  = "com.company.partner.capability-enabled"
	capabilityEnabledProto = "company/partner/event/v1/capability_enabled.proto#CapabilityEnabled"
)

// GatewayChannel is the channel of the integration events, named after the
// destination the use cases author, because the publisher resolves by it.
func GatewayChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.GatewayTopic, cfg.GatewayDLQ, cfg.Group, submittedType, submittedContract)
}

// DispatchChannel carries the dispatch intent alone (KFK-03), to the handler
// of this context that sends the command to the partner.
func DispatchChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.DispatchDestination, cfg.DispatchTopic, cfg.DispatchDLQ, cfg.Group, dispatchRequestedType, dispatchContract)
}

// PartnerChannel is the channel the consumer reads.
func PartnerChannel(cfg Config) channel.Channel {
	return channel.NewKafka(partnerDestination, cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group, capabilityEnabledType, capabilityEnabledProto)
}
