package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (p *FinancialProduct) Define(cmd DefineProduct) (kernel.Accepted[DefineProductResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*FinancialProduct).clone, func(next *FinancialProduct) (kernel.Accepted[DefineProductResponse], *kernel.Rejection) {
		switch {
		case len(cmd.AssetTypes) == 0:
			return kernel.Refuse[DefineProductResponse](CodeFinancialProductAssetTypesEmpty, "at least one asset type is required")
		case cmd.MinAmountMinor > cmd.MaxAmountMinor || cmd.MinTermDays > cmd.MaxTermDays:
			return kernel.Refuse[DefineProductResponse](CodeFinancialProductRangeInvalid, "the minimum amount and term must not exceed the maximum")
		case cmd.ValidUntil != 0 && cmd.ValidUntil <= cmd.ValidFrom:
			return kernel.Refuse[DefineProductResponse](CodeFinancialProductValidityInvalid, "the end of the validity must come after its start")
		case !cmd.HasEffectiveAgreement:
			return kernel.Refuse[DefineProductResponse](CodeFinancialProductAgreementNotActive, "an active agreement of the partner in effect is required")
		case cmd.SupersedesID != cmd.Head.ID:
			return kernel.Refuse[DefineProductResponse](CodeFinancialProductSupersedesInvalid,
				"supersedes must name the last version of the series of the partner and code, and a series holds one first version",
				kernel.Detail{Key: "supersedes", Value: string(cmd.SupersedesID)},
			)
		}
		next.partnerID = cmd.PartnerID
		next.agreementID = cmd.AgreementID
		next.code = cmd.Code
		next.name = cmd.Name
		next.version = cmd.Head.Version + 1
		next.supersedesID = cmd.SupersedesID
		next.status = ProductActive
		next.validFrom = cmd.ValidFrom
		next.validUntil = cmd.ValidUntil
		next.assetTypes = slices.Clone(cmd.AssetTypes)
		next.regions = slices.Clone(cmd.Regions)
		next.currency = cmd.Currency
		next.minAmountMinor = cmd.MinAmountMinor
		next.maxAmountMinor = cmd.MaxAmountMinor
		next.minTermDays = cmd.MinTermDays
		next.maxTermDays = cmd.MaxTermDays
		next.requiredCheckIDs = slices.Clone(cmd.RequiredCheckIDs)
		return kernel.Accept(DefineProductResponse{ProductID: next.id}, next.definition(cmd.At)), nil
	})
}

func (p *FinancialProduct) definition(at kernel.Instant) kernel.DomainEvent {
	if p.supersedesID == "" {
		return PartnerProductDefined{
			ProductID:      p.id,
			PartnerID:      p.partnerID,
			AgreementID:    p.agreementID,
			Code:           p.code,
			Version:        p.version,
			ValidFrom:      p.validFrom,
			ValidUntil:     p.validUntil,
			AssetTypes:     slices.Clone(p.assetTypes),
			Regions:        slices.Clone(p.regions),
			Currency:       p.currency,
			MinAmountMinor: p.minAmountMinor,
			MaxAmountMinor: p.maxAmountMinor,
			MinTermDays:    p.minTermDays,
			MaxTermDays:    p.maxTermDays,
			At:             at,
		}
	}
	return PartnerProductChanged{
		ProductID:      p.id,
		PartnerID:      p.partnerID,
		AgreementID:    p.agreementID,
		Code:           p.code,
		Version:        p.version,
		SupersedesID:   p.supersedesID,
		ValidFrom:      p.validFrom,
		ValidUntil:     p.validUntil,
		AssetTypes:     slices.Clone(p.assetTypes),
		Regions:        slices.Clone(p.regions),
		Currency:       p.currency,
		MinAmountMinor: p.minAmountMinor,
		MaxAmountMinor: p.maxAmountMinor,
		MinTermDays:    p.minTermDays,
		MaxTermDays:    p.maxTermDays,
		At:             at,
	}
}
