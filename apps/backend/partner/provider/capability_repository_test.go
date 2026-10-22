//go:build integration

package provider_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

const capabilitiesTable = "capabilities"

func capabilityState(marker int) domain.CapabilitySnapshot {
	return domain.CapabilitySnapshot{
		PartnerID:           domain.PartnerID("prt-" + strconv.Itoa(marker)),
		Operation:           "register-receivable",
		AssetType:           "duplicata",
		Environment:         domain.EnvironmentProduction,
		Version:             1,
		IsWrite:             true,
		StateQuerySupported: true,
		CallbackApplicable:  true,
		Status:              domain.CapabilityStatusSuspended,
		Homologation: []domain.CriterionResult{
			{CriterionID: "H1", Passed: true, EvidenceID: "evd-1", RecordedAt: kernel.Instant(1_755_432_000_000_000_000)},
			{CriterionID: "H9", Passed: false, RecordedAt: kernel.Instant(1_755_432_000_000_000_001)},
		},
		Rollout: domain.Rollout{
			Mode: domain.RolloutPercentage, AllowlistTenantIDs: []string{"tenant-a"}, Percentage: 10, ExcludedTenantIDs: []string{"tenant-x"},
		},
		Suspension: domain.Suspension{Reason: "partner incident", Trigger: domain.TriggerManual, SuspendedAt: kernel.Instant(1_755_518_400_000_000_000)},
		EnabledAt:  kernel.Instant(1_755_432_000_000_000_000 + marker),
	}
}

func capabilityVersion(id domain.CapabilityID, partnerID domain.PartnerID, operation string, n int, supersedes domain.CapabilityID) domain.CapabilitySnapshot {
	s := capabilityState(1)
	s.ID, s.PartnerID, s.Operation, s.Version, s.SupersedesID = id, partnerID, operation, n, supersedes
	return s
}

func saveCapability(ctx context.Context, pool *pgxpool.Pool, s domain.CapabilitySnapshot) error {
	return pg.Within(ctx, pool, provider.NewCapabilityRepository, func(ctx context.Context, repo port.Repository[domain.CapabilityID, domain.CapabilitySnapshot]) error {
		return repo.Save(ctx, s.ID, s, 0)
	})
}

func TestCapabilityRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.CapabilityID, domain.CapabilitySnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.CapabilityID, domain.CapabilitySnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.CapabilityID, domain.CapabilitySnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewCapabilityRepository, fn)
			},
			Reader: provider.NewCapabilityReader(postgres.NewReadPool(pool)),
			NewID:  func(n int) domain.CapabilityID { return domain.CapabilityID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int) domain.CapabilitySnapshot {
				s := capabilityState(marker)
				s.PartnerID = kitPartner
				return s
			},
			Marker:           func(s domain.CapabilitySnapshot) int { return int(s.EnabledAt - kitBase) },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestCapabilityRoundTripsTheWholeStateWithAndWithoutSupersedes(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewCapabilityReader(postgres.NewReadPool(pool))
	first := capabilityVersion("cap-1", "prt-1", "register-receivable", 1, "")
	first.Status, first.Homologation, first.Rollout, first.Suspension, first.EnabledAt = domain.CapabilityStatusDeclared, nil, domain.Rollout{Mode: domain.RolloutDisabled}, domain.Suspension{}, 0
	second := capabilityVersion("cap-2", "prt-1", "register-receivable", 2, "cap-1")

	for _, want := range []domain.CapabilitySnapshot{first, second} {
		pg.Seed(t, ctx, pool, provider.NewCapabilityRepository, want.ID, want)

		if got, stored, err := reader.Load(ctx, want.ID); err != nil || stored != 1 || !got.Equal(want) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want %+v v1", want.ID, got, stored, err, want)
		}
	}
	if got := count(t, pool, "SELECT count(*) FROM capabilities WHERE supersedes_id IS NULL AND snapshot->'homologation' = '[]'::jsonb AND snapshot->'rollout'->'allowlistTenantIds' = '[]'::jsonb"); got != 1 {
		t.Fatalf("first versions with empty lists = %d, want the first version stored with [] and no supersedes", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM capabilities WHERE status = 'suspended' AND partner_id = 'prt-1' AND operation = 'register-receivable' AND asset_type = 'duplicata' AND environment = 'production'"); got != 1 {
		t.Fatalf("suspended rows with the typed series = %d, want 1", got)
	}
}

func TestTheSnapshotOfACapabilityHoldsItsStateUnderStableNames(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	pg.Seed(t, ctx, pool, provider.NewCapabilityRepository, "cap-1", capabilityVersion("cap-1", "prt-1", "register-receivable", 1, ""))

	var keys []string
	if err := pool.QueryRow(context.Background(), "SELECT array_agg(k ORDER BY k) FROM capabilities, jsonb_object_keys(snapshot) AS k").Scan(&keys); err != nil {
		t.Fatalf("read snapshot keys: %v", err)
	}

	want := []string{"callbackApplicable", "enabledAt", "homologation", "isWrite", "restrictionNote", "rollout", "stateQuerySupported", "suspension", "version"}
	if !slices.Equal(keys, want) {
		t.Fatalf("snapshot keys = %v, want %v", keys, want)
	}
}

func TestTheSeriesOfACapabilityHoldsOneFirstVersionAndOneSuccessorOfEachVersion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   []domain.CapabilitySnapshot
		second domain.CapabilitySnapshot
		within string
	}{
		{"a second first version of the series", []domain.CapabilitySnapshot{capabilityVersion("cap-1", "prt-1", "register-receivable", 1, "")},
			capabilityVersion("cap-x", "prt-1", "register-receivable", 1, ""), "capabilities_tenant_id_series_origin_key"},
		{"a second successor of the same version", []domain.CapabilitySnapshot{
			capabilityVersion("cap-1", "prt-1", "register-receivable", 1, ""), capabilityVersion("cap-2", "prt-1", "register-receivable", 2, "cap-1"),
		}, capabilityVersion("cap-x", "prt-1", "register-receivable", 2, "cap-1"), "capabilities_tenant_id_supersedes_id_key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := appkit.OpenPool(t)
			ctx := withExecution(t, context.Background())
			for _, s := range tc.held {
				pg.Seed(t, ctx, pool, provider.NewCapabilityRepository, s.ID, s)
			}

			err := saveCapability(ctx, pool, tc.second)

			var violation *pgconn.PgError
			if !errors.As(err, &violation) || violation.Code != "23505" || violation.ConstraintName != tc.within || errors.Is(err, port.ErrVersionConflict) {
				t.Fatalf("Save(%s) = %v, want the unique violation of %s, declared to the Save (ADR-066)", tc.second.ID, err, tc.within)
			}
			if got := pg.Counts(t, pool, capabilitiesTable)[capabilitiesTable]; got != int64(len(tc.held)) {
				t.Fatalf("capabilities = %d rows, want %d", got, len(tc.held))
			}
		})
	}
}

func TestFirstVersionsOfAnotherSeriesOrTenantAreAccepted(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := saveCapability(withExecution(t, context.Background()), pool, capabilityVersion("cap-1", "prt-1", "register-receivable", 1, "")); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}
	another := func(id domain.CapabilityID, edit func(*domain.CapabilitySnapshot)) domain.CapabilitySnapshot {
		s := capabilityVersion(id, "prt-1", "register-receivable", 1, "")
		edit(&s)
		return s
	}

	for name, save := range map[string]func() error{
		"another operation": func() error {
			return saveCapability(withExecution(t, context.Background()), pool, another("cap-2", func(s *domain.CapabilitySnapshot) { s.Operation = "settle" }))
		},
		"another asset type": func() error {
			return saveCapability(withExecution(t, context.Background()), pool, another("cap-3", func(s *domain.CapabilitySnapshot) { s.AssetType = "cpr" }))
		},
		"another environment": func() error {
			return saveCapability(withExecution(t, context.Background()), pool, another("cap-4", func(s *domain.CapabilitySnapshot) {
				s.Environment = domain.EnvironmentHomologation
			}))
		},
		"another partner": func() error {
			return saveCapability(withExecution(t, context.Background()), pool, another("cap-5", func(s *domain.CapabilitySnapshot) { s.PartnerID = "prt-2" }))
		},
		"another tenant": func() error {
			return saveCapability(withTenant(t, context.Background(), "globex"), pool, capabilityVersion("cap-1", "prt-1", "register-receivable", 1, ""))
		},
	} {
		if err := save(); err != nil {
			t.Fatalf("Save(%s) = %v, want nil", name, err)
		}
	}
}

func TestLoadByPartnerAnswersEveryCapabilityOfThePartner(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	for _, s := range []domain.CapabilitySnapshot{
		capabilityVersion("cap-1", "prt-1", "register-receivable", 1, ""),
		capabilityVersion("cap-2", "prt-1", "settle", 1, ""),
		capabilityVersion("cap-9", "prt-9", "register-receivable", 1, ""),
	} {
		pg.Seed(t, ctx, pool, provider.NewCapabilityRepository, s.ID, s)
	}

	found, err := provider.NewCapabilitiesByPartnerReader(postgres.NewReadPool(pool)).LoadByPartner(ctx, "prt-1")

	ids := make([]string, 0, len(found))
	for _, s := range found {
		ids = append(ids, string(s.ID))
	}
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []string{"cap-1", "cap-2"}) {
		t.Fatalf("LoadByPartner() = %v, %v; want cap-1 and cap-2 of prt-1", ids, err)
	}
}
