package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerProductDefinedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-product-defined.golden",
	Source:  fixtureSource,
	Subject: "prd-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-product-defined",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerProductDefined"},
		Type:       "com.company.partner.partner-product-defined.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerProductDefined",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.PartnerProductDefined{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerProductDefined(fields) },
	Build: leanFixture(
		productFields("prd-1001", "prt-1001", "agr-1001", "ANT-30", "1", "", "2026-09-01T00:00:00Z", "2027-09-01T00:00:00Z",
			"duplicata,cdca", "SP,RJ", "BRL", "100000", "50000000", "7", "180", "2026-09-02T12:00:00Z"),
		productFields("prd-1002", "prt-1001", "agr-1001", "ANT-90", "1", "", "2026-09-01T00:00:00Z", "",
			"duplicata", "BR", "BRL", "500000", "100000000", "30", "360", "2026-09-02T12:00:01Z"),
		productFields("prd-2001", "prt-2001", "agr-2001", "CPR-60", "1", "", "2026-09-01T00:00:00Z", "2027-03-01T00:00:00Z",
			"cpr", "MT,GO", "BRL", "1000000", "20000000", "15", "120", "2026-09-02T12:00:02Z"),
		productFields("prd-2002", "prt-2001", "agr-2001", "CPR-90", "1", "", "2026-09-01T00:00:00Z", "2027-09-01T00:00:00Z",
			"", "", "BRL", "2000000", "40000000", "30", "90", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func productFields(productID, partnerID, agreementID, code, version, supersedesID, validFrom, validUntil, assetTypes, regions, currency,
	minAmount, maxAmount, minTerm, maxTerm, at string,
) map[string]string {
	fields := map[string]string{
		"product_id":       productID,
		"partner_id":       partnerID,
		"agreement_id":     agreementID,
		"code":             code,
		"version":          version,
		"valid_from":       validFrom,
		"valid_until":      validUntil,
		"asset_types":      assetTypes,
		"regions":          regions,
		"currency":         currency,
		"min_amount_minor": minAmount,
		"max_amount_minor": maxAmount,
		"min_term_days":    minTerm,
		"max_term_days":    maxTerm,
		"at":               at,
	}
	if supersedesID != "" {
		fields["supersedes_id"] = supersedesID
	}
	return fields
}

type productTerms struct {
	version                   int32
	validFrom, validUntil, at *timestamppb.Timestamp
	minAmount, maxAmount      int64
	minTerm, maxTerm          int32
	assetTypes, regions       []string
}

func productTermsOf(fields map[string]string) (productTerms, error) {
	var (
		terms productTerms
		err   error
	)
	if terms.version, err = parseInt32(fields, "version"); err != nil {
		return terms, err
	}
	if terms.validFrom, err = parseInstant(fields, "valid_from"); err != nil {
		return terms, err
	}
	if terms.validUntil, err = optionalInstant(fields, "valid_until"); err != nil {
		return terms, err
	}
	if terms.at, err = parseInstant(fields, "at"); err != nil {
		return terms, err
	}
	if terms.minAmount, err = tb.ParseInt(fields, "min_amount_minor", 64); err != nil {
		return terms, err
	}
	if terms.maxAmount, err = tb.ParseInt(fields, "max_amount_minor", 64); err != nil {
		return terms, err
	}
	if terms.minTerm, err = parseInt32(fields, "min_term_days"); err != nil {
		return terms, err
	}
	if terms.maxTerm, err = parseInt32(fields, "max_term_days"); err != nil {
		return terms, err
	}
	terms.assetTypes, terms.regions = listOf(fields["asset_types"]), listOf(fields["regions"])
	return terms, nil
}

func partnerProductDefined(fields map[string]string) (*eventv1.PartnerProductDefined, error) {
	terms, err := productTermsOf(fields)
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerProductDefined{
		ProductId:      fields["product_id"],
		PartnerId:      fields["partner_id"],
		AgreementId:    fields["agreement_id"],
		Code:           fields["code"],
		Version:        terms.version,
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
