package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

type (
	OperationalPolicyRepository = port.Repository[domain.PolicyID, domain.OperationalPolicySnapshot]
	OperationalPolicyReader     = port.Reader[domain.PolicyID, domain.OperationalPolicySnapshot]
)

type OperationalPoliciesByPartnerReader interface {
	LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.OperationalPolicySnapshot, error)
}
