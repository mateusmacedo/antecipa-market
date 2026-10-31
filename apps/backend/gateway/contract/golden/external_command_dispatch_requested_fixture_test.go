package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalCommandDispatchRequestedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-command-dispatch-requested.golden",
	Source:  fixtureSource,
	Subject: "prt-1:production:submitEffect:eff-1",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-command-dispatch-requested",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalCommandDispatchRequested"},
		Type:       "com.company.gateway.external-command-dispatch-requested.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalCommandDispatchRequested",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.ExternalCommandDispatchRequested{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalCommandDispatchRequested(fields) },
	Build: leanFixture(
		externalCommandDispatchRequestedFields("prt-1:production:submitEffect:eff-1", "prt-1:production", "submitEffect", "2026-09-02T12:00:00Z"),
		externalCommandDispatchRequestedFields("prt-1:production:submitEffect:eff-2", "prt-1:production", "submitEffect", "2026-09-02T12:00:01Z"),
		externalCommandDispatchRequestedFields("prt-2:production:cancelEffect:eff-3", "prt-2:production", "cancelEffect", "2026-09-02T12:00:02Z"),
		externalCommandDispatchRequestedFields("prt-2:homologation:submitEffect:eff-4", "prt-2:homologation", "submitEffect", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func externalCommandDispatchRequestedFields(idempotencyKey, connectorID, operation, at string) map[string]string {
	return map[string]string{
		"idempotency_key": idempotencyKey,
		"connector_id":    connectorID,
		"operation":       operation,
		"at":              at,
	}
}

func externalCommandDispatchRequested(fields map[string]string) (*eventv1.ExternalCommandDispatchRequested, error) {
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalCommandDispatchRequested{
		IdempotencyKey: fields["idempotency_key"],
		ConnectorId:    fields["connector_id"],
		Operation:      fields["operation"],
		At:             at,
	}, nil
}
