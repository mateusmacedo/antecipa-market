package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var capabilitySuspendedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/capability-suspended.golden",
	Source:  fixtureSource,
	Subject: "cap-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/capability-suspended",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "CapabilitySuspended"},
		Type:       "com.company.partner.capability-suspended.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.CapabilitySuspended",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "trigger", Values: []string{
		"SUSPENSION_TRIGGER_MANUAL", "SUSPENSION_TRIGGER_SCHEMA_MISMATCH", "SUSPENSION_TRIGGER_OFFBOARDING",
	}},
	New:        func() proto.Message { return &eventv1.CapabilitySuspended{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return capabilitySuspended(fields) },
	Build: leanFixture(
		capabilitySuspendedFields("cap-1001", "prt-1001", "register_effect", "duplicata", "ENVIRONMENT_PRODUCTION", "1",
			"incident under analysis", "SUSPENSION_TRIGGER_MANUAL", "2026-09-02T12:00:00Z"),
		capabilitySuspendedFields("cap-1002", "prt-1001", "register_effect", "cdca", "ENVIRONMENT_HOMOLOGATION", "2",
			"schema of the response changed", "SUSPENSION_TRIGGER_SCHEMA_MISMATCH", "2026-09-02T12:00:01Z"),
		capabilitySuspendedFields("cap-2001", "prt-2001", "query_assets", "duplicata", "ENVIRONMENT_INTEGRATION", "1",
			"partner offboarding", "SUSPENSION_TRIGGER_OFFBOARDING", "2026-09-02T12:00:02Z"),
		capabilitySuspendedFields("cap-2002", "prt-2001", "query_assets", "cpr", "ENVIRONMENT_DEVELOPMENT", "3",
			"maintenance window", "SUSPENSION_TRIGGER_MANUAL", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func capabilitySuspendedFields(capabilityID, partnerID, operation, assetType, environment, version, reason, trigger, at string) map[string]string {
	return map[string]string{
		"capability_id": capabilityID,
		"partner_id":    partnerID,
		"operation":     operation,
		"asset_type":    assetType,
		"environment":   environment,
		"version":       version,
		"reason":        reason,
		"trigger":       trigger,
		"at":            at,
	}
}

func capabilitySuspended(fields map[string]string) (*eventv1.CapabilitySuspended, error) {
	environment, err := enumOf(eventv1.Environment_value, fields, "environment")
	if err != nil {
		return nil, err
	}
	version, err := parseInt32(fields, "version")
	if err != nil {
		return nil, err
	}
	trigger, err := enumOf(eventv1.SuspensionTrigger_value, fields, "trigger")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.CapabilitySuspended{
		CapabilityId: fields["capability_id"],
		PartnerId:    fields["partner_id"],
		Operation:    fields["operation"],
		AssetType:    fields["asset_type"],
		Environment:  eventv1.Environment(environment),
		Version:      version,
		Reason:       fields["reason"],
		Trigger:      eventv1.SuspensionTrigger(trigger),
		At:           at,
	}, nil
}
