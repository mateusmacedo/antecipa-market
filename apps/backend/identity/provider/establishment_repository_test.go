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

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

const establishmentsTable = "establishments"

func establishment(marker int) domain.EstablishmentSnapshot {
	return domain.EstablishmentSnapshot{
		OrganizationID:      repoOrganizationID,
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               "11222333000181",
		TaxIDHash:           "hash-" + strconv.Itoa(marker),
		TaxIDHashKeyVersion: marker,
		TaxIDMasked:         "**.***.***/**01-81",
		TradeName:           "Loja Sintetica",
		Receiver:            true,
		ExternalReferences: []domain.ExternalReference{{
			ProviderID:      "partner-7",
			System:          domain.ExternalSystemAcquirer,
			ReferenceType:   domain.ExternalReferenceAuthorization,
			Reference:       "EC-000123",
			AsOf:            kernel.Instant(1_755_432_000_000_000_000),
			ObservedVersion: "3",
		}},
		CreatedAt: kernel.Instant(1_755_432_000_000_000_000),
	}
}

func identified(s domain.EstablishmentSnapshot, id domain.EstablishmentID) domain.EstablishmentSnapshot {
	s.ID = id
	return s
}

// repositoryReader loads through the repository in a transaction of its own:
// the provider exposes no reader of establishments nor of memberships, because
// no query loads one by its id.
type repositoryReader[ID comparable, S any] struct {
	pool       *pgxpool.Pool
	repository func(*postgres.Tx) port.Repository[ID, S]
}

func (r repositoryReader[ID, S]) Load(ctx context.Context, id ID) (S, port.Version, error) {
	var (
		state   S
		version port.Version
	)
	err := pg.Within(ctx, r.pool, r.repository, func(ctx context.Context, repo port.Repository[ID, S]) error {
		var err error
		state, version, err = repo.Load(ctx, id)
		return err
	})
	return state, version, err
}

func establishmentReader(pool *pgxpool.Pool) repositoryReader[domain.EstablishmentID, domain.EstablishmentSnapshot] {
	return repositoryReader[domain.EstablishmentID, domain.EstablishmentSnapshot]{pool: pool, repository: provider.NewEstablishmentRepository}
}

func saveEstablishment(ctx context.Context, pool *pgxpool.Pool, id domain.EstablishmentID, s domain.EstablishmentSnapshot) error {
	return pg.Within(ctx, pool, provider.NewEstablishmentRepository, func(ctx context.Context, repo port.Repository[domain.EstablishmentID, domain.EstablishmentSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestEstablishmentRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.EstablishmentID, domain.EstablishmentSnapshot] {
		pg.ResetTables(t, pool, establishmentsTable)
		return providerkit.RepositorySubject[domain.EstablishmentID, domain.EstablishmentSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.EstablishmentID, domain.EstablishmentSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewEstablishmentRepository, fn)
			},
			Reader:           establishmentReader(pool),
			NewID:            func(n int) domain.EstablishmentID { return domain.EstablishmentID("kit-" + strconv.Itoa(n)) },
			NewState:         establishment,
			Marker:           func(s domain.EstablishmentSnapshot) int { return s.TaxIDHashKeyVersion },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestEstablishmentRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := establishmentReader(pool)

	if err := saveEstablishment(ctx, pool, "est-1", establishment(1)); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	if got, version, err := reader.Load(ctx, "est-1"); err != nil || version != 1 || !got.Equal(identified(establishment(1), "est-1")) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", got, version, err, establishment(1))
	}

	bare := establishment(2)
	bare.ExternalReferences, bare.TradeName, bare.Receiver = nil, "", false
	if err := saveEstablishment(ctx, pool, "est-2", bare); err != nil {
		t.Fatalf("Save(bare) = %v", err)
	}
	got, _, err := reader.Load(ctx, "est-2")
	if err != nil || !got.Equal(identified(bare, "est-2")) || got.ExternalReferences == nil {
		t.Fatalf("Load(bare) = %#v, %v; want no reference as an empty collection", got, err)
	}
}

// The unique constraint is the authority the pre-check of the use case only
// anticipates: it fails as a technical error, never a conflict.
func TestTheSameTaxIDHashInTheOrganizationIsRefusedInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := saveEstablishment(ctx, pool, "est-a", establishment(1)); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}

	err := saveEstablishment(ctx, pool, "est-b", establishment(1))

	if err == nil || errors.Is(err, port.ErrVersionConflict) {
		t.Fatalf("Save(second) = %v, want the unique violation as a technical error", err)
	}
	if got := pg.Counts(t, pool, establishmentsTable)[establishmentsTable]; got != 1 {
		t.Fatalf("establishments = %d rows, want 1", got)
	}
}

func TestTheSameTaxIDHashIsAcceptedInAnotherOrganizationOrTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := saveEstablishment(withExecution(t, context.Background()), pool, "est-a", establishment(1)); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}
	elsewhere := establishment(1)
	elsewhere.OrganizationID = "org-other"

	if err := saveEstablishment(withExecution(t, context.Background()), pool, "est-b", elsewhere); err != nil {
		t.Fatalf("Save(another organization) = %v, want nil", err)
	}
	if err := saveEstablishment(withTenant(t, context.Background(), "globex"), pool, "est-a", establishment(1)); err != nil {
		t.Fatalf("Save(another tenant) = %v, want nil: uniqueness is per tenant", err)
	}
}

func TestTwoConcurrentAdditionsOfTheSameTaxIDLeaveOneRow(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	barrier := make(chan struct{})
	var (
		wg   sync.WaitGroup
		errs = make([]error, 2)
	)
	for i, id := range []domain.EstablishmentID{"est-a", "est-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = pg.Within(ctx, pool, provider.NewEstablishmentRepository, func(ctx context.Context, repo port.Repository[domain.EstablishmentID, domain.EstablishmentSnapshot]) error {
				<-barrier
				return repo.Save(ctx, id, establishment(1), 0)
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
	if got := pg.Counts(t, pool, establishmentsTable)[establishmentsTable]; got != 1 {
		t.Fatalf("establishments = %d rows, want 1", got)
	}
}

func establishmentIDs(snapshots []domain.EstablishmentSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}

func TestLoadByTaxIDHashReturnsEveryOrganizationOfTheHashInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	elsewhere := establishment(1)
	elsewhere.OrganizationID = "org-other"
	for id, s := range map[domain.EstablishmentID]domain.EstablishmentSnapshot{
		"est-a": establishment(1), "est-b": elsewhere, "est-c": establishment(2),
	} {
		if err := saveEstablishment(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}

	found, err := provider.NewEstablishmentsByTaxIDReader(postgres.NewReadPool(pool)).LoadByTaxIDHash(ctx, "hash-1")

	if got, want := establishmentIDs(found), []string{"est-a", "est-b"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("LoadByTaxIDHash() = %v, %v; want %v", got, err, want)
	}
}

func TestLoadByTaxIDHashOfEstablishmentsHeldOnlyByAnotherTenantIsACrossTenantAccess(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := saveEstablishment(withExecution(t, context.Background()), pool, "est-a", establishment(1)); err != nil {
		t.Fatalf("save: %v", err)
	}

	found, err := provider.NewEstablishmentsByTaxIDReader(postgres.NewReadPool(pool)).
		LoadByTaxIDHash(withTenant(t, context.Background(), "globex"), "hash-1")

	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.DataTenant != "acme" || len(found) != 0 {
		t.Fatalf("LoadByTaxIDHash() from another tenant = %v, %v; want a CrossTenantAccess and no row", found, err)
	}
}

func TestLoadByOrganizationReturnsTheEstablishmentsOfTheOrganization(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	elsewhere := establishment(3)
	elsewhere.OrganizationID = "org-other"
	for id, s := range map[domain.EstablishmentID]domain.EstablishmentSnapshot{
		"est-a": establishment(1), "est-b": establishment(2), "est-c": elsewhere,
	} {
		if err := saveEstablishment(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	reader := provider.NewEstablishmentsByOrganizationReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByOrganization(ctx, repoOrganizationID)

	if got, want := establishmentIDs(found), []string{"est-a", "est-b"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("LoadByOrganization() = %v, %v; want %v", got, err, want)
	}
	for _, s := range found {
		if want := identified(establishment(s.TaxIDHashKeyVersion), s.ID); !s.Equal(want) {
			t.Fatalf("establishment %+v lost state, want %+v", s, want)
		}
	}
	if none, err := reader.LoadByOrganization(ctx, "org-absent"); err != nil || len(none) != 0 {
		t.Fatalf("LoadByOrganization(org-absent) = %v, %v; want nothing and no error", none, err)
	}
}
