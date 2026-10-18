package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRegisterAgreementOpensTheSeriesAsADraftAtVersionOne(t *testing.T) {
	a := domain.NewPartnerAgreement(agreementID)

	acc, rej := a.Register(registerAgreementCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RegisterAgreementResponse{AgreementID: agreementID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %+v, want none", acc.Events())
	}
	want := domain.PartnerAgreementSnapshot{
		ID:                   agreementID,
		PartnerID:            partnerID,
		Version:              1,
		Status:               domain.AgreementDraft,
		ValidFrom:            at,
		ValidUntil:           at + 365*day,
		InstrumentTemplateID: "tpl-1",
	}
	if got := a.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestRegisterAgreementAcceptsAnIndefiniteValidity(t *testing.T) {
	cmd := registerAgreementCommand()
	cmd.ValidUntil = 0
	a := domain.NewPartnerAgreement(agreementID)

	_, rej := a.Register(cmd)

	tb.RequireAccepted(t, rej)
	if got := a.Snapshot().ValidUntil; got != 0 {
		t.Fatalf("ValidUntil = %d, want 0 for an indefinite validity", got)
	}
}

func TestRegisterAgreementSupersedesTheLastVersionOfTheSeries(t *testing.T) {
	cmd := registerAgreementCommand()
	cmd.SupersedesID = "agr-2"
	cmd.Head = domain.AgreementHead{ID: "agr-2", Version: 2}
	a := domain.NewPartnerAgreement("agr-3")

	_, rej := a.Register(cmd)

	tb.RequireAccepted(t, rej)
	if s := a.Snapshot(); s.Version != 3 || s.SupersedesID != "agr-2" || s.Status != domain.AgreementDraft {
		t.Fatalf("Snapshot() = %+v, want a draft at version 3 superseding agr-2", s)
	}
}

func TestRegisterAgreementRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.RegisterAgreement)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"valid until before valid from", func(c *domain.RegisterAgreement) { c.ValidUntil = c.ValidFrom - 1 },
			domain.CodePartnerAgreementValidityInvalid, nil},
		{"valid until equal to valid from", func(c *domain.RegisterAgreement) { c.ValidUntil = c.ValidFrom },
			domain.CodePartnerAgreementValidityInvalid, nil},
		{"supersedes in an empty series", func(c *domain.RegisterAgreement) { c.SupersedesID = "agr-0" },
			domain.CodePartnerAgreementSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "agr-0"}}},
		{"another first version of the series", func(c *domain.RegisterAgreement) { c.Head = domain.AgreementHead{ID: "agr-1", Version: 1} },
			domain.CodePartnerAgreementSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: ""}}},
		{"supersedes a version that is not the last", func(c *domain.RegisterAgreement) {
			c.SupersedesID, c.Head = "agr-1", domain.AgreementHead{ID: "agr-2", Version: 2}
		}, domain.CodePartnerAgreementSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "agr-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registerAgreementCommand()
			tt.mutate(&cmd)
			a := domain.NewPartnerAgreement(agreementID)
			before := a.Snapshot()

			acc, rej := a.Register(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireAgreementUnchanged(t, before, a.Snapshot())
		})
	}
}

func TestRegisterAgreementReportsTheValidityBeforeTheSeries(t *testing.T) {
	cmd := registerAgreementCommand()
	cmd.ValidUntil, cmd.SupersedesID = cmd.ValidFrom, "agr-0"

	acc, rej := domain.NewPartnerAgreement(agreementID).Register(cmd)

	tb.RequireRejected(t, acc, rej, domain.CodePartnerAgreementValidityInvalid)
}
