package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalCommandAmbiguityExpiredSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-command-ambiguity-expired.golden",
	Source:  fixtureSource,
	Subject: "prt-1:production:submitEffect:eff-1",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-command-ambiguity-expired",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalCommandAmbiguityExpired"},
		Type:       "com.company.gateway.external-command-ambiguity-expired.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalCommandAmbiguityExpired",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "status", Values: []string{
		"WAITING_STATUS_ACKNOWLEDGED", "WAITING_STATUS_UNKNOWN_OUTCOME",
	}},
	New:        func() proto.Message { return &eventv1.ExternalCommandAmbiguityExpired{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalCommandAmbiguityExpired(fields) },
	Build: leanFixture(
		externalCommandAmbiguityExpiredFields("prt-1:production:submitEffect:eff-1", "tnt-1001", "prt-1:production", "prt-1", "submitEffect", "eff-1",
			"WAITING_STATUS_ACKNOWLEDGED", "2026-09-02T10:00:00Z", "3600", "true", "2026-09-02T12:00:00Z"),
		externalCommandAmbiguityExpiredFields("prt-1:production:submitEffect:eff-2", "tnt-1001", "prt-1:production", "prt-1", "submitEffect", "eff-2",
			"WAITING_STATUS_UNKNOWN_OUTCOME", "2026-09-02T09:00:00Z", "7200", "false", "2026-09-02T12:00:01Z"),
		externalCommandAmbiguityExpiredFields("prt-2:production:cancelEffect:eff-3", "tnt-2001", "prt-2:production", "prt-2", "cancelEffect", "eff-3",
			"WAITING_STATUS_ACKNOWLEDGED", "2026-09-01T12:00:00Z", "86400", "true", "2026-09-02T12:00:02Z"),
		externalCommandAmbiguityExpiredFields("prt-2:homologation:submitEffect:eff-4", "tnt-2001", "prt-2:homologation", "prt-2", "submitEffect", "eff-4",
			"WAITING_STATUS_UNKNOWN_OUTCOME", "2026-09-02T11:30:00.123456789Z", "1800", "false", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func externalCommandAmbiguityExpiredFields(idempotencyKey, tenantID, connectorID, partnerID, operation, businessKey, status, waitingSince, ambiguityDeadline, stateQuerySupported, at string) map[string]string {
	return map[string]string{
		"idempotency_key":       idempotencyKey,
		"tenant_id":             tenantID,
		"connector_id":          connectorID,
		"partner_id":            partnerID,
		"operation":             operation,
		"business_key":          businessKey,
		"status":                status,
		"waiting_since":         waitingSince,
		"ambiguity_deadline":    ambiguityDeadline,
		"state_query_supported": stateQuerySupported,
		"at":                    at,
	}
}

func externalCommandAmbiguityExpired(fields map[string]string) (*eventv1.ExternalCommandAmbiguityExpired, error) {
	status, err := enumOf(eventv1.WaitingStatus_value, fields, "status")
	if err != nil {
		return nil, err
	}
	waitingSince, err := parseInstant(fields, "waiting_since")
	if err != nil {
		return nil, err
	}
	deadline, err := parseInt32(fields, "ambiguity_deadline")
	if err != nil {
		return nil, err
	}
	stateQuery, err := parseBool(fields, "state_query_supported")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalCommandAmbiguityExpired{
		IdempotencyKey:      fields["idempotency_key"],
		TenantId:            fields["tenant_id"],
		ConnectorId:         fields["connector_id"],
		PartnerId:           fields["partner_id"],
		Operation:           fields["operation"],
		BusinessKey:         fields["business_key"],
		Status:              eventv1.WaitingStatus(status),
		WaitingSince:        waitingSince,
		AmbiguityDeadline:   deadline,
		StateQuerySupported: stateQuery,
		At:                  at,
	}, nil
}
