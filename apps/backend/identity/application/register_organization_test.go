package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestRegisterWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register()); err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"protector.Protect",
		"organizations.LoadByTaxIDHash",
		"organizations.Load",
		"organizations.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRegisterCreatesAnActiveOrganization(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterOrganization(ctx, register())

	if err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterOrganization() rejected with %s, want accepted", rej.Code())
	}
	if got := outcome.Response(); got != (domain.RegisterOrganizationResponse{OrganizationID: testOrganizationID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.service.Reader.Load(ctx, testOrganizationID)
	if err != nil || version != 1 {
		t.Fatalf("Load() = v%d, %v; want v1", version, err)
	}
	want := registeredSnapshot(testOrganizationID, testTenant, testJurisdiction)
	want.TradeName = testTradeName
	if !snap.Equal(want) {
		t.Fatalf("snapshot = %+v\nwant       %+v", snap, want)
	}
}

func TestRegisterEnqueuesTheRegistrationWithHashAndMaskInTheSameTransaction(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register()); err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}

	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeOrganization || entry.AggregateID != string(testOrganizationID) ||
		entry.AggregateVersion != 1 || entry.Intent.Destination != application.Destination ||
		entry.Intent.PartitionKey != string(testOrganizationID) {
		t.Fatalf("outbox entry = %+v, want the registration of the organization at version 1", entry)
	}
	want := domain.OrganizationRegistered{
		OrganizationID: testOrganizationID,
		Kinds:          domain.KindsOf(domain.KindClient),
		Jurisdiction:   testJurisdiction,
		TaxIDType:      domain.TaxIDTypeCNPJ,
		TaxIDHash:      fakeHash(testTenant, domain.TaxIDTypeCNPJ, alphanumericCNPJ),
		TaxIDMasked:    "**.***.***/**DE-35",
		At:             testOccurred,
	}
	if got, ok := entry.Event.(domain.OrganizationRegistered); !ok || got != want {
		t.Fatalf("event = %+v, want %+v", entry.Event, want)
	}
}

func TestRegisterProtectsTheTaxIDUnderTheTenantOfTheRequest(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterOrganization(withTenant(t, context.Background(), "globex"), register()); err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}

	want := []protectCall{{tenant: "globex", kind: domain.TaxIDTypeCNPJ, value: alphanumericCNPJ}}
	if !slices.Equal(h.protector.calls, want) {
		t.Fatalf("Protect calls = %+v, want %+v", h.protector.calls, want)
	}
	if want := []string{fakeHash("globex", domain.TaxIDTypeCNPJ, alphanumericCNPJ)}; !slices.Equal(h.taxIDs.asked, want) {
		t.Fatalf("the uniqueness read asked for %v, want the hash of the tenant %v", h.taxIDs.asked, want)
	}
}

func TestRegisterRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*application.RegisterOrganization)
		held   bool
		code   kernel.Code
	}{
		{"kinds empty", func(c *application.RegisterOrganization) { c.Kinds = nil }, false, domain.CodeOrganizationKindsEmpty},
		{"jurisdiction of three letters", func(c *application.RegisterOrganization) { c.Jurisdiction = "BRA" }, false, domain.CodeOrganizationJurisdictionInvalid},
		{"tax id empty", func(c *application.RegisterOrganization) { c.TaxID = "" }, false, domain.CodeOrganizationTaxIDEmpty},
		{"legal name empty", func(c *application.RegisterOrganization) { c.LegalName = "" }, false, domain.CodeOrganizationLegalNameEmpty},
		{"tax id already registered in the jurisdiction", func(*application.RegisterOrganization) {}, true, domain.CodeOrganizationDuplicateTaxID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.held {
				h.taxIDs.hold(registeredSnapshot("org-held", testTenant, testJurisdiction))
			}
			cmd := register()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.RegisterOrganization(ctx, cmd)

			if err != nil {
				t.Fatalf("RegisterOrganization() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
			if _, _, err := h.service.Reader.Load(ctx, testOrganizationID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func TestRegisterAcceptsTheSameTaxIDInAnotherJurisdiction(t *testing.T) {
	h := newHarness(t)
	h.taxIDs.hold(registeredSnapshot("org-held", testTenant, "PT"))

	outcome, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register())

	if err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterOrganization() rejected with %s: the key is the hash and the jurisdiction together", rej.Code())
	}
}

// A CrossTenantAccess on the uniqueness read means another tenant holds the
// hash; uniqueness is per tenant, so it counts as absent.
func TestRegisterCountsACrossTenantAccessAsAbsent(t *testing.T) {
	h := newHarness(t)
	h.taxIDs.err = port.CrossTenantAccess{Object: "organizations?tax_id_hash=h", ContextTenant: testTenant, DataTenant: "globex"}

	outcome, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register())

	if err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterOrganization() rejected with %s, want accepted", rej.Code())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1", got)
	}
}
