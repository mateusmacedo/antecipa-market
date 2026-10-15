package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

func NewBankAccountChangeReader(pool postgres.ReadPool) ports.BankAccountChangeReader {
	return bankAccountChangeTable.Reader(pool)
}

var bankAccountChangesByOrganization = bankAccountChangeTable.Relation("organization_id")

type BankAccountChangesByOrganizationReader struct{ pool postgres.ReadPool }

func NewBankAccountChangesByOrganizationReader(pool postgres.ReadPool) *BankAccountChangesByOrganizationReader {
	return &BankAccountChangesByOrganizationReader{pool: pool}
}

func (r *BankAccountChangesByOrganizationReader) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.BankAccountChangeSnapshot, error) {
	return bankAccountChangesByOrganization.Query(ctx, r.pool, string(organizationID))
}

var _ ports.BankAccountChangesByOrganizationReader = (*BankAccountChangesByOrganizationReader)(nil)
