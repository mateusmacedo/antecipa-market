package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalOutcomeUnknownSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-outcome-unknown.golden",
	Source:  fixtureSource,
	Subject: "prt-1:production:submitEffect:eff-1",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-outcome-unknown",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalOutcomeUnknown"},
		Type:       "com.company.gateway.external-outcome-unknown.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalOutcomeUnknown",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "failure_category", Values: []string{
		"FAILURE_CATEGORY_FINAL_VALIDATION", "FAILURE_CATEGORY_FINAL_BUSINESS", "FAILURE_CATEGORY_FINAL_AUTHORIZATION",
		"FAILURE_CATEGORY_UNKNOWN_SENT", "FAILURE_CATEGORY_UNKNOWN_AMBIGUOUS", "FAILURE_CATEGORY_SCHEMA_MISMATCH",
	}},
	New:        func() proto.Message { return &eventv1.ExternalOutcomeUnknown{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalOutcomeUnknown(fields) },
	Build: withEnumCoverage(
		leanFixture(
			externalOutcomeUnknownFields("prt-1:production:submitEffect:eff-1", "tnt-1001", "prt-1:production", "submitEffect", "eff-1",
				"FAILURE_CATEGORY_UNKNOWN_SENT", "1", "2026-09-02T11:59:00Z", "true", "false", "2026-09-02T12:00:00Z"),
			externalOutcomeUnknownFields("prt-1:production:submitEffect:eff-2", "tnt-1001", "prt-1:production", "submitEffect", "eff-2",
				"FAILURE_CATEGORY_UNKNOWN_AMBIGUOUS", "2", "2026-09-02T11:59:30Z", "false", "true", "2026-09-02T12:00:01Z"),
			externalOutcomeUnknownFields("prt-2:production:cancelEffect:eff-3", "tnt-2001", "prt-2:production", "cancelEffect", "eff-3",
				"FAILURE_CATEGORY_SCHEMA_MISMATCH", "1", "2026-09-02T11:58:00Z", "true", "true", "2026-09-02T12:00:02Z"),
			externalOutcomeUnknownFields("prt-2:homologation:submitEffect:eff-4", "tnt-2001", "prt-2:homologation", "submitEffect", "eff-4",
				"FAILURE_CATEGORY_UNKNOWN_SENT", "3", "2026-09-02T11:59:59.123456789Z", "false", "false", "2026-09-02T12:00:03.123456789Z"),
		),
		externalOutcomeUnknownFields("prt-3:production:submitEffect:eff-5", "tnt-3001", "prt-3:production", "submitEffect", "eff-5",
			"FAILURE_CATEGORY_FINAL_VALIDATION", "1", "2026-09-02T11:59:00Z", "true", "false", "2026-09-02T12:00:04Z"),
		externalOutcomeUnknownFields("prt-3:production:submitEffect:eff-6", "tnt-3001", "prt-3:production", "submitEffect", "eff-6",
			"FAILURE_CATEGORY_FINAL_BUSINESS", "1", "2026-09-02T11:59:00Z", "true", "false", "2026-09-02T12:00:05Z"),
		externalOutcomeUnknownFields("prt-3:production:submitEffect:eff-7", "tnt-3001", "prt-3:production", "submitEffect", "eff-7",
			"FAILURE_CATEGORY_FINAL_AUTHORIZATION", "1", "2026-09-02T11:59:00Z", "true", "false", "2026-09-02T12:00:06Z"),
	),
	WantCases:          5,
	WantDiscriminators: 2,
}

func externalOutcomeUnknownFields(idempotencyKey, tenantID, connectorID, operation, businessKey, failureCategory, submissions, lastSubmittedAt, stateQuerySupported, requiresHumanResolution, unknownSince string) map[string]string {
	return map[string]string{
		"idempotency_key":           idempotencyKey,
		"tenant_id":                 tenantID,
		"connector_id":              connectorID,
		"operation":                 operation,
		"business_key":              businessKey,
		"failure_category":          failureCategory,
		"submissions":               submissions,
		"last_submitted_at":         lastSubmittedAt,
		"state_query_supported":     stateQuerySupported,
		"requires_human_resolution": requiresHumanResolution,
		"unknown_since":             unknownSince,
	}
}

func externalOutcomeUnknown(fields map[string]string) (*eventv1.ExternalOutcomeUnknown, error) {
	category, err := enumOf(eventv1.FailureCategory_value, fields, "failure_category")
	if err != nil {
		return nil, err
	}
	submissions, err := parseInt32(fields, "submissions")
	if err != nil {
		return nil, err
	}
	lastSubmittedAt, err := parseInstant(fields, "last_submitted_at")
	if err != nil {
		return nil, err
	}
	stateQuery, err := parseBool(fields, "state_query_supported")
	if err != nil {
		return nil, err
	}
	humanResolution, err := parseBool(fields, "requires_human_resolution")
	if err != nil {
		return nil, err
	}
	unknownSince, err := parseInstant(fields, "unknown_since")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalOutcomeUnknown{
		IdempotencyKey:          fields["idempotency_key"],
		TenantId:                fields["tenant_id"],
		ConnectorId:             fields["connector_id"],
		Operation:               fields["operation"],
		BusinessKey:             fields["business_key"],
		FailureCategory:         eventv1.FailureCategory(category),
		Submissions:             submissions,
		LastSubmittedAt:         lastSubmittedAt,
		StateQuerySupported:     stateQuery,
		RequiresHumanResolution: humanResolution,
		UnknownSince:            unknownSince,
	}, nil
}
