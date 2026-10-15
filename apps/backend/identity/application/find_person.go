package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) FindPerson(ctx context.Context, id domain.PersonID) (domain.PersonSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindPerson, FindPerson{Person: id},
		func(ctx context.Context) (domain.PersonSnapshot, error) {
			snapshot, _, err := s.PersonReader.Load(ctx, id)
			if err != nil {
				return domain.PersonSnapshot{}, fmt.Errorf("application: find person %s: %w", id, err)
			}
			return snapshot, nil
		})
}
