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

const userAccountsTable = "user_accounts"

func userAccount(marker int) domain.UserAccountSnapshot {
	return domain.UserAccountSnapshot{
		PersonID:         "per-1",
		Status:           domain.UserAccountSuspended,
		ProvisionedAt:    kernel.Instant(1_755_432_000_000_000_000),
		SuspendedAt:      kernel.Instant(1_755_432_000_000_000_000 + int64(marker)),
		SuspensionReason: "acesso indevido",
	}
}

func saveUserAccount(ctx context.Context, pool *pgxpool.Pool, id domain.UserID, s domain.UserAccountSnapshot) error {
	return pg.Within(ctx, pool, provider.NewUserAccountRepository, func(ctx context.Context, repo port.Repository[domain.UserID, domain.UserAccountSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestUserAccountRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.UserID, domain.UserAccountSnapshot] {
		pg.ResetTables(t, pool, userAccountsTable)
		return providerkit.RepositorySubject[domain.UserID, domain.UserAccountSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.UserID, domain.UserAccountSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewUserAccountRepository, fn)
			},
			Reader:           provider.NewUserAccountReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.UserID { return domain.UserID("kit-" + strconv.Itoa(n)) },
			NewState:         userAccount,
			Marker:           func(s domain.UserAccountSnapshot) int { return int(int64(s.SuspendedAt) - 1_755_432_000_000_000_000) },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestUserAccountRoundTripsTheWholeStateUnderTheSubject(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewUserAccountReader(postgres.NewReadPool(pool))
	active := domain.UserAccountSnapshot{Status: domain.UserAccountActive, ProvisionedAt: kernel.Instant(1_755_432_000_000_000_000)}

	for id, s := range map[domain.UserID]domain.UserAccountSnapshot{"user-sub-suspended": userAccount(1), "user-sub-active": active} {
		if err := saveUserAccount(ctx, pool, id, s); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
		got, version, err := reader.Load(ctx, id)
		s.Subject = id
		if err != nil || version != 1 || !got.Equal(s) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want %+v v1", id, got, version, err, s)
		}
	}
}

func TestTheSameSubjectHasOneAccountInEachTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	acme := withExecution(t, context.Background())
	globex := withTenant(t, context.Background(), "globex")
	if err := saveUserAccount(acme, pool, "user-sub-1", userAccount(1)); err != nil {
		t.Fatalf("Save(acme) = %v", err)
	}

	if err := saveUserAccount(globex, pool, "user-sub-1", userAccount(2)); err != nil {
		t.Fatalf("Save(globex) = %v, want nil: the natural identity is per tenant", err)
	}

	reader := provider.NewUserAccountReader(postgres.NewReadPool(pool))
	inAcme, _, errAcme := reader.Load(acme, "user-sub-1")
	inGlobex, _, errGlobex := reader.Load(globex, "user-sub-1")
	if errAcme != nil || errGlobex != nil || inAcme.SuspendedAt == inGlobex.SuspendedAt {
		t.Fatalf("Load() = %+v, %v and %+v, %v; want one account of each tenant", inAcme, errAcme, inGlobex, errGlobex)
	}
}

func TestAnAccountHeldOnlyByAnotherTenantIsACrossTenantAccess(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := saveUserAccount(withExecution(t, context.Background()), pool, "user-sub-1", userAccount(1)); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	_, _, err := provider.NewUserAccountReader(postgres.NewReadPool(pool)).Load(withTenant(t, context.Background(), "globex"), "user-sub-1")

	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.DataTenant != "acme" || !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("Load() from another tenant = %v, want a CrossTenantAccess read as not found", err)
	}
}

func TestTwoConcurrentFirstProvisionsOfTheSameSubjectLeaveOneAccount(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	barrier := make(chan struct{})
	var (
		wg   sync.WaitGroup
		errs = make([]error, 2)
	)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = pg.Within(ctx, pool, provider.NewUserAccountRepository, func(ctx context.Context, repo port.Repository[domain.UserID, domain.UserAccountSnapshot]) error {
				<-barrier
				return repo.Save(ctx, "user-sub-1", userAccount(i), 0)
			})
		}()
	}
	close(barrier)
	wg.Wait()

	conflicts := 0
	for _, err := range errs {
		switch {
		case errors.Is(err, port.ErrVersionConflict):
			conflicts++
		case err != nil:
			t.Fatalf("Save() = %v, want nil or a retryable version conflict", err)
		}
	}
	if conflicts != 1 {
		t.Fatalf("errors = %v, want exactly one version conflict", errs)
	}
	if got := pg.Counts(t, pool, userAccountsTable)[userAccountsTable]; got != 1 {
		t.Fatalf("user_accounts = %d rows, want 1", got)
	}
}
