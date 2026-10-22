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

// CapabilitiesByPartnerWithin reads the series inside the command transaction,
// under the guard on partner_id (ADR-067).
type CapabilitiesByPartnerWithin struct{ tx *postgres.Tx }

func NewCapabilitiesByPartnerWithin(tx *postgres.Tx) *CapabilitiesByPartnerWithin {
	return &CapabilitiesByPartnerWithin{tx: tx}
}

func (r *CapabilitiesByPartnerWithin) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.CapabilitySnapshot, error) {
	return capabilitiesByPartner.QueryWithin(ctx, r.tx, string(partnerID))
}

var (
	_ ports.CapabilitiesByPartnerReader = (*CapabilitiesByPartnerReader)(nil)
	_ ports.CapabilitiesByPartnerReader = (*CapabilitiesByPartnerWithin)(nil)
)
