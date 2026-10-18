package provider_test

import (
	"errors"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

type strayEvent struct{}

func (strayEvent) EventName() string { return "partner-management.stray" }

var _ kernel.DomainEvent = strayEvent{}

func TestMapperMapsEachEventOfThePartnerTheAgreementAndTheProfile(t *testing.T) {
	t.Parallel()

	at := &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789}
	for _, tc := range []struct {
		name     string
		event    kernel.DomainEvent
		wantType string
		want     proto.Message
	}{
		{"partner registered", domain.PartnerRegistered{PartnerID: "prt-1", OrganizationID: "org-1", Role: domain.RoleSettlementInstitution, At: 1_755_432_000_123_456_789},
			"com.company.partner.partner-registered.v1",
			&eventv1.PartnerRegistered{PartnerId: "prt-1", OrganizationId: "org-1", Role: eventv1.PartnerRole_PARTNER_ROLE_SETTLEMENT_INSTITUTION, At: at}},
		{"offboarding started", domain.PartnerOffboardingStarted{PartnerID: "prt-1", OrganizationID: "org-1", Reason: "contract terminated", At: 1_755_432_000_123_456_789},
			"com.company.partner.partner-offboarding-started.v1",
			&eventv1.PartnerOffboardingStarted{PartnerId: "prt-1", OrganizationId: "org-1", Reason: "contract terminated", At: at}},
		{"agreement activated", domain.PartnerAgreementActivated{
			AgreementID: "agr-1", PartnerID: "prt-1", Version: 2, ValidFrom: 1_755_432_000_123_456_789, ValidUntil: 1_755_432_001_000_000_000, At: 1_755_432_000_123_456_789,
		}, "com.company.partner.partner-agreement-activated.v1",
			&eventv1.PartnerAgreementActivated{AgreementId: "agr-1", PartnerId: "prt-1", Version: 2, ValidFrom: at, ValidUntil: &timestamppb.Timestamp{Seconds: 1_755_432_001}, At: at}},
		{"agreement activated without an end", domain.PartnerAgreementActivated{AgreementID: "agr-1", PartnerID: "prt-1", Version: 1, ValidFrom: 1_755_432_000_123_456_789, At: 1_755_432_000_123_456_789},
			"com.company.partner.partner-agreement-activated.v1",
			&eventv1.PartnerAgreementActivated{AgreementId: "agr-1", PartnerId: "prt-1", Version: 1, ValidFrom: at, At: at}},
		{"certificate expiring", domain.PartnerCertificateExpiring{
			ProfileID: "ipf-1", PartnerID: "prt-1", Environment: domain.EnvironmentProduction,
			CertificateExpiresAt: 1_755_432_001_000_000_000, ThresholdDays: 30, At: 1_755_432_000_123_456_789,
		}, "com.company.partner.partner-certificate-expiring.v1",
			&eventv1.PartnerCertificateExpiring{
				ProfileId: "ipf-1", PartnerId: "prt-1", Environment: eventv1.Environment_ENVIRONMENT_PRODUCTION,
				CertificateExpiresAt: &timestamppb.Timestamp{Seconds: 1_755_432_001}, ThresholdDays: 30, At: at,
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapped, err := provider.Mapper{}.Map(tc.event)

			if err != nil {
				t.Fatalf("Map() = %v, want nil", err)
			}
			if !proto.Equal(mapped.Message, tc.want) || mapped.Type != tc.wantType {
				t.Fatalf("Map() = %s %v, want %s %v", mapped.Type, mapped.Message, tc.wantType, tc.want)
			}
		})
	}
}

func TestMapperMapsEachRole(t *testing.T) {
	t.Parallel()

	for role, want := range map[domain.PartnerRole]eventv1.PartnerRole{
		domain.RoleFunder:                eventv1.PartnerRole_PARTNER_ROLE_FUNDER,
		domain.RoleBookkeeper:            eventv1.PartnerRole_PARTNER_ROLE_BOOKKEEPER,
		domain.RoleRegistry:              eventv1.PartnerRole_PARTNER_ROLE_REGISTRY,
		domain.RoleDepository:            eventv1.PartnerRole_PARTNER_ROLE_DEPOSITORY,
		domain.RoleDataProvider:          eventv1.PartnerRole_PARTNER_ROLE_DATA_PROVIDER,
		domain.RoleSettlementInstitution: eventv1.PartnerRole_PARTNER_ROLE_SETTLEMENT_INSTITUTION,
		"acquirer":                       eventv1.PartnerRole_PARTNER_ROLE_UNSPECIFIED,
	} {
		mapped, err := provider.Mapper{}.Map(domain.PartnerRegistered{PartnerID: "prt-1", OrganizationID: "org-1", Role: role, At: 1})
		if err != nil {
			t.Fatalf("%s: Map() = %v, want nil", role, err)
		}
		if got := mapped.Message.(*eventv1.PartnerRegistered).GetRole(); got != want {
			t.Errorf("%s: role = %v, want %v", role, got, want)
		}
	}
}

func TestMapperReportsUnmappedEvent(t *testing.T) {
	t.Parallel()

	_, err := provider.Mapper{}.Map(strayEvent{})

	if !errors.Is(err, postgres.ErrUnmappedEvent) {
		t.Fatalf("Map() = %v, want ErrUnmappedEvent", err)
	}
}

func TestMapperMapsEachEnvironment(t *testing.T) {
	t.Parallel()

	for environment, want := range map[domain.Environment]eventv1.Environment{
		domain.EnvironmentDevelopment:  eventv1.Environment_ENVIRONMENT_DEVELOPMENT,
		domain.EnvironmentIntegration:  eventv1.Environment_ENVIRONMENT_INTEGRATION,
		domain.EnvironmentHomologation: eventv1.Environment_ENVIRONMENT_HOMOLOGATION,
		domain.EnvironmentProduction:   eventv1.Environment_ENVIRONMENT_PRODUCTION,
		"staging":                      eventv1.Environment_ENVIRONMENT_UNSPECIFIED,
	} {
		mapped, err := provider.Mapper{}.Map(domain.PartnerCertificateExpiring{ProfileID: "ipf-1", PartnerID: "prt-1", Environment: environment, ThresholdDays: 7, At: 1})
		if err != nil {
			t.Fatalf("%s: Map() = %v, want nil", environment, err)
		}
		if got := mapped.Message.(*eventv1.PartnerCertificateExpiring).GetEnvironment(); got != want {
			t.Errorf("%s: environment = %v, want %v", environment, got, want)
		}
	}
}

func TestMapperMapsEachEventOfTheCapability(t *testing.T) {
	t.Parallel()

	at := &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789}
	for _, tc := range []struct {
		name     string
		event    kernel.DomainEvent
		wantType string
		want     proto.Message
	}{
		{"capability enabled", domain.CapabilityEnabled{
			CapabilityID: "cap-1", PartnerID: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
			Environment: domain.EnvironmentProduction, Version: 2, RolloutMode: domain.RolloutDisabled,
			StateQuerySupported: false, RestrictionNote: "reconciled by the daily file", At: 1_755_432_000_123_456_789,
		}, "com.company.partner.capability-enabled.v1",
			&eventv1.CapabilityEnabled{
				CapabilityId: "cap-1", PartnerId: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
				Environment: eventv1.Environment_ENVIRONMENT_PRODUCTION, Version: 2, RolloutMode: eventv1.RolloutMode_ROLLOUT_MODE_DISABLED,
				StateQuerySupported: false, RestrictionNote: "reconciled by the daily file", At: at,
			}},
		{"capability suspended", domain.CapabilitySuspended{
			CapabilityID: "cap-1", PartnerID: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
			Environment: domain.EnvironmentHomologation, Version: 1, Reason: "partner incident", Trigger: domain.TriggerManual,
			At: 1_755_432_000_123_456_789,
		}, "com.company.partner.capability-suspended.v1",
			&eventv1.CapabilitySuspended{
				CapabilityId: "cap-1", PartnerId: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
				Environment: eventv1.Environment_ENVIRONMENT_HOMOLOGATION, Version: 1, Reason: "partner incident",
				Trigger: eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_MANUAL, At: at,
			}},
		{"capability rollout changed", domain.CapabilityRolloutChanged{
			CapabilityID: "cap-1", PartnerID: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
			Environment: domain.EnvironmentProduction, Version: 1, Mode: domain.RolloutPercentage, Percentage: 10,
			At: 1_755_432_000_123_456_789,
		}, "com.company.partner.capability-rollout-changed.v1",
			&eventv1.CapabilityRolloutChanged{
				CapabilityId: "cap-1", PartnerId: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
				Environment: eventv1.Environment_ENVIRONMENT_PRODUCTION, Version: 1, Mode: eventv1.RolloutMode_ROLLOUT_MODE_PERCENTAGE,
				Percentage: 10, At: at,
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapped, err := provider.Mapper{}.Map(tc.event)

			if err != nil {
				t.Fatalf("Map() = %v, want nil", err)
			}
			if !proto.Equal(mapped.Message, tc.want) || mapped.Type != tc.wantType {
				t.Fatalf("Map() = %s %v, want %s %v", mapped.Type, mapped.Message, tc.wantType, tc.want)
			}
		})
	}
}

func TestMapperMapsEachRolloutModeAndTrigger(t *testing.T) {
	t.Parallel()

	for mode, want := range map[domain.RolloutMode]eventv1.RolloutMode{
		domain.RolloutDisabled:   eventv1.RolloutMode_ROLLOUT_MODE_DISABLED,
		domain.RolloutAllowlist:  eventv1.RolloutMode_ROLLOUT_MODE_ALLOWLIST,
		domain.RolloutPercentage: eventv1.RolloutMode_ROLLOUT_MODE_PERCENTAGE,
		domain.RolloutEnabled:    eventv1.RolloutMode_ROLLOUT_MODE_ENABLED,
		"gradual":                eventv1.RolloutMode_ROLLOUT_MODE_UNSPECIFIED,
	} {
		mapped, err := provider.Mapper{}.Map(domain.CapabilityRolloutChanged{CapabilityID: "cap-1", Mode: mode, At: 1})
		if err != nil {
			t.Fatalf("%s: Map() = %v, want nil", mode, err)
		}
		if got := mapped.Message.(*eventv1.CapabilityRolloutChanged).GetMode(); got != want {
			t.Errorf("%s: mode = %v, want %v", mode, got, want)
		}
	}
	for trigger, want := range map[domain.SuspensionTrigger]eventv1.SuspensionTrigger{
		domain.TriggerManual:         eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_MANUAL,
		domain.TriggerSchemaMismatch: eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_SCHEMA_MISMATCH,
		domain.TriggerOffboarding:    eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_OFFBOARDING,
		"automatic":                  eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_UNSPECIFIED,
	} {
		mapped, err := provider.Mapper{}.Map(domain.CapabilitySuspended{CapabilityID: "cap-1", Trigger: trigger, At: 1})
		if err != nil {
			t.Fatalf("%s: Map() = %v, want nil", trigger, err)
		}
		if got := mapped.Message.(*eventv1.CapabilitySuspended).GetTrigger(); got != want {
			t.Errorf("%s: trigger = %v, want %v", trigger, got, want)
		}
	}
}

func TestMapperMapsEachEventOfTheFinancialProduct(t *testing.T) {
	t.Parallel()

	at := &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789}
	until := &timestamppb.Timestamp{Seconds: 1_786_968_000}
	for _, tc := range []struct {
		name     string
		event    kernel.DomainEvent
		wantType string
		want     proto.Message
	}{
		{"product defined", domain.PartnerProductDefined{
			ProductID: "fpr-1", PartnerID: "prt-1", AgreementID: "agr-1", Code: "receivables-advance", Version: 1,
			ValidFrom: 1_755_432_000_123_456_789, ValidUntil: 1_786_968_000_000_000_000, AssetTypes: []string{"duplicata", "cheque"},
			Regions: []string{"SP"}, Currency: "BRL", MinAmountMinor: 100_000, MaxAmountMinor: 50_000_000, MinTermDays: 7, MaxTermDays: 180,
			At: 1_755_432_000_123_456_789,
		}, "com.company.partner.partner-product-defined.v1",
			&eventv1.PartnerProductDefined{
				ProductId: "fpr-1", PartnerId: "prt-1", AgreementId: "agr-1", Code: "receivables-advance", Version: 1,
				ValidFrom: at, ValidUntil: until, AssetTypes: []string{"duplicata", "cheque"}, Regions: []string{"SP"}, Currency: "BRL",
				MinAmountMinor: 100_000, MaxAmountMinor: 50_000_000, MinTermDays: 7, MaxTermDays: 180, At: at,
			}},
		{"product defined without an end", domain.PartnerProductDefined{
			ProductID: "fpr-1", PartnerID: "prt-1", AgreementID: "agr-1", Code: "receivables-advance", Version: 1,
			ValidFrom: 1_755_432_000_123_456_789, AssetTypes: []string{"duplicata"}, Currency: "BRL", At: 1_755_432_000_123_456_789,
		}, "com.company.partner.partner-product-defined.v1",
			&eventv1.PartnerProductDefined{
				ProductId: "fpr-1", PartnerId: "prt-1", AgreementId: "agr-1", Code: "receivables-advance", Version: 1,
				ValidFrom: at, AssetTypes: []string{"duplicata"}, Currency: "BRL", At: at,
			}},
		{"product changed", domain.PartnerProductChanged{
			ProductID: "fpr-2", PartnerID: "prt-1", AgreementID: "agr-1", Code: "receivables-advance", Version: 2, SupersedesID: "fpr-1",
			ValidFrom: 1_755_432_000_123_456_789, ValidUntil: 1_786_968_000_000_000_000, AssetTypes: []string{"duplicata"},
			Regions: []string{"SP", "RJ"}, Currency: "BRL", MinAmountMinor: 1, MaxAmountMinor: 2, MinTermDays: 3, MaxTermDays: 4,
			At: 1_755_432_000_123_456_789,
		}, "com.company.partner.partner-product-changed.v1",
			&eventv1.PartnerProductChanged{
				ProductId: "fpr-2", PartnerId: "prt-1", AgreementId: "agr-1", Code: "receivables-advance", Version: 2, SupersedesId: "fpr-1",
				ValidFrom: at, ValidUntil: until, AssetTypes: []string{"duplicata"}, Regions: []string{"SP", "RJ"}, Currency: "BRL",
				MinAmountMinor: 1, MaxAmountMinor: 2, MinTermDays: 3, MaxTermDays: 4, At: at,
			}},
		{"product expired", domain.PartnerProductExpired{
			ProductID: "fpr-2", PartnerID: "prt-1", Version: 2, ValidUntil: 1_786_968_000_000_000_000, At: 1_755_432_000_123_456_789,
		}, "com.company.partner.partner-product-expired.v1",
			&eventv1.PartnerProductExpired{ProductId: "fpr-2", PartnerId: "prt-1", Version: 2, ValidUntil: until, At: at}},
		{"product retired", domain.PartnerProductRetired{ProductID: "fpr-2", PartnerID: "prt-1", Version: 2, At: 1_755_432_000_123_456_789},
			"com.company.partner.partner-product-retired.v1",
			&eventv1.PartnerProductRetired{ProductId: "fpr-2", PartnerId: "prt-1", Version: 2, At: at}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapped, err := provider.Mapper{}.Map(tc.event)

			if err != nil {
				t.Fatalf("Map() = %v, want nil", err)
			}
			if !proto.Equal(mapped.Message, tc.want) || mapped.Type != tc.wantType {
				t.Fatalf("Map() = %s %v, want %s %v", mapped.Type, mapped.Message, tc.wantType, tc.want)
			}
		})
	}
}
