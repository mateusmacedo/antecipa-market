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

const policiesTable = "operational_policies"

func policy(marker int) domain.OperationalPolicySnapshot {
	return domain.OperationalPolicySnapshot{
		PartnerID:          domain.PartnerID("prt-" + strconv.Itoa(marker)),
		Version:            1,
		EffectiveFrom:      kernel.Instant(1_755_432_000_000_000_000 + marker),
		RateLimitPerMinute: 100,
		BusinessDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		CutoffTime:         "17:00",
		ProcessingWindow:   "08:00-18:00",
		SlaRef:             "sla-gold",
		Fallback:           "manual queue",
		SupportContact:     "support@partner.example",
		IncidentContact:    "+55 11 99999-0000",
	}
}

func policyVersion(id domain.PolicyID, partnerID domain.PartnerID, n int, supersedes domain.PolicyID) domain.OperationalPolicySnapshot {
	s := policy(1)
	s.ID, s.PartnerID, s.Version, s.SupersedesID = id, partnerID, n, supersedes
	return s
}

func savePolicy(ctx context.Context, pool *pgxpool.Pool, s domain.OperationalPolicySnapshot) error {
	return pg.Within(ctx, pool, provider.NewOperationalPolicyRepository, func(ctx context.Context, repo port.Repository[domain.PolicyID, domain.OperationalPolicySnapshot]) error {
		return repo.Save(ctx, s.ID, s, 0)
	})
}

func TestOperationalPolicyRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.PolicyID, domain.OperationalPolicySnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.PolicyID, domain.OperationalPolicySnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.PolicyID, domain.OperationalPolicySnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewOperationalPolicyRepository, fn)
			},
			Reader:   provider.NewOperationalPolicyReader(postgres.NewReadPool(pool)),
			NewID:    func(n int) domain.PolicyID { return domain.PolicyID("kit-" + strconv.Itoa(n)) },
			NewState: policy,
			Marker: func(s domain.OperationalPolicySnapshot) int {
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

func TestOperationalPolicyRoundTripsTheWholeStateWithAndWithoutSupersedes(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewOperationalPolicyReader(postgres.NewReadPool(pool))
	first := policyVersion("pol-1", "prt-1", 1, "")
	first.BusinessDays, first.Fallback = nil, ""
	second := policyVersion("pol-2", "prt-1", 2, "pol-1")

	for _, want := range []domain.OperationalPolicySnapshot{first, second} {
		pg.Seed(t, ctx, pool, provider.NewOperationalPolicyRepository, want.ID, want)

		if got, stored, err := reader.Load(ctx, want.ID); err != nil || stored != 1 || !got.Equal(want) {
			t.Fatalf("Load(%s) = v%d, %v; want the stored policy back at v1", want.ID, stored, err)
		}
	}
	if got := count(t, pool, "SELECT count(*) FROM operational_policies WHERE supersedes_id IS NULL AND snapshot->'businessDays' = '[]'::jsonb"); got != 1 {
		t.Fatalf("first versions with an empty list of days = %d, want the first version stored with [] and no supersedes", got)
	}
}

func TestTheSeriesOfAPartnerHoldsOneFirstPolicyAndOneSuccessorOfEachVersion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   []domain.OperationalPolicySnapshot
		second domain.OperationalPolicySnapshot
		within string
	}{
		{"a second first version of the partner", []domain.OperationalPolicySnapshot{policyVersion("pol-1", "prt-1", 1, "")},
			policyVersion("pol-x", "prt-1", 1, ""), "operational_policies_tenant_id_partner_id_idx"},
		{"a second successor of the same version", []domain.OperationalPolicySnapshot{policyVersion("pol-1", "prt-1", 1, ""), policyVersion("pol-2", "prt-1", 2, "pol-1")},
			policyVersion("pol-x", "prt-1", 2, "pol-1"), "operational_policies_tenant_id_supersedes_id_key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := appkit.OpenPool(t)
			ctx := withExecution(t, context.Background())
			for _, s := range tc.held {
				pg.Seed(t, ctx, pool, provider.NewOperationalPolicyRepository, s.ID, s)
			}

			err := savePolicy(ctx, pool, tc.second)

			var violation *pgconn.PgError
			if !errors.As(err, &violation) || violation.Code != "23505" || violation.ConstraintName != tc.within || errors.Is(err, port.ErrVersionConflict) {
				t.Fatalf("Save(%s) = %v, want the unique violation of %s as a technical error", tc.second.ID, err, tc.within)
			}
			if got := pg.Counts(t, pool, policiesTable)[policiesTable]; got != int64(len(tc.held)) {
				t.Fatalf("operational_policies = %d rows, want %d", got, len(tc.held))
			}
		})
	}
}

func TestFirstPoliciesOfAnotherPartnerOrTenantAreAccepted(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := savePolicy(withExecution(t, context.Background()), pool, policyVersion("pol-1", "prt-1", 1, "")); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}

	if err := savePolicy(withExecution(t, context.Background()), pool, policyVersion("pol-2", "prt-2", 1, "")); err != nil {
		t.Fatalf("Save(another partner) = %v, want nil", err)
	}
	if err := savePolicy(withTenant(t, context.Background(), "globex"), pool, policyVersion("pol-1", "prt-1", 1, "")); err != nil {
		t.Fatalf("Save(another tenant) = %v, want nil", err)
	}
}

func TestLoadByPartnerAnswersEveryPolicyOfThePartner(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	for _, s := range []domain.OperationalPolicySnapshot{
		policyVersion("pol-1", "prt-1", 1, ""), policyVersion("pol-2", "prt-1", 2, "pol-1"), policyVersion("pol-9", "prt-9", 1, ""),
	} {
		pg.Seed(t, ctx, pool, provider.NewOperationalPolicyRepository, s.ID, s)
	}

	found, err := provider.NewOperationalPoliciesByPartnerReader(postgres.NewReadPool(pool)).LoadByPartner(ctx, "prt-1")

	versions := make([]int, 0, len(found))
	for _, s := range found {
		versions = append(versions, s.Version)
	}
	slices.Sort(versions)
	if err != nil || !slices.Equal(versions, []int{1, 2}) {
		t.Fatalf("LoadByPartner() = %v, %v; want versions 1 and 2 of prt-1", versions, err)
	}
}
