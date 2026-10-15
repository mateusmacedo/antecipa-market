package application

import (
	"context"
	"fmt"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) FindMembershipsByUser(ctx context.Context, user domain.UserID) ([]domain.MembershipSnapshot, error) {
	return findRelated(ctx, s, OperationFindMembershipsByUser, FindMembershipsByUser{User: user},
		fmt.Sprintf("find memberships by user %s", user),
		func(ctx context.Context) ([]domain.MembershipSnapshot, error) {
			return s.MembershipsByUser.LoadByUser(ctx, user)
		})
}

func (s Service) FindMembershipsByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.MembershipSnapshot, error) {
	return findRelated(ctx, s, OperationFindMembershipsByOrganization, FindMembershipsByOrganization{Organization: organizationID},
		fmt.Sprintf("find memberships by organization %s", organizationID),
		func(ctx context.Context) ([]domain.MembershipSnapshot, error) {
			return s.MembershipsByOrganization.LoadByOrganization(ctx, organizationID)
		})
}
