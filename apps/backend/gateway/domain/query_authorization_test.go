package domain_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const consentID = domain.ConsentID("cns-1")

func grant(version int, assetTypes ...string) domain.GrantQueryAuthorization {
	return domain.GrantQueryAuthorization{
		OrganizationID: "org-1", ConsentVersion: version, AssetTypes: assetTypes,
		PeriodFromDate: "2026-01-01", PeriodUntilDate: "2026-12-31", At: at,
	}
}

func newGranted(t *testing.T) *domain.QueryAuthorization {
	t.Helper()
	a := domain.NewQueryAuthorization(consentID)
	if _, rej := a.Grant(grant(1, "duplicata")); rej != nil {
		t.Fatalf("setup: Grant rejected: %v", rej)
	}
	return a
}

func TestGrantQueryAuthorizationBornsGrantedWithTheConsentScope(t *testing.T) {
	a := domain.NewQueryAuthorization(consentID)

	acc, rej := a.Grant(grant(1, "duplicata", "cartao"))

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	if got := acc.Response(); got != (domain.AuthorizationResponse{ConsentID: consentID}) {
		t.Fatalf("Response() = %+v", got)
	}
	s := a.Snapshot()
	if s.Status != domain.AuthorizationGranted || s.OrganizationID != "org-1" || s.ConsentVersion != 1 || s.GrantedAt != at ||
		!slices.Equal(s.AssetTypes, []string{"duplicata", "cartao"}) || s.PeriodFromDate != "2026-01-01" || s.PeriodUntilDate != "2026-12-31" {
		t.Fatalf("Snapshot() = %+v", s)
	}
}

func TestGrantQueryAuthorizationRenewsTheScopeOnANewerVersion(t *testing.T) {
	a := newGranted(t)
	renewal := grant(2, "cartao")
	renewal.PeriodUntilDate = "2027-06-30"
	renewal.At = at + second

	_, rej := a.Grant(renewal)

	tb.RequireAccepted(t, rej)
	s := a.Snapshot()
	if s.ConsentVersion != 2 || !slices.Equal(s.AssetTypes, []string{"cartao"}) || s.PeriodUntilDate != "2027-06-30" || s.GrantedAt != at {
		t.Fatalf("Snapshot() = %+v, want the scope of version 2 and the original grant instant", s)
	}
}

func TestGrantQueryAuthorizationAcceptsAnOlderOrEqualVersionUnchanged(t *testing.T) {
	for _, version := range []int{0, 1} {
		a := newGranted(t)
		before := a.Snapshot()

		_, rej := a.Grant(grant(version, "cartao"))

		tb.RequireAccepted(t, rej)
		if !before.Equal(a.Snapshot()) {
			t.Fatalf("version %d changed the authorization: %+v", version, a.Snapshot())
		}
	}
}

func TestRevokeQueryAuthorizationIsFinal(t *testing.T) {
	a := newGranted(t)

	acc, rej := a.Revoke(domain.RevokeQueryAuthorization{EffectiveAt: at + second})

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	s := a.Snapshot()
	if s.Status != domain.AuthorizationRevoked || s.RevokedAt != at+second {
		t.Fatalf("Snapshot() = %+v, want revoked at the effective instant", s)
	}
	revoked := a.Snapshot()

	if _, rej := a.Grant(grant(5, "cartao")); rej != nil {
		t.Fatalf("Grant after revoke rejected: %v", rej)
	}
	if _, rej := a.Revoke(domain.RevokeQueryAuthorization{EffectiveAt: at + 2*second}); rej != nil {
		t.Fatalf("second Revoke rejected: %v", rej)
	}
	if !revoked.Equal(a.Snapshot()) {
		t.Fatalf("a revoked authorization changed: %+v", a.Snapshot())
	}
}
