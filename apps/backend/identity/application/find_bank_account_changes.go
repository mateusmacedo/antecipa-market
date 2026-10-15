package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) FindBankAccountChange(ctx context.Context, id domain.BankAccountChangeID) (domain.BankAccountChangeSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindBankAccountChange, FindBankAccountChange{BankAccountChange: id},
		func(ctx context.Context) (domain.BankAccountChangeSnapshot, error) {
			snapshot, _, err := s.BankAccountChangeReader.Load(ctx, id)
			if err != nil {
				return domain.BankAccountChangeSnapshot{}, fmt.Errorf("application: find bank account change %s: %w", id, err)
			}
			return snapshot, nil
		})
}

func (s Service) FindBankAccountChangesByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.BankAccountChangeSnapshot, error) {
	return findRelated(ctx, s, OperationFindBankAccountChangesByOrganization, FindBankAccountChangesByOrganization{Organization: organizationID},
		fmt.Sprintf("find bank account changes by organization %s", organizationID),
		func(ctx context.Context) ([]domain.BankAccountChangeSnapshot, error) {
			return s.BankAccountChangesByOrganization.LoadByOrganization(ctx, organizationID)
		})
}
