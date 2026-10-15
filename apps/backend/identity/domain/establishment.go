package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type Establishment struct {
	id                  EstablishmentID
	organizationID      OrganizationID
	taxIDType           TaxIDType
	taxID               string
	taxIDHash           string
	taxIDHashKeyVersion int
	taxIDMasked         string
	tradeName           string
	receiver            bool
	externalReferences  []ExternalReference
	createdAt           kernel.Instant
}

func NewEstablishment(id EstablishmentID) *Establishment {
	return &Establishment{id: id}
}

func FromEstablishmentSnapshot(s EstablishmentSnapshot) *Establishment {
	return &Establishment{
		id:                  s.ID,
		organizationID:      s.OrganizationID,
		taxIDType:           s.TaxIDType,
		taxID:               s.TaxID,
		taxIDHash:           s.TaxIDHash,
		taxIDHashKeyVersion: s.TaxIDHashKeyVersion,
		taxIDMasked:         s.TaxIDMasked,
		tradeName:           s.TradeName,
		receiver:            s.Receiver,
		externalReferences:  slices.Clone(s.ExternalReferences),
		createdAt:           s.CreatedAt,
	}
}

type EstablishmentSnapshot struct {
	ID                  EstablishmentID
	OrganizationID      OrganizationID
	TaxIDType           TaxIDType
	TaxID               string
	TaxIDHash           string
	TaxIDHashKeyVersion int
	TaxIDMasked         string
	TradeName           string
	Receiver            bool
	ExternalReferences  []ExternalReference
	CreatedAt           kernel.Instant
}

func (e *Establishment) Snapshot() EstablishmentSnapshot {
	return EstablishmentSnapshot{
		ID:                  e.id,
		OrganizationID:      e.organizationID,
		TaxIDType:           e.taxIDType,
		TaxID:               e.taxID,
		TaxIDHash:           e.taxIDHash,
		TaxIDHashKeyVersion: e.taxIDHashKeyVersion,
		TaxIDMasked:         e.taxIDMasked,
		TradeName:           e.tradeName,
		Receiver:            e.receiver,
		ExternalReferences:  slices.Clone(e.externalReferences),
		CreatedAt:           e.createdAt,
	}
}

func (s EstablishmentSnapshot) Equal(other EstablishmentSnapshot) bool {
	return s.ID == other.ID &&
		s.OrganizationID == other.OrganizationID &&
		s.TaxIDType == other.TaxIDType &&
		s.TaxID == other.TaxID &&
		s.TaxIDHash == other.TaxIDHash &&
		s.TaxIDHashKeyVersion == other.TaxIDHashKeyVersion &&
		s.TaxIDMasked == other.TaxIDMasked &&
		s.TradeName == other.TradeName &&
		s.Receiver == other.Receiver &&
		slices.Equal(s.ExternalReferences, other.ExternalReferences) &&
		s.CreatedAt == other.CreatedAt
}

func (e *Establishment) clone() Establishment {
	c := *e
	c.externalReferences = slices.Clone(e.externalReferences)
	return c
}
