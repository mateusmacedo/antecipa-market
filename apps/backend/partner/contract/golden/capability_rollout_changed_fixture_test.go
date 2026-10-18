package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var capabilityRolloutChangedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/capability-rollout-changed.golden",
	Source:  fixtureSource,
	Subject: "cap-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/capability-rollout-changed",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "CapabilityRolloutChanged"},
		Type:       "com.company.partner.capability-rollout-changed.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.CapabilityRolloutChanged",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "mode", Values: []string{
		"ROLLOUT_MODE_DISABLED", "ROLLOUT_MODE_ALLOWLIST", "ROLLOUT_MODE_PERCENTAGE", "ROLLOUT_MODE_ENABLED",
	}},
	New:        func() proto.Message { return &eventv1.CapabilityRolloutChanged{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return capabilityRolloutChanged(fields) },
	Build: leanFixture(
		capabilityRolloutChangedFields("cap-1001", "prt-1001", "register_effect", "duplicata", "ENVIRONMENT_PRODUCTION", "1",
			"ROLLOUT_MODE_PERCENTAGE", "10", "2026-09-02T12:00:00Z"),
		capabilityRolloutChangedFields("cap-1002", "prt-1001", "register_effect", "cdca", "ENVIRONMENT_HOMOLOGATION", "2",
			"ROLLOUT_MODE_ALLOWLIST", "0", "2026-09-02T12:00:01Z"),
		capabilityRolloutChangedFields("cap-2001", "prt-2001", "query_assets", "duplicata", "ENVIRONMENT_INTEGRATION", "1",
			"ROLLOUT_MODE_ENABLED", "0", "2026-09-02T12:00:02Z"),
		capabilityRolloutChangedFields("cap-2002", "prt-2001", "query_assets", "cpr", "ENVIRONMENT_DEVELOPMENT", "3",
			"ROLLOUT_MODE_DISABLED", "0", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func capabilityRolloutChangedFields(capabilityID, partnerID, operation, assetType, environment, version, mode, percentage, at string) map[string]string {
	return map[string]string{
		"capability_id": capabilityID,
		"partner_id":    partnerID,
		"operation":     operation,
		"asset_type":    assetType,
		"environment":   environment,
		"version":       version,
		"mode":          mode,
		"percentage":    percentage,
		"at":            at,
	}
}

func capabilityRolloutChanged(fields map[string]string) (*eventv1.CapabilityRolloutChanged, error) {
	environment, err := enumOf(eventv1.Environment_value, fields, "environment")
	if err != nil {
		return nil, err
	}
	version, err := parseInt32(fields, "version")
	if err != nil {
		return nil, err
	}
	mode, err := enumOf(eventv1.RolloutMode_value, fields, "mode")
	if err != nil {
		return nil, err
	}
	percentage, err := parseInt32(fields, "percentage")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.CapabilityRolloutChanged{
		CapabilityId: fields["capability_id"],
		PartnerId:    fields["partner_id"],
		Operation:    fields["operation"],
		AssetType:    fields["asset_type"],
		Environment:  eventv1.Environment(environment),
		Version:      version,
		Mode:         eventv1.RolloutMode(mode),
		Percentage:   percentage,
		At:           at,
	}, nil
}
