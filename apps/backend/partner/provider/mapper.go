package provider

import (
	"fmt"
	"time"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const (
	partnerRegisteredType         = "com.company.partner.partner-registered.v1"
	partnerOffboardingStartedType = "com.company.partner.partner-offboarding-started.v1"
	partnerAgreementActivatedType = "com.company.partner.partner-agreement-activated.v1"

	partnerCertificateExpiringType = "com.company.partner.partner-certificate-expiring.v1"

	capabilityEnabledType        = "com.company.partner.capability-enabled.v1"
	capabilitySuspendedType      = "com.company.partner.capability-suspended.v1"
	capabilityRolloutChangedType = "com.company.partner.capability-rollout-changed.v1"

	partnerProductDefinedType = "com.company.partner.partner-product-defined.v1"
	partnerProductChangedType = "com.company.partner.partner-product-changed.v1"
	partnerProductExpiredType = "com.company.partner.partner-product-expired.v1"
	partnerProductRetiredType = "com.company.partner.partner-product-retired.v1"
)

type Mapper struct{}

func (Mapper) Map(event kernel.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case domain.PartnerRegistered:
		return mapped(partnerRegisteredType, &eventv1.PartnerRegistered{
			PartnerId:      string(e.PartnerID),
			OrganizationId: string(e.OrganizationID),
			Role:           roleOf(e.Role),
			At:             timestampOf(e.At),
		})
	case domain.PartnerOffboardingStarted:
		return mapped(partnerOffboardingStartedType, &eventv1.PartnerOffboardingStarted{
			PartnerId:      string(e.PartnerID),
			OrganizationId: string(e.OrganizationID),
			Reason:         e.Reason,
			At:             timestampOf(e.At),
		})
	case domain.PartnerAgreementActivated:
		return mapped(partnerAgreementActivatedType, &eventv1.PartnerAgreementActivated{
			AgreementId: string(e.AgreementID),
			PartnerId:   string(e.PartnerID),
			Version:     int32(e.Version),
			ValidFrom:   timestampOf(e.ValidFrom),
			ValidUntil:  optionalTimestampOf(e.ValidUntil),
			At:          timestampOf(e.At),
		})
	case domain.PartnerCertificateExpiring:
		return mapped(partnerCertificateExpiringType, &eventv1.PartnerCertificateExpiring{
			ProfileId:            string(e.ProfileID),
			PartnerId:            string(e.PartnerID),
			Environment:          environmentOf(e.Environment),
			CertificateExpiresAt: timestampOf(e.CertificateExpiresAt),
			ThresholdDays:        int32(e.ThresholdDays),
			At:                   timestampOf(e.At),
		})
	case domain.CapabilityEnabled:
		return mapped(capabilityEnabledType, &eventv1.CapabilityEnabled{
			CapabilityId:        string(e.CapabilityID),
			PartnerId:           string(e.PartnerID),
			Operation:           e.Operation,
			AssetType:           e.AssetType,
			Environment:         environmentOf(e.Environment),
			Version:             int32(e.Version),
			RolloutMode:         rolloutModesOnTheWire[e.RolloutMode],
			StateQuerySupported: e.StateQuerySupported,
			RestrictionNote:     e.RestrictionNote,
			At:                  timestampOf(e.At),
		})
	case domain.CapabilitySuspended:
		return mapped(capabilitySuspendedType, &eventv1.CapabilitySuspended{
			CapabilityId: string(e.CapabilityID),
			PartnerId:    string(e.PartnerID),
			Operation:    e.Operation,
			AssetType:    e.AssetType,
			Environment:  environmentOf(e.Environment),
			Version:      int32(e.Version),
			Reason:       e.Reason,
			Trigger:      triggersOnTheWire[e.Trigger],
			At:           timestampOf(e.At),
		})
	case domain.CapabilityRolloutChanged:
		return mapped(capabilityRolloutChangedType, &eventv1.CapabilityRolloutChanged{
			CapabilityId: string(e.CapabilityID),
			PartnerId:    string(e.PartnerID),
			Operation:    e.Operation,
			AssetType:    e.AssetType,
			Environment:  environmentOf(e.Environment),
			Version:      int32(e.Version),
			Mode:         rolloutModesOnTheWire[e.Mode],
			Percentage:   int32(e.Percentage),
			At:           timestampOf(e.At),
		})
	case domain.PartnerProductDefined:
		return mapped(partnerProductDefinedType, &eventv1.PartnerProductDefined{
			ProductId:      string(e.ProductID),
			PartnerId:      string(e.PartnerID),
			AgreementId:    string(e.AgreementID),
			Code:           e.Code,
			Version:        int32(e.Version),
			ValidFrom:      timestampOf(e.ValidFrom),
			ValidUntil:     optionalTimestampOf(e.ValidUntil),
			AssetTypes:     e.AssetTypes,
			Regions:        e.Regions,
			Currency:       e.Currency,
			MinAmountMinor: e.MinAmountMinor,
			MaxAmountMinor: e.MaxAmountMinor,
			MinTermDays:    int32(e.MinTermDays),
			MaxTermDays:    int32(e.MaxTermDays),
			At:             timestampOf(e.At),
		})
	case domain.PartnerProductChanged:
		return mapped(partnerProductChangedType, &eventv1.PartnerProductChanged{
			ProductId:      string(e.ProductID),
			PartnerId:      string(e.PartnerID),
			AgreementId:    string(e.AgreementID),
			Code:           e.Code,
			Version:        int32(e.Version),
			SupersedesId:   string(e.SupersedesID),
			ValidFrom:      timestampOf(e.ValidFrom),
			ValidUntil:     optionalTimestampOf(e.ValidUntil),
			AssetTypes:     e.AssetTypes,
			Regions:        e.Regions,
			Currency:       e.Currency,
			MinAmountMinor: e.MinAmountMinor,
			MaxAmountMinor: e.MaxAmountMinor,
			MinTermDays:    int32(e.MinTermDays),
			MaxTermDays:    int32(e.MaxTermDays),
			At:             timestampOf(e.At),
		})
	case domain.PartnerProductExpired:
		return mapped(partnerProductExpiredType, &eventv1.PartnerProductExpired{
			ProductId:  string(e.ProductID),
			PartnerId:  string(e.PartnerID),
			Version:    int32(e.Version),
			ValidUntil: timestampOf(e.ValidUntil),
			At:         timestampOf(e.At),
		})
	case domain.PartnerProductRetired:
		return mapped(partnerProductRetiredType, &eventv1.PartnerProductRetired{
			ProductId: string(e.ProductID),
			PartnerId: string(e.PartnerID),
			Version:   int32(e.Version),
			At:        timestampOf(e.At),
		})
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}

func mapped(eventType string, message proto.Message) (postgres.Mapped, error) {
	return postgres.Mapped{Message: message, Type: eventType}, nil
}

func timestampOf(at kernel.Instant) *timestamppb.Timestamp {
	return timestamppb.New(time.Unix(0, int64(at)))
}

func optionalTimestampOf(at kernel.Instant) *timestamppb.Timestamp {
	if at == 0 {
		return nil
	}
	return timestampOf(at)
}

var rolesOnTheWire = map[domain.PartnerRole]eventv1.PartnerRole{
	domain.RoleFunder:                eventv1.PartnerRole_PARTNER_ROLE_FUNDER,
	domain.RoleBookkeeper:            eventv1.PartnerRole_PARTNER_ROLE_BOOKKEEPER,
	domain.RoleRegistry:              eventv1.PartnerRole_PARTNER_ROLE_REGISTRY,
	domain.RoleDepository:            eventv1.PartnerRole_PARTNER_ROLE_DEPOSITORY,
	domain.RoleDataProvider:          eventv1.PartnerRole_PARTNER_ROLE_DATA_PROVIDER,
	domain.RoleSettlementInstitution: eventv1.PartnerRole_PARTNER_ROLE_SETTLEMENT_INSTITUTION,
}

func roleOf(role domain.PartnerRole) eventv1.PartnerRole {
	return rolesOnTheWire[role]
}

var environmentsOnTheWire = map[domain.Environment]eventv1.Environment{
	domain.EnvironmentDevelopment:  eventv1.Environment_ENVIRONMENT_DEVELOPMENT,
	domain.EnvironmentIntegration:  eventv1.Environment_ENVIRONMENT_INTEGRATION,
	domain.EnvironmentHomologation: eventv1.Environment_ENVIRONMENT_HOMOLOGATION,
	domain.EnvironmentProduction:   eventv1.Environment_ENVIRONMENT_PRODUCTION,
}

func environmentOf(environment domain.Environment) eventv1.Environment {
	return environmentsOnTheWire[environment]
}

var rolloutModesOnTheWire = map[domain.RolloutMode]eventv1.RolloutMode{
	domain.RolloutDisabled:   eventv1.RolloutMode_ROLLOUT_MODE_DISABLED,
	domain.RolloutAllowlist:  eventv1.RolloutMode_ROLLOUT_MODE_ALLOWLIST,
	domain.RolloutPercentage: eventv1.RolloutMode_ROLLOUT_MODE_PERCENTAGE,
	domain.RolloutEnabled:    eventv1.RolloutMode_ROLLOUT_MODE_ENABLED,
}

var triggersOnTheWire = map[domain.SuspensionTrigger]eventv1.SuspensionTrigger{
	domain.TriggerManual:         eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_MANUAL,
	domain.TriggerSchemaMismatch: eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_SCHEMA_MISMATCH,
	domain.TriggerOffboarding:    eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_OFFBOARDING,
}
