package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const (
	partnerID      = domain.PartnerID("prt-100")
	organizationID = domain.OrganizationID("org-100")
	agreementID    = domain.AgreementID("agr-100")
	at             = kernel.Instant(1_755_432_000_000_000_000)
	day            = kernel.Instant(86_400_000_000_000)
)

func registerPartnerCommand() domain.RegisterPartner {
	return domain.RegisterPartner{
		OrganizationID:    organizationID,
		Role:              domain.RoleFunder,
		OrganizationFound: true,
		OrganizationKinds: []domain.OrganizationKind{domain.KindFunder},
		At:                at,
	}
}

func partnerAt(status domain.PartnerStatus) *domain.Partner {
	return domain.FromPartnerSnapshot(domain.PartnerSnapshot{
		ID:             partnerID,
		OrganizationID: organizationID,
		Role:           domain.RoleFunder,
		Status:         status,
		RegisteredAt:   at,
		StageChangedAt: at,
	})
}

func registerAgreementCommand() domain.RegisterAgreement {
	return domain.RegisterAgreement{
		PartnerID:            partnerID,
		ValidFrom:            at,
		ValidUntil:           at + 365*day,
		InstrumentTemplateID: "tpl-1",
		At:                   at,
	}
}

func draftAgreement(t *testing.T) *domain.PartnerAgreement {
	t.Helper()
	a := domain.NewPartnerAgreement(agreementID)
	if _, rej := a.Register(registerAgreementCommand()); rej != nil {
		t.Fatalf("setup: Register rejected: %v", rej)
	}
	return a
}

func activateAgreementCommand() domain.ActivateAgreement {
	return domain.ActivateAgreement{
		ContractEvidenceID: "evd-1",
		SatisfiedCriteria:  []string{"H14", "H15", "H17", "H18"},
		At:                 at + day,
	}
}

func requirePartnerUnchanged(t *testing.T, before, after domain.PartnerSnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func requireAgreementUnchanged(t *testing.T, before, after domain.PartnerAgreementSnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}

const (
	profileID = domain.ProfileID("ipf-100")
	policyID  = domain.PolicyID("pol-100")
)

func registerProfileCommand() domain.RegisterIntegrationProfile {
	return domain.RegisterIntegrationProfile{
		PartnerID:            partnerID,
		Environment:          domain.EnvironmentProduction,
		ChangeKind:           domain.ChangeInitial,
		Protocols:            []string{"https", "mtls"},
		Endpoint:             "https://partner.example/api",
		SchemaVersion:        "v3",
		SecretRef:            "vault://partners/prt-100/api-key",
		CertificateRef:       "cert-100",
		CertificateExpiresAt: at + 90*day,
		EffectiveFrom:        at,
		At:                   at,
	}
}

func profileExpiringIn(t *testing.T, remaining kernel.Instant) *domain.IntegrationProfile {
	t.Helper()
	cmd := registerProfileCommand()
	cmd.CertificateExpiresAt = at + remaining
	p := domain.NewIntegrationProfile(profileID)
	if _, rej := p.Register(cmd); rej != nil {
		t.Fatalf("setup: Register rejected: %v", rej)
	}
	return p
}

func alertedProfile(t *testing.T, remaining kernel.Instant, lastAlerted int) *domain.IntegrationProfile {
	t.Helper()
	s := profileExpiringIn(t, remaining).Snapshot()
	s.LastAlertedThresholdDays = lastAlerted
	return domain.FromIntegrationProfileSnapshot(s)
}

func raiseAlert(thresholdDays int) domain.RaiseCertificateExpiryAlert {
	return domain.RaiseCertificateExpiryAlert{ThresholdDays: thresholdDays, At: at}
}

func definePolicyCommand() domain.DefineOperationalPolicy {
	return domain.DefineOperationalPolicy{
		PartnerID:          partnerID,
		EffectiveFrom:      at,
		RateLimitPerMinute: 100,
		BusinessDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		CutoffTime:         "17:00",
		ProcessingWindow:   "08:00-18:00",
		SlaRef:             "sla-gold",
		Fallback:           "manual queue",
		SupportContact:     "support@partner.example",
		IncidentContact:    "+55 11 99999-0000",
		At:                 at,
	}
}

func requireProfileUnchanged(t *testing.T, before, after domain.IntegrationProfileSnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func requirePolicyUnchanged(t *testing.T, before, after domain.OperationalPolicySnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatal("snapshot changed on rejection")
	}
}
