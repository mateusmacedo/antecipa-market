package golden

import (
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
)

var membershipGrantedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/membership-granted.golden",
	Source:  fixtureSource,
	Subject: "mbr-1001",
	Identity: golden.Identity{
		Fixture:    "identity/event/v1/membership-granted",
		Contract:   golden.Contract{Package: "company.identity.event.v1", Message: "MembershipGranted"},
		Type:       "com.company.identity.membership-granted.v1",
		DataSchema: "type.googleapis.com/company.identity.event.v1.MembershipGranted",
	},
	Unknown:    unknownFieldNumber,
	Enum:       tb.Enum{Field: "role", Values: []string{"MEMBERSHIP_ROLE_OPERATOR", "MEMBERSHIP_ROLE_AUDITOR"}},
	New:        func() proto.Message { return &eventv1.MembershipGranted{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return membershipGranted(fields) },
	Build: leanFixture(
		membershipGrantedFields("mbr-1001", "org-1001", "sub-1001", "MEMBERSHIP_ROLE_OPERATOR",
			"VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED", "2026-09-02T12:00:00Z"),
		membershipGrantedFields("mbr-1002", "org-1001", "sub-1002", "MEMBERSHIP_ROLE_AUDITOR",
			"VERIFICATION_LEVEL_CHANNEL_OWNERSHIP", "2026-09-02T12:00:01Z"),
		membershipGrantedFields("mbr-2001", "org-2001", "sub-2001", "MEMBERSHIP_ROLE_AUDITOR",
			"VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED", "2026-09-02T12:00:02Z"),
		membershipGrantedFields("mbr-2002", "org-2001", "sub-2002", "MEMBERSHIP_ROLE_OPERATOR",
			"VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func membershipGrantedFields(membershipID, organizationID, userID, role, verificationLevel, grantedAt string) map[string]string {
	return map[string]string{
		"membership_id":      membershipID,
		"organization_id":    organizationID,
		"user_id":            userID,
		"role":               role,
		"verification_level": verificationLevel,
		"granted_at":         grantedAt,
	}
}

func membershipGranted(fields map[string]string) (*eventv1.MembershipGranted, error) {
	role, err := membershipRole(fields["role"])
	if err != nil {
		return nil, err
	}
	level, ok := eventv1.VerificationLevel_value[fields["verification_level"]]
	if !ok {
		return nil, fmt.Errorf("verification_level %q is not a VerificationLevel", fields["verification_level"])
	}
	grantedAt, err := parseInstant(fields, "granted_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.MembershipGranted{
		MembershipId:      fields["membership_id"],
		OrganizationId:    fields["organization_id"],
		UserId:            fields["user_id"],
		Role:              role,
		VerificationLevel: eventv1.VerificationLevel(level),
		GrantedAt:         grantedAt,
	}, nil
}

func membershipRole(name string) (eventv1.MembershipRole, error) {
	role, ok := eventv1.MembershipRole_value[name]
	if !ok {
		return 0, fmt.Errorf("role %q is not a MembershipRole", name)
	}
	return eventv1.MembershipRole(role), nil
}
