package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func AgreementInEffect(agreements []PartnerAgreementSnapshot, at kernel.Instant) bool {
	current, found := CurrentAgreement(agreements, at)
	return found && FromPartnerAgreementSnapshot(current).validThrough(at)
}

func ProfileInEffect(profiles []IntegrationProfileSnapshot, environment Environment, at kernel.Instant) bool {
	for _, p := range profiles {
		if p.Environment == environment && p.EffectiveFrom <= at {
			return true
		}
	}
	return false
}

func AgreementSustains(agreement PartnerAgreementSnapshot, agreements []PartnerAgreementSnapshot, partnerID PartnerID, at kernel.Instant) bool {
	current, found := CurrentAgreement(agreements, at)
	return found && current.ID == agreement.ID &&
		agreement.PartnerID == partnerID && agreement.Status == AgreementActive && agreement.ValidFrom <= at &&
		FromPartnerAgreementSnapshot(agreement).validThrough(at)
}

func HasEnabledCapability(capabilities []CapabilitySnapshot) bool {
	type seriesKey struct {
		partnerID   PartnerID
		operation   string
		assetType   string
		environment Environment
	}
	series := map[seriesKey][]CapabilitySnapshot{}
	for _, c := range capabilities {
		key := seriesKey{c.PartnerID, c.Operation, c.AssetType, c.Environment}
		series[key] = append(series[key], c)
	}
	for _, versions := range series {
		if current, found := currentCapability(versions); found && current.Status == CapabilityStatusEnabled {
			return true
		}
	}
	return false
}
