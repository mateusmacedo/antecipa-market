package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type UserAccount struct {
	subject          UserID
	personID         PersonID
	status           UserAccountStatus
	provisionedAt    kernel.Instant
	suspendedAt      kernel.Instant
	suspensionReason string
}

func NewUserAccount(subject UserID) *UserAccount {
	return &UserAccount{subject: subject}
}

func FromUserAccountSnapshot(s UserAccountSnapshot) *UserAccount {
	return &UserAccount{
		subject:          s.Subject,
		personID:         s.PersonID,
		status:           s.Status,
		provisionedAt:    s.ProvisionedAt,
		suspendedAt:      s.SuspendedAt,
		suspensionReason: s.SuspensionReason,
	}
}

type UserAccountSnapshot struct {
	Subject          UserID
	PersonID         PersonID
	Status           UserAccountStatus
	ProvisionedAt    kernel.Instant
	SuspendedAt      kernel.Instant
	SuspensionReason string
}

func (u *UserAccount) Snapshot() UserAccountSnapshot {
	return UserAccountSnapshot{
		Subject:          u.subject,
		PersonID:         u.personID,
		Status:           u.status,
		ProvisionedAt:    u.provisionedAt,
		SuspendedAt:      u.suspendedAt,
		SuspensionReason: u.suspensionReason,
	}
}

func (s UserAccountSnapshot) Equal(other UserAccountSnapshot) bool {
	return s == other
}

func (u *UserAccount) clone() UserAccount {
	return *u
}
