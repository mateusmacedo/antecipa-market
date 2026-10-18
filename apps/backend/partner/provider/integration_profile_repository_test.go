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

const profilesTable = "integration_profiles"

func profile(marker int) domain.IntegrationProfileSnapshot {
	return domain.IntegrationProfileSnapshot{
		PartnerID:                domain.PartnerID("prt-" + strconv.Itoa(marker)),
		Environment:              domain.EnvironmentProduction,
		Version:                  1,
		ChangeKind:               domain.ChangeInitial,
		Protocols:                []string{"https", "mtls"},
		Endpoint:                 "https://partner.example/api",
		SchemaVersion:            "v3",
		SecretRef:                "vault://partners/api-key",
		CertificateRef:           "cert-1",
		CertificateExpiresAt:     kernel.Instant(1_786_968_000_000_000_000 + marker),
		EffectiveFrom:            kernel.Instant(1_755_432_000_000_000_000),
		LastAlertedThresholdDays: 60,
	}
}

func profileVersion(id domain.ProfileID, partnerID domain.PartnerID, environment domain.Environment, n int, supersedes domain.ProfileID) domain.IntegrationProfileSnapshot {
	s := profile(1)
	s.ID, s.PartnerID, s.Environment, s.Version, s.SupersedesID = id, partnerID, environment, n, supersedes
	return s
}

func saveProfile(ctx context.Context, pool *pgxpool.Pool, s domain.IntegrationProfileSnapshot) error {
	return pg.Within(ctx, pool, provider.NewIntegrationProfileRepository, func(ctx context.Context, repo port.Repository[domain.ProfileID, domain.IntegrationProfileSnapshot]) error {
		return repo.Save(ctx, s.ID, s, 0)
	})
}

func TestIntegrationProfileRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.ProfileID, domain.IntegrationProfileSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.ProfileID, domain.IntegrationProfileSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.ProfileID, domain.IntegrationProfileSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewIntegrationProfileRepository, fn)
			},
			Reader:   provider.NewIntegrationProfileReader(postgres.NewReadPool(pool)),
			NewID:    func(n int) domain.ProfileID { return domain.ProfileID("kit-" + strconv.Itoa(n)) },
			NewState: profile,
			Marker: func(s domain.IntegrationProfileSnapshot) int {
				n, _ := strconv.Atoi(string(s.PartnerID)[len("prt-"):])
				return n
			},
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestIntegrationProfileRoundTripsTheWholeStateWithAndWithoutSupersedes(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewIntegrationProfileReader(postgres.NewReadPool(pool))
	first := profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, "")
	first.Protocols, first.CertificateRef, first.LastAlertedThresholdDays = nil, "", 0
	second := profileVersion("ipf-2", "prt-1", domain.EnvironmentProduction, 2, "ipf-1")
	second.ChangeKind = domain.ChangeUnannounced

	for _, want := range []domain.IntegrationProfileSnapshot{first, second} {
		pg.Seed(t, ctx, pool, provider.NewIntegrationProfileRepository, want.ID, want)

		if got, stored, err := reader.Load(ctx, want.ID); err != nil || stored != 1 || !got.Equal(want) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want %+v v1", want.ID, got, stored, err, want)
		}
	}
	if got := count(t, pool, "SELECT count(*) FROM integration_profiles WHERE supersedes_id IS NULL AND snapshot->'protocols' = '[]'::jsonb"); got != 1 {
		t.Fatalf("first versions with an empty protocol list = %d, want the first version stored with [] and no supersedes", got)
	}
	if got := countWhere(t, pool, "SELECT count(*) FROM integration_profiles WHERE certificate_expires_at = $1 AND environment = 'production'", int64(second.CertificateExpiresAt)); got != 2 {
		t.Fatalf("rows with the typed expiry and environment = %d, want 2", got)
	}
}

func TestTheSnapshotOfAProfileHoldsTheSecretReferenceAndNoOtherSecretField(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	pg.Seed(t, ctx, pool, provider.NewIntegrationProfileRepository, "ipf-1", profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, ""))

	var keys []string
	if err := pool.QueryRow(context.Background(), "SELECT array_agg(k ORDER BY k) FROM integration_profiles, jsonb_object_keys(snapshot) AS k").Scan(&keys); err != nil {
		t.Fatalf("read snapshot keys: %v", err)
	}

	want := []string{"certificateRef", "changeKind", "effectiveFrom", "endpoint", "lastAlertedThresholdDays", "protocols", "schemaVersion", "secretRef", "version"}
	if !slices.Equal(keys, want) {
		t.Fatalf("snapshot keys = %v, want %v", keys, want)
	}
}

func TestTheSeriesOfAnEnvironmentHoldsOneFirstVersionAndOneSuccessorOfEachVersion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   []domain.IntegrationProfileSnapshot
		second domain.IntegrationProfileSnapshot
		within string
	}{
		{"a second first version of the environment", []domain.IntegrationProfileSnapshot{profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, "")},
			profileVersion("ipf-x", "prt-1", domain.EnvironmentProduction, 1, ""), "integration_profiles_tenant_id_partner_id_environment_idx"},
		{"a second successor of the same version", []domain.IntegrationProfileSnapshot{
			profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, ""), profileVersion("ipf-2", "prt-1", domain.EnvironmentProduction, 2, "ipf-1"),
		}, profileVersion("ipf-x", "prt-1", domain.EnvironmentProduction, 2, "ipf-1"), "integration_profiles_tenant_id_supersedes_id_key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := appkit.OpenPool(t)
			ctx := withExecution(t, context.Background())
			for _, s := range tc.held {
				pg.Seed(t, ctx, pool, provider.NewIntegrationProfileRepository, s.ID, s)
			}

			err := saveProfile(ctx, pool, tc.second)

			var violation *pgconn.PgError
			if !errors.As(err, &violation) || violation.Code != "23505" || violation.ConstraintName != tc.within || errors.Is(err, port.ErrVersionConflict) {
				t.Fatalf("Save(%s) = %v, want the unique violation of %s as a technical error", tc.second.ID, err, tc.within)
			}
			if got := pg.Counts(t, pool, profilesTable)[profilesTable]; got != int64(len(tc.held)) {
				t.Fatalf("integration_profiles = %d rows, want %d", got, len(tc.held))
			}
		})
	}
}

func TestFirstVersionsOfAnotherEnvironmentPartnerOrTenantAreAccepted(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := saveProfile(withExecution(t, context.Background()), pool, profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, "")); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}

	for name, save := range map[string]func() error{
		"another environment": func() error {
			return saveProfile(withExecution(t, context.Background()), pool, profileVersion("ipf-2", "prt-1", domain.EnvironmentHomologation, 1, ""))
		},
		"another partner": func() error {
			return saveProfile(withExecution(t, context.Background()), pool, profileVersion("ipf-3", "prt-2", domain.EnvironmentProduction, 1, ""))
		},
		"another tenant": func() error {
			return saveProfile(withTenant(t, context.Background(), "globex"), pool, profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, ""))
		},
	} {
		if err := save(); err != nil {
			t.Fatalf("Save(%s) = %v, want nil", name, err)
		}
	}
}

func TestLoadByPartnerAnswersEveryProfileOfThePartner(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	for _, s := range []domain.IntegrationProfileSnapshot{
		profileVersion("ipf-1", "prt-1", domain.EnvironmentProduction, 1, ""),
		profileVersion("ipf-2", "prt-1", domain.EnvironmentHomologation, 1, ""),
		profileVersion("ipf-9", "prt-9", domain.EnvironmentProduction, 1, ""),
	} {
		pg.Seed(t, ctx, pool, provider.NewIntegrationProfileRepository, s.ID, s)
	}

	found, err := provider.NewIntegrationProfilesByPartnerReader(postgres.NewReadPool(pool)).LoadByPartner(ctx, "prt-1")

	ids := make([]string, 0, len(found))
	for _, s := range found {
		ids = append(ids, string(s.ID))
	}
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []string{"ipf-1", "ipf-2"}) {
		t.Fatalf("LoadByPartner() = %v, %v; want ipf-1 and ipf-2 of prt-1", ids, err)
	}
}

func countWhere(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}
