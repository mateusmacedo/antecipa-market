package domain_test

import (
	"strconv"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func agreementVersion(version int, status domain.AgreementStatus, from, until kernel.Instant) domain.PartnerAgreementSnapshot {
	return domain.PartnerAgreementSnapshot{
		ID: domain.AgreementID("agr-" + strconv.Itoa(version)), PartnerID: partnerID, Version: version,
		Status: status, ValidFrom: from, ValidUntil: until,
	}
}

func TestAnAgreementIsInEffectByItsCurrentActiveVersion(t *testing.T) {
	for _, tc := range []struct {
		name       string
		agreements []domain.PartnerAgreementSnapshot
		want       bool
	}{
		{"no agreement", nil, false},
		{"only a draft", []domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementDraft, at, 0)}, false},
		{"active of indefinite validity", []domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at, 0)}, true},
		{"active until after the instant", []domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at, at+2*day)}, true},
		{"active until the instant", []domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at, at+day)}, false},
		{"active from after the instant", []domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at+2*day, 0)}, false},
		{"a later active version that expired", []domain.PartnerAgreementSnapshot{
			agreementVersion(1, domain.AgreementActive, at, 0), agreementVersion(2, domain.AgreementActive, at, at+day),
		}, false},
		{"a later version still draft", []domain.PartnerAgreementSnapshot{
			agreementVersion(1, domain.AgreementActive, at, 0), agreementVersion(2, domain.AgreementDraft, at, at+day),
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.AgreementInEffect(tc.agreements, at+day); got != tc.want {
				t.Fatalf("AgreementInEffect() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAProfileIsInEffectForItsEnvironmentFromItsEffectiveInstant(t *testing.T) {
	profile := func(environment domain.Environment, effectiveFrom kernel.Instant) domain.IntegrationProfileSnapshot {
		return domain.IntegrationProfileSnapshot{ID: "ipf-1", PartnerID: partnerID, Environment: environment, Version: 1, EffectiveFrom: effectiveFrom}
	}
	for _, tc := range []struct {
		name     string
		profiles []domain.IntegrationProfileSnapshot
		want     bool
	}{
		{"no profile", nil, false},
		{"profile of the environment", []domain.IntegrationProfileSnapshot{profile(domain.EnvironmentProduction, at)}, true},
		{"profile effective at the instant", []domain.IntegrationProfileSnapshot{profile(domain.EnvironmentProduction, at+day)}, true},
		{"profile not yet effective", []domain.IntegrationProfileSnapshot{profile(domain.EnvironmentProduction, at+2*day)}, false},
		{"profile of another environment", []domain.IntegrationProfileSnapshot{profile(domain.EnvironmentHomologation, at)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.ProfileInEffect(tc.profiles, domain.EnvironmentProduction, at+day); got != tc.want {
				t.Fatalf("ProfileInEffect() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTheCurrentAgreementIsTheHighestActiveVersionStartedAtTheInstant(t *testing.T) {
	for _, tc := range []struct {
		name       string
		agreements []domain.PartnerAgreementSnapshot
		want       domain.AgreementID
	}{
		{"no agreement", nil, ""},
		{"only a draft", []domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementDraft, at, 0)}, ""},
		{"a later active version that expired", []domain.PartnerAgreementSnapshot{
			agreementVersion(1, domain.AgreementActive, at, 0), agreementVersion(2, domain.AgreementActive, at, at+day),
		}, "agr-2"},
		{"a later version still draft", []domain.PartnerAgreementSnapshot{
			agreementVersion(1, domain.AgreementActive, at, 0), agreementVersion(2, domain.AgreementDraft, at, 0),
		}, "agr-1"},
		{"a later version not yet started", []domain.PartnerAgreementSnapshot{
			agreementVersion(1, domain.AgreementActive, at, 0), agreementVersion(2, domain.AgreementActive, at+2*day, 0),
		}, "agr-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, found := domain.CurrentAgreement(tc.agreements, at+day)
			if found != (tc.want != "") || got.ID != tc.want {
				t.Fatalf("CurrentAgreement() = %q, %v; want %q", got.ID, found, tc.want)
			}
		})
	}
}

func TestAPartnerHasAnEnabledCapabilityWhenTheCurrentVersionOfSomeSeriesIsEnabled(t *testing.T) {
	version := func(id domain.CapabilityID, assetType string, version int, status domain.CapabilityStatus) domain.CapabilitySnapshot {
		return domain.CapabilitySnapshot{
			ID: id, PartnerID: partnerID, Operation: "register-receivable", AssetType: assetType,
			Environment: domain.EnvironmentProduction, Version: version, Status: status,
		}
	}
	for _, tc := range []struct {
		name         string
		capabilities []domain.CapabilitySnapshot
		want         bool
	}{
		{"no capability", nil, false},
		{"only homologated", []domain.CapabilitySnapshot{version("cap-1", "duplicata", 1, domain.CapabilityStatusHomologated)}, false},
		{"enabled", []domain.CapabilitySnapshot{version("cap-1", "duplicata", 1, domain.CapabilityStatusEnabled)}, true},
		{"enabled superseded by a suspended version of the series", []domain.CapabilitySnapshot{
			version("cap-1", "duplicata", 1, domain.CapabilityStatusEnabled), version("cap-2", "duplicata", 2, domain.CapabilityStatusSuspended),
		}, false},
		{"enabled followed by a version still homologated", []domain.CapabilitySnapshot{
			version("cap-1", "duplicata", 1, domain.CapabilityStatusEnabled), version("cap-2", "duplicata", 2, domain.CapabilityStatusHomologated),
		}, true},
		{"suspended in one series and enabled in another", []domain.CapabilitySnapshot{
			version("cap-1", "duplicata", 1, domain.CapabilityStatusSuspended), version("cap-2", "cheque", 1, domain.CapabilityStatusEnabled),
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.HasEnabledCapability(tc.capabilities); got != tc.want {
				t.Fatalf("HasEnabledCapability() = %v, want %v", got, tc.want)
			}
		})
	}
}
