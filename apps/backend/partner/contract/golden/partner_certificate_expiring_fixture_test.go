package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

var partnerCertificateExpiringSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/partner-certificate-expiring.golden",
	Source:  fixtureSource,
	Subject: "ipf-1001",
	Identity: golden.Identity{
		Fixture:    "partner/event/v1/partner-certificate-expiring",
		Contract:   golden.Contract{Package: "company.partner.event.v1", Message: "PartnerCertificateExpiring"},
		Type:       "com.company.partner.partner-certificate-expiring.v1",
		DataSchema: "type.googleapis.com/company.partner.event.v1.PartnerCertificateExpiring",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "environment", Values: []string{
		"ENVIRONMENT_DEVELOPMENT", "ENVIRONMENT_INTEGRATION", "ENVIRONMENT_HOMOLOGATION", "ENVIRONMENT_PRODUCTION",
	}},
	New:        func() proto.Message { return &eventv1.PartnerCertificateExpiring{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return partnerCertificateExpiring(fields) },
	Build: leanFixture(
		partnerCertificateExpiringFields("ipf-1001", "prt-1001", "ENVIRONMENT_PRODUCTION", "2026-10-01T00:00:00Z", "30", "2026-09-02T12:00:00Z"),
		partnerCertificateExpiringFields("ipf-1002", "prt-1001", "ENVIRONMENT_HOMOLOGATION", "2026-11-01T00:00:00Z", "60", "2026-09-02T12:00:01Z"),
		partnerCertificateExpiringFields("ipf-2001", "prt-2001", "ENVIRONMENT_INTEGRATION", "2026-09-08T00:00:00Z", "7", "2026-09-02T12:00:02Z"),
		partnerCertificateExpiringFields("ipf-2002", "prt-2001", "ENVIRONMENT_DEVELOPMENT", "2026-09-30T00:00:00Z", "30", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func partnerCertificateExpiringFields(profileID, partnerID, environment, certificateExpiresAt, thresholdDays, at string) map[string]string {
	return map[string]string{
		"profile_id":             profileID,
		"partner_id":             partnerID,
		"environment":            environment,
		"certificate_expires_at": certificateExpiresAt,
		"threshold_days":         thresholdDays,
		"at":                     at,
	}
}

func partnerCertificateExpiring(fields map[string]string) (*eventv1.PartnerCertificateExpiring, error) {
	environment, err := enumOf(eventv1.Environment_value, fields, "environment")
	if err != nil {
		return nil, err
	}
	expiresAt, err := parseInstant(fields, "certificate_expires_at")
	if err != nil {
		return nil, err
	}
	threshold, err := parseInt32(fields, "threshold_days")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.PartnerCertificateExpiring{
		ProfileId:            fields["profile_id"],
		PartnerId:            fields["partner_id"],
		Environment:          eventv1.Environment(environment),
		CertificateExpiresAt: expiresAt,
		ThresholdDays:        threshold,
		At:                   at,
	}, nil
}
