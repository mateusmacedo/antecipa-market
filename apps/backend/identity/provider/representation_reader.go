package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

func NewRepresentationReader(pool postgres.ReadPool) ports.RepresentationReader {
	return representationTable.Reader(pool)
}

var representationsByOrganization = representationTable.Relation("organization_id")

type RepresentationsByOrganizationReader struct{ pool postgres.ReadPool }

func NewRepresentationsByOrganizationReader(pool postgres.ReadPool) *RepresentationsByOrganizationReader {
	return &RepresentationsByOrganizationReader{pool: pool}
}

func (r *RepresentationsByOrganizationReader) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.RepresentationSnapshot, error) {
	return representationsByOrganization.Query(ctx, r.pool, string(organizationID))
}

var _ ports.RepresentationsByOrganizationReader = (*RepresentationsByOrganizationReader)(nil)
