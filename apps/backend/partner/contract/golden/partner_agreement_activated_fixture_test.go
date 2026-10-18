package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerAgreementActivatedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-agreement-activated.golden",
	Source:  fixtureSource,
	Subject: "agr-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-agreement-activated",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerAgreementActivated"},
		Type:       "com.company.partner.partner-agreement-activated.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerAgreementActivated",
	},
	Unknown:    unknownFieldNumber,
	New:        func() proto.Message { return &eventv1.PartnerAgreementActivated{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerAgreementActivated(fields) },
	Build: leanFixture(
		partnerAgreementActivatedFields("agr-1001", "prt-1001", "1", "2026-09-01T00:00:00Z", "2027-09-01T00:00:00Z", "2026-09-02T12:00:00Z"),
		partnerAgreementActivatedFields("agr-1002", "prt-1002", "2", "2026-09-01T00:00:00Z", "", "2026-09-02T12:00:01Z"),
		partnerAgreementActivatedFields("agr-2001", "prt-2001", "1", "2026-09-01T00:00:00Z", "2027-03-01T00:00:00Z", "2026-09-02T12:00:02Z"),
		partnerAgreementActivatedFields("agr-2002", "prt-2002", "3", "2026-09-01T00:00:00Z", "2028-09-01T00:00:00Z", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func partnerAgreementActivatedFields(agreementID, partnerID, version, validFrom, validUntil, at string) map[string]string {
	return map[string]string{
		"agreement_id": agreementID,
		"partner_id":   partnerID,
		"version":      version,
		"valid_from":   validFrom,
		"valid_until":  validUntil,
		"at":           at,
	}
}

func partnerAgreementActivated(fields map[string]string) (*eventv1.PartnerAgreementActivated, error) {
	version, err := parseInt32(fields, "version")
	if err != nil {
		return nil, err
	}
	validFrom, err := parseInstant(fields, "valid_from")
	if err != nil {
		return nil, err
	}
	validUntil, err := optionalInstant(fields, "valid_until")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerAgreementActivated{
		AgreementId: fields["agreement_id"],
		PartnerId:   fields["partner_id"],
		Version:     version,
		ValidFrom:   validFrom,
		ValidUntil:  validUntil,
		At:          at,
	}, nil
}
