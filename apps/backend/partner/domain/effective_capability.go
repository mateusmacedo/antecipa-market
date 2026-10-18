package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type EffectiveCondition string

const (
	ConditionCapabilityNotFound     EffectiveCondition = "capability_not_found"
	ConditionPartnerNotActive       EffectiveCondition = "partner_not_active"
	ConditionAgreementNotEffective  EffectiveCondition = "agreement_not_effective"
	ConditionCapabilityNotEnabled   EffectiveCondition = "capability_not_enabled"
	ConditionProductNotEffective    EffectiveCondition = "product_not_effective"
	ConditionNotInRollout           EffectiveCondition = "not_in_rollout"
	ConditionParametersIncompatible EffectiveCondition = "parameters_incompatible"
)

type EffectiveCapability struct {
	Allowed         bool
	FailedCondition EffectiveCondition
}

type EffectiveQuery struct {
	Operation   string
	AssetType   string
	Environment Environment
	TenantID    string
	AsOf        kernel.Instant
	ProductID   ProductID
	AmountMinor *int64
	TermDays    *int
}

type EffectiveFacts struct {
	Partner      PartnerSnapshot
	PartnerFound bool
	Agreements   []PartnerAgreementSnapshot
	Capabilities []CapabilitySnapshot
	Products     []FinancialProductSnapshot
	Bucket       func(CapabilityID) int
}

func ResolveEffectiveCapability(facts EffectiveFacts, q EffectiveQuery) EffectiveCapability {
	series := slices.DeleteFunc(slices.Clone(facts.Capabilities), func(c CapabilitySnapshot) bool {
		return c.Operation != q.Operation || c.AssetType != q.AssetType || c.Environment != q.Environment
	})
	if len(series) == 0 {
		return denied(ConditionCapabilityNotFound)
	}
	if !facts.PartnerFound || facts.Partner.Status != PartnerActive {
		return denied(ConditionPartnerNotActive)
	}
	if !AgreementInEffect(facts.Agreements, q.AsOf) {
		return denied(ConditionAgreementNotEffective)
	}
	current, found := currentCapability(series)
	if !found || current.Status != CapabilityStatusEnabled {
		return denied(ConditionCapabilityNotEnabled)
	}
	var product FinancialProductSnapshot
	if q.ProductID != "" {
		if product, found = productInEffect(facts.Products, q.ProductID, q.AsOf); !found {
			return denied(ConditionProductNotEffective)
		}
	}
	if !inRollout(current, q.TenantID, facts.Bucket) {
		return denied(ConditionNotInRollout)
	}
	if q.ProductID != "" && !product.admits(q.AmountMinor, q.TermDays) {
		return denied(ConditionParametersIncompatible)
	}
	return EffectiveCapability{Allowed: true}
}

func denied(condition EffectiveCondition) EffectiveCapability {
	return EffectiveCapability{FailedCondition: condition}
}

func currentCapability(series []CapabilitySnapshot) (CapabilitySnapshot, bool) {
	var current CapabilitySnapshot
	found := false
	for _, c := range series {
		live := c.Status == CapabilityStatusEnabled || c.Status == CapabilityStatusSuspended
		if live && (!found || c.Version > current.Version) {
			current, found = c, true
		}
	}
	return current, found
}

func inRollout(c CapabilitySnapshot, tenantID string, bucket func(CapabilityID) int) bool {
	if c.Rollout.Mode != RolloutPercentage || slices.Contains(c.Rollout.ExcludedTenantIDs, tenantID) {
		return c.Rollout.Includes(tenantID, 0)
	}
	if bucket == nil {
		return false
	}
	return c.Rollout.Includes(tenantID, bucket(c.ID))
}

func productInEffect(products []FinancialProductSnapshot, id ProductID, at kernel.Instant) (FinancialProductSnapshot, bool) {
	for _, p := range CurrentProducts(products, at) {
		if p.ID == id && p.inEffectAt(at) {
			return p, true
		}
	}
	return FinancialProductSnapshot{}, false
}

type ProductRequest struct {
	AssetType   string
	Region      string
	AmountMinor int64
	TermDays    int
	AsOf        kernel.Instant
}

func ServesRequest(products []FinancialProductSnapshot, r ProductRequest) bool {
	return slices.ContainsFunc(CurrentProducts(products, r.AsOf), func(p FinancialProductSnapshot) bool {
		return p.inEffectAt(r.AsOf) && slices.Contains(p.AssetTypes, r.AssetType) && slices.Contains(p.Regions, r.Region) &&
			p.admits(&r.AmountMinor, &r.TermDays)
	})
}

func (s FinancialProductSnapshot) inEffectAt(at kernel.Instant) bool {
	return s.Status == ProductActive && s.ValidFrom <= at && (s.ValidUntil == 0 || s.ValidUntil > at)
}

func (s FinancialProductSnapshot) admits(amountMinor *int64, termDays *int) bool {
	amountFits := amountMinor == nil || (*amountMinor >= s.MinAmountMinor && *amountMinor <= s.MaxAmountMinor)
	termFits := termDays == nil || (*termDays >= s.MinTermDays && *termDays <= s.MaxTermDays)
	return amountFits && termFits
}
