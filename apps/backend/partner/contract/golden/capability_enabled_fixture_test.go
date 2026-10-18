package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var capabilityEnabledSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/capability-enabled.golden",
	Source:  fixtureSource,
	Subject: "cap-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/capability-enabled",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "CapabilityEnabled"},
		Type:       "com.company.partner.capability-enabled.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.CapabilityEnabled",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "environment", Values: []string{
		"ENVIRONMENT_DEVELOPMENT", "ENVIRONMENT_INTEGRATION", "ENVIRONMENT_HOMOLOGATION", "ENVIRONMENT_PRODUCTION",
	}},
	New:        func() proto.Message { return &eventv1.CapabilityEnabled{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return capabilityEnabled(fields) },
	Build: leanFixture(
		capabilityEnabledFields("cap-1001", "prt-1001", "register_effect", "duplicata", "ENVIRONMENT_PRODUCTION", "1",
			"ROLLOUT_MODE_DISABLED", "true", "", "2026-09-02T12:00:00Z"),
		capabilityEnabledFields("cap-1002", "prt-1001", "register_effect", "cdca", "ENVIRONMENT_HOMOLOGATION", "2",
			"ROLLOUT_MODE_PERCENTAGE", "false", "write without state query, reconciled daily", "2026-09-02T12:00:01Z"),
		capabilityEnabledFields("cap-2001", "prt-2001", "query_assets", "duplicata", "ENVIRONMENT_INTEGRATION", "1",
			"ROLLOUT_MODE_ALLOWLIST", "true", "", "2026-09-02T12:00:02Z"),
		capabilityEnabledFields("cap-2002", "prt-2001", "query_assets", "cpr", "ENVIRONMENT_DEVELOPMENT", "3",
			"ROLLOUT_MODE_ENABLED", "false", "read only, no state to query", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func capabilityEnabledFields(capabilityID, partnerID, operation, assetType, environment, version, rolloutMode, stateQuerySupported, restrictionNote, at string) map[string]string {
	return map[string]string{
		"capability_id":         capabilityID,
		"partner_id":            partnerID,
		"operation":             operation,
		"asset_type":            assetType,
		"environment":           environment,
		"version":               version,
		"rollout_mode":          rolloutMode,
		"state_query_supported": stateQuerySupported,
		"restriction_note":      restrictionNote,
		"at":                    at,
	}
}

func capabilityEnabled(fields map[string]string) (*eventv1.CapabilityEnabled, error) {
	environment, err := enumOf(eventv1.Environment_value, fields, "environment")
	if err != nil {
		return nil, err
	}
	version, err := parseInt32(fields, "version")
	if err != nil {
		return nil, err
	}
	mode, err := enumOf(eventv1.RolloutMode_value, fields, "rollout_mode")
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
	return &eventv1.CapabilityEnabled{
		CapabilityId:        fields["capability_id"],
		PartnerId:           fields["partner_id"],
		Operation:           fields["operation"],
		AssetType:           fields["asset_type"],
		Environment:         eventv1.Environment(environment),
		Version:             version,
		RolloutMode:         eventv1.RolloutMode(mode),
		StateQuerySupported: stateQuery,
		RestrictionNote:     fields["restriction_note"],
		At:                  at,
	}, nil
}
