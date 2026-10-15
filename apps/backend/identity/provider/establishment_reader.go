package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

var (
	establishmentsByTaxIDHash    = establishmentTable.Relation("tax_id_hash")
	establishmentsByOrganization = establishmentTable.Relation("organization_id")
)

type EstablishmentsByTaxIDReader struct{ pool postgres.ReadPool }

func NewEstablishmentsByTaxIDReader(pool postgres.ReadPool) *EstablishmentsByTaxIDReader {
	return &EstablishmentsByTaxIDReader{pool: pool}
}

func (r *EstablishmentsByTaxIDReader) LoadByTaxIDHash(ctx context.Context, taxIDHash string) ([]domain.EstablishmentSnapshot, error) {
	return establishmentsByTaxIDHash.Query(ctx, r.pool, taxIDHash)
}

type EstablishmentsByOrganizationReader struct{ pool postgres.ReadPool }

func NewEstablishmentsByOrganizationReader(pool postgres.ReadPool) *EstablishmentsByOrganizationReader {
	return &EstablishmentsByOrganizationReader{pool: pool}
}

func (r *EstablishmentsByOrganizationReader) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.EstablishmentSnapshot, error) {
	return establishmentsByOrganization.Query(ctx, r.pool, string(organizationID))
}

var (
	_ ports.EstablishmentsByTaxIDReader        = (*EstablishmentsByTaxIDReader)(nil)
	_ ports.EstablishmentsByOrganizationReader = (*EstablishmentsByOrganizationReader)(nil)
)
