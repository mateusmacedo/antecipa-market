//go:build integration

package provider_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

const partnersTable = "partners"

func partner(marker int) domain.PartnerSnapshot {
	return domain.PartnerSnapshot{
		OrganizationID:       domain.OrganizationID("org-" + strconv.Itoa(marker)),
		Role:                 domain.RoleFunder,
		Status:               domain.PartnerOffboarding,
		RegisteredAt:         kernel.Instant(1_755_432_000_000_000_000),
		StageChangedAt:       kernel.Instant(1_755_518_400_000_000_000 + marker),
		StageReason:          "contract terminated",
		OffboardingStartedAt: kernel.Instant(1_755_518_400_000_000_000),
	}
}

func markerOf(s domain.PartnerSnapshot) int {
	n, _ := strconv.Atoi(string(s.OrganizationID)[len("org-"):])
	return n
}

func savePartner(ctx context.Context, pool *pgxpool.Pool, id domain.PartnerID, s domain.PartnerSnapshot) error {
	return pg.Within(ctx, pool, provider.NewPartnerRepository, func(ctx context.Context, repo port.Repository[domain.PartnerID, domain.PartnerSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestPartnerRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.PartnerID, domain.PartnerSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.PartnerID, domain.PartnerSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.PartnerID, domain.PartnerSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewPartnerRepository, fn)
			},
			Reader:           provider.NewPartnerReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.PartnerID { return domain.PartnerID("kit-" + strconv.Itoa(n)) },
			NewState:         partner,
			Marker:           markerOf,
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestPartnerRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewPartnerReader(postgres.NewReadPool(pool))
	want := partner(1)
	want.ID = "prt-1"

	pg.Seed(t, ctx, pool, provider.NewPartnerRepository, want.ID, want)

	if got, version, err := reader.Load(ctx, want.ID); err != nil || version != 1 || !got.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", got, version, err, want)
	}
}

func TestTheSameOrganizationAndRoleIsRefusedInTheTenantForAnyStatus(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := savePartner(ctx, pool, "prt-a", partner(1)); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}
	prospect := partner(1)
	prospect.Status, prospect.StageReason, prospect.OffboardingStartedAt = domain.PartnerProspect, "", 0

	err := savePartner(ctx, pool, "prt-b", prospect)

	if err == nil || errors.Is(err, port.ErrVersionConflict) {
		t.Fatalf("Save(second) = %v, want the unique violation as a technical error", err)
	}
	if got := pg.Counts(t, pool, partnersTable)[partnersTable]; got != 1 {
		t.Fatalf("partners = %d rows, want 1", got)
	}
}

func TestAnotherRoleOrAnotherTenantOfTheSameOrganizationIsAccepted(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := savePartner(withExecution(t, context.Background()), pool, "prt-a", partner(1)); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}
	registry := partner(1)
	registry.Role = domain.RoleRegistry

	if err := savePartner(withExecution(t, context.Background()), pool, "prt-b", registry); err != nil {
		t.Fatalf("Save(another role) = %v, want nil", err)
	}
	if err := savePartner(withTenant(t, context.Background(), "globex"), pool, "prt-a", partner(1)); err != nil {
		t.Fatalf("Save(another tenant) = %v, want nil: uniqueness is per tenant", err)
	}
}

func TestTwoConcurrentRegistrationsOfOneOrganizationAndRoleLeaveOneRow(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	barrier := make(chan struct{})
	var (
		wg   sync.WaitGroup
		errs = make([]error, 2)
	)
	for i, id := range []domain.PartnerID{"prt-a", "prt-b"} {
		wg.Go(func() {
			errs[i] = pg.Within(ctx, pool, provider.NewPartnerRepository, func(ctx context.Context, repo port.Repository[domain.PartnerID, domain.PartnerSnapshot]) error {
				<-barrier
				return repo.Save(ctx, id, partner(1), 0)
			})
		})
	}
	close(barrier)
	wg.Wait()

	if failed := len(slices.DeleteFunc(slices.Clone(errs), func(err error) bool { return err == nil })); failed != 1 {
		t.Fatalf("errors = %v, want exactly one of the two writers refused", errs)
	}
	if got := pg.Counts(t, pool, partnersTable)[partnersTable]; got != 1 {
		t.Fatalf("partners = %d rows, want 1", got)
	}
}

func TestLoadByOrganizationAnswersEveryRoleOfTheOrganizationInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	registry := partner(1)
	registry.Role = domain.RoleRegistry
	pg.Seed(t, ctx, pool, provider.NewPartnerRepository, "prt-a", partner(1))
	pg.Seed(t, ctx, pool, provider.NewPartnerRepository, "prt-b", registry)
	pg.Seed(t, ctx, pool, provider.NewPartnerRepository, "prt-c", partner(2))
	reader := provider.NewPartnersByOrganizationReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByOrganization(ctx, "org-1")

	ids := make([]domain.PartnerID, 0, len(found))
	for _, p := range found {
		ids = append(ids, p.ID)
	}
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []domain.PartnerID{"prt-a", "prt-b"}) {
		t.Fatalf("LoadByOrganization() = %v, %v; want prt-a and prt-b", ids, err)
	}
	_, err = reader.LoadByOrganization(withTenant(t, context.Background(), "globex"), "org-1")
	var access port.CrossTenantAccess
	if !errors.As(err, &access) {
		t.Fatalf("LoadByOrganization() from another tenant = %v, want CrossTenantAccess", err)
	}
}
