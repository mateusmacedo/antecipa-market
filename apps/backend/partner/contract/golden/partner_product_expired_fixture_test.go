package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerProductExpiredSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-product-expired.golden",
	Source:  fixtureSource,
	Subject: "prd-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-product-expired",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerProductExpired"},
		Type:       "com.company.partner.partner-product-expired.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerProductExpired",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.PartnerProductExpired{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerProductExpired(fields) },
	Build: leanFixture(
		partnerProductExpiredFields("prd-1001", "prt-1001", "1", "2026-09-01T00:00:00Z", "2026-09-02T12:00:00Z"),
		partnerProductExpiredFields("prd-1002", "prt-1001", "2", "2026-08-31T23:59:59Z", "2026-09-02T12:00:01Z"),
		partnerProductExpiredFields("prd-2001", "prt-2001", "1", "2026-09-02T00:00:00Z", "2026-09-02T12:00:02Z"),
		partnerProductExpiredFields("prd-2002", "prt-2001", "3", "2026-09-02T12:00:00Z", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func partnerProductExpiredFields(productID, partnerID, version, validUntil, at string) map[string]string {
	return map[string]string{
		"product_id":  productID,
		"partner_id":  partnerID,
		"version":     version,
		"valid_until": validUntil,
		"at":          at,
	}
}

func partnerProductExpired(fields map[string]string) (*eventv1.PartnerProductExpired, error) {
	version, err := parseInt32(fields, "version")
	if err != nil {
		return nil, err
	}
	validUntil, err := parseInstant(fields, "valid_until")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerProductExpired{
		ProductId:  fields["product_id"],
		PartnerId:  fields["partner_id"],
		Version:    version,
		ValidUntil: validUntil,
		At:         at,
	}, nil
}
