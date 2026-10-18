package domain

import (
	"maps"
	"slices"
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func CurrentProducts(products []FinancialProductSnapshot, at kernel.Instant) []FinancialProductSnapshot {
	current := map[string]FinancialProductSnapshot{}
	for _, p := range products {
		if held, seen := current[p.Code]; p.ValidFrom <= at && (!seen || p.Version > held.Version) {
			current[p.Code] = p
		}
	}
	out := slices.AppendSeq(make([]FinancialProductSnapshot, 0, len(current)), maps.Values(current))
	slices.SortFunc(out, func(a, b FinancialProductSnapshot) int { return strings.Compare(a.Code, b.Code) })
	return out
}

func CurrentAgreement(agreements []PartnerAgreementSnapshot, at kernel.Instant) (PartnerAgreementSnapshot, bool) {
	var current PartnerAgreementSnapshot
	found := false
	for _, a := range agreements {
		if a.Status == AgreementActive && a.ValidFrom <= at && (!found || a.Version > current.Version) {
			current, found = a, true
		}
	}
	return current, found
}

func CurrentProfile(profiles []IntegrationProfileSnapshot, environment Environment, at kernel.Instant) (IntegrationProfileSnapshot, bool) {
	var current IntegrationProfileSnapshot
	found := false
	for _, p := range profiles {
		if p.Environment == environment && p.EffectiveFrom <= at && (!found || p.Version > current.Version) {
			current, found = p, true
		}
	}
	return current, found
}

func CurrentPolicy(policies []OperationalPolicySnapshot, at kernel.Instant) (OperationalPolicySnapshot, bool) {
	var current OperationalPolicySnapshot
	found := false
	for _, p := range policies {
		if p.EffectiveFrom <= at && (!found || p.Version > current.Version) {
			current, found = p, true
		}
	}
	return current, found
}
