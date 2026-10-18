package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestEventNamesKeepTheirPublishedValues(t *testing.T) {
	events := map[string]kernel.DomainEvent{
		"partner-management.partner.registered":                       domain.PartnerRegistered{},
		"partner-management.partner.offboarding-started":              domain.PartnerOffboardingStarted{},
		"partner-management.partner-agreement.activated":              domain.PartnerAgreementActivated{},
		"partner-management.integration-profile.certificate-expiring": domain.PartnerCertificateExpiring{},
		"partner-management.capability.enabled":                       domain.CapabilityEnabled{},
		"partner-management.capability.suspended":                     domain.CapabilitySuspended{},
		"partner-management.capability.rollout-changed":               domain.CapabilityRolloutChanged{},
		"partner-management.financial-product.defined":                domain.PartnerProductDefined{},
		"partner-management.financial-product.changed":                domain.PartnerProductChanged{},
		"partner-management.financial-product.expired":                domain.PartnerProductExpired{},
		"partner-management.financial-product.retired":                domain.PartnerProductRetired{},
	}
	for want, event := range events {
		if got := event.EventName(); got != want {
			t.Errorf("EventName() = %q, want the published value %q", got, want)
		}
	}
}

func TestEnumerationsKeepTheirPublishedValues(t *testing.T) {
	for _, value := range []struct{ got, want string }{
		{string(domain.RoleFunder), "funder"},
		{string(domain.RoleBookkeeper), "bookkeeper"},
		{string(domain.RoleRegistry), "registry"},
		{string(domain.RoleDepository), "depository"},
		{string(domain.RoleDataProvider), "data_provider"},
		{string(domain.RoleSettlementInstitution), "settlement_institution"},
		{string(domain.PartnerProspect), "prospect"},
		{string(domain.PartnerContracting), "contracting"},
		{string(domain.PartnerIntegrating), "integrating"},
		{string(domain.PartnerHomologating), "homologating"},
		{string(domain.PartnerActive), "active"},
		{string(domain.PartnerSuspended), "suspended"},
		{string(domain.PartnerOffboarding), "offboarding"},
		{string(domain.AgreementDraft), "draft"},
		{string(domain.AgreementActive), "active"},
		{string(domain.KindFunder), "funder"},
		{string(domain.KindPartner), "partner"},
		{string(domain.EnvironmentDevelopment), "development"},
		{string(domain.EnvironmentIntegration), "integration"},
		{string(domain.EnvironmentHomologation), "homologation"},
		{string(domain.EnvironmentProduction), "production"},
		{string(domain.ChangeInitial), "initial"},
		{string(domain.ChangeAnnounced), "announced"},
		{string(domain.ChangeUnannounced), "unannounced"},
		{string(domain.CapabilityStatusDeclared), "declared"},
		{string(domain.CapabilityStatusHomologated), "homologated"},
		{string(domain.CapabilityStatusEnabled), "enabled"},
		{string(domain.CapabilityStatusSuspended), "suspended"},
		{string(domain.TriggerManual), "manual"},
		{string(domain.TriggerSchemaMismatch), "schema_mismatch"},
		{string(domain.TriggerOffboarding), "offboarding"},
		{string(domain.RolloutDisabled), "disabled"},
		{string(domain.RolloutAllowlist), "allowlist"},
		{string(domain.RolloutPercentage), "percentage"},
		{string(domain.RolloutEnabled), "enabled"},
		{string(domain.ProductActive), "active"},
		{string(domain.ProductRetired), "retired"},
		{string(domain.RetireExpired), "expired"},
		{string(domain.RetireWithdrawn), "withdrawn"},
	} {
		if value.got != value.want {
			t.Errorf("value = %q, want %q", value.got, value.want)
		}
	}
}
