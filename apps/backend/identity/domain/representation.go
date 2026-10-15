package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type Representation struct {
	id                  RepresentationID
	organizationID      OrganizationID
	personID            PersonID
	powerType           PowerType
	sourceType          SourceType
	sourceEvidenceID    string
	scopeCurrency       string
	scopeMaxAmountMinor int64
	scopeActTypes       []string
	validFrom           kernel.Instant
	validUntil          kernel.Instant
	verificationStatus  RepresentationStatus
	proposedByUserID    string
}

func NewRepresentation(id RepresentationID) *Representation {
	return &Representation{id: id}
}

func FromRepresentationSnapshot(s RepresentationSnapshot) *Representation {
	return &Representation{
		id:                  s.ID,
		organizationID:      s.OrganizationID,
		personID:            s.PersonID,
		powerType:           s.PowerType,
		sourceType:          s.SourceType,
		sourceEvidenceID:    s.SourceEvidenceID,
		scopeCurrency:       s.ScopeCurrency,
		scopeMaxAmountMinor: s.ScopeMaxAmountMinor,
		scopeActTypes:       slices.Clone(s.ScopeActTypes),
		validFrom:           s.ValidFrom,
		validUntil:          s.ValidUntil,
		verificationStatus:  s.VerificationStatus,
		proposedByUserID:    s.ProposedByUserID,
	}
}

type RepresentationSnapshot struct {
	ID                  RepresentationID
	OrganizationID      OrganizationID
	PersonID            PersonID
	PowerType           PowerType
	SourceType          SourceType
	SourceEvidenceID    string
	ScopeCurrency       string
	ScopeMaxAmountMinor int64
	ScopeActTypes       []string
	ValidFrom           kernel.Instant
	ValidUntil          kernel.Instant
	VerificationStatus  RepresentationStatus
	ProposedByUserID    string
}

func (r *Representation) Snapshot() RepresentationSnapshot {
	return RepresentationSnapshot{
		ID:                  r.id,
		OrganizationID:      r.organizationID,
		PersonID:            r.personID,
		PowerType:           r.powerType,
		SourceType:          r.sourceType,
		SourceEvidenceID:    r.sourceEvidenceID,
		ScopeCurrency:       r.scopeCurrency,
		ScopeMaxAmountMinor: r.scopeMaxAmountMinor,
		ScopeActTypes:       slices.Clone(r.scopeActTypes),
		ValidFrom:           r.validFrom,
		ValidUntil:          r.validUntil,
		VerificationStatus:  r.verificationStatus,
		ProposedByUserID:    r.proposedByUserID,
	}
}

func (s RepresentationSnapshot) Equal(other RepresentationSnapshot) bool {
	return s.ID == other.ID &&
		s.OrganizationID == other.OrganizationID &&
		s.PersonID == other.PersonID &&
		s.PowerType == other.PowerType &&
		s.SourceType == other.SourceType &&
		s.SourceEvidenceID == other.SourceEvidenceID &&
		s.ScopeCurrency == other.ScopeCurrency &&
		s.ScopeMaxAmountMinor == other.ScopeMaxAmountMinor &&
		slices.Equal(s.ScopeActTypes, other.ScopeActTypes) &&
		s.ValidFrom == other.ValidFrom &&
		s.ValidUntil == other.ValidUntil &&
		s.VerificationStatus == other.VerificationStatus &&
		s.ProposedByUserID == other.ProposedByUserID
}

func (s RepresentationSnapshot) InForceAt(at kernel.Instant) bool {
	return s.ValidFrom <= at && (s.ValidUntil == 0 || at < s.ValidUntil)
}

func (r *Representation) clone() Representation {
	c := *r
	c.scopeActTypes = slices.Clone(r.scopeActTypes)
	return c
}
