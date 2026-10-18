package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type IntegrationChangeKind string

const (
	ChangeInitial     IntegrationChangeKind = "initial"
	ChangeAnnounced   IntegrationChangeKind = "announced"
	ChangeUnannounced IntegrationChangeKind = "unannounced"
)

type IntegrationProfile struct {
	id                       ProfileID
	partnerID                PartnerID
	environment              Environment
	version                  int
	supersedesID             ProfileID
	changeKind               IntegrationChangeKind
	protocols                []string
	endpoint                 string
	schemaVersion            string
	secretRef                string
	certificateRef           string
	certificateExpiresAt     kernel.Instant
	effectiveFrom            kernel.Instant
	lastAlertedThresholdDays int
}

func NewIntegrationProfile(id ProfileID) *IntegrationProfile {
	return &IntegrationProfile{id: id}
}

func FromIntegrationProfileSnapshot(s IntegrationProfileSnapshot) *IntegrationProfile {
	return &IntegrationProfile{
		id:                       s.ID,
		partnerID:                s.PartnerID,
		environment:              s.Environment,
		version:                  s.Version,
		supersedesID:             s.SupersedesID,
		changeKind:               s.ChangeKind,
		protocols:                slices.Clone(s.Protocols),
		endpoint:                 s.Endpoint,
		schemaVersion:            s.SchemaVersion,
		secretRef:                s.SecretRef,
		certificateRef:           s.CertificateRef,
		certificateExpiresAt:     s.CertificateExpiresAt,
		effectiveFrom:            s.EffectiveFrom,
		lastAlertedThresholdDays: s.LastAlertedThresholdDays,
	}
}

type IntegrationProfileSnapshot struct {
	ID                       ProfileID
	PartnerID                PartnerID
	Environment              Environment
	Version                  int
	SupersedesID             ProfileID
	ChangeKind               IntegrationChangeKind
	Protocols                []string
	Endpoint                 string
	SchemaVersion            string
	SecretRef                string
	CertificateRef           string
	CertificateExpiresAt     kernel.Instant
	EffectiveFrom            kernel.Instant
	LastAlertedThresholdDays int
}

func (p *IntegrationProfile) Snapshot() IntegrationProfileSnapshot {
	return IntegrationProfileSnapshot{
		ID:                       p.id,
		PartnerID:                p.partnerID,
		Environment:              p.environment,
		Version:                  p.version,
		SupersedesID:             p.supersedesID,
		ChangeKind:               p.changeKind,
		Protocols:                slices.Clone(p.protocols),
		Endpoint:                 p.endpoint,
		SchemaVersion:            p.schemaVersion,
		SecretRef:                p.secretRef,
		CertificateRef:           p.certificateRef,
		CertificateExpiresAt:     p.certificateExpiresAt,
		EffectiveFrom:            p.effectiveFrom,
		LastAlertedThresholdDays: p.lastAlertedThresholdDays,
	}
}

func (s IntegrationProfileSnapshot) Equal(other IntegrationProfileSnapshot) bool {
	return s.ID == other.ID &&
		s.PartnerID == other.PartnerID &&
		s.Environment == other.Environment &&
		s.Version == other.Version &&
		s.SupersedesID == other.SupersedesID &&
		s.ChangeKind == other.ChangeKind &&
		slices.Equal(s.Protocols, other.Protocols) &&
		s.Endpoint == other.Endpoint &&
		s.SchemaVersion == other.SchemaVersion &&
		s.SecretRef == other.SecretRef &&
		s.CertificateRef == other.CertificateRef &&
		s.CertificateExpiresAt == other.CertificateExpiresAt &&
		s.EffectiveFrom == other.EffectiveFrom &&
		s.LastAlertedThresholdDays == other.LastAlertedThresholdDays
}

func (p *IntegrationProfile) clone() IntegrationProfile {
	next := *p
	next.protocols = slices.Clone(p.protocols)
	return next
}
