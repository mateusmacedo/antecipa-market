package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type Membership struct {
	id                  MembershipID
	organizationID      OrganizationID
	inviteeEmail        string
	role                Role
	status              MembershipStatus
	invitedByUserID     string
	invitedAt           kernel.Instant
	invitationExpiresAt kernel.Instant
	userID              UserID
	personID            PersonID
	verificationLevel   VerificationLevel
	grantedAt           kernel.Instant
	revokedAt           kernel.Instant
	revocationReason    string
}

func NewMembership(id MembershipID) *Membership {
	return &Membership{id: id}
}

func FromMembershipSnapshot(s MembershipSnapshot) *Membership {
	return &Membership{
		id:                  s.ID,
		organizationID:      s.OrganizationID,
		inviteeEmail:        s.InviteeEmail,
		role:                s.Role,
		status:              s.Status,
		invitedByUserID:     s.InvitedByUserID,
		invitedAt:           s.InvitedAt,
		invitationExpiresAt: s.InvitationExpiresAt,
		userID:              s.UserID,
		personID:            s.PersonID,
		verificationLevel:   s.VerificationLevel,
		grantedAt:           s.GrantedAt,
		revokedAt:           s.RevokedAt,
		revocationReason:    s.RevocationReason,
	}
}

type MembershipSnapshot struct {
	ID                  MembershipID
	OrganizationID      OrganizationID
	InviteeEmail        string
	Role                Role
	Status              MembershipStatus
	InvitedByUserID     string
	InvitedAt           kernel.Instant
	InvitationExpiresAt kernel.Instant
	UserID              UserID
	PersonID            PersonID
	VerificationLevel   VerificationLevel
	GrantedAt           kernel.Instant
	RevokedAt           kernel.Instant
	RevocationReason    string
}

func (m *Membership) Snapshot() MembershipSnapshot {
	return MembershipSnapshot{
		ID:                  m.id,
		OrganizationID:      m.organizationID,
		InviteeEmail:        m.inviteeEmail,
		Role:                m.role,
		Status:              m.status,
		InvitedByUserID:     m.invitedByUserID,
		InvitedAt:           m.invitedAt,
		InvitationExpiresAt: m.invitationExpiresAt,
		UserID:              m.userID,
		PersonID:            m.personID,
		VerificationLevel:   m.verificationLevel,
		GrantedAt:           m.grantedAt,
		RevokedAt:           m.revokedAt,
		RevocationReason:    m.revocationReason,
	}
}

func (s MembershipSnapshot) Equal(other MembershipSnapshot) bool {
	return s == other
}

func (m *Membership) clone() Membership {
	return *m
}
