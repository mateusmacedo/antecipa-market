package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestActivateAgreementActivatesADraftWithTheContractualCriteria(t *testing.T) {
	a := draftAgreement(t)

	acc, rej := a.Activate(activateAgreementCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.ActivateAgreementResponse{AgreementID: agreementID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	want := domain.PartnerAgreementActivated{
		AgreementID: agreementID, PartnerID: partnerID, Version: 1, ValidFrom: at, ValidUntil: at + 365*day, At: at + day,
	}
	if events := acc.Events(); len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want [%+v]", events, want)
	}
	if s := a.Snapshot(); s.Status != domain.AgreementActive || s.ContractEvidenceID != "evd-1" || s.ActivatedAt != at+day {
		t.Fatalf("Snapshot() = %+v, want active with the evidence, activated at %d", s, at+day)
	}
}

func TestActivateAgreementAcceptsTheRegulatoryCriterionWhenItApplies(t *testing.T) {
	cmd := activateAgreementCommand()
	cmd.RegulatoryCheckApplicable = true
	cmd.SatisfiedCriteria = []string{"H18", "H16", "H17", "H15", "H14"}

	_, rej := draftAgreement(t).Activate(cmd)

	tb.RequireAccepted(t, rej)
}

func TestActivateAgreementAcceptsAnIndefiniteValidity(t *testing.T) {
	cmd := registerAgreementCommand()
	cmd.ValidUntil = 0
	a := domain.NewPartnerAgreement(agreementID)
	if _, rej := a.Register(cmd); rej != nil {
		t.Fatalf("setup: Register rejected: %v", rej)
	}

	acc, rej := a.Activate(activateAgreementCommand())

	tb.RequireAccepted(t, rej)
	if got := acc.Events()[0].(domain.PartnerAgreementActivated).ValidUntil; got != 0 {
		t.Fatalf("ValidUntil = %d, want 0 for an indefinite validity", got)
	}
}

func TestActivateAgreementRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.ActivateAgreement)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"evidence missing", func(c *domain.ActivateAgreement) { c.ContractEvidenceID = "" },
			domain.CodePartnerAgreementContractEvidenceMissing, nil},
		{"evidence blank", func(c *domain.ActivateAgreement) { c.ContractEvidenceID = "  " },
			domain.CodePartnerAgreementContractEvidenceMissing, nil},
		{"without H17", func(c *domain.ActivateAgreement) { c.SatisfiedCriteria = []string{"H14", "H15", "H18"} },
			domain.CodePartnerAgreementContractualCriteriaIncomplete, []kernel.Detail{{Key: "missing", Value: "H17"}}},
		{"without any criterion", func(c *domain.ActivateAgreement) { c.SatisfiedCriteria = nil },
			domain.CodePartnerAgreementContractualCriteriaIncomplete, []kernel.Detail{{Key: "missing", Value: "H14,H15,H17,H18"}}},
		{"without H16 when the regulatory check applies", func(c *domain.ActivateAgreement) { c.RegulatoryCheckApplicable = true },
			domain.CodePartnerAgreementContractualCriteriaIncomplete, []kernel.Detail{{Key: "missing", Value: "H16"}}},
		{"validity ended at the instant", func(c *domain.ActivateAgreement) { c.At = at + 365*day },
			domain.CodePartnerAgreementValidityInvalid, nil},
		{"validity ended before the instant", func(c *domain.ActivateAgreement) { c.At = at + 366*day },
			domain.CodePartnerAgreementValidityInvalid, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := activateAgreementCommand()
			tt.mutate(&cmd)
			a := draftAgreement(t)
			before := a.Snapshot()

			acc, rej := a.Activate(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireAgreementUnchanged(t, before, a.Snapshot())
		})
	}
}

func TestActivateAgreementRejectsAnAgreementAlreadyActive(t *testing.T) {
	a := draftAgreement(t)
	if _, rej := a.Activate(activateAgreementCommand()); rej != nil {
		t.Fatalf("setup: Activate rejected: %v", rej)
	}
	before := a.Snapshot()

	acc, rej := a.Activate(activateAgreementCommand())

	tb.RequireRejected(t, acc, rej, domain.CodePartnerAgreementNotDraft)
	requireAgreementUnchanged(t, before, a.Snapshot())
}

func TestActivateAgreementReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	active := draftAgreement(t).Snapshot()
	active.Status = domain.AgreementActive
	cmd := domain.ActivateAgreement{At: at + 400*day}
	targets := []domain.PartnerAgreementSnapshot{active, draftAgreement(t).Snapshot(), draftAgreement(t).Snapshot(), draftAgreement(t).Snapshot()}
	want := []kernel.Code{
		domain.CodePartnerAgreementNotDraft,
		domain.CodePartnerAgreementContractEvidenceMissing,
		domain.CodePartnerAgreementContractualCriteriaIncomplete,
		domain.CodePartnerAgreementValidityInvalid,
	}
	fixes := []func(*domain.ActivateAgreement){
		func(*domain.ActivateAgreement) {},
		func(c *domain.ActivateAgreement) { c.ContractEvidenceID = "evd-1" },
		func(c *domain.ActivateAgreement) { c.SatisfiedCriteria = []string{"H14", "H15", "H17", "H18"} },
		func(*domain.ActivateAgreement) {},
	}
	for i, code := range want {
		acc, rej := domain.FromPartnerAgreementSnapshot(targets[i]).Activate(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}
