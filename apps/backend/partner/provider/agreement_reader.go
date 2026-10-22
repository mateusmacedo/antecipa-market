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

// AgreementsByPartnerWithin reads the series inside the command transaction:
// the guard on partner_id holds it until the commit against any Save of the
// partner's agreements (ADR-067).
type AgreementsByPartnerWithin struct{ tx *postgres.Tx }

func NewAgreementsByPartnerWithin(tx *postgres.Tx) *AgreementsByPartnerWithin {
	return &AgreementsByPartnerWithin{tx: tx}
}

func (r *AgreementsByPartnerWithin) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	return agreementsByPartner.QueryWithin(ctx, r.tx, string(partnerID))
}

var (
	_ ports.AgreementsByPartnerReader = (*AgreementsByPartnerReader)(nil)
	_ ports.AgreementsByPartnerReader = (*AgreementsByPartnerWithin)(nil)
)
