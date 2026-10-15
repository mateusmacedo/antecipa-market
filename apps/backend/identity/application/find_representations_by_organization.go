package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

// FindRepresentationsByOrganization refines the validity in memory: the kernel
// Relation filters by one column, and the organization is that column.
func (s Service) FindRepresentationsByOrganization(ctx context.Context, q FindRepresentationsByOrganization) ([]domain.RepresentationSnapshot, error) {
	found, err := findRelated(ctx, s, OperationFindRepresentationsByOrganization, q,
		fmt.Sprintf("find representations by organization %s", q.Organization),
		func(ctx context.Context) ([]domain.RepresentationSnapshot, error) {
			return s.RepresentationsByOrganization.LoadByOrganization(ctx, q.Organization)
		})
	if err != nil || q.ValidAt == 0 {
		return found, err
	}
	inForce := make([]domain.RepresentationSnapshot, 0, len(found))
	for _, r := range found {
		if r.InForceAt(q.ValidAt) {
			inForce = append(inForce, r)
		}
	}
	return inForce, nil
}
