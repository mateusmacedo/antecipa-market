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

const productsTable = "financial_products"

func financialProduct(marker int) domain.FinancialProductSnapshot {
	return domain.FinancialProductSnapshot{
		PartnerID:        domain.PartnerID("prt-" + strconv.Itoa(marker)),
		AgreementID:      "agr-1",
		Code:             "receivables-advance",
		Name:             "Antecipação de duplicatas",
		Version:          1,
		Status:           domain.ProductActive,
		ValidFrom:        kernel.Instant(1_755_432_000_000_000_000 + marker),
		ValidUntil:       kernel.Instant(1_786_968_000_000_000_000),
		AssetTypes:       []string{"duplicata", "cheque"},
		Regions:          []string{"SP", "RJ"},
		Currency:         "BRL",
		MinAmountMinor:   100_000,
		MaxAmountMinor:   50_000_000,
		MinTermDays:      7,
		MaxTermDays:      180,
		RequiredCheckIDs: []string{"CHK-01", "CHK-07"},
	}
}

func productVersion(id domain.ProductID, partnerID domain.PartnerID, code string, n int, supersedes domain.ProductID) domain.FinancialProductSnapshot {
	s := financialProduct(1)
	s.ID, s.PartnerID, s.Code, s.Version, s.SupersedesID = id, partnerID, code, n, supersedes
	return s
}

func saveProduct(ctx context.Context, pool *pgxpool.Pool, s domain.FinancialProductSnapshot) error {
	return pg.Within(ctx, pool, provider.NewFinancialProductRepository, func(ctx context.Context, repo port.Repository[domain.ProductID, domain.FinancialProductSnapshot]) error {
		return repo.Save(ctx, s.ID, s, 0)
	})
}

func TestFinancialProductRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.ProductID, domain.FinancialProductSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.ProductID, domain.FinancialProductSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.ProductID, domain.FinancialProductSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewFinancialProductRepository, fn)
			},
			Reader:   provider.NewFinancialProductReader(postgres.NewReadPool(pool)),
			NewID:    func(n int) domain.ProductID { return domain.ProductID("kit-" + strconv.Itoa(n)) },
			NewState: financialProduct,
			Marker: func(s domain.FinancialProductSnapshot) int {
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

func TestFinancialProductRoundTripsTheWholeStateWithAndWithoutSupersedes(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewFinancialProductReader(postgres.NewReadPool(pool))
	first := productVersion("fpr-1", "prt-1", "receivables-advance", 1, "")
	first.Regions, first.RequiredCheckIDs, first.ValidUntil = nil, nil, 0
	second := productVersion("fpr-2", "prt-1", "receivables-advance", 2, "fpr-1")
	second.Status = domain.ProductRetired

	for _, want := range []domain.FinancialProductSnapshot{first, second} {
		pg.Seed(t, ctx, pool, provider.NewFinancialProductRepository, want.ID, want)

		if got, stored, err := reader.Load(ctx, want.ID); err != nil || stored != 1 || !got.Equal(want) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want the stored product back at v1", want.ID, got, stored, err)
		}
	}
	if got := count(t, pool, "SELECT count(*) FROM financial_products WHERE supersedes_id IS NULL AND valid_until = 0 AND snapshot->'regions' = '[]'::jsonb AND snapshot->'requiredCheckIds' = '[]'::jsonb"); got != 1 {
		t.Fatalf("first versions of indefinite validity with empty lists = %d, want the first version stored with [] and no supersedes", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM financial_products WHERE status = 'retired' AND code = 'receivables-advance' AND valid_until = 1786968000000000000 AND supersedes_id = 'fpr-1'"); got != 1 {
		t.Fatalf("retired second versions with their typed columns = %d, want 1", got)
	}
}

func TestTheSeriesOfAPartnerAndCodeHoldsOneFirstProductAndOneSuccessorOfEachVersion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   []domain.FinancialProductSnapshot
		second domain.FinancialProductSnapshot
		within string
	}{
		{"a second first version of the code", []domain.FinancialProductSnapshot{productVersion("fpr-1", "prt-1", "receivables-advance", 1, "")},
			productVersion("fpr-x", "prt-1", "receivables-advance", 1, ""), "financial_products_tenant_id_series_origin_key"},
		{"a second successor of the same version", []domain.FinancialProductSnapshot{
			productVersion("fpr-1", "prt-1", "receivables-advance", 1, ""), productVersion("fpr-2", "prt-1", "receivables-advance", 2, "fpr-1"),
		}, productVersion("fpr-x", "prt-1", "receivables-advance", 2, "fpr-1"), "financial_products_tenant_id_supersedes_id_key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := appkit.OpenPool(t)
			ctx := withExecution(t, context.Background())
			for _, s := range tc.held {
				pg.Seed(t, ctx, pool, provider.NewFinancialProductRepository, s.ID, s)
			}

			err := saveProduct(ctx, pool, tc.second)

			var violation *pgconn.PgError
			if !errors.As(err, &violation) || violation.Code != "23505" || violation.ConstraintName != tc.within || errors.Is(err, port.ErrVersionConflict) {
				t.Fatalf("Save(%s) = %v, want the unique violation of %s, declared to the Save (ADR-066)", tc.second.ID, err, tc.within)
			}
			if got := pg.Counts(t, pool, productsTable)[productsTable]; got != int64(len(tc.held)) {
				t.Fatalf("financial_products = %d rows, want %d", got, len(tc.held))
			}
		})
	}
}

func TestFirstProductsOfAnotherCodePartnerOrTenantAreAccepted(t *testing.T) {
	pool := appkit.OpenPool(t)
	if err := saveProduct(withExecution(t, context.Background()), pool, productVersion("fpr-1", "prt-1", "receivables-advance", 1, "")); err != nil {
		t.Fatalf("Save(first) = %v", err)
	}

	if err := saveProduct(withExecution(t, context.Background()), pool, productVersion("fpr-2", "prt-1", "invoice-advance", 1, "")); err != nil {
		t.Fatalf("Save(another code) = %v, want nil", err)
	}
	if err := saveProduct(withExecution(t, context.Background()), pool, productVersion("fpr-3", "prt-2", "receivables-advance", 1, "")); err != nil {
		t.Fatalf("Save(another partner) = %v, want nil", err)
	}
	if err := saveProduct(withTenant(t, context.Background(), "globex"), pool, productVersion("fpr-1", "prt-1", "receivables-advance", 1, "")); err != nil {
		t.Fatalf("Save(another tenant) = %v, want nil", err)
	}
}

func TestLoadByPartnerAnswersEveryProductOfThePartner(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	for _, s := range []domain.FinancialProductSnapshot{
		productVersion("fpr-1", "prt-1", "receivables-advance", 1, ""),
		productVersion("fpr-2", "prt-1", "receivables-advance", 2, "fpr-1"),
		productVersion("fpr-3", "prt-1", "invoice-advance", 1, ""),
		productVersion("fpr-9", "prt-9", "receivables-advance", 1, ""),
	} {
		pg.Seed(t, ctx, pool, provider.NewFinancialProductRepository, s.ID, s)
	}

	found, err := provider.NewFinancialProductsByPartnerReader(postgres.NewReadPool(pool)).LoadByPartner(ctx, "prt-1")

	ids := make([]string, 0, len(found))
	for _, s := range found {
		ids = append(ids, string(s.ID))
	}
	slices.Sort(ids)
	if err != nil || !slices.Equal(ids, []string{"fpr-1", "fpr-2", "fpr-3"}) {
		t.Fatalf("LoadByPartner() = %v, %v; want every product of prt-1", ids, err)
	}
}
