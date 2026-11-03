package app

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

// partnerDestination is the flow the Partner publishes. A context never
// imports another context's blocks: the channel name is the contract (ASY-01).
const partnerDestination = "partner.events"

var (
	externalCommandSubmitted = channel.Event{
		Type:        "com.company.gateway.external-command-submitted",
		ContractRef: "company/gateway/event/v1/external_command_submitted.proto#ExternalCommandSubmitted",
	}
	externalCommandAcknowledged = channel.Event{
		Type:        "com.company.gateway.external-command-acknowledged",
		ContractRef: "company/gateway/event/v1/external_command_acknowledged.proto#ExternalCommandAcknowledged",
	}
	externalCommandConfirmed = channel.Event{
		Type:        "com.company.gateway.external-command-confirmed",
		ContractRef: "company/gateway/event/v1/external_command_confirmed.proto#ExternalCommandConfirmed",
	}
	externalCommandFailed = channel.Event{
		Type:        "com.company.gateway.external-command-failed",
		ContractRef: "company/gateway/event/v1/external_command_failed.proto#ExternalCommandFailed",
	}
	externalOutcomeUnknown = channel.Event{
		Type:        "com.company.gateway.external-outcome-unknown",
		ContractRef: "company/gateway/event/v1/external_outcome_unknown.proto#ExternalOutcomeUnknown",
	}
	externalCommandAmbiguityExpired = channel.Event{
		Type:        "com.company.gateway.external-command-ambiguity-expired",
		ContractRef: "company/gateway/event/v1/external_command_ambiguity_expired.proto#ExternalCommandAmbiguityExpired",
	}
	externalSchemaMismatchDetected = channel.Event{
		Type:        "com.company.gateway.external-schema-mismatch-detected",
		ContractRef: "company/gateway/event/v1/external_schema_mismatch_detected.proto#ExternalSchemaMismatchDetected",
	}
	externalInteractionUncorrelated = channel.Event{
		Type:        "com.company.gateway.external-interaction-uncorrelated",
		ContractRef: "company/gateway/event/v1/external_interaction_uncorrelated.proto#ExternalInteractionUncorrelated",
	}
	externalInteractionReprocessed = channel.Event{
		Type:        "com.company.gateway.external-interaction-reprocessed",
		ContractRef: "company/gateway/event/v1/external_interaction_reprocessed.proto#ExternalInteractionReprocessed",
	}
	externalCommandDispatchRequested = channel.Event{
		Type:        "com.company.gateway.external-command-dispatch-requested",
		ContractRef: "company/gateway/event/v1/external_command_dispatch_requested.proto#ExternalCommandDispatchRequested",
	}
	externalResponseCaptured = channel.Event{
		Type:        "com.company.gateway.external-response-captured",
		ContractRef: "company/gateway/event/v1/external_response_captured.proto#ExternalResponseCaptured",
	}
	externalInteractionStored = channel.Event{
		Type:        "com.company.gateway.external-interaction-stored",
		ContractRef: "company/gateway/event/v1/external_interaction_stored.proto#ExternalInteractionStored",
	}
)

// The Partner publishes eleven types on partner.events; the consumer takes the
// capability events and discards the rest (KFK-03c).
var (
	partnerRegistered          = partnerEvent("partner-registered", "partner_registered", "PartnerRegistered")
	partnerOffboardingStarted  = partnerEvent("partner-offboarding-started", "partner_offboarding_started", "PartnerOffboardingStarted")
	partnerAgreementActivated  = partnerEvent("partner-agreement-activated", "partner_agreement_activated", "PartnerAgreementActivated")
	partnerCertificateExpiring = partnerEvent("partner-certificate-expiring", "partner_certificate_expiring", "PartnerCertificateExpiring")
	capabilityEnabled          = partnerEvent("capability-enabled", "capability_enabled", "CapabilityEnabled")
	capabilitySuspended        = partnerEvent("capability-suspended", "capability_suspended", "CapabilitySuspended")
	capabilityRolloutChanged   = partnerEvent("capability-rollout-changed", "capability_rollout_changed", "CapabilityRolloutChanged")
	partnerProductDefined      = partnerEvent("partner-product-defined", "partner_product_defined", "PartnerProductDefined")
	partnerProductChanged      = partnerEvent("partner-product-changed", "partner_product_changed", "PartnerProductChanged")
	partnerProductExpired      = partnerEvent("partner-product-expired", "partner_product_expired", "PartnerProductExpired")
	partnerProductRetired      = partnerEvent("partner-product-retired", "partner_product_retired", "PartnerProductRetired")
)

func partnerEvent(name, file, message string) channel.Event {
	return channel.Event{
		Type:        "com.company.partner." + name,
		ContractRef: "company/partner/event/v1/" + file + ".proto#" + message,
	}
}

// GatewayChannel is the channel of the nine integration events, named after the
// destination the use cases author, because the publisher resolves by it.
func GatewayChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.Destination, cfg.GatewayTopic, cfg.GatewayDLQ, cfg.Group,
		externalCommandSubmitted, externalCommandAcknowledged, externalCommandConfirmed,
		externalCommandFailed, externalOutcomeUnknown, externalCommandAmbiguityExpired,
		externalSchemaMismatchDetected, externalInteractionUncorrelated, externalInteractionReprocessed)
}

// DispatchChannel carries the dispatch intent alone (KFK-03), to the handler
// of this context that sends the command to the partner.
func DispatchChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.DispatchDestination, cfg.DispatchTopic, cfg.DispatchDLQ, cfg.Group, externalCommandDispatchRequested)
}

// InteractionChannel carries the capture of a submission's response alone
// (KFK-03), to the consumer of this context that records the interaction.
func InteractionChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.InteractionDestination, cfg.InteractionTopic, cfg.InteractionDLQ, cfg.Group, externalResponseCaptured)
}

// NormalizationChannel carries the interaction stored for normalization alone
// (KFK-03), to the consumer of this context that concludes it (decision 18).
func NormalizationChannel(cfg Config) channel.Channel {
	return channel.NewKafka(application.NormalizationDestination, cfg.NormalizationTopic, cfg.NormalizationDLQ, cfg.Group, externalInteractionStored)
}

// PartnerChannel is the channel the consumer reads, with every event the
// Partner relay publishes on it.
func PartnerChannel(cfg Config) channel.Channel {
	return channel.NewKafka(partnerDestination, cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group,
		partnerRegistered, partnerOffboardingStarted, partnerAgreementActivated, partnerCertificateExpiring,
		capabilityEnabled, capabilitySuspended, capabilityRolloutChanged,
		partnerProductDefined, partnerProductChanged, partnerProductExpired, partnerProductRetired)
}
