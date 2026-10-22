//go:build integration

package provider_test

import (
	"context"
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

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

const agreementsTable = "partner_agreements"

func agreement(marker int) domain.PartnerAgreementSnapshot {
	return domain.PartnerAgreementSnapshot{
		PartnerID:            domain.PartnerID("prt-" + strconv.Itoa(marker)),
		Version:              1,
		Status:               domain.AgreementActive,
		ValidFrom:            kernel.Instant(1_755_432_000_000_000_000),
		ValidUntil:           kernel.Instant(1_786_968_000_000_000_000),
		ContractEvidenceID:   "evd-1",
		InstrumentTemplateID: "tpl-1",
		ActivatedAt:          kernel.Instant(1_755_518_400_000_000_000 + marker),
	}
}

func version(id domain.AgreementID, partnerID domain.PartnerID, n int, supersedes domain.AgreementID) domain.PartnerAgreementSnapshot {
	s := agreement(1)
	s.ID, s.PartnerID, s.Version, s.SupersedesID, s.Status = id, partnerID, n, supersedes, domain.AgreementDraft
	return s
}

func saveAgreement(ctx context.Context, pool *pgxpool.Pool, s domain.PartnerAgreementSnapshot) error {
	return pg.Within(ctx, pool, provider.NewAgreementRepository, func(ctx context.Context, repo port.Repository[domain.AgreementID, domain.PartnerAgreementSnapshot]) error {
		return repo.Save(ctx, s.ID, s, 0)
	})
}

func TestAgreementRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.AgreementID, domain.PartnerAgreementSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.AgreementID, domain.PartnerAgreementSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.AgreementID, domain.PartnerAgreementSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewAgreementRepository, fn)
			},
			Reader: provider.NewAgreementReader(postgres.NewReadPool(pool)),
			NewID:  func(n int) domain.AgreementID { return domain.AgreementID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int) domain.PartnerAgreementSnapshot {
				s := agreement(marker)
				s.PartnerID = kitPartner
				return s
			},
			Marker:           func(s domain.PartnerAgreementSnapshot) int { return int(s.ActivatedAt - 1_755_518_400_000_000_000) },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestAgreementRoundTripsTheWholeStateWithAndWithoutSupersedes(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewAgreementReader(postgres.NewReadPool(pool))
	first := version("agr-1", "prt-1", 1, "")
	first.ValidUntil, first.InstrumentTemplateID = 0, ""
	second := agreement(1)
	second.ID, second.Version, second.SupersedesID = "agr-2", 2, "agr-1"

	for _, want := range []domain.PartnerAgreementSnapshot{first, second} {
		pg.Seed(t, ctx, pool, provider.NewAgreementRepository, want.ID, want)

		if got, stored, err := reader.Load(ctx, want.ID); err != nil || stored != 1 || !got.Equal(want) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want %+v v1", want.ID, got, stored, err, want)
		}
	}
	if got := count(t, pool, "SELECT count(*) FROM partner_agreements WHERE supersedes_id IS NULL"); got != 1 {
		t.Fatalf("rows without supersedes = %d, want the first version stored as null", got)
	}
}

func TestTheSeriesHoldsOneFirstVersionAndOneSuccessorOfEachVersion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   []domain.PartnerAgreementSnapshot
		second domain.PartnerAgreementSnapshot
	}{
		{"a second first version of the partner", []domain.PartnerAgreementSnapshot{version("agr-1", "prt-1", 1, "")},
			version("agr-x", "prt-1", 1, "")},
		{"a second successor of the same version", []domain.PartnerAgreementSnapshot{version("agr-1", "prt-1", 1, ""), version("agr-2", "prt-1", 2, "agr-1")},
			version("agr-x", "prt-1", 2, "agr-1")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := appkit.OpenPool(t)
			ctx := withExecution(t, context.Background())
			for _, s := range tc.held {
				pg.Seed(t, ctx, pool, provider.NewAgreementRepository, s.ID, s)
			}

			err := saveAgreement(ctx, pool, tc.second)

			if err == nil || errors.Is(err, port.ErrVersionConflict) {
				t.Fatalf("Save(%s) = %v, want the unique violation, declared to the Save (ADR-066)", tc.second.ID, err)
			}
			if got := pg.Counts(t, pool, agreementsTable)[agreementsTable]; got != int64(len(tc.held)) {
				t.Fatalf("partner_agreements = %d rows, want %d", got, len(tc.held))
			}
		})
	}
}

func TestFirstVersionsOfAnotherPartnerOrTenantAreAccepted(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := saveAgreement(withExecution(t, context.Background()), pool, version("agr-1", "prt-1", 1, "")); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}

	if err := saveAgreement(withExecution(t, context.Background()), pool, version("agr-2", "prt-2", 1, "")); err != nil {
		t.Fatalf("Save(another partner) = %v, want nil", err)
	}
	if err := saveAgreement(withTenant(t, context.Background(), "globex"), pool, version("agr-1", "prt-1", 1, "")); err != nil {
		t.Fatalf("Save(another tenant) = %v, want nil", err)
	}
}

func TestLoadByPartnerAnswersEveryVersionOfTheSeries(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	for _, s := range []domain.PartnerAgreementSnapshot{
		version("agr-1", "prt-1", 1, ""), version("agr-2", "prt-1", 2, "agr-1"), version("agr-9", "prt-9", 1, ""),
	} {
		pg.Seed(t, ctx, pool, provider.NewAgreementRepository, s.ID, s)
	}

	found, err := provider.NewAgreementsByPartnerReader(postgres.NewReadPool(pool)).LoadByPartner(ctx, "prt-1")

	versions := make([]int, 0, len(found))
	for _, s := range found {
		versions = append(versions, s.Version)
	}
	slices.Sort(versions)
	if err != nil || !slices.Equal(versions, []int{1, 2}) {
		t.Fatalf("LoadByPartner() = %v, %v; want versions 1 and 2 of prt-1", versions, err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, query string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}
