package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

type (
	IntegrationProfileRepository = port.Repository[domain.ProfileID, domain.IntegrationProfileSnapshot]
	IntegrationProfileReader     = port.Reader[domain.ProfileID, domain.IntegrationProfileSnapshot]
)

// IntegrationProfilesByPartnerReader serves the series pre-check of
// RegisterIntegrationProfile. The kernel Relation filters by one column, so it
// answers every environment of the partner and the caller refines in memory.
type IntegrationProfilesByPartnerReader interface {
	LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error)
}

type IntegrationProfilesByEnvironmentReader interface {
	LoadByEnvironment(ctx context.Context, environment domain.Environment) ([]domain.IntegrationProfileSnapshot, error)
}
