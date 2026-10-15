package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) FindOrganization(ctx context.Context, id domain.OrganizationID) (domain.OrganizationSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindOrganization, FindOrganization{Organization: id},
		func(ctx context.Context) (domain.OrganizationSnapshot, error) {
			snapshot, _, err := s.Reader.Load(ctx, id)
			if err != nil {
				return domain.OrganizationSnapshot{}, fmt.Errorf("application: find organization %s: %w", id, err)
			}
			return snapshot, nil
		})
}
