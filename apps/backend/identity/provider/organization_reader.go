package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// NewOrganizationReader serves the read side without the write side (UOW-11):
// it takes the pool because a query must not open a transaction.
func NewOrganizationReader(pool postgres.ReadPool) ports.OrganizationReader {
	return organizationTable.Reader(pool)
}

var organizationsByTaxIDHash = organizationTable.Relation("tax_id_hash")

type OrganizationsByTaxIDReader struct{ pool postgres.ReadPool }

func NewOrganizationsByTaxIDReader(pool postgres.ReadPool) *OrganizationsByTaxIDReader {
	return &OrganizationsByTaxIDReader{pool: pool}
}

func (r *OrganizationsByTaxIDReader) LoadByTaxIDHash(ctx context.Context, taxIDHash string) ([]domain.OrganizationSnapshot, error) {
	return organizationsByTaxIDHash.Query(ctx, r.pool, taxIDHash)
}

var _ ports.OrganizationsByTaxIDReader = (*OrganizationsByTaxIDReader)(nil)
