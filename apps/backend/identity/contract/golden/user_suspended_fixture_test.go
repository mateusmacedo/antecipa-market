package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
)

var userSuspendedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/user-suspended.golden",
	Source:  fixtureSource,
	Subject: "sub-1001",
	Identity: golden.Identity{
		Fixture:    "identity/event/v1/user-suspended",
		Contract:   golden.Contract{Package: "company.identity.event.v1", Message: "UserSuspended"},
		Type:       "com.company.identity.user-suspended.v1",
		DataSchema: "type.googleapis.com/company.identity.event.v1.UserSuspended",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.UserSuspended{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return userSuspended(fields) },
	Build: leanFixture(
		userSuspendedFields("sub-1001", "fraud review", "2026-09-02T12:00:00Z"),
		userSuspendedFields("sub-1002", "left the company", "2026-09-02T12:00:01Z"),
		userSuspendedFields("sub-2001", "credential leak", "2026-09-02T12:00:02Z"),
		userSuspendedFields("sub-2002", "access review", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func userSuspendedFields(userID, reason, suspendedAt string) map[string]string {
	return map[string]string{
		"user_id":      userID,
		"reason":       reason,
		"suspended_at": suspendedAt,
	}
}

func userSuspended(fields map[string]string) (*eventv1.UserSuspended, error) {
	suspendedAt, err := parseInstant(fields, "suspended_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.UserSuspended{
		UserId:      fields["user_id"],
		Reason:      fields["reason"],
		SuspendedAt: suspendedAt,
	}, nil
}
