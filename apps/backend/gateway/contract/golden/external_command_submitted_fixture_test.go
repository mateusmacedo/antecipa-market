package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalCommandSubmittedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-command-submitted.golden",
	Source:  fixtureSource,
	Subject: "prt-1:production:submitEffect:eff-1",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-command-submitted",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalCommandSubmitted"},
		Type:       "com.company.gateway.external-command-submitted.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalCommandSubmitted",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.ExternalCommandSubmitted{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalCommandSubmitted(fields) },
	Build: leanFixture(
		externalCommandSubmittedFields("prt-1:production:submitEffect:eff-1", "tnt-1001", "prt-1:production", "submitEffect", "eff-1",
			"1", "2026-09-02T12:00:00Z"),
		externalCommandSubmittedFields("prt-1:production:submitEffect:eff-2", "tnt-1001", "prt-1:production", "submitEffect", "eff-2",
			"2", "2026-09-02T12:00:01Z"),
		externalCommandSubmittedFields("prt-2:production:cancelEffect:eff-3", "tnt-2001", "prt-2:production", "cancelEffect", "eff-3",
			"1", "2026-09-02T12:00:02Z"),
		externalCommandSubmittedFields("prt-2:homologation:submitEffect:eff-4", "tnt-2001", "prt-2:homologation", "submitEffect", "eff-4",
			"3", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func externalCommandSubmittedFields(idempotencyKey, tenantID, connectorID, operation, businessKey, submission, at string) map[string]string {
	return map[string]string{
		"idempotency_key": idempotencyKey,
		"tenant_id":       tenantID,
		"connector_id":    connectorID,
		"operation":       operation,
		"business_key":    businessKey,
		"submission":      submission,
		"at":              at,
	}
}

func externalCommandSubmitted(fields map[string]string) (*eventv1.ExternalCommandSubmitted, error) {
	submission, err := parseInt32(fields, "submission")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalCommandSubmitted{
		IdempotencyKey: fields["idempotency_key"],
		TenantId:       fields["tenant_id"],
		ConnectorId:    fields["connector_id"],
		Operation:      fields["operation"],
		BusinessKey:    fields["business_key"],
		Submission:     submission,
		At:             at,
	}, nil
}
