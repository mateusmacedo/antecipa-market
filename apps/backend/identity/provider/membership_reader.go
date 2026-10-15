package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

var (
	membershipsByUser         = membershipTable.Relation("user_id")
	membershipsByOrganization = membershipTable.Relation("organization_id")
)

type MembershipsByUserReader struct{ pool postgres.ReadPool }

func NewMembershipsByUserReader(pool postgres.ReadPool) *MembershipsByUserReader {
	return &MembershipsByUserReader{pool: pool}
}

func (r *MembershipsByUserReader) LoadByUser(ctx context.Context, userID domain.UserID) ([]domain.MembershipSnapshot, error) {
	return membershipsByUser.Query(ctx, r.pool, string(userID))
}

type MembershipsByOrganizationReader struct{ pool postgres.ReadPool }

func NewMembershipsByOrganizationReader(pool postgres.ReadPool) *MembershipsByOrganizationReader {
	return &MembershipsByOrganizationReader{pool: pool}
}

func (r *MembershipsByOrganizationReader) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.MembershipSnapshot, error) {
	return membershipsByOrganization.Query(ctx, r.pool, string(organizationID))
}

var (
	_ ports.MembershipsByUserReader         = (*MembershipsByUserReader)(nil)
	_ ports.MembershipsByOrganizationReader = (*MembershipsByOrganizationReader)(nil)
)
