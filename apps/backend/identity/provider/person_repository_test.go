//go:build integration

package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
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

const personsTable = "persons"

func person(marker int) domain.PersonSnapshot {
	return domain.PersonSnapshot{
		FullName:               "Pessoa Sintetica da Silva",
		DocumentHash:           "hash-" + strconv.Itoa(marker),
		DocumentHashKeyVersion: marker,
		DocumentMasked:         "***.***.*89-09",
		CreatedAt:              kernel.Instant(1_755_432_000_000_000_000),
	}
}

func savePerson(ctx context.Context, pool *pgxpool.Pool, id domain.PersonID, s domain.PersonSnapshot) error {
	return pg.Within(ctx, pool, provider.NewPersonRepository, func(ctx context.Context, repo port.Repository[domain.PersonID, domain.PersonSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestPersonRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.PersonID, domain.PersonSnapshot] {
		pg.ResetTables(t, pool, personsTable)
		return providerkit.RepositorySubject[domain.PersonID, domain.PersonSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.PersonID, domain.PersonSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewPersonRepository, fn)
			},
			Reader:           provider.NewPersonReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.PersonID { return domain.PersonID("kit-" + strconv.Itoa(n)) },
			NewState:         person,
			Marker:           func(s domain.PersonSnapshot) int { return s.DocumentHashKeyVersion },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestPersonRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := savePerson(ctx, pool, "per-1", person(1)); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	got, version, err := provider.NewPersonReader(postgres.NewReadPool(pool)).Load(ctx, "per-1")

	want := person(1)
	want.ID = "per-1"
	if err != nil || version != 1 || !got.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", got, version, err, want)
	}
}

func TestThePersonSnapshotHoldsNoDocument(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := savePerson(ctx, pool, "per-1", person(1)); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	var raw []byte
	if err := pool.QueryRow(ctx, "SELECT snapshot FROM persons").Scan(&raw); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("decode snapshot %s: %v", raw, err)
	}

	got := slices.Sorted(maps.Keys(stored))

	if want := []string{"createdAt", "documentHashKeyVersion", "documentMasked", "fullName"}; !slices.Equal(got, want) {
		t.Fatalf("snapshot keys = %v, want %v", got, want)
	}
}

func TestTheSameDocumentHashIsRefusedInTheTenantAndAcceptedInAnother(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := savePerson(ctx, pool, "per-a", person(1)); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}

	err := savePerson(ctx, pool, "per-b", person(1))

	if err == nil || errors.Is(err, port.ErrVersionConflict) {
		t.Fatalf("Save(second) = %v, want the unique violation as a technical error", err)
	}
	if err := savePerson(withTenant(t, context.Background(), "globex"), pool, "per-b", person(1)); err != nil {
		t.Fatalf("Save(another tenant) = %v, want nil: uniqueness is per tenant", err)
	}
}

func TestTwoConcurrentRegistrationsOfTheSameDocumentLeaveOneRow(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	barrier := make(chan struct{})
	var (
		wg   sync.WaitGroup
		errs = make([]error, 2)
	)
	for i, id := range []domain.PersonID{"per-a", "per-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = pg.Within(ctx, pool, provider.NewPersonRepository, func(ctx context.Context, repo port.Repository[domain.PersonID, domain.PersonSnapshot]) error {
				<-barrier
				return repo.Save(ctx, id, person(1), 0)
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
	if got := pg.Counts(t, pool, personsTable)[personsTable]; got != 1 {
		t.Fatalf("persons = %d rows, want 1", got)
	}
}

func TestLoadByDocumentHashReturnsThePersonOfTheHashInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	for id, s := range map[domain.PersonID]domain.PersonSnapshot{"per-a": person(1), "per-b": person(2)} {
		if err := savePerson(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	reader := provider.NewPersonsByDocumentReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByDocumentHash(ctx, "hash-1")

	if err != nil || len(found) != 1 || found[0].ID != "per-a" || found[0].FullName != person(1).FullName {
		t.Fatalf("LoadByDocumentHash() = %+v, %v; want per-a alone", found, err)
	}
	if none, err := reader.LoadByDocumentHash(ctx, "hash-absent"); err != nil || len(none) != 0 {
		t.Fatalf("LoadByDocumentHash(hash-absent) = %v, %v; want nothing and no error", none, err)
	}
	_, err = reader.LoadByDocumentHash(withTenant(t, context.Background(), "globex"), "hash-1")
	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.DataTenant != "acme" {
		t.Fatalf("LoadByDocumentHash() from another tenant = %v, want a CrossTenantAccess", err)
	}
}
