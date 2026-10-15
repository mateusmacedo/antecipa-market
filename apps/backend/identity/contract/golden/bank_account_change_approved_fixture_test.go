package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
)

var bankAccountChangeApprovedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/bank-account-change-approved.golden",
	Source:  fixtureSource,
	Subject: "bac-1001",
	Identity: golden.Identity{
		Fixture:    "identity/event/v1/bank-account-change-approved",
		Contract:   golden.Contract{Package: "company.identity.event.v1", Message: "BankAccountChangeApproved"},
		Type:       "com.company.identity.bank-account-change-approved.v1",
		DataSchema: "type.googleapis.com/company.identity.event.v1.BankAccountChangeApproved",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.BankAccountChangeApproved{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return bankAccountChangeApproved(fields) },
	Build: leanFixture(
		bankAccountChangeApprovedFields("bac-1001", "org-1001", "sub-1002", "2026-09-02T12:00:00Z"),
		bankAccountChangeApprovedFields("bac-1002", "org-1001", "sub-1001", "2026-09-02T12:00:01Z"),
		bankAccountChangeApprovedFields("bac-2001", "org-2001", "sub-2002", "2026-09-02T12:00:02Z"),
		bankAccountChangeApprovedFields("bac-2002", "org-2001", "sub-2001", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func bankAccountChangeApprovedFields(bankAccountChangeID, organizationID, approvedByUserID, approvedAt string) map[string]string {
	return map[string]string{
		"bank_account_change_id": bankAccountChangeID,
		"organization_id":        organizationID,
		"approved_by_user_id":    approvedByUserID,
		"approved_at":            approvedAt,
	}
}

func bankAccountChangeApproved(fields map[string]string) (*eventv1.BankAccountChangeApproved, error) {
	approvedAt, err := parseInstant(fields, "approved_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.BankAccountChangeApproved{
		BankAccountChangeId: fields["bank_account_change_id"],
		OrganizationId:      fields["organization_id"],
		ApprovedByUserId:    fields["approved_by_user_id"],
		ApprovedAt:          approvedAt,
	}, nil
}
