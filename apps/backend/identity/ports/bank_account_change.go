package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type (
	BankAccountChangeRepository = port.Repository[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]
	BankAccountChangeReader     = port.Reader[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]
)

type BankAccountChangesByOrganizationReader interface {
	LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.BankAccountChangeSnapshot, error)
}
