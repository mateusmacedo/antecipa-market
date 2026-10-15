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

func representationIDs(snapshots []domain.RepresentationSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}

func holdValidities(h *harness) {
	bounded := declaredRepresentation("rep-bounded", testOrganizationID)
	indefinite := declaredRepresentation("rep-indefinite", testOrganizationID)
	indefinite.ValidUntil = 0
	later := declaredRepresentation("rep-later", testOrganizationID)
	later.ValidFrom, later.ValidUntil = testValidUntil, 0
	h.representationsByOrg.hold(testOrganizationID, bounded, indefinite, later)
}

func TestFindRepresentationsByOrganizationWithoutAnInstantAnswersThemAll(t *testing.T) {
	h := newHarness(t)
	holdValidities(h)

	got, err := h.service.FindRepresentationsByOrganization(withExecution(t, context.Background()),
		application.FindRepresentationsByOrganization{Organization: testOrganizationID})

	if want := []string{"rep-bounded", "rep-indefinite", "rep-later"}; err != nil || !slices.Equal(representationIDs(got), want) {
		t.Fatalf("FindRepresentationsByOrganization() = %v, %v; want %v", representationIDs(got), err, want)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindRepresentationsByOrganizationKeepsThoseInForceAtTheInstant(t *testing.T) {
	for _, tc := range []struct {
		name    string
		validAt kernel.Instant
		want    []string
	}{
		{"before any starts", testValidFrom - 1, []string{}},
		{"the instant the first ones start", testValidFrom, []string{"rep-bounded", "rep-indefinite"}},
		{"the instant the bounded one ends", testValidUntil, []string{"rep-indefinite", "rep-later"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			holdValidities(h)

			got, err := h.service.FindRepresentationsByOrganization(withExecution(t, context.Background()),
				application.FindRepresentationsByOrganization{Organization: testOrganizationID, ValidAt: tc.validAt})

			if err != nil || !slices.Equal(representationIDs(got), tc.want) {
				t.Fatalf("FindRepresentationsByOrganization(validAt %d) = %v, %v; want %v", tc.validAt, representationIDs(got), err, tc.want)
			}
		})
	}
}

func TestFindRepresentationsByOrganizationLeavesTheReadUntouched(t *testing.T) {
	h := newHarness(t)
	holdValidities(h)
	ctx := withExecution(t, context.Background())

	if _, err := h.service.FindRepresentationsByOrganization(ctx,
		application.FindRepresentationsByOrganization{Organization: testOrganizationID, ValidAt: testValidUntil}); err != nil {
		t.Fatalf("filtered FindRepresentationsByOrganization() = %v", err)
	}
	got, err := h.service.FindRepresentationsByOrganization(ctx, application.FindRepresentationsByOrganization{Organization: testOrganizationID})

	if err != nil || len(got) != 3 {
		t.Fatalf("FindRepresentationsByOrganization() after a filtered one = %v, %v; want the three held", representationIDs(got), err)
	}
}

func TestFindRepresentationsOfAnotherTenantAnswersEmptyAndReportsTheAccess(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	access := port.CrossTenantAccess{Object: "representations?organization_id=org-100", ContextTenant: "globex", DataTenant: testTenant}
	h.representationsByOrg.err = access

	got, err := h.service.FindRepresentationsByOrganization(withExecution(t, context.Background()),
		application.FindRepresentationsByOrganization{Organization: testOrganizationID, ValidAt: testValidFrom})

	if err != nil || len(got) != 0 {
		t.Fatalf("FindRepresentationsByOrganization() = %v, %v; want the empty answer (IDN-13)", got, err)
	}
	var reported port.CrossTenantAccess
	if len(instr.results) != 1 || !errors.As(instr.results[0].Err, &reported) || reported != access {
		t.Fatalf("EndOperation(%+v), want the access handed over for the security record (IDN-12)", instr.results)
	}
}

func TestFindRepresentationsByOrganizationCarriesThePrefixOnFailure(t *testing.T) {
	h := newHarness(t)
	h.representationsByOrg.err = errBoom

	_, err := h.service.FindRepresentationsByOrganization(withExecution(t, context.Background()),
		application.FindRepresentationsByOrganization{Organization: testOrganizationID})

	if want := "application: find representations by organization org-100: boom"; !errors.Is(err, errBoom) || err.Error() != want {
		t.Fatalf("FindRepresentationsByOrganization() = %v, want %q", err, want)
	}
}
