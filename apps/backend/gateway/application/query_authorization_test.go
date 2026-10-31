package application_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func grantAuthorization() application.GrantQueryAuthorization {
	return application.GrantQueryAuthorization{
		ConsentID: "cns-1", OrganizationID: "org-1", ConsentVersion: 1, AssetTypes: []string{"duplicata"},
		PeriodFromDate: "2026-01-01", PeriodUntilDate: "2026-12-31",
	}
}

func TestGrantQueryAuthorizationProjectsTheConsent(t *testing.T) {
	h := newHarness(t)

	got := must(h.service.GrantQueryAuthorization(withExecution(t), grantAuthorization())).accepted(t)

	if got.ConsentID != "cns-1" {
		t.Fatalf("ConsentID = %q", got.ConsentID)
	}
	found, err := h.service.FindQueryAuthorizationByOrganization(withExecution(t), "org-1")
	if err != nil || len(found) != 1 || found[0].Status != domain.AuthorizationGranted || !slices.Equal(found[0].AssetTypes, []string{"duplicata"}) {
		t.Fatalf("FindQueryAuthorizationByOrganization() = %+v, %v", found, err)
	}
}

func TestRevokeQueryAuthorizationRevokesAtTheEffectiveInstant(t *testing.T) {
	h := newHarness(t)
	must(h.service.GrantQueryAuthorization(withExecution(t), grantAuthorization())).accepted(t)

	must(h.service.RevokeQueryAuthorization(withExecution(t), application.RevokeQueryAuthorization{ConsentID: "cns-1", EffectiveAt: testOccurred + second})).accepted(t)

	found, err := h.service.FindQueryAuthorizationByOrganization(withExecution(t), "org-1")
	if err != nil || len(found) != 1 || found[0].Status != domain.AuthorizationRevoked || found[0].RevokedAt != testOccurred+second {
		t.Fatalf("FindQueryAuthorizationByOrganization() = %+v, %v", found, err)
	}
}

func TestRevokeQueryAuthorizationOfAnUnknownConsentFails(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.RevokeQueryAuthorization(withExecution(t), application.RevokeQueryAuthorization{ConsentID: "cns-x", EffectiveAt: testOccurred})

	if err == nil {
		t.Fatalf("RevokeQueryAuthorization() over no authorization succeeded")
	}
}

func TestFindQueryAuthorizationByOrganizationAnswersEmptyForAnOrganizationWithoutConsent(t *testing.T) {
	h := newHarness(t)

	found, err := h.service.FindQueryAuthorizationByOrganization(withExecution(t), "org-none")

	if err != nil || len(found) != 0 {
		t.Fatalf("FindQueryAuthorizationByOrganization() = %+v, %v, want an empty answer", found, err)
	}
}
