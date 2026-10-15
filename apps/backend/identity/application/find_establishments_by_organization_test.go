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

func TestFindEstablishmentsByOrganizationAnswersEveryEstablishmentOfIt(t *testing.T) {
	h := newHarness(t)
	held := []string{"est-1", "est-2"}
	for _, id := range held {
		h.establishmentsByOrg.hold(testOrganizationID, addedEstablishment(domain.EstablishmentID(id), testOrganizationID, testTenant))
	}

	got, err := h.service.FindEstablishmentsByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if err != nil || len(got) != len(held) {
		t.Fatalf("FindEstablishmentsByOrganization() = %v, %v; want %d establishments", got, err, len(held))
	}
	for i, s := range got {
		if !s.Equal(addedEstablishment(domain.EstablishmentID(held[i]), testOrganizationID, testTenant)) {
			t.Fatalf("establishment %d = %+v lost state", i, s)
		}
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindEstablishmentsOfAnOrganizationWithoutAnyIsEmpty(t *testing.T) {
	h := newHarness(t)

	got, err := h.service.FindEstablishmentsByOrganization(withExecution(t, context.Background()), "org-absent")

	if err != nil || len(got) != 0 {
		t.Fatalf("FindEstablishmentsByOrganization() = %v, %v; want nothing and no error", got, err)
	}
}

func TestFindEstablishmentsOfAnotherTenantAnswersEmptyAndReportsTheAccess(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	access := port.CrossTenantAccess{Object: "establishments?organization_id=org-100", ContextTenant: "globex", DataTenant: testTenant}
	h.establishmentsByOrg.err = access

	got, err := h.service.FindEstablishmentsByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("FindEstablishmentsByOrganization() = %#v, %v; want the empty answer an organization nobody holds gets (IDN-13)", got, err)
	}
	var reported port.CrossTenantAccess
	if len(instr.results) != 1 || !errors.As(instr.results[0].Err, &reported) || reported != access {
		t.Fatalf("EndOperation(%+v), want the access handed over for the security record (IDN-12)", instr.results)
	}
}

func TestFindEstablishmentsByOrganizationCarriesThePrefixOnFailure(t *testing.T) {
	h := newHarness(t)
	h.establishmentsByOrg.err = errBoom

	_, err := h.service.FindEstablishmentsByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if want := "application: find establishments by organization org-100: boom"; !errors.Is(err, errBoom) || err.Error() != want {
		t.Fatalf("FindEstablishmentsByOrganization() = %v, want %q", err, want)
	}
}

func TestFindEstablishmentsByOrganizationIsDeniedBeforeAnyRead(t *testing.T) {
	h := newHarness(t)
	h.service.Authorize = func(context.Context, application.Operation) error { return port.ErrDenied }

	_, err := h.service.FindEstablishmentsByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if !errors.Is(err, port.ErrDenied) || len(h.establishmentsByOrg.asked) != 0 {
		t.Fatalf("FindEstablishmentsByOrganization() = %v after %d reads, want a denial before any", err, len(h.establishmentsByOrg.asked))
	}
}
