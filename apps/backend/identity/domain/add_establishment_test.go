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

const (
	establishmentID        = domain.EstablishmentID("est-100")
	establishmentTaxIDHash = "2ad689d9d2bdfa27268b09d5f0bd86e01cc7545bfb72a11b4c28450f3098b053"
)

func acquirerReference() domain.ExternalReference {
	return domain.ExternalReference{
		ProviderID:      "partner-7",
		System:          domain.ExternalSystemAcquirer,
		ReferenceType:   domain.ExternalReferenceAuthorization,
		Reference:       "EC-000123",
		AsOf:            at,
		ObservedVersion: "3",
	}
}

func addCommand() domain.AddEstablishment {
	return domain.AddEstablishment{
		OrganizationID:      organizationID,
		OrganizationStatus:  domain.OrganizationActive,
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               numericCNPJ,
		TaxIDHash:           establishmentTaxIDHash,
		TaxIDHashKeyVersion: 1,
		TradeName:           tradeName,
		Receiver:            true,
		ExternalReferences:  []domain.ExternalReference{acquirerReference()},
		At:                  at,
	}
}

func addedSnapshot() domain.EstablishmentSnapshot {
	return domain.EstablishmentSnapshot{
		ID:                  establishmentID,
		OrganizationID:      organizationID,
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               numericCNPJ,
		TaxIDHash:           establishmentTaxIDHash,
		TaxIDHashKeyVersion: 1,
		TaxIDMasked:         "**.***.***/**01-81",
		TradeName:           tradeName,
		Receiver:            true,
		ExternalReferences:  []domain.ExternalReference{acquirerReference()},
		CreatedAt:           at,
	}
}

func newAddedEstablishment(t *testing.T) *domain.Establishment {
	t.Helper()
	e := domain.NewEstablishment(establishmentID)
	if _, rej := e.Add(addCommand()); rej != nil {
		t.Fatalf("setup: Add rejected: %v", rej)
	}
	return e
}

func TestAddAcceptsAReceiverOfAnActiveOrganization(t *testing.T) {
	e := domain.NewEstablishment(establishmentID)

	acc, rej := e.Add(addCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.AddEstablishmentResponse{EstablishmentID: establishmentID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	want := domain.EstablishmentAdded{
		EstablishmentID: establishmentID,
		OrganizationID:  organizationID,
		TaxIDType:       domain.TaxIDTypeCNPJ,
		TaxIDHash:       establishmentTaxIDHash,
		Receiver:        true,
		At:              at,
	}
	if got := events[0].(domain.EstablishmentAdded); got != want {
		t.Fatalf("EstablishmentAdded = %+v, want %+v", got, want)
	}
}

func TestAddLeavesTheEstablishmentWithTheCommandFields(t *testing.T) {
	e := domain.NewEstablishment(establishmentID)

	_, rej := e.Add(addCommand())

	tb.RequireAccepted(t, rej)
	if got := e.Snapshot(); !got.Equal(addedSnapshot()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, addedSnapshot())
	}
}

func TestAddAcceptsAnEstablishmentWithoutReferencesNorTradeName(t *testing.T) {
	cmd := addCommand()
	cmd.ExternalReferences = nil
	cmd.TradeName = ""
	cmd.Receiver = false
	e := domain.NewEstablishment(establishmentID)

	acc, rej := e.Add(cmd)

	tb.RequireAccepted(t, rej)
	if got := e.Snapshot(); len(got.ExternalReferences) != 0 || got.TradeName != "" || got.Receiver {
		t.Fatalf("Snapshot() = %+v, want no references, no trade name and no receiver", got)
	}
	if acc.Events()[0].(domain.EstablishmentAdded).Receiver {
		t.Fatal("EstablishmentAdded.Receiver = true, want false")
	}
}

func TestAddKeepsTheTaxIDOutOfTheEvent(t *testing.T) {
	for _, raw := range []string{numericCNPJ, "11.222.333/0001-81", alphanumericCNPJ} {
		cmd := addCommand()
		cmd.TaxID = raw
		acc, rej := domain.NewEstablishment(establishmentID).Add(cmd)
		tb.RequireAccepted(t, rej)

		printed := fmt.Sprintf("%+v", acc.Events()[0])
		for _, leaked := range []string{raw, domain.NormalizeTaxID(domain.TaxIDTypeCNPJ, raw)} {
			if strings.Contains(printed, leaked) {
				t.Fatalf("EstablishmentAdded %s carries the tax id %q", printed, leaked)
			}
		}
	}
}

func TestAddRejectsEachPrecondition(t *testing.T) {
	organization := []kernel.Detail{{Key: "organization", Value: string(organizationID)}}
	tests := []struct {
		name    string
		mutate  func(*domain.AddEstablishment)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"organization not found", func(c *domain.AddEstablishment) { c.OrganizationStatus = domain.OrganizationNew }, domain.CodeEstablishmentOrganizationNotActive, organization},
		{"organization suspended", func(c *domain.AddEstablishment) { c.OrganizationStatus = domain.OrganizationSuspended }, domain.CodeEstablishmentOrganizationNotActive, organization},
		{"organization offboarded", func(c *domain.AddEstablishment) { c.OrganizationStatus = domain.OrganizationOffboarded }, domain.CodeEstablishmentOrganizationNotActive, organization},
		{"tax id empty", func(c *domain.AddEstablishment) { c.TaxID = "" }, domain.CodeEstablishmentTaxIDEmpty, nil},
		{"tax id of blanks", func(c *domain.AddEstablishment) { c.TaxID = "   " }, domain.CodeEstablishmentTaxIDEmpty, nil},
		{"tax id of punctuation alone", func(c *domain.AddEstablishment) { c.TaxID = "../-" }, domain.CodeEstablishmentTaxIDEmpty, nil},
		{"tax id already registered in the organization", func(c *domain.AddEstablishment) { c.TaxIDAlreadyRegistered = true }, domain.CodeEstablishmentDuplicateTaxID, organization},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := addCommand()
			tt.mutate(&cmd)
			e := domain.NewEstablishment(establishmentID)
			before := e.Snapshot()

			acc, rej := e.Add(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			if after := e.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestAddReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.AddEstablishment{OrganizationID: organizationID, TaxIDAlreadyRegistered: true, At: at}
	want := []kernel.Code{
		domain.CodeEstablishmentOrganizationNotActive,
		domain.CodeEstablishmentTaxIDEmpty,
		domain.CodeEstablishmentDuplicateTaxID,
	}
	fixes := []func(*domain.AddEstablishment){
		func(c *domain.AddEstablishment) { c.OrganizationStatus = domain.OrganizationActive },
		func(c *domain.AddEstablishment) { c.TaxIDType, c.TaxID = domain.TaxIDTypeCNPJ, numericCNPJ },
		func(*domain.AddEstablishment) {},
	}
	for i, code := range want {
		acc, rej := domain.NewEstablishment(establishmentID).Add(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}

func TestAddKeepsItsOwnCopyOfTheExternalReferences(t *testing.T) {
	cmd := addCommand()
	e := domain.NewEstablishment(establishmentID)
	if _, rej := e.Add(cmd); rej != nil {
		t.Fatalf("Add rejected: %v", rej)
	}

	cmd.ExternalReferences[0].Reference = "changed by the caller"
	e.Snapshot().ExternalReferences[0].Reference = "changed through a snapshot"

	if got := e.Snapshot(); !got.Equal(addedSnapshot()) {
		t.Fatalf("Snapshot() = %+v: a slice held outside reached the aggregate", got)
	}
}

func TestAddDeterminism(t *testing.T) {
	e1 := domain.NewEstablishment(establishmentID)
	acc1, _ := e1.Add(addCommand())

	e2 := domain.NewEstablishment(establishmentID)
	acc2, _ := e2.Add(addCommand())

	if !e1.Snapshot().Equal(e2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestFromEstablishmentSnapshotRoundTrips(t *testing.T) {
	before := newAddedEstablishment(t).Snapshot()

	got := domain.FromEstablishmentSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestEstablishmentEqualTellsEveryFieldApart(t *testing.T) {
	base := addedSnapshot()
	for name, mutate := range map[string]func(*domain.EstablishmentSnapshot){
		"id":              func(s *domain.EstablishmentSnapshot) { s.ID = "est-other" },
		"organization":    func(s *domain.EstablishmentSnapshot) { s.OrganizationID = "org-other" },
		"tax id type":     func(s *domain.EstablishmentSnapshot) { s.TaxIDType = domain.TaxIDTypeCPF },
		"tax id":          func(s *domain.EstablishmentSnapshot) { s.TaxID = alphanumericCNPJ },
		"tax id hash":     func(s *domain.EstablishmentSnapshot) { s.TaxIDHash = "other" },
		"key version":     func(s *domain.EstablishmentSnapshot) { s.TaxIDHashKeyVersion = 2 },
		"tax id masked":   func(s *domain.EstablishmentSnapshot) { s.TaxIDMasked = "other" },
		"trade name":      func(s *domain.EstablishmentSnapshot) { s.TradeName = "other" },
		"receiver":        func(s *domain.EstablishmentSnapshot) { s.Receiver = false },
		"reference value": func(s *domain.EstablishmentSnapshot) { s.ExternalReferences[0].Reference = "other" },
		"reference count": func(s *domain.EstablishmentSnapshot) { s.ExternalReferences = nil },
		"another reference": func(s *domain.EstablishmentSnapshot) {
			s.ExternalReferences = append(s.ExternalReferences, acquirerReference())
		},
		"created at":       func(s *domain.EstablishmentSnapshot) { s.CreatedAt++ },
		"reference as of":  func(s *domain.EstablishmentSnapshot) { s.ExternalReferences[0].AsOf++ },
		"reference system": func(s *domain.EstablishmentSnapshot) { s.ExternalReferences[0].System = domain.ExternalSystemRegistry },
		"reference type": func(s *domain.EstablishmentSnapshot) {
			s.ExternalReferences[0].ReferenceType = domain.ExternalReferenceAsset
		},
		"reference provider": func(s *domain.EstablishmentSnapshot) { s.ExternalReferences[0].ProviderID = "partner-8" },
		"reference observed": func(s *domain.EstablishmentSnapshot) { s.ExternalReferences[0].ObservedVersion = "4" },
	} {
		changed := addedSnapshot()
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}
