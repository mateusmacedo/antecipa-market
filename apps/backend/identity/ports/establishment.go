package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type EstablishmentRepository = port.Repository[domain.EstablishmentID, domain.EstablishmentSnapshot]

// EstablishmentsByTaxIDReader serves the uniqueness pre-check. The kernel
// Relation filters by one column, so it answers every organization of the hash
// and the caller refines the result in memory.
type EstablishmentsByTaxIDReader interface {
	LoadByTaxIDHash(ctx context.Context, taxIDHash string) ([]domain.EstablishmentSnapshot, error)
}

type EstablishmentsByOrganizationReader interface {
	LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.EstablishmentSnapshot, error)
}
