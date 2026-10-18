package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerOffboardingStartedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-offboarding-started.golden",
	Source:  fixtureSource,
	Subject: "prt-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-offboarding-started",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerOffboardingStarted"},
		Type:       "com.company.partner.partner-offboarding-started.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerOffboardingStarted",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.PartnerOffboardingStarted{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerOffboardingStarted(fields) },
	Build: leanFixture(
		partnerOffboardingStartedFields("prt-1001", "org-1001", "contract terminated", "2026-09-02T12:00:00Z"),
		partnerOffboardingStartedFields("prt-1002", "org-1002", "regulatory authorization revoked", "2026-09-02T12:00:01Z"),
		partnerOffboardingStartedFields("prt-2001", "org-2001", "partner request", "2026-09-02T12:00:02Z"),
		partnerOffboardingStartedFields("prt-2002", "org-2002", "merger", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func partnerOffboardingStartedFields(partnerID, organizationID, reason, at string) map[string]string {
	return map[string]string{
		"partner_id":      partnerID,
		"organization_id": organizationID,
		"reason":          reason,
		"at":              at,
	}
}

func partnerOffboardingStarted(fields map[string]string) (*eventv1.PartnerOffboardingStarted, error) {
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerOffboardingStarted{
		PartnerId:      fields["partner_id"],
		OrganizationId: fields["organization_id"],
		Reason:         fields["reason"],
		At:             at,
	}, nil
}
