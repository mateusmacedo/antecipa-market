package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) FindRepresentation(ctx context.Context, id domain.RepresentationID) (domain.RepresentationSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindRepresentation, FindRepresentation{Representation: id},
		func(ctx context.Context) (domain.RepresentationSnapshot, error) {
			snapshot, _, err := s.RepresentationReader.Load(ctx, id)
			if err != nil {
				return domain.RepresentationSnapshot{}, fmt.Errorf("application: find representation %s: %w", id, err)
			}
			return snapshot, nil
		})
}
