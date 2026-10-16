package domain_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestRegisterAcceptsAnAlphanumericCNPJ(t *testing.T) {
	o := domain.NewOrganization(organizationID)

	acc, rej := o.Register(registerCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RegisterOrganizationResponse{OrganizationID: organizationID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	want := domain.OrganizationRegistered{
		OrganizationID: organizationID,
		Kinds:          domain.KindsOf(domain.KindClient),
		Jurisdiction:   jurisdiction,
		TaxIDType:      domain.TaxIDTypeCNPJ,
		TaxIDHash:      taxIDHash,
		TaxIDMasked:    "**.***.***/**DE-35",
		At:             at,
	}
	if got := events[0].(domain.OrganizationRegistered); got != want {
		t.Fatalf("OrganizationRegistered = %+v, want %+v", got, want)
	}
}

func TestRegisterLeavesTheOrganizationActiveWithTheCommandFields(t *testing.T) {
	o := domain.NewOrganization(organizationID)

	_, rej := o.Register(registerCommand())

	tb.RequireAccepted(t, rej)
	want := domain.OrganizationSnapshot{
		ID:                  organizationID,
		Kinds:               domain.KindsOf(domain.KindClient),
		Jurisdiction:        jurisdiction,
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               alphanumericCNPJ,
		TaxIDHash:           taxIDHash,
		TaxIDHashKeyVersion: 1,
		TaxIDMasked:         "**.***.***/**DE-35",
		LegalName:           legalName,
		TradeName:           tradeName,
		Status:              domain.OrganizationActive,
		CreatedAt:           at,
	}
	if got := o.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestRegisterAcceptsACPFAndMasksIt(t *testing.T) {
	cmd := registerCommand()
	cmd.TaxIDType = domain.TaxIDTypeCPF
	cmd.TaxID = "123.456.789-09"
	o := domain.NewOrganization(organizationID)

	acc, rej := o.Register(cmd)

	tb.RequireAccepted(t, rej)
	if got := acc.Events()[0].(domain.OrganizationRegistered).TaxIDMasked; got != "***.***.*89-09" {
		t.Fatalf("TaxIDMasked = %q, want %q", got, "***.***.*89-09")
	}
	if got := o.Snapshot().TaxID; got != "123.456.789-09" {
		t.Fatalf("TaxID = %q, want the value as received", got)
	}
}

func TestRegisterAcceptsATradeNameLeftEmpty(t *testing.T) {
	cmd := registerCommand()
	cmd.TradeName = ""
	o := domain.NewOrganization(organizationID)

	_, rej := o.Register(cmd)

	tb.RequireAccepted(t, rej)
	if got := o.Snapshot().TradeName; got != "" {
		t.Fatalf("TradeName = %q, want empty", got)
	}
}

// The event leaves the context, so the tax id may cross it only as hash and
// mask (spec, personal data policy): neither the value as received nor its
// normalized form may appear anywhere in it.
func TestRegisterKeepsTheTaxIDOutOfTheEvent(t *testing.T) {
	for _, raw := range []string{alphanumericCNPJ, "12.ABC.345/01DE-35", numericCNPJ} {
		cmd := registerCommand()
		cmd.TaxID = raw
		acc, rej := domain.NewOrganization(organizationID).Register(cmd)
		tb.RequireAccepted(t, rej)

		printed := fmt.Sprintf("%+v", acc.Events()[0])
		for _, leaked := range []string{raw, domain.NormalizeTaxID(domain.TaxIDTypeCNPJ, raw)} {
			if strings.Contains(printed, leaked) {
				t.Fatalf("OrganizationRegistered %s carries the tax id %q", printed, leaked)
			}
		}
	}
}

func TestRegisterRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.RegisterOrganization)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"kinds empty", func(c *domain.RegisterOrganization) { c.Kinds = domain.KindsOf() }, domain.CodeOrganizationKindsEmpty, nil},
		{"kinds of no known value", func(c *domain.RegisterOrganization) { c.Kinds = domain.KindsOf("issuer") }, domain.CodeOrganizationKindsEmpty, nil},
		{"jurisdiction of one letter", func(c *domain.RegisterOrganization) { c.Jurisdiction = "B" }, domain.CodeOrganizationJurisdictionInvalid, []kernel.Detail{{Key: "jurisdiction", Value: "B"}}},
		{"jurisdiction of three letters", func(c *domain.RegisterOrganization) { c.Jurisdiction = "BRA" }, domain.CodeOrganizationJurisdictionInvalid, []kernel.Detail{{Key: "jurisdiction", Value: "BRA"}}},
		{"jurisdiction empty", func(c *domain.RegisterOrganization) { c.Jurisdiction = "" }, domain.CodeOrganizationJurisdictionInvalid, []kernel.Detail{{Key: "jurisdiction", Value: ""}}},
		{"jurisdiction with a digit", func(c *domain.RegisterOrganization) { c.Jurisdiction = "B1" }, domain.CodeOrganizationJurisdictionInvalid, []kernel.Detail{{Key: "jurisdiction", Value: "B1"}}},
		{"jurisdiction in lower case", func(c *domain.RegisterOrganization) { c.Jurisdiction = "br" }, domain.CodeOrganizationJurisdictionInvalid, []kernel.Detail{{Key: "jurisdiction", Value: "br"}}},
		{"tax id empty", func(c *domain.RegisterOrganization) { c.TaxID = "" }, domain.CodeOrganizationTaxIDEmpty, nil},
		{"tax id of blanks", func(c *domain.RegisterOrganization) { c.TaxID = "   " }, domain.CodeOrganizationTaxIDEmpty, nil},
		{"tax id of punctuation alone", func(c *domain.RegisterOrganization) { c.TaxID = "../-" }, domain.CodeOrganizationTaxIDEmpty, nil},
		{"legal name empty", func(c *domain.RegisterOrganization) { c.LegalName = "" }, domain.CodeOrganizationLegalNameEmpty, nil},
		{"legal name of blanks", func(c *domain.RegisterOrganization) { c.LegalName = " \t " }, domain.CodeOrganizationLegalNameEmpty, nil},
		{"tax id already registered", func(c *domain.RegisterOrganization) { c.TaxIDAlreadyRegistered = true }, domain.CodeOrganizationDuplicateTaxID, []kernel.Detail{{Key: "jurisdiction", Value: jurisdiction}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registerCommand()
			tt.mutate(&cmd)
			o := domain.NewOrganization(organizationID)
			before := o.Snapshot()

			acc, rej := o.Register(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireOrganizationUnchanged(t, before, o.Snapshot())
		})
	}
}

func TestRegisterReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.RegisterOrganization{TaxIDAlreadyRegistered: true, At: at}
	want := []kernel.Code{
		domain.CodeOrganizationKindsEmpty,
		domain.CodeOrganizationJurisdictionInvalid,
		domain.CodeOrganizationTaxIDEmpty,
		domain.CodeOrganizationLegalNameEmpty,
		domain.CodeOrganizationDuplicateTaxID,
	}
	fixes := []func(*domain.RegisterOrganization){
		func(c *domain.RegisterOrganization) { c.Kinds = domain.KindsOf(domain.KindFunder) },
		func(c *domain.RegisterOrganization) { c.Jurisdiction = jurisdiction },
		func(c *domain.RegisterOrganization) {
			c.TaxIDType, c.TaxID = domain.TaxIDTypeCNPJ, numericCNPJ
		},
		func(c *domain.RegisterOrganization) { c.LegalName = legalName },
		func(*domain.RegisterOrganization) {},
	}
	for i, code := range want {
		acc, rej := domain.NewOrganization(organizationID).Register(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}

func TestRejectionsCarryNoTaxID(t *testing.T) {
	cmd := registerCommand()
	cmd.TaxIDAlreadyRegistered = true

	_, rej := domain.NewOrganization(organizationID).Register(cmd)

	printed := fmt.Sprintf("%s %v", rej.Error(), rej.Details())
	if strings.Contains(printed, alphanumericCNPJ) || strings.Contains(printed, taxIDHash) {
		t.Fatalf("rejection %q carries the tax id or its hash", printed)
	}
}
