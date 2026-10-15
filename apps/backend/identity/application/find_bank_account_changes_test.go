package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestFindBankAccountChangeAnswersTheStoredState(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	want := approvedBankAccountChange(testBankAccountChangeID, testOrganizationID)
	seed(t, h, ctx, bankAccountChangeTable, testBankAccountChangeID, want)
	h.rec.Reset()

	got, err := h.service.FindBankAccountChange(ctx, testBankAccountChangeID)

	if err != nil || !got.Equal(want) {
		t.Fatalf("FindBankAccountChange() = %+v, %v; want %+v", got, err, want)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindBankAccountChangeOfAnUnknownIDIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.FindBankAccountChange(withExecution(t, context.Background()), "bac-absent")

	if want := "application: find bank account change bac-absent: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("FindBankAccountChange() = %v, want %q", err, want)
	}
}

func TestFindBankAccountChangeOfAnotherTenantReadsAsNotFound(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withExecution(t, context.Background()), bankAccountChangeTable, testBankAccountChangeID,
		requestedBankAccountChange(testBankAccountChangeID, testOrganizationID))

	_, err := h.service.FindBankAccountChange(withTenant(t, context.Background(), "globex"), testBankAccountChangeID)

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindBankAccountChange() from another tenant = %v, want ErrNotFound (IDN-13)", err)
	}
}

func bankAccountChangeIDs(snapshots []domain.BankAccountChangeSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}

func TestFindBankAccountChangesByOrganizationAnswersEveryChangeOfIt(t *testing.T) {
	h := newHarness(t)
	h.bankAccountChangesByOrg.hold(testOrganizationID,
		requestedBankAccountChange("bac-a", testOrganizationID), approvedBankAccountChange("bac-b", testOrganizationID))
	h.bankAccountChangesByOrg.hold("org-other", requestedBankAccountChange("bac-c", "org-other"))

	got, err := h.service.FindBankAccountChangesByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if want := []string{"bac-a", "bac-b"}; err != nil || !slices.Equal(bankAccountChangeIDs(got), want) {
		t.Fatalf("FindBankAccountChangesByOrganization() = %v, %v; want %v", bankAccountChangeIDs(got), err, want)
	}
	if asked := h.bankAccountChangesByOrg.asked; !slices.Equal(asked, []domain.OrganizationID{testOrganizationID}) {
		t.Fatalf("the relation was keyed on %v, want the organization", asked)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindBankAccountChangesOfAnotherTenantAnswersEmptyAndReportsTheAccess(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	access := port.CrossTenantAccess{Object: "bank_account_changes?organization_id=org-100", ContextTenant: "globex", DataTenant: testTenant}
	h.bankAccountChangesByOrg.err = access

	got, err := h.service.FindBankAccountChangesByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("FindBankAccountChangesByOrganization() = %v, %v; want the empty answer (IDN-13)", got, err)
	}
	var reported port.CrossTenantAccess
	if len(instr.results) != 1 || !errors.As(instr.results[0].Err, &reported) || reported != access {
		t.Fatalf("EndOperation(%+v), want the access handed over for the security record (IDN-12)", instr.results)
	}
}

func TestFindBankAccountChangesByOrganizationCarriesThePrefixOnFailure(t *testing.T) {
	h := newHarness(t)
	h.bankAccountChangesByOrg.err = errBoom

	_, err := h.service.FindBankAccountChangesByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if want := "application: find bank account changes by organization org-100: boom"; !errors.Is(err, errBoom) || err.Error() != want {
		t.Fatalf("FindBankAccountChangesByOrganization() = %v, want %q", err, want)
	}
}
