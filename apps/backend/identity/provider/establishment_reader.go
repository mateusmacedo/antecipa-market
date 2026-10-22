package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

var establishmentsByOrganization = establishmentTable.Relation("organization_id")

type EstablishmentsByOrganizationReader struct{ pool postgres.ReadPool }

func NewEstablishmentsByOrganizationReader(pool postgres.ReadPool) *EstablishmentsByOrganizationReader {
	return &EstablishmentsByOrganizationReader{pool: pool}
}

func (r *EstablishmentsByOrganizationReader) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.EstablishmentSnapshot, error) {
	return establishmentsByOrganization.Query(ctx, r.pool, string(organizationID))
}

var _ ports.EstablishmentsByOrganizationReader = (*EstablishmentsByOrganizationReader)(nil)
