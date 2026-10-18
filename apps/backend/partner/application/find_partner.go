package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) FindPartner(ctx context.Context, id domain.PartnerID) (domain.PartnerSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindPartner, FindPartner{Partner: id},
		func(ctx context.Context) (domain.PartnerSnapshot, error) {
			snapshot, _, err := s.PartnerReader.Load(ctx, id)
			if err != nil {
				return domain.PartnerSnapshot{}, fmt.Errorf("application: find partner %s: %w", id, err)
			}
			return snapshot, nil
		})
}

func (s Service) FindPartnersByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.PartnerSnapshot, error) {
	return findRelated(ctx, s, OperationFindPartnersByOrganization, FindPartnersByOrganization{Organization: organizationID},
		fmt.Sprintf("find partners by organization %s", organizationID),
		func(ctx context.Context) ([]domain.PartnerSnapshot, error) {
			return s.PartnersByOrganization.LoadByOrganization(ctx, organizationID)
		})
}
