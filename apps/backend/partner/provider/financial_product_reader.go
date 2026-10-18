package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

func NewFinancialProductReader(pool postgres.ReadPool) ports.FinancialProductReader {
	return productTable.Reader(pool)
}

var productsByPartner = productTable.Relation("partner_id")

type FinancialProductsByPartnerReader struct{ pool postgres.ReadPool }

func NewFinancialProductsByPartnerReader(pool postgres.ReadPool) *FinancialProductsByPartnerReader {
	return &FinancialProductsByPartnerReader{pool: pool}
}

func (r *FinancialProductsByPartnerReader) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.FinancialProductSnapshot, error) {
	return productsByPartner.Query(ctx, r.pool, string(partnerID))
}

var _ ports.FinancialProductsByPartnerReader = (*FinancialProductsByPartnerReader)(nil)
