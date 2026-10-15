package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type (
	OrganizationRepository = port.Repository[domain.OrganizationID, domain.OrganizationSnapshot]
	OrganizationReader     = port.Reader[domain.OrganizationID, domain.OrganizationSnapshot]
)

// OrganizationsByTaxIDReader serves FindOrganizationByTaxId and the uniqueness
// pre-check. The kernel Relation filters by one column, so it answers every
// jurisdiction of the hash and the caller refines the result in memory.
type OrganizationsByTaxIDReader interface {
	LoadByTaxIDHash(ctx context.Context, taxIDHash string) ([]domain.OrganizationSnapshot, error)
}
