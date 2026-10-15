package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
)

var membershipRevokedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/membership-revoked.golden",
	Source:  fixtureSource,
	Subject: "mbr-1001",
	Identity: golden.Identity{
		Fixture:    "identity/event/v1/membership-revoked",
		Contract:   golden.Contract{Package: "company.identity.event.v1", Message: "MembershipRevoked"},
		Type:       "com.company.identity.membership-revoked.v1",
		DataSchema: "type.googleapis.com/company.identity.event.v1.MembershipRevoked",
	},
	Unknown:    unknownFieldNumber,
	Enum:       tb.Enum{Field: "role", Values: []string{"MEMBERSHIP_ROLE_OPERATOR", "MEMBERSHIP_ROLE_AUDITOR"}},
	New:        func() proto.Message { return &eventv1.MembershipRevoked{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return membershipRevoked(fields) },
	Build: leanFixture(
		membershipRevokedFields("mbr-1001", "org-1001", "sub-1001", "MEMBERSHIP_ROLE_OPERATOR", "left the company", "2026-09-02T12:00:00Z"),
		membershipRevokedFields("mbr-1002", "org-1001", "sub-1002", "MEMBERSHIP_ROLE_AUDITOR", "audit concluded", "2026-09-02T12:00:01Z"),
		membershipRevokedFields("mbr-2001", "org-2001", "sub-2001", "MEMBERSHIP_ROLE_AUDITOR", "role changed", "2026-09-02T12:00:02Z"),
		membershipRevokedFields("mbr-2002", "org-2001", "sub-2002", "MEMBERSHIP_ROLE_OPERATOR", "access review", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func membershipRevokedFields(membershipID, organizationID, userID, role, reason, revokedAt string) map[string]string {
	return map[string]string{
		"membership_id":   membershipID,
		"organization_id": organizationID,
		"user_id":         userID,
		"role":            role,
		"reason":          reason,
		"revoked_at":      revokedAt,
	}
}

func membershipRevoked(fields map[string]string) (*eventv1.MembershipRevoked, error) {
	role, err := membershipRole(fields["role"])
	if err != nil {
		return nil, err
	}
	revokedAt, err := parseInstant(fields, "revoked_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.MembershipRevoked{
		MembershipId:   fields["membership_id"],
		OrganizationId: fields["organization_id"],
		UserId:         fields["user_id"],
		Role:           role,
		Reason:         fields["reason"],
		RevokedAt:      revokedAt,
	}, nil
}
