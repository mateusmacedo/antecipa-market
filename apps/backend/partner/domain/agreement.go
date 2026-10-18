package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type AgreementStatus string

const (
	AgreementDraft  AgreementStatus = "draft"
	AgreementActive AgreementStatus = "active"
)

type PartnerAgreement struct {
	id                   AgreementID
	partnerID            PartnerID
	version              int
	supersedesID         AgreementID
	status               AgreementStatus
	validFrom            kernel.Instant
	validUntil           kernel.Instant
	contractEvidenceID   string
	instrumentTemplateID string
	activatedAt          kernel.Instant
}

func NewPartnerAgreement(id AgreementID) *PartnerAgreement {
	return &PartnerAgreement{id: id}
}

func FromPartnerAgreementSnapshot(s PartnerAgreementSnapshot) *PartnerAgreement {
	return &PartnerAgreement{
		id:                   s.ID,
		partnerID:            s.PartnerID,
		version:              s.Version,
		supersedesID:         s.SupersedesID,
		status:               s.Status,
		validFrom:            s.ValidFrom,
		validUntil:           s.ValidUntil,
		contractEvidenceID:   s.ContractEvidenceID,
		instrumentTemplateID: s.InstrumentTemplateID,
		activatedAt:          s.ActivatedAt,
	}
}

type PartnerAgreementSnapshot struct {
	ID                   AgreementID
	PartnerID            PartnerID
	Version              int
	SupersedesID         AgreementID
	Status               AgreementStatus
	ValidFrom            kernel.Instant
	ValidUntil           kernel.Instant
	ContractEvidenceID   string
	InstrumentTemplateID string
	ActivatedAt          kernel.Instant
}

func (a *PartnerAgreement) Snapshot() PartnerAgreementSnapshot {
	return PartnerAgreementSnapshot{
		ID:                   a.id,
		PartnerID:            a.partnerID,
		Version:              a.version,
		SupersedesID:         a.supersedesID,
		Status:               a.status,
		ValidFrom:            a.validFrom,
		ValidUntil:           a.validUntil,
		ContractEvidenceID:   a.contractEvidenceID,
		InstrumentTemplateID: a.instrumentTemplateID,
		ActivatedAt:          a.activatedAt,
	}
}

func (s PartnerAgreementSnapshot) Equal(other PartnerAgreementSnapshot) bool {
	return s == other
}

func (a *PartnerAgreement) clone() PartnerAgreement {
	return *a
}

func (a *PartnerAgreement) validThrough(at kernel.Instant) bool {
	return a.validUntil == 0 || a.validUntil > at
}
