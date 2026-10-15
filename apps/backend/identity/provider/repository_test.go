//go:build integration

package provider_test

import (
	"context"
	"errors"
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

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

const organizationsTable = "organizations"

const repoOrganizationID = domain.OrganizationID("org-1001")

func organization(marker int) domain.OrganizationSnapshot {
	return domain.OrganizationSnapshot{
		ID:                  repoOrganizationID,
		Kinds:               domain.KindsOf(domain.KindClient, domain.KindFunder),
		Jurisdiction:        "BR",
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               "12ABC34501DE35",
		TaxIDHash:           "hash-" + strconv.Itoa(marker),
		TaxIDHashKeyVersion: marker,
		TaxIDMasked:         "**.***.***/**DE-35",
		LegalName:           "Recebiveis Sinteticos Ltda",
		TradeName:           "Sintetica",
		Status:              domain.OrganizationActive,
		CreatedAt:           kernel.Instant(1_755_432_000_000_000_000),
	}
}

func save(ctx context.Context, pool *pgxpool.Pool, id domain.OrganizationID, s domain.OrganizationSnapshot) error {
	return pg.Within(ctx, pool, provider.NewOrganizationRepository, func(ctx context.Context, repo port.Repository[domain.OrganizationID, domain.OrganizationSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestOrganizationRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.OrganizationID, domain.OrganizationSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.OrganizationID, domain.OrganizationSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.OrganizationID, domain.OrganizationSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewOrganizationRepository, fn)
			},
			Reader:           provider.NewOrganizationReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.OrganizationID { return domain.OrganizationID("kit-" + strconv.Itoa(n)) },
			NewState:         organization,
			Marker:           func(s domain.OrganizationSnapshot) int { return s.TaxIDHashKeyVersion },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestOrganizationRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewOrganizationReader(postgres.NewReadPool(pool))

	pg.Seed(t, ctx, pool, provider.NewOrganizationRepository, repoOrganizationID, organization(1))
	if got, version, err := reader.Load(ctx, repoOrganizationID); err != nil || version != 1 || !got.Equal(organization(1)) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", got, version, err, organization(1))
	}

	suspended := organization(2)
	suspended.Status = domain.OrganizationSuspended
	suspended.TradeName = ""
	suspended.Kinds = domain.KindsOf(domain.KindPlatform)
	err := pg.Within(ctx, pool, provider.NewOrganizationRepository, func(ctx context.Context, repo port.Repository[domain.OrganizationID, domain.OrganizationSnapshot]) error {
		return repo.Save(ctx, repoOrganizationID, suspended, 1)
	})
	if err != nil {
		t.Fatalf("Save(update) = %v", err)
	}
	if got, version, err := reader.Load(ctx, repoOrganizationID); err != nil || version != 2 || !got.Equal(suspended) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v2", got, version, err, suspended)
	}
}

// The unique constraint is the authority the pre-check of the use case only
// anticipates: a second organization with the same hash and jurisdiction in the
// tenant fails as a technical error, never as a version conflict.
func TestTheSameTaxIDHashAndJurisdictionIsRefusedInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := save(ctx, pool, "org-a", organization(1)); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}

	err := save(ctx, pool, "org-b", organization(1))

	if err == nil || errors.Is(err, port.ErrVersionConflict) {
		t.Fatalf("Save(second) = %v, want the unique violation as a technical error", err)
	}
	if got := pg.Counts(t, pool, organizationsTable)[organizationsTable]; got != 1 {
		t.Fatalf("organizations = %d rows, want 1", got)
	}
}

func TestTheSameTaxIDHashIsAcceptedInAnotherJurisdictionOrTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := save(withExecution(t, context.Background()), pool, "org-a", organization(1)); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}
	elsewhere := organization(1)
	elsewhere.Jurisdiction = "PT"

	if err := save(withExecution(t, context.Background()), pool, "org-b", elsewhere); err != nil {
		t.Fatalf("Save(another jurisdiction) = %v, want nil", err)
	}
	if err := save(withTenant(t, context.Background(), "globex"), pool, "org-a", organization(1)); err != nil {
		t.Fatalf("Save(another tenant) = %v, want nil: uniqueness is per tenant", err)
	}
}

func TestTwoConcurrentRegistrationsOfTheSameTaxIDLeaveOneRow(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	barrier := make(chan struct{})
	var (
		wg   sync.WaitGroup
		errs = make([]error, 2)
	)
	for i, id := range []domain.OrganizationID{"org-a", "org-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = pg.Within(ctx, pool, provider.NewOrganizationRepository, func(ctx context.Context, repo port.Repository[domain.OrganizationID, domain.OrganizationSnapshot]) error {
				<-barrier
				return repo.Save(ctx, id, organization(1), 0)
			})
		}()
	}
	close(barrier)
	wg.Wait()

	failed := 0
	for _, err := range errs {
		if err != nil {
			failed++
		}
	}
	if failed != 1 {
		t.Fatalf("errors = %v, want exactly one of the two writers refused", errs)
	}
	if got := pg.Counts(t, pool, organizationsTable)[organizationsTable]; got != 1 {
		t.Fatalf("organizations = %d rows, want 1", got)
	}
}
