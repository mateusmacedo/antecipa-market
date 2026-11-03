package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalInteractionUncorrelatedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-interaction-uncorrelated.golden",
	Source:  fixtureSource,
	Subject: "int-1001",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-interaction-uncorrelated",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalInteractionUncorrelated"},
		Type:       "com.company.gateway.external-interaction-uncorrelated.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalInteractionUncorrelated",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "environment", Values: []string{
		"ENVIRONMENT_DEVELOPMENT", "ENVIRONMENT_INTEGRATION", "ENVIRONMENT_HOMOLOGATION", "ENVIRONMENT_PRODUCTION",
	}},
	New:        func() proto.Message { return &eventv1.ExternalInteractionUncorrelated{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalInteractionUncorrelated(fields) },
	Build: leanFixture(
		externalInteractionUncorrelatedFields("int-1001", "prt-1:production", "prt-1", "ENVIRONMENT_PRODUCTION", "submitEffect",
			"evt-p-1", "2026-09-02", "2026-09-02T11:59:59Z", "2026-09-02T12:00:00Z"),
		externalInteractionUncorrelatedFields("int-1002", "prt-1:homologation", "prt-1", "ENVIRONMENT_HOMOLOGATION", "queryAsset",
			"", "", "2026-09-02T12:00:00Z", "2026-09-02T12:00:01Z"),
		externalInteractionUncorrelatedFields("int-2001", "prt-2:integration", "prt-2", "ENVIRONMENT_INTEGRATION", "requestRelease",
			"evt-p-2", "2026-09-01", "2026-09-02T12:00:01Z", "2026-09-02T12:00:02Z"),
		externalInteractionUncorrelatedFields("int-2002", "prt-2:development", "prt-2", "ENVIRONMENT_DEVELOPMENT", "submitEffect",
			"evt-p-3", "2026-09-02", "2026-09-02T12:00:02.5Z", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func externalInteractionUncorrelatedFields(interactionID, connectorID, partnerID, environment, operation, partnerEventID, businessDate, receivedAt, at string) map[string]string {
	return map[string]string{
		"interaction_id":   interactionID,
		"connector_id":     connectorID,
		"connector_key":    connectorID,
		"partner_id":       partnerID,
		"environment":      environment,
		"operation":        operation,
		"partner_event_id": partnerEventID,
		"business_date":    businessDate,
		"received_at":      receivedAt,
		"at":               at,
	}
}

func externalInteractionUncorrelated(fields map[string]string) (*eventv1.ExternalInteractionUncorrelated, error) {
	environment, err := enumOf(eventv1.Environment_value, fields, "environment")
	if err != nil {
		return nil, err
	}
	receivedAt, err := parseInstant(fields, "received_at")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalInteractionUncorrelated{
		InteractionId:  fields["interaction_id"],
		ConnectorId:    fields["connector_id"],
		ConnectorKey:   fields["connector_key"],
		PartnerId:      fields["partner_id"],
		Environment:    eventv1.Environment(environment),
		Operation:      fields["operation"],
		PartnerEventId: fields["partner_event_id"],
		BusinessDate:   fields["business_date"],
		ReceivedAt:     receivedAt,
		At:             at,
	}, nil
}
