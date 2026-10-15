package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
)

var bankAccountChangeRequestedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/bank-account-change-requested.golden",
	Source:  fixtureSource,
	Subject: "bac-1001",
	Identity: golden.Identity{
		Fixture:    "identity/event/v1/bank-account-change-requested",
		Contract:   golden.Contract{Package: "company.identity.event.v1", Message: "BankAccountChangeRequested"},
		Type:       "com.company.identity.bank-account-change-requested.v1",
		DataSchema: "type.googleapis.com/company.identity.event.v1.BankAccountChangeRequested",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.BankAccountChangeRequested{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return bankAccountChangeRequested(fields) },
	Build: leanFixture(
		bankAccountChangeRequestedFields("bac-1001", "org-1001", "evd-1001", "sub-1001", "2026-09-02T12:00:00Z"),
		bankAccountChangeRequestedFields("bac-1002", "org-1001", "evd-1002", "sub-1002", "2026-09-02T12:00:01Z"),
		bankAccountChangeRequestedFields("bac-2001", "org-2001", "evd-2001", "sub-2001", "2026-09-02T12:00:02Z"),
		bankAccountChangeRequestedFields("bac-2002", "org-2001", "evd-2002", "sub-2002", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func bankAccountChangeRequestedFields(bankAccountChangeID, organizationID, destinationEvidenceID, requestedByUserID, requestedAt string) map[string]string {
	return map[string]string{
		"bank_account_change_id":  bankAccountChangeID,
		"organization_id":         organizationID,
		"destination_evidence_id": destinationEvidenceID,
		"requested_by_user_id":    requestedByUserID,
		"requested_at":            requestedAt,
	}
}

func bankAccountChangeRequested(fields map[string]string) (*eventv1.BankAccountChangeRequested, error) {
	requestedAt, err := parseInstant(fields, "requested_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.BankAccountChangeRequested{
		BankAccountChangeId:   fields["bank_account_change_id"],
		OrganizationId:        fields["organization_id"],
		DestinationEvidenceId: fields["destination_evidence_id"],
		RequestedByUserId:     fields["requested_by_user_id"],
		RequestedAt:           requestedAt,
	}, nil
}
