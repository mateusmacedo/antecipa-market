package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerRegisteredSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-registered.golden",
	Source:  fixtureSource,
	Subject: "prt-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-registered",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerRegistered"},
		Type:       "com.company.partner.partner-registered.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerRegistered",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "role", Values: []string{
		"PARTNER_ROLE_FUNDER", "PARTNER_ROLE_BOOKKEEPER", "PARTNER_ROLE_REGISTRY",
		"PARTNER_ROLE_DEPOSITORY", "PARTNER_ROLE_DATA_PROVIDER", "PARTNER_ROLE_SETTLEMENT_INSTITUTION",
	}},
	New:        func() proto.Message { return &eventv1.PartnerRegistered{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerRegistered(fields) },
	Build: withEnumCoverage(
		leanFixture(
			partnerRegisteredFields("prt-1001", "org-1001", "PARTNER_ROLE_FUNDER", "2026-09-02T12:00:00Z"),
			partnerRegisteredFields("prt-1002", "org-1002", "PARTNER_ROLE_BOOKKEEPER", "2026-09-02T12:00:01Z"),
			partnerRegisteredFields("prt-2001", "org-2001", "PARTNER_ROLE_REGISTRY", "2026-09-02T12:00:02Z"),
			partnerRegisteredFields("prt-2002", "org-2002", "PARTNER_ROLE_DATA_PROVIDER", "2026-09-02T12:00:03.123456789Z"),
		),
		partnerRegisteredFields("prt-3001", "org-3001", "PARTNER_ROLE_DEPOSITORY", "2026-09-02T12:00:04Z"),
		partnerRegisteredFields("prt-3002", "org-3001", "PARTNER_ROLE_SETTLEMENT_INSTITUTION", "2026-09-02T12:00:05Z"),
	),
	WantCases:          4,
	WantDiscriminators: 2,
}

func partnerRegisteredFields(partnerID, organizationID, role, at string) map[string]string {
	return map[string]string{
		"partner_id":      partnerID,
		"organization_id": organizationID,
		"role":            role,
		"at":              at,
	}
}

func partnerRegistered(fields map[string]string) (*eventv1.PartnerRegistered, error) {
	role, err := enumOf(eventv1.PartnerRole_value, fields, "role")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerRegistered{
		PartnerId:      fields["partner_id"],
		OrganizationId: fields["organization_id"],
		Role:           eventv1.PartnerRole(role),
		At:             at,
	}, nil
}
