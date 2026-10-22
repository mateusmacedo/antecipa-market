package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

type (
	PartnerRepository = port.Repository[domain.PartnerID, domain.PartnerSnapshot]
	PartnerReader     = port.Reader[domain.PartnerID, domain.PartnerSnapshot]
)

// PartnersByOrganizationReader serves FindPartnersByOrganization: every role
// of the organization.
type PartnersByOrganizationReader interface {
	LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.PartnerSnapshot, error)
}

// OrganizationDirectory answers the kinds Identity holds for an organization,
// or ErrNotFound when the tenant holds no such organization.
type OrganizationDirectory interface {
	FindOrganization(ctx context.Context, tenant port.TenantID, organizationID domain.OrganizationID) ([]domain.OrganizationKind, error)
}

type PartnersByRoleReader interface {
	LoadByRole(ctx context.Context, role domain.PartnerRole) ([]domain.PartnerSnapshot, error)
}
