package ports_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// The aliases are the kernel generics instantiated for Organization: a value of
// either side is a value of the other, so no adapter is needed in between.
var (
	_ port.Repository[domain.OrganizationID, domain.OrganizationSnapshot] = ports.OrganizationRepository(nil)
	_ ports.OrganizationReader                                            = port.Reader[domain.OrganizationID, domain.OrganizationSnapshot](nil)
)

type taxIDReaderFunc func(context.Context, string) ([]domain.OrganizationSnapshot, error)

func (f taxIDReaderFunc) LoadByTaxIDHash(ctx context.Context, taxIDHash string) ([]domain.OrganizationSnapshot, error) {
	return f(ctx, taxIDHash)
}

func TestLoadByTaxIDHashAnswersWithEveryJurisdictionOfTheHash(t *testing.T) {
	held := []domain.OrganizationSnapshot{
		{ID: "org-1", Jurisdiction: "BR", TaxIDHash: "h-1"},
		{ID: "org-2", Jurisdiction: "PT", TaxIDHash: "h-1"},
	}
	var asked string
	var reader ports.OrganizationsByTaxIDReader = taxIDReaderFunc(
		func(_ context.Context, taxIDHash string) ([]domain.OrganizationSnapshot, error) {
			asked = taxIDHash
			return held, nil
		})

	got, err := reader.LoadByTaxIDHash(context.Background(), "h-1")

	if err != nil {
		t.Fatalf("LoadByTaxIDHash returned %v, want no error", err)
	}
	if asked != "h-1" {
		t.Fatalf("the reader was keyed on %q, want the hash %q: the tax id in clear never reaches it", asked, "h-1")
	}
	if len(got) != len(held) {
		t.Fatalf("LoadByTaxIDHash returned %d organizations, want %d: the jurisdiction is refined by the caller", len(got), len(held))
	}
}

func TestLoadByTaxIDHashSurfacesTheCrossTenantAccessUntranslated(t *testing.T) {
	access := port.CrossTenantAccess{Object: "organizations?tax_id_hash=h-1", ContextTenant: "globex", DataTenant: "acme"}
	var reader ports.OrganizationsByTaxIDReader = taxIDReaderFunc(
		func(context.Context, string) ([]domain.OrganizationSnapshot, error) { return nil, access })

	_, err := reader.LoadByTaxIDHash(context.Background(), "h-1")

	var got port.CrossTenantAccess
	if !errors.As(err, &got) || got != access {
		t.Fatalf("LoadByTaxIDHash returned %v, want the CrossTenantAccess the caller reads as absent", err)
	}
}
