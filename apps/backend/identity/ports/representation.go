package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type (
	RepresentationRepository = port.Repository[domain.RepresentationID, domain.RepresentationSnapshot]
	RepresentationReader     = port.Reader[domain.RepresentationID, domain.RepresentationSnapshot]
)

type RepresentationsByOrganizationReader interface {
	LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.RepresentationSnapshot, error)
}
