package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func byTaxID(jurisdiction string) application.FindOrganizationByTaxID {
	return application.FindOrganizationByTaxID{
		TaxIDType:    domain.TaxIDTypeCNPJ,
		TaxID:        "12.abc.345/01de-35",
		Jurisdiction: jurisdiction,
	}
}

func TestFindOrganizationByTaxIDRefinesTheJurisdictionInMemory(t *testing.T) {
	h := newHarness(t)
	brazilian := registeredSnapshot("org-br", testTenant, testJurisdiction)
	h.taxIDs.hold(registeredSnapshot("org-pt", testTenant, "PT"), brazilian)
	h.rec.Reset()

	got, err := h.service.FindOrganizationByTaxID(withExecution(t, context.Background()), byTaxID(testJurisdiction))

	if err != nil || !got.Equal(brazilian) {
		t.Fatalf("FindOrganizationByTaxID() = %+v, %v; want %+v", got, err, brazilian)
	}
	if want := []string{fakeHash(testTenant, domain.TaxIDTypeCNPJ, alphanumericCNPJ)}; !slices.Equal(h.taxIDs.asked, want) {
		t.Fatalf("the reader was asked for %v, want the hash derived under the tenant %v", h.taxIDs.asked, want)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func requireNotFound(t *testing.T, err error) {
	t.Helper()
	if want := "application: find organization by tax id: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("FindOrganizationByTaxID() = %v, want %q", err, want)
	}
	var access port.CrossTenantAccess
	if errors.As(err, &access) {
		t.Fatalf("FindOrganizationByTaxID() = %v hands the CrossTenantAccess to the caller (IDN-13)", err)
	}
}

func TestFindOrganizationByTaxIDOfAnUnknownHashIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.FindOrganizationByTaxID(withExecution(t, context.Background()), byTaxID(testJurisdiction))

	requireNotFound(t, err)
}

func TestFindOrganizationByTaxIDInAnotherJurisdictionIsNotFound(t *testing.T) {
	h := newHarness(t)
	h.taxIDs.hold(registeredSnapshot("org-pt", testTenant, "PT"))

	_, err := h.service.FindOrganizationByTaxID(withExecution(t, context.Background()), byTaxID(testJurisdiction))

	requireNotFound(t, err)
}

// The tenant takes part in the hash, so the organization of tenant A is not
// found from tenant B: B derives another hash for the same tax id.
func TestFindOrganizationByTaxIDFromAnotherTenantIsNotFound(t *testing.T) {
	h := newHarness(t)
	h.taxIDs.hold(registeredSnapshot("org-br", testTenant, testJurisdiction))

	_, err := h.service.FindOrganizationByTaxID(withTenant(t, context.Background(), "globex"), byTaxID(testJurisdiction))

	requireNotFound(t, err)
}

func TestFindOrganizationByTaxIDReportsTheCrossTenantAccessOnlyToTheInstrumentation(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	access := port.CrossTenantAccess{Object: "organizations?tax_id_hash=h", ContextTenant: "globex", DataTenant: testTenant}
	h.taxIDs.err = access

	_, err := h.service.FindOrganizationByTaxID(withExecution(t, context.Background()), byTaxID(testJurisdiction))

	requireNotFound(t, err)
	var got port.CrossTenantAccess
	if len(instr.results) != 1 || !errors.As(instr.results[0].Err, &got) || got != access {
		t.Fatalf("EndOperation(%+v), want the access handed over for the security record (IDN-12)", instr.results)
	}
}

func TestFindOrganizationByTaxIDFailsWhenTheProtectionFails(t *testing.T) {
	h := newHarness(t)
	h.protector.err = errBoom

	_, err := h.service.FindOrganizationByTaxID(withExecution(t, context.Background()), byTaxID(testJurisdiction))

	if want := "application: find organization by tax id: boom"; !errors.Is(err, errBoom) || err.Error() != want {
		t.Fatalf("FindOrganizationByTaxID() = %v, want %q", err, want)
	}
}

func TestFindOrganizationByTaxIDIsDeniedBeforeAnyProtection(t *testing.T) {
	h := newHarness(t)
	h.service.Authorize = func(context.Context, application.Operation) error { return port.ErrDenied }

	_, err := h.service.FindOrganizationByTaxID(withExecution(t, context.Background()), byTaxID(testJurisdiction))

	if !errors.Is(err, port.ErrDenied) || len(h.protector.calls) != 0 {
		t.Fatalf("FindOrganizationByTaxID() = %v after %d protections, want a denial before any", err, len(h.protector.calls))
	}
}
