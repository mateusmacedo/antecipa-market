package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalCommandConfirmedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-command-confirmed.golden",
	Source:  fixtureSource,
	Subject: "prt-1:production:submitEffect:eff-1",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-command-confirmed",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalCommandConfirmed"},
		Type:       "com.company.gateway.external-command-confirmed.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalCommandConfirmed",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "confirmation_source", Values: []string{
		"CONFIRMATION_SOURCE_SUBMIT_RESPONSE", "CONFIRMATION_SOURCE_STATE_QUERY", "CONFIRMATION_SOURCE_CALLBACK",
		"CONFIRMATION_SOURCE_FILE", "CONFIRMATION_SOURCE_HUMAN",
	}},
	New:        func() proto.Message { return &eventv1.ExternalCommandConfirmed{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalCommandConfirmed(fields) },
	Build: withEnumCoverage(
		leanFixture(
			externalCommandConfirmedFields("prt-1:production:submitEffect:eff-1", "tnt-1001", "prt-1:production", "submitEffect", "eff-1",
				"ext-1001", "CONFIRMATION_SOURCE_SUBMIT_RESPONSE", "true", "int-1001", "2026-09-02T12:00:00Z"),
			externalCommandConfirmedFields("prt-1:production:submitEffect:eff-2", "tnt-1001", "prt-1:production", "submitEffect", "eff-2",
				"ext-1002", "CONFIRMATION_SOURCE_STATE_QUERY", "false", "int-1002", "2026-09-02T12:00:01Z"),
			externalCommandConfirmedFields("prt-2:production:cancelEffect:eff-3", "tnt-2001", "prt-2:production", "cancelEffect", "eff-3",
				"ext-2001", "CONFIRMATION_SOURCE_CALLBACK", "true", "int-2001", "2026-09-02T12:00:02Z"),
			externalCommandConfirmedFields("prt-2:homologation:submitEffect:eff-4", "tnt-2001", "prt-2:homologation", "submitEffect", "eff-4",
				"ext-2002", "CONFIRMATION_SOURCE_FILE", "false", "int-2002", "2026-09-02T12:00:03.123456789Z"),
		),
		externalCommandConfirmedFields("prt-3:production:submitEffect:eff-5", "tnt-3001", "prt-3:production", "submitEffect", "eff-5",
			"ext-3001", "CONFIRMATION_SOURCE_HUMAN", "false", "int-3001", "2026-09-02T12:00:04Z"),
	),
	WantCases:          3,
	WantDiscriminators: 2,
}

func externalCommandConfirmedFields(idempotencyKey, tenantID, connectorID, operation, businessKey, externalReferenceID, confirmationSource, partial, interactionID, confirmedAt string) map[string]string {
	return map[string]string{
		"idempotency_key":       idempotencyKey,
		"tenant_id":             tenantID,
		"connector_id":          connectorID,
		"operation":             operation,
		"business_key":          businessKey,
		"external_reference_id": externalReferenceID,
		"confirmation_source":   confirmationSource,
		"partial":               partial,
		"interaction_id":        interactionID,
		"confirmed_at":          confirmedAt,
	}
}

func externalCommandConfirmed(fields map[string]string) (*eventv1.ExternalCommandConfirmed, error) {
	source, err := enumOf(eventv1.ConfirmationSource_value, fields, "confirmation_source")
	if err != nil {
		return nil, err
	}
	partial, err := parseBool(fields, "partial")
	if err != nil {
		return nil, err
	}
	confirmedAt, err := parseInstant(fields, "confirmed_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalCommandConfirmed{
		IdempotencyKey:      fields["idempotency_key"],
		TenantId:            fields["tenant_id"],
		ConnectorId:         fields["connector_id"],
		Operation:           fields["operation"],
		BusinessKey:         fields["business_key"],
		ExternalReferenceId: fields["external_reference_id"],
		ConfirmationSource:  eventv1.ConfirmationSource(source),
		Partial:             partial,
		InteractionId:       fields["interaction_id"],
		ConfirmedAt:         confirmedAt,
	}, nil
}
