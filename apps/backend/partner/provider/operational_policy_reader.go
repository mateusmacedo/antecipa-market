package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

func NewOperationalPolicyReader(pool postgres.ReadPool) ports.OperationalPolicyReader {
	return policyTable.Reader(pool)
}

var policiesByPartner = policyTable.Relation("partner_id")

type OperationalPoliciesByPartnerReader struct{ pool postgres.ReadPool }

func NewOperationalPoliciesByPartnerReader(pool postgres.ReadPool) *OperationalPoliciesByPartnerReader {
	return &OperationalPoliciesByPartnerReader{pool: pool}
}

func (r *OperationalPoliciesByPartnerReader) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.OperationalPolicySnapshot, error) {
	return policiesByPartner.Query(ctx, r.pool, string(partnerID))
}

var _ ports.OperationalPoliciesByPartnerReader = (*OperationalPoliciesByPartnerReader)(nil)
