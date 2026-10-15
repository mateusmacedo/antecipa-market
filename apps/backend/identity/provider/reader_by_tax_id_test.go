//go:build integration

package provider_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

func ids(snapshots []domain.OrganizationSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}

func TestLoadByTaxIDHashReturnsEveryJurisdictionOfTheHashInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	portuguese := organization(1)
	portuguese.Jurisdiction = "PT"
	for id, s := range map[domain.OrganizationID]domain.OrganizationSnapshot{
		"org-br": organization(1), "org-pt": portuguese, "org-other": organization(2),
	} {
		if err := save(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}

	found, err := provider.NewOrganizationsByTaxIDReader(postgres.NewReadPool(pool)).LoadByTaxIDHash(ctx, "hash-1")

	if err != nil {
		t.Fatalf("LoadByTaxIDHash() = %v, want nil", err)
	}
	if got, want := ids(found), []string{"org-br", "org-pt"}; !slices.Equal(got, want) {
		t.Fatalf("LoadByTaxIDHash() = %v, want %v", got, want)
	}
	for _, s := range found {
		if want := organization(1); s.TaxIDHash != want.TaxIDHash || s.LegalName != want.LegalName || s.Kinds != want.Kinds {
			t.Fatalf("snapshot %+v lost state", s)
		}
	}
}

func TestLoadByTaxIDHashOfAnUnknownHashIsEmpty(t *testing.T) {
	pool := appkit.OpenPool(t)

	found, err := provider.NewOrganizationsByTaxIDReader(postgres.NewReadPool(pool)).
		LoadByTaxIDHash(withExecution(t, context.Background()), "hash-absent")

	if err != nil || len(found) != 0 {
		t.Fatalf("LoadByTaxIDHash(hash-absent) = %v, %v; want nothing and no error", found, err)
	}
}

func TestLoadByTaxIDHashHeldOnlyByAnotherTenantIsACrossTenantAccess(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := save(withExecution(t, context.Background()), pool, "org-br", organization(1)); err != nil {
		t.Fatalf("save: %v", err)
	}

	found, err := provider.NewOrganizationsByTaxIDReader(postgres.NewReadPool(pool)).
		LoadByTaxIDHash(withTenant(t, context.Background(), "globex"), "hash-1")

	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.ContextTenant != "globex" || access.DataTenant != "acme" || len(found) != 0 {
		t.Fatalf("LoadByTaxIDHash() from another tenant = %v, %v; want a CrossTenantAccess and no row", found, err)
	}
}
