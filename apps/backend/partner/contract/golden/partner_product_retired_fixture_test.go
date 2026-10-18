package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerProductRetiredSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-product-retired.golden",
	Source:  fixtureSource,
	Subject: "prd-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-product-retired",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerProductRetired"},
		Type:       "com.company.partner.partner-product-retired.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerProductRetired",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.PartnerProductRetired{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerProductRetired(fields) },
	Build: leanFixture(
		partnerProductRetiredFields("prd-1001", "prt-1001", "1", "2026-09-02T12:00:00Z"),
		partnerProductRetiredFields("prd-1002", "prt-1001", "2", "2026-09-02T12:00:01Z"),
		partnerProductRetiredFields("prd-2001", "prt-2001", "1", "2026-09-02T12:00:02Z"),
		partnerProductRetiredFields("prd-2002", "prt-2001", "4", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func partnerProductRetiredFields(productID, partnerID, version, at string) map[string]string {
	return map[string]string{
		"product_id": productID,
		"partner_id": partnerID,
		"version":    version,
		"at":         at,
	}
}

func partnerProductRetired(fields map[string]string) (*eventv1.PartnerProductRetired, error) {
	version, err := parseInt32(fields, "version")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerProductRetired{
		ProductId: fields["product_id"],
		PartnerId: fields["partner_id"],
		Version:   version,
		At:        at,
	}, nil
}
