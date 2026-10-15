package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	bankAccountChangeID   = domain.BankAccountChangeID("bac-100")
	destinationEvidenceID = "evd-200"
	requesterID           = "user-sub-1"
	approverID            = "user-sub-2"
	approvedAt            = at + 86_400_000_000_000
)

func requestChangeCommand() domain.RequestBankAccountChange {
	return domain.RequestBankAccountChange{
		OrganizationID:        organizationID,
		OrganizationStatus:    domain.OrganizationActive,
		DestinationEvidenceID: destinationEvidenceID,
		RequestedByUserID:     requesterID,
		At:                    at,
	}
}

func requestedChange() domain.BankAccountChangeSnapshot {
	return domain.BankAccountChangeSnapshot{
		ID:                    bankAccountChangeID,
		OrganizationID:        organizationID,
		DestinationEvidenceID: destinationEvidenceID,
		RequestedByUserID:     requesterID,
		Status:                domain.BankAccountChangeStatusRequested,
		RequestedAt:           at,
	}
}

func approveChangeCommand() domain.ApproveBankAccountChange {
	return domain.ApproveBankAccountChange{ApprovedByUserID: approverID, At: approvedAt}
}

func approvedChange() domain.BankAccountChangeSnapshot {
	s := requestedChange()
	s.Status = domain.BankAccountChangeStatusApproved
	s.ApprovedByUserID = approverID
	s.ApprovedAt = approvedAt
	return s
}

func TestRequestBankAccountChangeRequestsItAndEmitsTheRequest(t *testing.T) {
	c := domain.NewBankAccountChange(bankAccountChangeID)

	acc, rej := c.Request(requestChangeCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RequestBankAccountChangeResponse{BankAccountChangeID: bankAccountChangeID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	tb.RequireSameEvents(t, acc.Events(), []kernel.DomainEvent{domain.BankAccountChangeRequested{
		BankAccountChangeID:   bankAccountChangeID,
		OrganizationID:        organizationID,
		DestinationEvidenceID: destinationEvidenceID,
		RequestedByUserID:     requesterID,
		At:                    at,
	}})
	if got := c.Snapshot(); !got.Equal(requestedChange()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, requestedChange())
	}
}

func TestRequestBankAccountChangeRejectsEachPrecondition(t *testing.T) {
	organization := kernel.Detail{Key: "organization", Value: string(organizationID)}
	for _, tc := range []struct {
		name    string
		mutate  func(*domain.RequestBankAccountChange)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"destination evidence empty", func(c *domain.RequestBankAccountChange) { c.DestinationEvidenceID = "" },
			domain.CodeBankAccountChangeDestinationEvidenceMissing, nil},
		{"destination evidence of blanks", func(c *domain.RequestBankAccountChange) { c.DestinationEvidenceID = "  " },
			domain.CodeBankAccountChangeDestinationEvidenceMissing, nil},
		{"organization not found", func(c *domain.RequestBankAccountChange) { c.OrganizationStatus = domain.OrganizationNew },
			domain.CodeBankAccountChangeOrganizationNotActive, []kernel.Detail{organization}},
		{"organization suspended", func(c *domain.RequestBankAccountChange) { c.OrganizationStatus = domain.OrganizationSuspended },
			domain.CodeBankAccountChangeOrganizationNotActive, []kernel.Detail{organization}},
		{"organization offboarded", func(c *domain.RequestBankAccountChange) { c.OrganizationStatus = domain.OrganizationOffboarded },
			domain.CodeBankAccountChangeOrganizationNotActive, []kernel.Detail{organization}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := requestChangeCommand()
			tc.mutate(&cmd)
			c := domain.NewBankAccountChange(bankAccountChangeID)
			before := c.Snapshot()

			acc, rej := c.Request(cmd)

			tb.RequireRejected(t, acc, rej, tc.code)
			if got := rej.Details(); !slices.Equal(got, tc.details) {
				t.Fatalf("Details() = %v, want %v", got, tc.details)
			}
			if after := c.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestRequestBankAccountChangeReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.RequestBankAccountChange{OrganizationID: organizationID, RequestedByUserID: requesterID, At: at}

	acc, rej := domain.NewBankAccountChange(bankAccountChangeID).Request(cmd)
	tb.RequireRejected(t, acc, rej, domain.CodeBankAccountChangeDestinationEvidenceMissing)

	cmd.DestinationEvidenceID = destinationEvidenceID
	acc, rej = domain.NewBankAccountChange(bankAccountChangeID).Request(cmd)
	tb.RequireRejected(t, acc, rej, domain.CodeBankAccountChangeOrganizationNotActive)

	cmd.OrganizationStatus = domain.OrganizationActive
	if _, rej := domain.NewBankAccountChange(bankAccountChangeID).Request(cmd); rej != nil {
		t.Fatalf("Request() after every fix rejected with %s", rej.Code())
	}
}

func TestApproveBankAccountChangeApprovesItAndEmitsTheApproval(t *testing.T) {
	c := domain.FromBankAccountChangeSnapshot(requestedChange())

	acc, rej := c.Approve(approveChangeCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.ApproveBankAccountChangeResponse{BankAccountChangeID: bankAccountChangeID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	tb.RequireSameEvents(t, acc.Events(), []kernel.DomainEvent{domain.BankAccountChangeApproved{
		BankAccountChangeID: bankAccountChangeID,
		OrganizationID:      organizationID,
		ApprovedByUserID:    approverID,
		At:                  approvedAt,
	}})
	if got := c.Snapshot(); !got.Equal(approvedChange()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, approvedChange())
	}
}

func TestApproveBankAccountChangeRejectsEachPrecondition(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   domain.BankAccountChangeSnapshot
		mutate func(*domain.ApproveBankAccountChange)
		code   kernel.Code
	}{
		{"change already approved", approvedChange(), func(*domain.ApproveBankAccountChange) {}, domain.CodeBankAccountChangeNotRequested},
		{"change never requested", domain.NewBankAccountChange(bankAccountChangeID).Snapshot(), func(*domain.ApproveBankAccountChange) {},
			domain.CodeBankAccountChangeNotRequested},
		{"approved by the requester", requestedChange(), func(c *domain.ApproveBankAccountChange) { c.ApprovedByUserID = requesterID },
			domain.CodeBankAccountChangeSameActorApproval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := approveChangeCommand()
			tc.mutate(&cmd)
			c := domain.FromBankAccountChangeSnapshot(tc.held)
			before := c.Snapshot()

			acc, rej := c.Approve(cmd)

			tb.RequireRejected(t, acc, rej, tc.code)
			if after := c.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestApproveBankAccountChangeReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	sameActor := domain.ApproveBankAccountChange{ApprovedByUserID: requesterID, At: approvedAt}

	acc, rej := domain.FromBankAccountChangeSnapshot(approvedChange()).Approve(sameActor)
	tb.RequireRejected(t, acc, rej, domain.CodeBankAccountChangeNotRequested)

	acc, rej = domain.FromBankAccountChangeSnapshot(requestedChange()).Approve(sameActor)
	tb.RequireRejected(t, acc, rej, domain.CodeBankAccountChangeSameActorApproval)
}

func TestBankAccountChangeDeterminism(t *testing.T) {
	c1 := domain.FromBankAccountChangeSnapshot(requestedChange())
	acc1, _ := c1.Approve(approveChangeCommand())

	c2 := domain.FromBankAccountChangeSnapshot(requestedChange())
	acc2, _ := c2.Approve(approveChangeCommand())

	if !c1.Snapshot().Equal(c2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestFromBankAccountChangeSnapshotRoundTrips(t *testing.T) {
	for _, before := range []domain.BankAccountChangeSnapshot{requestedChange(), approvedChange()} {
		if got := domain.FromBankAccountChangeSnapshot(before).Snapshot(); !got.Equal(before) {
			t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
		}
	}
}

func TestBankAccountChangeEqualTellsEveryFieldApart(t *testing.T) {
	base := approvedChange()
	for name, mutate := range map[string]func(*domain.BankAccountChangeSnapshot){
		"id":                   func(s *domain.BankAccountChangeSnapshot) { s.ID = "bac-other" },
		"organization":         func(s *domain.BankAccountChangeSnapshot) { s.OrganizationID = "org-other" },
		"destination evidence": func(s *domain.BankAccountChangeSnapshot) { s.DestinationEvidenceID = "evd-other" },
		"requester":            func(s *domain.BankAccountChangeSnapshot) { s.RequestedByUserID = "user-other" },
		"approver":             func(s *domain.BankAccountChangeSnapshot) { s.ApprovedByUserID = "user-other" },
		"status":               func(s *domain.BankAccountChangeSnapshot) { s.Status = domain.BankAccountChangeStatusRequested },
		"requested at":         func(s *domain.BankAccountChangeSnapshot) { s.RequestedAt++ },
		"approved at":          func(s *domain.BankAccountChangeSnapshot) { s.ApprovedAt++ },
	} {
		changed := approvedChange()
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func TestEachBankAccountChangeStatusKeepsItsStoredValue(t *testing.T) {
	statuses := []domain.BankAccountChangeStatus{
		domain.BankAccountChangeStatusNew, domain.BankAccountChangeStatusRequested, domain.BankAccountChangeStatusApproved,
	}
	if want := []domain.BankAccountChangeStatus{0, 1, 2}; !slices.Equal(statuses, want) {
		t.Fatalf("statuses = %v, want %v: the snapshot stores them as integers", statuses, want)
	}
}
