package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

type (
	FinancialProductRepository = port.Repository[domain.ProductID, domain.FinancialProductSnapshot]
	FinancialProductReader     = port.Reader[domain.ProductID, domain.FinancialProductSnapshot]
)

// FinancialProductsByPartnerReader serves the series pre-check of DefineProduct.
// The kernel Relation filters by one column, so it answers every code of the
// partner and the caller refines the series in memory.
type FinancialProductsByPartnerReader interface {
	LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.FinancialProductSnapshot, error)
}
