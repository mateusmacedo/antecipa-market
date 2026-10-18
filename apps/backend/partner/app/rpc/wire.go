package rpc

import (
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func onTheWire[S any, W any](found []S, convert func(S) *W) []*W {
	out := make([]*W, 0, len(found))
	for _, s := range found {
		out = append(out, convert(s))
	}
	return out
}

func partnerOnTheWire(s domain.PartnerSnapshot) *servicev1.Partner {
	return &servicev1.Partner{
		PartnerId:            string(s.ID),
		OrganizationId:       string(s.OrganizationID),
		Role:                 rolesToTheWire[s.Role],
		Status:               partnerStatusesOnTheWire[s.Status],
		RegisteredAt:         int64(s.RegisteredAt),
		StageChangedAt:       int64(s.StageChangedAt),
		StageReason:          s.StageReason,
		OffboardingStartedAt: int64(s.OffboardingStartedAt),
	}
}

func agreementOnTheWire(s domain.PartnerAgreementSnapshot) *servicev1.PartnerAgreement {
	return &servicev1.PartnerAgreement{
		AgreementId:          string(s.ID),
		PartnerId:            string(s.PartnerID),
		Version:              int32(s.Version),
		SupersedesId:         string(s.SupersedesID),
		Status:               agreementStatusesToTheWire[s.Status],
		ValidFrom:            int64(s.ValidFrom),
		ValidUntil:           int64(s.ValidUntil),
		ContractEvidenceId:   s.ContractEvidenceID,
		InstrumentTemplateId: s.InstrumentTemplateID,
		ActivatedAt:          int64(s.ActivatedAt),
	}
}

func capabilityOnTheWire(s domain.CapabilitySnapshot) *servicev1.Capability {
	homologation := make([]*servicev1.CriterionResult, 0, len(s.Homologation))
	for _, r := range s.Homologation {
		homologation = append(homologation, &servicev1.CriterionResult{
			CriterionId: r.CriterionID, Passed: r.Passed, EvidenceId: r.EvidenceID, RecordedAt: int64(r.RecordedAt),
		})
	}
	var suspension *servicev1.Suspension
	if s.Suspension.Trigger != "" {
		suspension = &servicev1.Suspension{
			Reason: s.Suspension.Reason, Trigger: triggersToTheWire[s.Suspension.Trigger], SuspendedAt: int64(s.Suspension.SuspendedAt),
		}
	}
	return &servicev1.Capability{
		CapabilityId:        string(s.ID),
		PartnerId:           string(s.PartnerID),
		Operation:           s.Operation,
		AssetType:           s.AssetType,
		Environment:         environmentsToTheWire[s.Environment],
		Version:             int32(s.Version),
		SupersedesId:        string(s.SupersedesID),
		IsWrite:             s.IsWrite,
		StateQuerySupported: s.StateQuerySupported,
		RestrictionNote:     s.RestrictionNote,
		CallbackApplicable:  s.CallbackApplicable,
		Status:              capabilityStatusesOnTheWire[s.Status],
		Homologation:        homologation,
		Rollout: &servicev1.Rollout{
			Mode:               rolloutModesToTheWire[s.Rollout.Mode],
			AllowlistTenantIds: s.Rollout.AllowlistTenantIDs,
			Percentage:         int32(s.Rollout.Percentage),
			ExcludedTenantIds:  s.Rollout.ExcludedTenantIDs,
		},
		Suspension: suspension,
		EnabledAt:  int64(s.EnabledAt),
	}
}

func productOnTheWire(s domain.FinancialProductSnapshot) *servicev1.FinancialProduct {
	return &servicev1.FinancialProduct{
		ProductId:        string(s.ID),
		PartnerId:        string(s.PartnerID),
		AgreementId:      string(s.AgreementID),
		Code:             s.Code,
		Name:             s.Name,
		Version:          int32(s.Version),
		SupersedesId:     string(s.SupersedesID),
		Status:           productStatusesToTheWire[s.Status],
		ValidFrom:        int64(s.ValidFrom),
		ValidUntil:       int64(s.ValidUntil),
		AssetTypes:       s.AssetTypes,
		Regions:          s.Regions,
		Currency:         s.Currency,
		MinAmountMinor:   s.MinAmountMinor,
		MaxAmountMinor:   s.MaxAmountMinor,
		MinTermDays:      int32(s.MinTermDays),
		MaxTermDays:      int32(s.MaxTermDays),
		RequiredCheckIds: s.RequiredCheckIDs,
	}
}

func profileOnTheWire(s domain.IntegrationProfileSnapshot) *servicev1.IntegrationProfile {
	return &servicev1.IntegrationProfile{
		ProfileId:                string(s.ID),
		PartnerId:                string(s.PartnerID),
		Environment:              environmentsToTheWire[s.Environment],
		Version:                  int32(s.Version),
		SupersedesId:             string(s.SupersedesID),
		ChangeKind:               changeKindsToTheWire[s.ChangeKind],
		Protocols:                s.Protocols,
		Endpoint:                 s.Endpoint,
		SchemaVersion:            s.SchemaVersion,
		SecretRef:                s.SecretRef,
		CertificateRef:           s.CertificateRef,
		CertificateExpiresAt:     int64(s.CertificateExpiresAt),
		EffectiveFrom:            int64(s.EffectiveFrom),
		LastAlertedThresholdDays: int32(s.LastAlertedThresholdDays),
	}
}

func policyOnTheWire(s domain.OperationalPolicySnapshot) *servicev1.OperationalPolicy {
	return &servicev1.OperationalPolicy{
		PolicyId:           string(s.ID),
		PartnerId:          string(s.PartnerID),
		Version:            int32(s.Version),
		SupersedesId:       string(s.SupersedesID),
		EffectiveFrom:      int64(s.EffectiveFrom),
		RateLimitPerMinute: int32(s.RateLimitPerMinute),
		BusinessDays:       s.BusinessDays,
		CutoffTime:         s.CutoffTime,
		ProcessingWindow:   s.ProcessingWindow,
		SlaRef:             s.SlaRef,
		Fallback:           s.Fallback,
		SupportContact:     s.SupportContact,
		IncidentContact:    s.IncidentContact,
	}
}

func inverse[K, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

var (
	rolesToTheWire                = inverse(rolesOnTheWire)
	environmentsToTheWire         = inverse(environmentsOnTheWire)
	rolloutModesToTheWire         = inverse(rolloutModesOnTheWire)
	changeKindsToTheWire          = inverse(changeKindsOnTheWire)
	capabilityStatusesFromTheWire = inverse(capabilityStatusesOnTheWire)
)

var agreementStatusesToTheWire = map[domain.AgreementStatus]servicev1.AgreementStatus{
	domain.AgreementDraft:  servicev1.AgreementStatus_AGREEMENT_STATUS_DRAFT,
	domain.AgreementActive: servicev1.AgreementStatus_AGREEMENT_STATUS_ACTIVE,
}

var triggersToTheWire = map[domain.SuspensionTrigger]servicev1.SuspensionTrigger{
	domain.TriggerManual:         servicev1.SuspensionTrigger_SUSPENSION_TRIGGER_MANUAL,
	domain.TriggerSchemaMismatch: servicev1.SuspensionTrigger_SUSPENSION_TRIGGER_SCHEMA_MISMATCH,
	domain.TriggerOffboarding:    servicev1.SuspensionTrigger_SUSPENSION_TRIGGER_OFFBOARDING,
}

var productStatusesToTheWire = map[domain.ProductStatus]servicev1.ProductStatus{
	domain.ProductActive:  servicev1.ProductStatus_PRODUCT_STATUS_ACTIVE,
	domain.ProductRetired: servicev1.ProductStatus_PRODUCT_STATUS_RETIRED,
}

var conditionsOnTheWire = map[domain.EffectiveCondition]servicev1.EffectiveCapabilityCondition{
	domain.ConditionCapabilityNotFound:     servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_CAPABILITY_NOT_FOUND,
	domain.ConditionPartnerNotActive:       servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PARTNER_NOT_ACTIVE,
	domain.ConditionAgreementNotEffective:  servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_AGREEMENT_NOT_EFFECTIVE,
	domain.ConditionCapabilityNotEnabled:   servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_CAPABILITY_NOT_ENABLED,
	domain.ConditionProductNotEffective:    servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PRODUCT_NOT_EFFECTIVE,
	domain.ConditionNotInRollout:           servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_NOT_IN_ROLLOUT,
	domain.ConditionParametersIncompatible: servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PARAMETERS_INCOMPATIBLE,
}
