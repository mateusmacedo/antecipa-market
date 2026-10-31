package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalCommandFailedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-command-failed.golden",
	Source:  fixtureSource,
	Subject: "prt-1:production:submitEffect:eff-1",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-command-failed",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalCommandFailed"},
		Type:       "com.company.gateway.external-command-failed.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalCommandFailed",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "category", Values: []string{
		"FAILURE_CATEGORY_FINAL_VALIDATION", "FAILURE_CATEGORY_FINAL_BUSINESS", "FAILURE_CATEGORY_FINAL_AUTHORIZATION",
		"FAILURE_CATEGORY_UNKNOWN_SENT", "FAILURE_CATEGORY_UNKNOWN_AMBIGUOUS", "FAILURE_CATEGORY_SCHEMA_MISMATCH",
	}},
	New:        func() proto.Message { return &eventv1.ExternalCommandFailed{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalCommandFailed(fields) },
	Build: withEnumCoverage(
		leanFixture(
			externalCommandFailedFields("prt-1:production:submitEffect:eff-1", "tnt-1001", "prt-1:production", "submitEffect", "eff-1",
				"FAILURE_CATEGORY_FINAL_BUSINESS", "E42", "registro recusado", "int-1001", "2026-09-02T12:00:00Z"),
			externalCommandFailedFields("prt-1:production:submitEffect:eff-2", "tnt-1001", "prt-1:production", "submitEffect", "eff-2",
				"FAILURE_CATEGORY_FINAL_VALIDATION", "E10", "invalid asset code", "int-1002", "2026-09-02T12:00:01Z"),
			externalCommandFailedFields("prt-2:production:cancelEffect:eff-3", "tnt-2001", "prt-2:production", "cancelEffect", "eff-3",
				"FAILURE_CATEGORY_FINAL_AUTHORIZATION", "E401", "credential rejected", "int-2001", "2026-09-02T12:00:02Z"),
			externalCommandFailedFields("prt-2:homologation:submitEffect:eff-4", "tnt-2001", "prt-2:homologation", "submitEffect", "eff-4",
				"FAILURE_CATEGORY_SCHEMA_MISMATCH", "", "", "int-2002", "2026-09-02T12:00:03.123456789Z"),
		),
		externalCommandFailedFields("prt-3:production:submitEffect:eff-5", "tnt-3001", "prt-3:production", "submitEffect", "eff-5",
			"FAILURE_CATEGORY_UNKNOWN_SENT", "E503", "upstream timeout", "int-3001", "2026-09-02T12:00:04Z"),
		externalCommandFailedFields("prt-3:production:submitEffect:eff-6", "tnt-3001", "prt-3:production", "submitEffect", "eff-6",
			"FAILURE_CATEGORY_UNKNOWN_AMBIGUOUS", "E500", "connection reset", "int-3002", "2026-09-02T12:00:05Z"),
	),
	WantCases:          4,
	WantDiscriminators: 2,
}

func externalCommandFailedFields(idempotencyKey, tenantID, connectorID, operation, businessKey, category, partnerCode, partnerMessage, interactionID, at string) map[string]string {
	return map[string]string{
		"idempotency_key": idempotencyKey,
		"tenant_id":       tenantID,
		"connector_id":    connectorID,
		"operation":       operation,
		"business_key":    businessKey,
		"category":        category,
		"partner_code":    partnerCode,
		"partner_message": partnerMessage,
		"interaction_id":  interactionID,
		"at":              at,
	}
}

func externalCommandFailed(fields map[string]string) (*eventv1.ExternalCommandFailed, error) {
	category, err := enumOf(eventv1.FailureCategory_value, fields, "category")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalCommandFailed{
		IdempotencyKey: fields["idempotency_key"],
		TenantId:       fields["tenant_id"],
		ConnectorId:    fields["connector_id"],
		Operation:      fields["operation"],
		BusinessKey:    fields["business_key"],
		Category:       eventv1.FailureCategory(category),
		PartnerCode:    fields["partner_code"],
		PartnerMessage: fields["partner_message"],
		InteractionId:  fields["interaction_id"],
		At:             at,
	}, nil
}
