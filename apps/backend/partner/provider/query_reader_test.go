//go:build integration

package provider_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

func TestLoadByRoleAnswersEveryPartnerOfTheRoleInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	registry := partner(2)
	registry.Role = domain.RoleRegistry
	pg.Seed(t, ctx, pool, provider.NewPartnerRepository, "prt-a", partner(1))
	pg.Seed(t, ctx, pool, provider.NewPartnerRepository, "prt-b", registry)
	pg.Seed(t, ctx, pool, provider.NewPartnerRepository, "prt-c", partner(3))
	reader := provider.NewPartnersByRoleReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByRole(ctx, domain.RoleFunder)

	ids := make([]domain.PartnerID, 0, len(found))
	for _, p := range found {
		ids = append(ids, p.ID)
	}
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []domain.PartnerID{"prt-a", "prt-c"}) {
		t.Fatalf("LoadByRole() = %v, %v; want the funders prt-a and prt-c", ids, err)
	}
	_, err = reader.LoadByRole(withTenant(t, context.Background(), "globex"), domain.RoleFunder)
	var access port.CrossTenantAccess
	if !errors.As(err, &access) {
		t.Fatalf("LoadByRole() from another tenant = %v, want CrossTenantAccess", err)
	}
}

func TestLoadByEnvironmentAnswersEveryProfileOfTheEnvironmentInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	for _, s := range []domain.IntegrationProfileSnapshot{
		profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, ""),
		profileVersion("ipf-2", "prt-1", domain.EnvironmentHomologation, 1, ""),
		profileVersion("ipf-9", "prt-9", domain.EnvironmentProduction, 1, ""),
	} {
		pg.Seed(t, ctx, pool, provider.NewIntegrationProfileRepository, s.ID, s)
	}
	reader := provider.NewIntegrationProfilesByEnvironmentReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByEnvironment(ctx, domain.EnvironmentProduction)

	ids := make([]string, 0, len(found))
	for _, s := range found {
		ids = append(ids, string(s.ID))
	}
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []string{"ipf-1", "ipf-9"}) {
		t.Fatalf("LoadByEnvironment() = %v, %v; want ipf-1 and ipf-9, of every partner", ids, err)
	}
	_, err = reader.LoadByEnvironment(withTenant(t, context.Background(), "globex"), domain.EnvironmentProduction)
	var access port.CrossTenantAccess
	if !errors.As(err, &access) {
		t.Fatalf("LoadByEnvironment() from another tenant = %v, want CrossTenantAccess", err)
	}
}
