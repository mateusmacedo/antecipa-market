package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalInteractionReprocessedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-interaction-reprocessed.golden",
	Source:  fixtureSource,
	Subject: "int-1001",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-interaction-reprocessed",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalInteractionReprocessed"},
		Type:       "com.company.gateway.external-interaction-reprocessed.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalInteractionReprocessed",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "outcome", Values: []string{
		"INTERACTION_OUTCOME_NORMALIZED", "INTERACTION_OUTCOME_UNCORRELATED", "INTERACTION_OUTCOME_SCHEMA_MISMATCH",
	}},
	New:        func() proto.Message { return &eventv1.ExternalInteractionReprocessed{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalInteractionReprocessed(fields) },
	Build: leanFixture(
		externalInteractionReprocessedFields("int-1001", "prt-1:production", "prt-1", "submitEffect", "prt-1:production:submitEffect:eff-1",
			"1", "2", "INTERACTION_OUTCOME_NORMALIZED", "INTERACTION_OUTCOME_NORMALIZED", "true", "true", "2026-09-02T12:00:00Z"),
		externalInteractionReprocessedFields("int-1002", "prt-1:production", "prt-1", "submitEffect", "",
			"2", "3", "INTERACTION_OUTCOME_SCHEMA_MISMATCH", "INTERACTION_OUTCOME_UNCORRELATED", "true", "false", "2026-09-02T12:00:01Z"),
		externalInteractionReprocessedFields("int-2001", "prt-2:production", "prt-2", "queryAsset", "",
			"1", "2", "INTERACTION_OUTCOME_UNCORRELATED", "INTERACTION_OUTCOME_SCHEMA_MISMATCH", "true", "false", "2026-09-02T12:00:02Z"),
		externalInteractionReprocessedFields("int-2002", "prt-2:homologation", "prt-2", "submitEffect", "prt-2:homologation:submitEffect:eff-4",
			"3", "4", "INTERACTION_OUTCOME_NORMALIZED", "INTERACTION_OUTCOME_NORMALIZED", "false", "false", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func externalInteractionReprocessedFields(interactionID, connectorID, partnerID, operation, commandID, previousRevision, revision, previousOutcome, outcome, changed, terminalAffected, at string) map[string]string {
	return map[string]string{
		"interaction_id":            interactionID,
		"connector_id":              connectorID,
		"partner_id":                partnerID,
		"operation":                 operation,
		"command_id":                commandID,
		"previous_revision":         previousRevision,
		"normalization_revision":    revision,
		"previous_outcome":          previousOutcome,
		"outcome":                   outcome,
		"outcome_changed":           changed,
		"terminal_command_affected": terminalAffected,
		"at":                        at,
	}
}

func externalInteractionReprocessed(fields map[string]string) (*eventv1.ExternalInteractionReprocessed, error) {
	previousRevision, err := parseInt32(fields, "previous_revision")
	if err != nil {
		return nil, err
	}
	revision, err := parseInt32(fields, "normalization_revision")
	if err != nil {
		return nil, err
	}
	previousOutcome, err := enumOf(eventv1.InteractionOutcome_value, fields, "previous_outcome")
	if err != nil {
		return nil, err
	}
	outcome, err := enumOf(eventv1.InteractionOutcome_value, fields, "outcome")
	if err != nil {
		return nil, err
	}
	changed, err := parseBool(fields, "outcome_changed")
	if err != nil {
		return nil, err
	}
	terminalAffected, err := parseBool(fields, "terminal_command_affected")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalInteractionReprocessed{
		InteractionId:           fields["interaction_id"],
		ConnectorId:             fields["connector_id"],
		PartnerId:               fields["partner_id"],
		Operation:               fields["operation"],
		CommandId:               fields["command_id"],
		PreviousRevision:        previousRevision,
		NormalizationRevision:   revision,
		PreviousOutcome:         eventv1.InteractionOutcome(previousOutcome),
		Outcome:                 eventv1.InteractionOutcome(outcome),
		OutcomeChanged:          changed,
		TerminalCommandAffected: terminalAffected,
		At:                      at,
	}, nil
}
