package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type EstablishmentRepository = port.Repository[domain.EstablishmentID, domain.EstablishmentSnapshot]

type EstablishmentsByOrganizationReader interface {
	LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.EstablishmentSnapshot, error)
}
