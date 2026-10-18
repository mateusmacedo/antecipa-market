package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type OperationalPolicy struct {
	id                 PolicyID
	partnerID          PartnerID
	version            int
	supersedesID       PolicyID
	effectiveFrom      kernel.Instant
	rateLimitPerMinute int
	businessDays       []string
	cutoffTime         string
	processingWindow   string
	slaRef             string
	fallback           string
	supportContact     string
	incidentContact    string
}

func NewOperationalPolicy(id PolicyID) *OperationalPolicy {
	return &OperationalPolicy{id: id}
}

func FromOperationalPolicySnapshot(s OperationalPolicySnapshot) *OperationalPolicy {
	return &OperationalPolicy{
		id:                 s.ID,
		partnerID:          s.PartnerID,
		version:            s.Version,
		supersedesID:       s.SupersedesID,
		effectiveFrom:      s.EffectiveFrom,
		rateLimitPerMinute: s.RateLimitPerMinute,
		businessDays:       slices.Clone(s.BusinessDays),
		cutoffTime:         s.CutoffTime,
		processingWindow:   s.ProcessingWindow,
		slaRef:             s.SlaRef,
		fallback:           s.Fallback,
		supportContact:     s.SupportContact,
		incidentContact:    s.IncidentContact,
	}
}

type OperationalPolicySnapshot struct {
	ID                 PolicyID
	PartnerID          PartnerID
	Version            int
	SupersedesID       PolicyID
	EffectiveFrom      kernel.Instant
	RateLimitPerMinute int
	BusinessDays       []string
	CutoffTime         string
	ProcessingWindow   string
	SlaRef             string
	Fallback           string
	SupportContact     string
	IncidentContact    string
}

func (p *OperationalPolicy) Snapshot() OperationalPolicySnapshot {
	return OperationalPolicySnapshot{
		ID:                 p.id,
		PartnerID:          p.partnerID,
		Version:            p.version,
		SupersedesID:       p.supersedesID,
		EffectiveFrom:      p.effectiveFrom,
		RateLimitPerMinute: p.rateLimitPerMinute,
		BusinessDays:       slices.Clone(p.businessDays),
		CutoffTime:         p.cutoffTime,
		ProcessingWindow:   p.processingWindow,
		SlaRef:             p.slaRef,
		Fallback:           p.fallback,
		SupportContact:     p.supportContact,
		IncidentContact:    p.incidentContact,
	}
}

func (s OperationalPolicySnapshot) Equal(other OperationalPolicySnapshot) bool {
	return s.ID == other.ID &&
		s.PartnerID == other.PartnerID &&
		s.Version == other.Version &&
		s.SupersedesID == other.SupersedesID &&
		s.EffectiveFrom == other.EffectiveFrom &&
		s.RateLimitPerMinute == other.RateLimitPerMinute &&
		slices.Equal(s.BusinessDays, other.BusinessDays) &&
		s.CutoffTime == other.CutoffTime &&
		s.ProcessingWindow == other.ProcessingWindow &&
		s.SlaRef == other.SlaRef &&
		s.Fallback == other.Fallback &&
		s.SupportContact == other.SupportContact &&
		s.IncidentContact == other.IncidentContact
}

func (p *OperationalPolicy) clone() OperationalPolicy {
	next := *p
	next.businessDays = slices.Clone(p.businessDays)
	return next
}
