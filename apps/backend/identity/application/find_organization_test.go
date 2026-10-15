package application_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestFindOrganizationAnswersTheStoredState(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	want := registeredSnapshot(testOrganizationID, testTenant, testJurisdiction)
	h.seedOrganization(t, ctx, want)
	h.rec.Reset()

	got, err := h.service.FindOrganization(ctx, testOrganizationID)

	if err != nil || !got.Equal(want) {
		t.Fatalf("FindOrganization() = %+v, %v; want %+v", got, err, want)
	}
	for _, step := range h.rec.Observed() {
		if step == "within" {
			t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
		}
	}
}

func TestFindOrganizationOfAnUnknownIDIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.FindOrganization(withExecution(t, context.Background()), "org-absent")

	if want := "application: find organization org-absent: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("FindOrganization() = %v, want %q", err, want)
	}
}

func TestFindOrganizationOfAnotherTenantReadsAsNotFound(t *testing.T) {
	h := newHarness(t)
	h.seedOrganization(t, withExecution(t, context.Background()), registeredSnapshot(testOrganizationID, testTenant, testJurisdiction))

	_, err := h.service.FindOrganization(withTenant(t, context.Background(), "globex"), testOrganizationID)

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindOrganization() from another tenant = %v, want ErrNotFound (IDN-13)", err)
	}
}
