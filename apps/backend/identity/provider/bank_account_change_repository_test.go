//go:build integration

package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
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

const bankAccountChangesTable = "bank_account_changes"

const changeRequestedAt = 1_755_432_000_000_000_000

func requestedChange(marker int) domain.BankAccountChangeSnapshot {
	return domain.BankAccountChangeSnapshot{
		OrganizationID:        repoOrganizationID,
		DestinationEvidenceID: "evd-1",
		RequestedByUserID:     "user-sub-1",
		Status:                domain.BankAccountChangeStatusRequested,
		RequestedAt:           kernel.Instant(changeRequestedAt + int64(marker)),
	}
}

func approvedChange(marker int) domain.BankAccountChangeSnapshot {
	s := requestedChange(marker)
	s.Status = domain.BankAccountChangeStatusApproved
	s.ApprovedByUserID = "user-sub-2"
	s.ApprovedAt = kernel.Instant(1_755_518_400_000_000_000)
	return s
}

func saveBankAccountChange(ctx context.Context, pool *pgxpool.Pool, id domain.BankAccountChangeID, s domain.BankAccountChangeSnapshot) error {
	return pg.Within(ctx, pool, provider.NewBankAccountChangeRepository, func(ctx context.Context, repo port.Repository[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestBankAccountChangeRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot] {
		pg.ResetTables(t, pool, bankAccountChangesTable)
		return providerkit.RepositorySubject[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewBankAccountChangeRepository, fn)
			},
			Reader:           provider.NewBankAccountChangeReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.BankAccountChangeID { return domain.BankAccountChangeID("kit-" + strconv.Itoa(n)) },
			NewState:         approvedChange,
			Marker:           func(s domain.BankAccountChangeSnapshot) int { return int(int64(s.RequestedAt) - changeRequestedAt) },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestBankAccountChangeRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewBankAccountChangeReader(postgres.NewReadPool(pool))

	for id, s := range map[domain.BankAccountChangeID]domain.BankAccountChangeSnapshot{
		"bac-requested": requestedChange(1), "bac-approved": approvedChange(2),
	} {
		if err := saveBankAccountChange(ctx, pool, id, s); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
		got, version, err := reader.Load(ctx, id)
		s.ID = id
		if err != nil || version != 1 || !got.Equal(s) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want %+v v1", id, got, version, err, s)
		}
	}
}

func TestTheBankAccountChangeSnapshotKeepsItsStoredLayout(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := saveBankAccountChange(ctx, pool, "bac-approved", approvedChange(1)); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	var (
		organizationID string
		raw            []byte
	)
	if err := pool.QueryRow(ctx, "SELECT organization_id, snapshot FROM bank_account_changes WHERE bank_account_change_id = $1", "bac-approved").
		Scan(&organizationID, &raw); err != nil {
		t.Fatalf("select row: %v", err)
	}
	if organizationID != string(repoOrganizationID) {
		t.Fatalf("organization_id = %q, want %q in the typed column", organizationID, repoOrganizationID)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	got := make([]string, 0, len(fields))
	for key := range fields {
		got = append(got, key)
	}
	slices.Sort(got)
	want := []string{"approvedAt", "approvedByUserId", "destinationEvidenceId", "requestedAt", "requestedByUserId", "status"}
	if !slices.Equal(got, want) {
		t.Fatalf("snapshot keys = %v, want %v", got, want)
	}
}

func bankAccountChangeIDs(snapshots []domain.BankAccountChangeSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}

func TestLoadByOrganizationReturnsTheBankAccountChangesOfTheOrganization(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	elsewhere := requestedChange(3)
	elsewhere.OrganizationID = "org-other"
	held := map[domain.BankAccountChangeID]domain.BankAccountChangeSnapshot{
		"bac-a": requestedChange(1), "bac-b": approvedChange(2),
	}
	for id, s := range map[domain.BankAccountChangeID]domain.BankAccountChangeSnapshot{
		"bac-a": held["bac-a"], "bac-b": held["bac-b"], "bac-c": elsewhere,
	} {
		if err := saveBankAccountChange(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	reader := provider.NewBankAccountChangesByOrganizationReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByOrganization(ctx, repoOrganizationID)

	if got, want := bankAccountChangeIDs(found), []string{"bac-a", "bac-b"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("LoadByOrganization() = %v, %v; want %v", got, err, want)
	}
	for _, s := range found {
		want := held[s.ID]
		want.ID = s.ID
		if !s.Equal(want) {
			t.Fatalf("bank account change %+v lost state, want %+v", s, want)
		}
	}
	_, err = reader.LoadByOrganization(withTenant(t, context.Background(), "globex"), repoOrganizationID)
	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.DataTenant != "acme" {
		t.Fatalf("LoadByOrganization() from another tenant = %v, want a CrossTenantAccess", err)
	}
}
