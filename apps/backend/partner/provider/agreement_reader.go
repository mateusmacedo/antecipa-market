package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

func NewAgreementReader(pool postgres.ReadPool) ports.AgreementReader {
	return agreementTable.Reader(pool)
}

var agreementsByPartner = agreementTable.Relation("partner_id")

type AgreementsByPartnerReader struct{ pool postgres.ReadPool }

func NewAgreementsByPartnerReader(pool postgres.ReadPool) *AgreementsByPartnerReader {
	return &AgreementsByPartnerReader{pool: pool}
}

func (r *AgreementsByPartnerReader) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	return agreementsByPartner.Query(ctx, r.pool, string(partnerID))
}

var _ ports.AgreementsByPartnerReader = (*AgreementsByPartnerReader)(nil)
