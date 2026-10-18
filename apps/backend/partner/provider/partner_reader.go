package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

func NewPartnerReader(pool postgres.ReadPool) ports.PartnerReader {
	return partnerTable.Reader(pool)
}

var partnersByOrganization = partnerTable.Relation("organization_id")

type PartnersByOrganizationReader struct{ pool postgres.ReadPool }

func NewPartnersByOrganizationReader(pool postgres.ReadPool) *PartnersByOrganizationReader {
	return &PartnersByOrganizationReader{pool: pool}
}

func (r *PartnersByOrganizationReader) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.PartnerSnapshot, error) {
	return partnersByOrganization.Query(ctx, r.pool, string(organizationID))
}

var _ ports.PartnersByOrganizationReader = (*PartnersByOrganizationReader)(nil)

var partnersByRole = partnerTable.Relation("role")

type PartnersByRoleReader struct{ pool postgres.ReadPool }

func NewPartnersByRoleReader(pool postgres.ReadPool) *PartnersByRoleReader {
	return &PartnersByRoleReader{pool: pool}
}

func (r *PartnersByRoleReader) LoadByRole(ctx context.Context, role domain.PartnerRole) ([]domain.PartnerSnapshot, error) {
	return partnersByRole.Query(ctx, r.pool, string(role))
}

var _ ports.PartnersByRoleReader = (*PartnersByRoleReader)(nil)
