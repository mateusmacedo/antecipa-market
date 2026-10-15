package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type Organization struct {
	id                  OrganizationID
	kinds               OrganizationKinds
	jurisdiction        string
	taxIDType           TaxIDType
	taxID               string
	taxIDHash           string
	taxIDHashKeyVersion int
	taxIDMasked         string
	legalName           string
	tradeName           string
	status              OrganizationStatus
	createdAt           kernel.Instant
}

func NewOrganization(id OrganizationID) *Organization {
	return &Organization{id: id}
}

func FromOrganizationSnapshot(s OrganizationSnapshot) *Organization {
	return &Organization{
		id:                  s.ID,
		kinds:               s.Kinds,
		jurisdiction:        s.Jurisdiction,
		taxIDType:           s.TaxIDType,
		taxID:               s.TaxID,
		taxIDHash:           s.TaxIDHash,
		taxIDHashKeyVersion: s.TaxIDHashKeyVersion,
		taxIDMasked:         s.TaxIDMasked,
		legalName:           s.LegalName,
		tradeName:           s.TradeName,
		status:              s.Status,
		createdAt:           s.CreatedAt,
	}
}

type OrganizationSnapshot struct {
	ID                  OrganizationID
	Kinds               OrganizationKinds
	Jurisdiction        string
	TaxIDType           TaxIDType
	TaxID               string
	TaxIDHash           string
	TaxIDHashKeyVersion int
	TaxIDMasked         string
	LegalName           string
	TradeName           string
	Status              OrganizationStatus
	CreatedAt           kernel.Instant
}

func (o *Organization) Snapshot() OrganizationSnapshot {
	return OrganizationSnapshot{
		ID:                  o.id,
		Kinds:               o.kinds,
		Jurisdiction:        o.jurisdiction,
		TaxIDType:           o.taxIDType,
		TaxID:               o.taxID,
		TaxIDHash:           o.taxIDHash,
		TaxIDHashKeyVersion: o.taxIDHashKeyVersion,
		TaxIDMasked:         o.taxIDMasked,
		LegalName:           o.legalName,
		TradeName:           o.tradeName,
		Status:              o.status,
		CreatedAt:           o.createdAt,
	}
}

func (s OrganizationSnapshot) Equal(other OrganizationSnapshot) bool {
	return s == other
}

func (o *Organization) clone() Organization {
	return *o
}
