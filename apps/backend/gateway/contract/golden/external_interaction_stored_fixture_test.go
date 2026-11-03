package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalInteractionStoredSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-interaction-stored.golden",
	Source:  fixtureSource,
	Subject: "int-1001",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-interaction-stored",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalInteractionStored"},
		Type:       "com.company.gateway.external-interaction-stored.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalInteractionStored",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "source", Values: []string{
		"INTERACTION_SOURCE_SUBMIT_RESPONSE", "INTERACTION_SOURCE_STATE_QUERY", "INTERACTION_SOURCE_CALLBACK",
		"INTERACTION_SOURCE_FILE", "INTERACTION_SOURCE_EXTERNAL_READ",
	}},
	New:        func() proto.Message { return &eventv1.ExternalInteractionStored{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalInteractionStored(fields) },
	Build: withEnumCoverage(
		leanFixture(
			externalInteractionStoredFields("int-1001", "prt-1:production", "submitEffect", "INTERACTION_DIRECTION_OUTBOUND",
				"INTERACTION_SOURCE_SUBMIT_RESPONSE", "prt-1:production:submitEffect:eff-1", "1", "2026-09-02T12:00:00Z"),
			externalInteractionStoredFields("int-1002", "prt-1:production", "submitEffect", "INTERACTION_DIRECTION_INBOUND",
				"INTERACTION_SOURCE_CALLBACK", "", "1", "2026-09-02T12:00:01Z"),
			externalInteractionStoredFields("int-2001", "prt-2:production", "submitEffect", "INTERACTION_DIRECTION_OUTBOUND",
				"INTERACTION_SOURCE_STATE_QUERY", "prt-2:production:submitEffect:eff-3", "2", "2026-09-02T12:00:02Z"),
			externalInteractionStoredFields("int-2002", "prt-2:homologation", "queryAsset", "INTERACTION_DIRECTION_OUTBOUND",
				"INTERACTION_SOURCE_EXTERNAL_READ", "", "3", "2026-09-02T12:00:03.123456789Z"),
		),
		externalInteractionStoredFields("int-3001", "prt-3:production", "submitEffect", "INTERACTION_DIRECTION_INBOUND",
			"INTERACTION_SOURCE_FILE", "", "1", "2026-09-02T12:00:04Z"),
	),
	WantCases:          3,
	WantDiscriminators: 2,
}

func externalInteractionStoredFields(interactionID, connectorID, operation, direction, source, commandID, revision, at string) map[string]string {
	return map[string]string{
		"interaction_id":         interactionID,
		"connector_id":           connectorID,
		"operation":              operation,
		"direction":              direction,
		"source":                 source,
		"command_id":             commandID,
		"normalization_revision": revision,
		"at":                     at,
	}
}

func externalInteractionStored(fields map[string]string) (*eventv1.ExternalInteractionStored, error) {
	direction, err := enumOf(eventv1.InteractionDirection_value, fields, "direction")
	if err != nil {
		return nil, err
	}
	source, err := enumOf(eventv1.InteractionSource_value, fields, "source")
	if err != nil {
		return nil, err
	}
	revision, err := parseInt32(fields, "normalization_revision")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalInteractionStored{
		InteractionId:         fields["interaction_id"],
		ConnectorId:           fields["connector_id"],
		Operation:             fields["operation"],
		Direction:             eventv1.InteractionDirection(direction),
		Source:                eventv1.InteractionSource(source),
		CommandId:             fields["command_id"],
		NormalizationRevision: revision,
		At:                    at,
	}, nil
}
