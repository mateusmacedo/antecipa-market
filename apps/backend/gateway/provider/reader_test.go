//go:build integration

package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

func within[T any](t *testing.T, ctx context.Context, pool *pgxpool.Pool, read func(context.Context, *postgres.Tx) (T, error)) (T, error) {
	t.Helper()
	var got T
	err := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) *postgres.Tx { return tx }).Within(ctx, func(ctx context.Context, tx *postgres.Tx) error {
		var err error
		got, err = read(ctx, tx)
		return err
	})
	return got, err
}

func seedCommands(t *testing.T, ctx context.Context, pool *pgxpool.Pool, commands ...domain.ExternalCommandSnapshot) {
	t.Helper()
	for _, c := range commands {
		pg.Seed(t, ctx, pool, provider.NewExternalCommandRepository, c.Key, c)
	}
}

func TestTheCommandRelationsFilterByTheirColumn(t *testing.T) {
	ctx := withExecution(t)
	pool := appkit.OpenPool(t)
	first, second := command(1), command(2)
	first.Key, second.Key = "idem-1", "idem-2"
	second.Status, second.BusinessKey, second.SeriesKey = domain.StatusCreated, "eff-2", "prt-1:production:submitEffect:eff-2"
	seedCommands(t, ctx, pool, first, second)
	reader := provider.NewExternalCommandsReader(postgres.NewReadPool(pool))

	byKey, err := reader.LoadByBusinessKey(ctx, "eff-2")
	if err != nil || len(byKey) != 1 || byKey[0].Key != "idem-2" {
		t.Fatalf("LoadByBusinessKey(eff-2) = %+v, %v", byKey, err)
	}
	byStatus, err := reader.LoadByStatus(ctx, domain.StatusUnknownOutcome)
	if err != nil || len(byStatus) != 1 || byStatus[0].Key != "idem-1" {
		t.Fatalf("LoadByStatus(unknown_outcome) = %+v, %v", byStatus, err)
	}
	byOutbound, err := reader.LoadByOutboundKey(ctx, "out-2")
	if err != nil || len(byOutbound) != 1 || byOutbound[0].Key != "idem-2" {
		t.Fatalf("LoadByOutboundKey(out-2) = %+v, %v", byOutbound, err)
	}
}

func TestTheGuardedReadsSeeTheSeriesAndTheProfilesInsideTheTransaction(t *testing.T) {
	ctx := withExecution(t)
	pool := appkit.OpenPool(t)
	first := command(1)
	first.Key = "idem-1"
	seedCommands(t, ctx, pool, first)
	stored := profile("prt-1:production", 2)
	pg.Seed(t, ctx, pool, provider.NewCapabilityProfileRepository, domain.ProfileKeyOf("prt-1:production", 2), stored)

	series, err := within(t, ctx, pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.ExternalCommandSnapshot, error) {
		return provider.NewExternalCommandsBySeriesWithin(tx).LoadBySeries(ctx, first.SeriesKey)
	})
	if err != nil || len(series) != 1 || series[0].Key != "idem-1" {
		t.Fatalf("LoadBySeries() = %+v, %v", series, err)
	}
	profiles, err := within(t, ctx, pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.CapabilityProfileSnapshot, error) {
		return provider.NewCapabilityProfilesByConnectorWithin(tx).LoadByConnector(ctx, "prt-1:production")
	})
	if err != nil || len(profiles) != 1 || profiles[0].Version != 2 {
		t.Fatalf("LoadByConnector() = %+v, %v", profiles, err)
	}
	empty, err := within(t, ctx, pool, func(ctx context.Context, tx *postgres.Tx) ([]domain.ExternalCommandSnapshot, error) {
		return provider.NewExternalCommandsBySeriesWithin(tx).LoadBySeries(ctx, "prt-1:production:submitEffect:none")
	})
	if err != nil || len(empty) != 0 {
		t.Fatalf("an empty series = %+v, %v; want nothing and no error", empty, err)
	}
}

func TestARelationOfAnotherTenantAnswersAsCrossTenantAccess(t *testing.T) {
	pool := appkit.OpenPool(t)
	first := command(1)
	first.Key = "idem-1"
	seedCommands(t, withTenant(t, "other"), pool, first)
	reader := provider.NewExternalCommandsReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByStatus(withExecution(t), domain.StatusUnknownOutcome)

	var access port.CrossTenantAccess
	if len(found) != 0 || !errors.As(err, &access) {
		t.Fatalf("LoadByStatus() = %+v, %v; want nothing and a cross-tenant access", found, err)
	}
}
