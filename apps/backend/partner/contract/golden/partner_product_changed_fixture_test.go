package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerProductChangedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-product-changed.golden",
	Source:  fixtureSource,
	Subject: "prd-1003",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-product-changed",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerProductChanged"},
		Type:       "com.company.partner.partner-product-changed.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerProductChanged",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.PartnerProductChanged{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerProductChanged(fields) },
	Build: leanFixture(
		productFields("prd-1003", "prt-1001", "agr-1001", "ANT-30", "2", "prd-1001", "2026-10-01T00:00:00Z", "2027-09-01T00:00:00Z",
			"duplicata,cdca", "SP,RJ,MG", "BRL", "100000", "80000000", "7", "180", "2026-09-02T12:00:00Z"),
		productFields("prd-1004", "prt-1001", "agr-1002", "ANT-90", "2", "prd-1002", "2026-10-01T00:00:00Z", "",
			"duplicata", "BR", "BRL", "500000", "100000000", "30", "360", "2026-09-02T12:00:01Z"),
		productFields("prd-2003", "prt-2001", "agr-2001", "CPR-60", "3", "prd-2001", "2026-10-01T00:00:00Z", "2027-03-01T00:00:00Z",
			"cpr", "MT,GO,MS", "BRL", "1000000", "30000000", "15", "120", "2026-09-02T12:00:02Z"),
		productFields("prd-2004", "prt-2001", "agr-2001", "CPR-90", "2", "prd-2002", "2026-10-01T00:00:00Z", "2027-09-01T00:00:00Z",
			"", "", "BRL", "2000000", "40000000", "30", "90", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func partnerProductChanged(fields map[string]string) (*eventv1.PartnerProductChanged, error) {
	terms, err := productTermsOf(fields)
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerProductChanged{
		ProductId:      fields["product_id"],
		PartnerId:      fields["partner_id"],
		AgreementId:    fields["agreement_id"],
		Code:           fields["code"],
		Version:        terms.version,
		SupersedesId:   fields["supersedes_id"],
		ValidFrom:      terms.validFrom,
		ValidUntil:     terms.validUntil,
		AssetTypes:     terms.assetTypes,
		Regions:        terms.regions,
		Currency:       fields["currency"],
		MinAmountMinor: terms.minAmount,
		MaxAmountMinor: terms.maxAmount,
		MinTermDays:    terms.minTerm,
		MaxTermDays:    terms.maxTerm,
		At:             terms.at,
	}, nil
}
