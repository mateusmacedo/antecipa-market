package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type MembershipRepository = port.Repository[domain.MembershipID, domain.MembershipSnapshot]

// MembershipsByUserReader serves FindMembershipsByUser and the first read of
// FindEffectivePermissions; an invitation not yet granted has no user to match.
type MembershipsByUserReader interface {
	LoadByUser(ctx context.Context, userID domain.UserID) ([]domain.MembershipSnapshot, error)
}

type MembershipsByOrganizationReader interface {
	LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.MembershipSnapshot, error)
}
