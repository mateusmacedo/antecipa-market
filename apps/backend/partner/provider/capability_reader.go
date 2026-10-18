package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

func NewCapabilityReader(pool postgres.ReadPool) ports.CapabilityReader {
	return capabilityTable.Reader(pool)
}

var capabilitiesByPartner = capabilityTable.Relation("partner_id")

type CapabilitiesByPartnerReader struct{ pool postgres.ReadPool }

func NewCapabilitiesByPartnerReader(pool postgres.ReadPool) *CapabilitiesByPartnerReader {
	return &CapabilitiesByPartnerReader{pool: pool}
}

func (r *CapabilitiesByPartnerReader) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.CapabilitySnapshot, error) {
	return capabilitiesByPartner.Query(ctx, r.pool, string(partnerID))
}

var _ ports.CapabilitiesByPartnerReader = (*CapabilitiesByPartnerReader)(nil)
