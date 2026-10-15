package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestFindPersonAnswersTheStoredState(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	want := registeredPerson(testPersonID, testTenant)
	seed(t, h, ctx, personTable, testPersonID, want)
	h.rec.Reset()

	got, err := h.service.FindPerson(ctx, testPersonID)

	if err != nil || !got.Equal(want) {
		t.Fatalf("FindPerson() = %+v, %v; want %+v", got, err, want)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindPersonOfAnUnknownIDIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.FindPerson(withExecution(t, context.Background()), "per-absent")

	if want := "application: find person per-absent: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("FindPerson() = %v, want %q", err, want)
	}
}

func TestFindPersonOfAnotherTenantReadsAsNotFound(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withExecution(t, context.Background()), personTable, testPersonID, registeredPerson(testPersonID, testTenant))

	_, err := h.service.FindPerson(withTenant(t, context.Background(), "globex"), testPersonID)

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindPerson() from another tenant = %v, want ErrNotFound (IDN-13)", err)
	}
}

func TestFindRepresentationAnswersTheStoredState(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	want := declaredRepresentation(testRepresentationID, testOrganizationID)
	seed(t, h, ctx, representationTable, testRepresentationID, want)
	h.rec.Reset()

	got, err := h.service.FindRepresentation(ctx, testRepresentationID)

	if err != nil || !got.Equal(want) {
		t.Fatalf("FindRepresentation() = %+v, %v; want %+v", got, err, want)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindRepresentationOfAnUnknownIDIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.FindRepresentation(withExecution(t, context.Background()), "rep-absent")

	if want := "application: find representation rep-absent: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("FindRepresentation() = %v, want %q", err, want)
	}
}

func TestFindRepresentationOfAnotherTenantReadsAsNotFound(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withExecution(t, context.Background()), representationTable, testRepresentationID, declaredRepresentation(testRepresentationID, testOrganizationID))

	_, err := h.service.FindRepresentation(withTenant(t, context.Background(), "globex"), testRepresentationID)

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindRepresentation() from another tenant = %v, want ErrNotFound (IDN-13)", err)
	}
}
