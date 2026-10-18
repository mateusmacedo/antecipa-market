package provider

import (
	"context"
	"errors"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

var ErrOrganizationDirectoryUnconfigured = errors.New("provider: organization directory not configured")

// UnconfiguredOrganizationDirectory is the production directory until the gRPC
// client of Identity exists: it fails closed, so no partner is registered over
// an organization nobody checked.
type UnconfiguredOrganizationDirectory struct{}

func (UnconfiguredOrganizationDirectory) FindOrganization(context.Context, port.TenantID, domain.OrganizationID) ([]domain.OrganizationKind, error) {
	return nil, ErrOrganizationDirectoryUnconfigured
}
