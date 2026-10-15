package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) FindEstablishmentsByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.EstablishmentSnapshot, error) {
	return findRelated(ctx, s, OperationFindEstablishmentsByOrganization, FindEstablishmentsByOrganization{Organization: organizationID},
		fmt.Sprintf("find establishments by organization %s", organizationID),
		func(ctx context.Context) ([]domain.EstablishmentSnapshot, error) {
			return s.EstablishmentsByOrganization.LoadByOrganization(ctx, organizationID)
		})
}
