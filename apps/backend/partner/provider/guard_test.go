//go:build integration

package provider_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

func within[T any](t *testing.T, ctx context.Context, pool *pgxpool.Pool, read func(context.Context, *postgres.Tx) (T, error)) (T, error) {
	t.Helper()
	var got T
	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) *postgres.Tx { return tx })
	err := uow.Within(ctx, func(ctx context.Context, tx *postgres.Tx) error {
		var err error
		got, err = read(ctx, tx)
		return err
	})
	return got, err
}

func TestTheGuardedSeriesReadsInsideTheTransaction(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	pg.Seed(t, ctx, pool, provider.NewAgreementRepository, "agr-1", version("agr-1", "prt-1", 1, ""))
	pg.Seed(t, ctx, pool, provider.NewIntegrationProfileRepository, "ipf-1", profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, ""))
	pg.Seed(t, ctx, pool, provider.NewCapabilityRepository, "cap-1", capabilityVersion("cap-1", "prt-1", "register-receivable", 1, ""))

	agreements, errAgreements := within(t, ctx, pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.PartnerAgreementSnapshot, error) {
		return provider.NewAgreementsByPartnerWithin(tx).LoadByPartner(ctx, "prt-1")
	})
	profiles, errProfiles := within(t, ctx, pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.IntegrationProfileSnapshot, error) {
		return provider.NewIntegrationProfilesByPartnerWithin(tx).LoadByPartner(ctx, "prt-1")
	})
	capabilities, errCapabilities := within(t, ctx, pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.CapabilitySnapshot, error) {
		return provider.NewCapabilitiesByPartnerWithin(tx).LoadByPartner(ctx, "prt-1")
	})

	if errAgreements != nil || len(agreements) != 1 || agreements[0].ID != "agr-1" {
		t.Fatalf("agreements of prt-1 = %v, %v; want agr-1", agreements, errAgreements)
	}
	if errProfiles != nil || len(profiles) != 1 || profiles[0].ID != "ipf-1" {
		t.Fatalf("profiles of prt-1 = %v, %v; want ipf-1", profiles, errProfiles)
	}
	if errCapabilities != nil || len(capabilities) != 1 || capabilities[0].ID != "cap-1" {
		t.Fatalf("capabilities of prt-1 = %v, %v; want cap-1", capabilities, errCapabilities)
	}
}

func TestTheGuardedSeriesOfAPartnerWithNoRowIsEmpty(t *testing.T) {
	pool := appkit.OpenPool(t)

	got, err := within(t, withExecution(t, context.Background()), pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.PartnerAgreementSnapshot, error) {
		return provider.NewAgreementsByPartnerWithin(tx).LoadByPartner(ctx, "prt-none")
	})

	if err != nil || len(got) != 0 {
		t.Fatalf("LoadByPartner(prt-none) = %v, %v; want nothing and no error", got, err)
	}
}

// No command of the context reads a guarded series and then writes the same
// one, so the cycle is forced here: both read the capabilities of the partner
// under the shared lock, then each saves one of them and asks for the
// exclusive one. PostgreSQL breaks the deadlock (40P01), and the kernel hands
// the victim a version conflict, which the edge answers as Aborted.
func TestADeadlockOverAGuardedSeriesEndsAsAVersionConflict(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	first := capabilityVersion("cap-1", "prt-1", "register-receivable", 1, "")
	second := capabilityVersion("cap-2", "prt-1", "settle-receivable", 1, "")
	pg.Seed(t, ctx, pool, provider.NewCapabilityRepository, first.ID, first)
	pg.Seed(t, ctx, pool, provider.NewCapabilityRepository, second.ID, second)

	var read sync.WaitGroup
	read.Add(2)
	errs := make(chan error, 2)
	for _, s := range []domain.CapabilitySnapshot{first, second} {
		go func() {
			uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) *postgres.Tx { return tx })
			errs <- uow.Within(ctx, func(ctx context.Context, tx *postgres.Tx) error {
				if _, err := provider.NewCapabilitiesByPartnerWithin(tx).LoadByPartner(ctx, "prt-1"); err != nil {
					read.Done()
					return err
				}
				read.Done()
				read.Wait()
				s.RestrictionNote = "changed"
				return provider.NewCapabilityRepository(tx).Save(ctx, s.ID, s, 1)
			})
		}()
	}

	var committed, conflicted int
	for range 2 {
		switch err := <-errs; {
		case err == nil:
			committed++
		case errors.Is(err, port.ErrVersionConflict):
			conflicted++
		default:
			t.Errorf("Within() = %v, want nil or the version conflict of the deadlock", err)
		}
	}
	if committed != 1 || conflicted != 1 {
		t.Fatalf("committed %d and conflicted %d, want exactly one of each", committed, conflicted)
	}
}

func TestTheGuardedSeriesHeldOnlyByAnotherTenantIsACrossTenantAccess(t *testing.T) {
	pool := appkit.OpenPool(t)
	pg.Seed(t, withExecution(t, context.Background()), pool, provider.NewAgreementRepository, "agr-1", version("agr-1", "prt-1", 1, ""))

	got, err := within(t, withTenant(t, context.Background(), "globex"), pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.PartnerAgreementSnapshot, error) {
		return provider.NewAgreementsByPartnerWithin(tx).LoadByPartner(ctx, "prt-1")
	})

	var access port.CrossTenantAccess
	if !errors.As(err, &access) || len(got) != 0 {
		t.Fatalf("LoadByPartner() from another tenant = %v, %v; want a CrossTenantAccess and no row", got, err)
	}
}
