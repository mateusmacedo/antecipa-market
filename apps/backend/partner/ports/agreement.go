package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

type (
	AgreementRepository = port.Repository[domain.AgreementID, domain.PartnerAgreementSnapshot]
	AgreementReader     = port.Reader[domain.AgreementID, domain.PartnerAgreementSnapshot]
)

type AgreementsByPartnerReader interface {
	LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error)
}
