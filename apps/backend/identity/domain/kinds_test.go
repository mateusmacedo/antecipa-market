package domain_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestKindsOfIsASetInTheCanonicalOrder(t *testing.T) {
	kinds := domain.KindsOf(domain.KindPlatform, domain.KindClient, domain.KindClient, domain.KindFunder)

	want := []domain.OrganizationKind{domain.KindClient, domain.KindFunder, domain.KindPlatform}
	if got := kinds.List(); !slices.Equal(got, want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
	if kinds != domain.KindsOf(domain.KindFunder, domain.KindPlatform, domain.KindClient) {
		t.Fatal("the same kinds in another order must be the same value")
	}
}

func TestKindsHas(t *testing.T) {
	kinds := domain.KindsOf(domain.KindClient, domain.KindPartner)

	for kind, want := range map[domain.OrganizationKind]bool{
		domain.KindClient:   true,
		domain.KindPartner:  true,
		domain.KindFunder:   false,
		domain.KindPlatform: false,
		"issuer":            false,
	} {
		if got := kinds.Has(kind); got != want {
			t.Errorf("Has(%s) = %v, want %v", kind, got, want)
		}
	}
}

func TestKindsOfDropsUnknownValues(t *testing.T) {
	if !domain.KindsOf("issuer", "").Empty() {
		t.Fatal("KindsOf of unknown values must be empty")
	}
	if domain.KindsOf(domain.KindFunder).Empty() {
		t.Fatal("KindsOf(funder) must not be empty")
	}
}

func TestEmptyKindsListAsAnEmptyCollection(t *testing.T) {
	got := domain.KindsOf().List()

	if got == nil || len(got) != 0 {
		t.Fatalf("List() = %#v, want an empty, non-nil slice", got)
	}
}
