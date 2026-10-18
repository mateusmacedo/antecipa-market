package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type ProductStatus string

const (
	ProductActive  ProductStatus = "active"
	ProductRetired ProductStatus = "retired"
)

type ProductRetireReason string

const (
	RetireExpired   ProductRetireReason = "expired"
	RetireWithdrawn ProductRetireReason = "withdrawn"
)

type FinancialProduct struct {
	id               ProductID
	partnerID        PartnerID
	agreementID      AgreementID
	code             string
	name             string
	version          int
	supersedesID     ProductID
	status           ProductStatus
	validFrom        kernel.Instant
	validUntil       kernel.Instant
	assetTypes       []string
	regions          []string
	currency         string
	minAmountMinor   int64
	maxAmountMinor   int64
	minTermDays      int
	maxTermDays      int
	requiredCheckIDs []string
}

func NewFinancialProduct(id ProductID) *FinancialProduct {
	return &FinancialProduct{id: id}
}

func FromFinancialProductSnapshot(s FinancialProductSnapshot) *FinancialProduct {
	return &FinancialProduct{
		id:               s.ID,
		partnerID:        s.PartnerID,
		agreementID:      s.AgreementID,
		code:             s.Code,
		name:             s.Name,
		version:          s.Version,
		supersedesID:     s.SupersedesID,
		status:           s.Status,
		validFrom:        s.ValidFrom,
		validUntil:       s.ValidUntil,
		assetTypes:       slices.Clone(s.AssetTypes),
		regions:          slices.Clone(s.Regions),
		currency:         s.Currency,
		minAmountMinor:   s.MinAmountMinor,
		maxAmountMinor:   s.MaxAmountMinor,
		minTermDays:      s.MinTermDays,
		maxTermDays:      s.MaxTermDays,
		requiredCheckIDs: slices.Clone(s.RequiredCheckIDs),
	}
}

type FinancialProductSnapshot struct {
	ID               ProductID
	PartnerID        PartnerID
	AgreementID      AgreementID
	Code             string
	Name             string
	Version          int
	SupersedesID     ProductID
	Status           ProductStatus
	ValidFrom        kernel.Instant
	ValidUntil       kernel.Instant
	AssetTypes       []string
	Regions          []string
	Currency         string
	MinAmountMinor   int64
	MaxAmountMinor   int64
	MinTermDays      int
	MaxTermDays      int
	RequiredCheckIDs []string
}

func (p *FinancialProduct) Snapshot() FinancialProductSnapshot {
	return FinancialProductSnapshot{
		ID:               p.id,
		PartnerID:        p.partnerID,
		AgreementID:      p.agreementID,
		Code:             p.code,
		Name:             p.name,
		Version:          p.version,
		SupersedesID:     p.supersedesID,
		Status:           p.status,
		ValidFrom:        p.validFrom,
		ValidUntil:       p.validUntil,
		AssetTypes:       slices.Clone(p.assetTypes),
		Regions:          slices.Clone(p.regions),
		Currency:         p.currency,
		MinAmountMinor:   p.minAmountMinor,
		MaxAmountMinor:   p.maxAmountMinor,
		MinTermDays:      p.minTermDays,
		MaxTermDays:      p.maxTermDays,
		RequiredCheckIDs: slices.Clone(p.requiredCheckIDs),
	}
}

func (s FinancialProductSnapshot) Equal(other FinancialProductSnapshot) bool {
	return s.ID == other.ID &&
		s.PartnerID == other.PartnerID &&
		s.AgreementID == other.AgreementID &&
		s.Code == other.Code &&
		s.Name == other.Name &&
		s.Version == other.Version &&
		s.SupersedesID == other.SupersedesID &&
		s.Status == other.Status &&
		s.ValidFrom == other.ValidFrom &&
		s.ValidUntil == other.ValidUntil &&
		slices.Equal(s.AssetTypes, other.AssetTypes) &&
		slices.Equal(s.Regions, other.Regions) &&
		s.Currency == other.Currency &&
		s.MinAmountMinor == other.MinAmountMinor &&
		s.MaxAmountMinor == other.MaxAmountMinor &&
		s.MinTermDays == other.MinTermDays &&
		s.MaxTermDays == other.MaxTermDays &&
		slices.Equal(s.RequiredCheckIDs, other.RequiredCheckIDs)
}

func (p *FinancialProduct) clone() FinancialProduct {
	next := *p
	next.assetTypes = slices.Clone(p.assetTypes)
	next.regions = slices.Clone(p.regions)
	next.requiredCheckIDs = slices.Clone(p.requiredCheckIDs)
	return next
}
