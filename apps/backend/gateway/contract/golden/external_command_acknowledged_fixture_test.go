package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalCommandAcknowledgedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-command-acknowledged.golden",
	Source:  fixtureSource,
	Subject: "prt-1:production:submitEffect:eff-1",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-command-acknowledged",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalCommandAcknowledged"},
		Type:       "com.company.gateway.external-command-acknowledged.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalCommandAcknowledged",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.ExternalCommandAcknowledged{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalCommandAcknowledged(fields) },
	Build: leanFixture(
		externalCommandAcknowledgedFields("prt-1:production:submitEffect:eff-1", "tnt-1001", "prt-1:production", "submitEffect", "eff-1",
			"ext-1001", "2026-09-02T12:00:00Z"),
		externalCommandAcknowledgedFields("prt-1:production:submitEffect:eff-2", "tnt-1001", "prt-1:production", "submitEffect", "eff-2",
			"ext-1002", "2026-09-02T12:00:01Z"),
		externalCommandAcknowledgedFields("prt-2:production:cancelEffect:eff-3", "tnt-2001", "prt-2:production", "cancelEffect", "eff-3",
			"ext-2001", "2026-09-02T12:00:02Z"),
		externalCommandAcknowledgedFields("prt-2:homologation:submitEffect:eff-4", "tnt-2001", "prt-2:homologation", "submitEffect", "eff-4",
			"ext-2002", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func externalCommandAcknowledgedFields(idempotencyKey, tenantID, connectorID, operation, businessKey, externalReferenceID, at string) map[string]string {
	return map[string]string{
		"idempotency_key":       idempotencyKey,
		"tenant_id":             tenantID,
		"connector_id":          connectorID,
		"operation":             operation,
		"business_key":          businessKey,
		"external_reference_id": externalReferenceID,
		"at":                    at,
	}
}

func externalCommandAcknowledged(fields map[string]string) (*eventv1.ExternalCommandAcknowledged, error) {
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalCommandAcknowledged{
		IdempotencyKey:      fields["idempotency_key"],
		TenantId:            fields["tenant_id"],
		ConnectorId:         fields["connector_id"],
		Operation:           fields["operation"],
		BusinessKey:         fields["business_key"],
		ExternalReferenceId: fields["external_reference_id"],
		At:                  at,
	}, nil
}
