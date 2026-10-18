package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

type (
	CapabilityRepository = port.Repository[domain.CapabilityID, domain.CapabilitySnapshot]
	CapabilityReader     = port.Reader[domain.CapabilityID, domain.CapabilitySnapshot]
)

// CapabilitiesByPartnerReader serves the series pre-check of DeclareCapability
// and the enabled capabilities ChangePartnerStage asks for. The kernel Relation
// filters by one column, so the caller refines the rest of the series in memory.
type CapabilitiesByPartnerReader interface {
	LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.CapabilitySnapshot, error)
}
