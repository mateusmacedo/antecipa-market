package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const testEstablishmentID = domain.EstablishmentID("est-100")

var numericCNPJ = cnpj("112223330001")

func acquirerReference() domain.ExternalReference {
	return domain.ExternalReference{
		ProviderID:      "partner-7",
		System:          domain.ExternalSystemAcquirer,
		ReferenceType:   domain.ExternalReferenceAuthorization,
		Reference:       "EC-000123",
		AsOf:            testOccurred,
		ObservedVersion: "3",
	}
}

func addEstablishment() application.AddEstablishment {
	return application.AddEstablishment{
		EstablishmentID:    testEstablishmentID,
		OrganizationID:     testOrganizationID,
		TaxIDType:          domain.TaxIDTypeCNPJ,
		TaxID:              numericCNPJ,
		TradeName:          testTradeName,
		Receiver:           true,
		ExternalReferences: []domain.ExternalReference{acquirerReference()},
	}
}

func addedEstablishment(id domain.EstablishmentID, organization domain.OrganizationID, tenant port.TenantID) domain.EstablishmentSnapshot {
	return domain.EstablishmentSnapshot{
		ID:                  id,
		OrganizationID:      organization,
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               numericCNPJ,
		TaxIDHash:           fakeHash(tenant, domain.TaxIDTypeCNPJ, numericCNPJ),
		TaxIDHashKeyVersion: 1,
		TaxIDMasked:         "**.***.***/**01-81",
		TradeName:           testTradeName,
		Receiver:            true,
		ExternalReferences:  []domain.ExternalReference{acquirerReference()},
		CreatedAt:           testOccurred,
	}
}

func withOrganization(t *testing.T, h *harness, status domain.OrganizationStatus) {
	t.Helper()
	organization := registeredSnapshot(testOrganizationID, testTenant, testJurisdiction)
	organization.Status = status
	h.seedOrganization(t, withExecution(t, context.Background()), organization)
	h.rec.Reset()
}

func TestAddEstablishmentWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)

	if _, err := h.service.AddEstablishment(withExecution(t, context.Background()), addEstablishment()); err != nil {
		t.Fatalf("AddEstablishment() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"protector.Protect",
		"establishments.LoadByTaxIDHash",
		"organizations.Load",
		"establishments.Load",
		"establishments.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2, spec notes † and ††)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestAddEstablishmentCreatesItAndEnqueuesTheAdditionInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.AddEstablishment(ctx, addEstablishment())

	if err != nil {
		t.Fatalf("AddEstablishment() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("AddEstablishment() rejected with %s, want accepted", rej.Code())
	}
	if got := outcome.Response(); got != (domain.AddEstablishmentResponse{EstablishmentID: testEstablishmentID}) {
		t.Fatalf("Response() = %+v", got)
	}
	stored, version, err := establishmentTable.Reader(h.store).Load(ctx, testEstablishmentID)
	if err != nil || version != 1 || !stored.Equal(addedEstablishment(testEstablishmentID, testOrganizationID, testTenant)) {
		t.Fatalf("Load() = %+v v%d, %v; want the added establishment at v1", stored, version, err)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeEstablishment || entry.AggregateID != string(testEstablishmentID) ||
		entry.AggregateVersion != 1 || entry.Intent.Destination != application.Destination ||
		entry.Intent.PartitionKey != string(testEstablishmentID) {
		t.Fatalf("outbox entry = %+v, want the addition of the establishment at version 1", entry)
	}
	want := domain.EstablishmentAdded{
		EstablishmentID: testEstablishmentID,
		OrganizationID:  testOrganizationID,
		TaxIDType:       domain.TaxIDTypeCNPJ,
		TaxIDHash:       fakeHash(testTenant, domain.TaxIDTypeCNPJ, numericCNPJ),
		Receiver:        true,
		At:              testOccurred,
	}
	if got, ok := entry.Event.(domain.EstablishmentAdded); !ok || got != want {
		t.Fatalf("event = %+v, want %+v", entry.Event, want)
	}
}

func TestAddEstablishmentProtectsTheTaxIDUnderTheTenantOfTheRequest(t *testing.T) {
	h := newHarness(t)
	organization := registeredSnapshot(testOrganizationID, "globex", testJurisdiction)
	h.seedOrganization(t, withTenant(t, context.Background(), "globex"), organization)

	if _, err := h.service.AddEstablishment(withTenant(t, context.Background(), "globex"), addEstablishment()); err != nil {
		t.Fatalf("AddEstablishment() error = %v, want nil", err)
	}

	if want := []protectCall{{tenant: "globex", kind: domain.TaxIDTypeCNPJ, value: numericCNPJ}}; !slices.Equal(h.protector.calls, want) {
		t.Fatalf("Protect calls = %+v, want %+v", h.protector.calls, want)
	}
	if want := []string{fakeHash("globex", domain.TaxIDTypeCNPJ, numericCNPJ)}; !slices.Equal(h.establishmentTaxIDs.asked, want) {
		t.Fatalf("the uniqueness read asked for %v, want the hash of the tenant %v", h.establishmentTaxIDs.asked, want)
	}
}

func TestAddEstablishmentRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status domain.OrganizationStatus
		seed   bool
		mutate func(*application.AddEstablishment)
		held   bool
		code   kernel.Code
	}{
		{"organization not found", 0, false, func(*application.AddEstablishment) {}, false, domain.CodeEstablishmentOrganizationNotActive},
		{"organization suspended", domain.OrganizationSuspended, true, func(*application.AddEstablishment) {}, false, domain.CodeEstablishmentOrganizationNotActive},
		{"organization offboarded", domain.OrganizationOffboarded, true, func(*application.AddEstablishment) {}, false, domain.CodeEstablishmentOrganizationNotActive},
		{"tax id empty", domain.OrganizationActive, true, func(c *application.AddEstablishment) { c.TaxID = "" }, false, domain.CodeEstablishmentTaxIDEmpty},
		{"tax id already registered in the organization", domain.OrganizationActive, true, func(*application.AddEstablishment) {}, true, domain.CodeEstablishmentDuplicateTaxID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.seed {
				withOrganization(t, h, tc.status)
			}
			if tc.held {
				held := addedEstablishment("est-held", testOrganizationID, testTenant)
				h.establishmentTaxIDs.hold(held.TaxIDHash, held)
			}
			cmd := addEstablishment()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.AddEstablishment(ctx, cmd)

			if err != nil {
				t.Fatalf("AddEstablishment() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
			if _, _, err := establishmentTable.Reader(h.store).Load(ctx, testEstablishmentID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

// The organization of another tenant is the organization nobody holds here
// (IDN-13): the precondition of spec note † refuses it as not active.
func TestAddEstablishmentToTheOrganizationOfAnotherTenantIsNotActive(t *testing.T) {
	h := newHarness(t)
	h.seedOrganization(t, withTenant(t, context.Background(), "globex"), registeredSnapshot(testOrganizationID, "globex", testJurisdiction))

	outcome, err := h.service.AddEstablishment(withExecution(t, context.Background()), addEstablishment())

	if err != nil {
		t.Fatalf("AddEstablishment() error = %v, want a rejection", err)
	}
	if rej, refused := outcome.Rejection(); !refused || rej.Code() != domain.CodeEstablishmentOrganizationNotActive {
		t.Fatalf("outcome = %+v, want %s", outcome, domain.CodeEstablishmentOrganizationNotActive)
	}
}

func TestAddEstablishmentAcceptsTheSameTaxIDInAnotherOrganization(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	held := addedEstablishment("est-held", "org-other", testTenant)
	h.establishmentTaxIDs.hold(held.TaxIDHash, held)

	outcome, err := h.service.AddEstablishment(withExecution(t, context.Background()), addEstablishment())

	if err != nil {
		t.Fatalf("AddEstablishment() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("AddEstablishment() rejected with %s: the key is the organization and the hash together", rej.Code())
	}
}

func TestAddEstablishmentCountsACrossTenantAccessAsAbsent(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	h.establishmentTaxIDs.err = port.CrossTenantAccess{Object: "establishments?tax_id_hash=h", ContextTenant: testTenant, DataTenant: "globex"}

	outcome, err := h.service.AddEstablishment(withExecution(t, context.Background()), addEstablishment())

	if err != nil {
		t.Fatalf("AddEstablishment() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("AddEstablishment() rejected with %s, want accepted", rej.Code())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1", got)
	}
}

func TestEachFailureOfAddEstablishmentCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		wantMsg string
	}{
		{"protect", nil, func(h *harness) { h.protector.err = errBoom }, "application: add establishment est-100: boom"},
		{"uniqueness read", nil, func(h *harness) { h.establishmentTaxIDs.err = errBoom }, "application: add establishment est-100: boom"},
		{"organization load", []option{withLoadError(errBoom)}, nil, "application: add establishment est-100: boom"},
		{"load", []option{withTableFaults("establishments", serviceskit.Faults{Load: errBoom})}, nil, "application: add establishment est-100: boom"},
		{"save", []option{withTableFaults("establishments", serviceskit.Faults{Save: errBoom})}, nil, "application: add establishment est-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, nil, "application: add establishment est-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withOrganization(t, h, domain.OrganizationActive)
			if tc.prepare != nil {
				tc.prepare(h)
			}

			_, err := h.service.AddEstablishment(withExecution(t, context.Background()), addEstablishment())

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a failure", h.outboxLen())
			}
		})
	}
}

func TestAddingAnExistingEstablishmentCarriesThePrefix(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	ctx := withExecution(t, context.Background())
	seed(t, h, ctx, establishmentTable, testEstablishmentID, addedEstablishment(testEstablishmentID, "org-other", "globex"))

	_, err := h.service.AddEstablishment(withExecution(t, context.Background()), addEstablishment())

	if want := "application: add establishment est-100: ports: aggregate already exists"; !errors.Is(err, port.ErrAlreadyExists) || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestARepeatedAddEstablishmentReplaysWithoutASecondEffect(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)

	first, err := h.service.AddEstablishment(withKey(t, context.Background(), "k-add"), addEstablishment())
	if err != nil {
		t.Fatalf("first AddEstablishment() = %v, want nil", err)
	}
	again, err := h.service.AddEstablishment(withKey(t, context.Background(), "k-add"), addEstablishment())
	if err != nil || again.Response() != first.Response() {
		t.Fatalf("replay = %+v, %v; want %+v", again.Response(), err, first.Response())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1: the replay produced a second event", got)
	}
}

func TestTheSameKeyWithAnotherEstablishmentIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.AddEstablishment){
		"organization":     func(c *application.AddEstablishment) { c.OrganizationID = "org-other" },
		"tax id type":      func(c *application.AddEstablishment) { c.TaxIDType = domain.TaxIDTypeCPF },
		"tax id":           func(c *application.AddEstablishment) { c.TaxID = alphanumericCNPJ },
		"trade name":       func(c *application.AddEstablishment) { c.TradeName = "" },
		"receiver":         func(c *application.AddEstablishment) { c.Receiver = false },
		"no references":    func(c *application.AddEstablishment) { c.ExternalReferences = nil },
		"reference value":  func(c *application.AddEstablishment) { c.ExternalReferences[0].Reference = "EC-999" },
		"reference system": func(c *application.AddEstablishment) { c.ExternalReferences[0].System = domain.ExternalSystemRegistry },
		"reference type": func(c *application.AddEstablishment) {
			c.ExternalReferences[0].ReferenceType = domain.ExternalReferenceAsset
		},
		"reference partner": func(c *application.AddEstablishment) { c.ExternalReferences[0].ProviderID = "partner-8" },
		"reference as of":   func(c *application.AddEstablishment) { c.ExternalReferences[0].AsOf++ },
		"reference version": func(c *application.AddEstablishment) { c.ExternalReferences[0].ObservedVersion = "4" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withOrganization(t, h, domain.OrganizationActive)
			if _, err := h.service.AddEstablishment(withKey(t, context.Background(), "k-reused"), addEstablishment()); err != nil {
				t.Fatalf("first AddEstablishment() = %v, want nil", err)
			}
			second := addEstablishment()
			mutate(&second)

			_, err := h.service.AddEstablishment(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second AddEstablishment() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
