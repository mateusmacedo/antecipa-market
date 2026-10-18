package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type Partner struct {
	id                   PartnerID
	organizationID       OrganizationID
	role                 PartnerRole
	status               PartnerStatus
	registeredAt         kernel.Instant
	stageChangedAt       kernel.Instant
	stageReason          string
	offboardingStartedAt kernel.Instant
}

func NewPartner(id PartnerID) *Partner {
	return &Partner{id: id}
}

func FromPartnerSnapshot(s PartnerSnapshot) *Partner {
	return &Partner{
		id:                   s.ID,
		organizationID:       s.OrganizationID,
		role:                 s.Role,
		status:               s.Status,
		registeredAt:         s.RegisteredAt,
		stageChangedAt:       s.StageChangedAt,
		stageReason:          s.StageReason,
		offboardingStartedAt: s.OffboardingStartedAt,
	}
}

type PartnerSnapshot struct {
	ID                   PartnerID
	OrganizationID       OrganizationID
	Role                 PartnerRole
	Status               PartnerStatus
	RegisteredAt         kernel.Instant
	StageChangedAt       kernel.Instant
	StageReason          string
	OffboardingStartedAt kernel.Instant
}

func (p *Partner) Snapshot() PartnerSnapshot {
	return PartnerSnapshot{
		ID:                   p.id,
		OrganizationID:       p.organizationID,
		Role:                 p.role,
		Status:               p.status,
		RegisteredAt:         p.registeredAt,
		StageChangedAt:       p.stageChangedAt,
		StageReason:          p.stageReason,
		OffboardingStartedAt: p.offboardingStartedAt,
	}
}

func (s PartnerSnapshot) Equal(other PartnerSnapshot) bool {
	return s == other
}

func (p *Partner) clone() Partner {
	return *p
}
