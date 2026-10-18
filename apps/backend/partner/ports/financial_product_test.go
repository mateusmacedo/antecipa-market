package ports_test

import (
	"context"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

var (
	_ port.Repository[domain.ProductID, domain.FinancialProductSnapshot] = ports.FinancialProductRepository(nil)
	_ ports.FinancialProductReader                                       = port.Reader[domain.ProductID, domain.FinancialProductSnapshot](nil)
)

type productsByPartner map[domain.PartnerID][]domain.FinancialProductSnapshot

func (r productsByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.FinancialProductSnapshot, error) {
	return r[id], nil
}

func TestTheProductsOfAPartnerAnswerEveryCodeAndVersion(t *testing.T) {
	var products ports.FinancialProductsByPartnerReader = productsByPartner{"prt-1": {
		{ID: "fpr-1", Code: "receivables-advance", Version: 1},
		{ID: "fpr-2", Code: "receivables-advance", Version: 2, SupersedesID: "fpr-1"},
		{ID: "fpr-3", Code: "invoice-advance", Version: 1},
	}}

	found, err := products.LoadByPartner(context.Background(), "prt-1")

	if err != nil || len(found) != 3 {
		t.Fatalf("LoadByPartner() = %v, %v; want every version of every code of the partner", found, err)
	}
}
