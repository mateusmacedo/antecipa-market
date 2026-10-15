package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type BankAccountChange struct {
	id                    BankAccountChangeID
	organizationID        OrganizationID
	destinationEvidenceID string
	requestedByUserID     string
	approvedByUserID      string
	status                BankAccountChangeStatus
	requestedAt           kernel.Instant
	approvedAt            kernel.Instant
}

func NewBankAccountChange(id BankAccountChangeID) *BankAccountChange {
	return &BankAccountChange{id: id}
}

func FromBankAccountChangeSnapshot(s BankAccountChangeSnapshot) *BankAccountChange {
	return &BankAccountChange{
		id:                    s.ID,
		organizationID:        s.OrganizationID,
		destinationEvidenceID: s.DestinationEvidenceID,
		requestedByUserID:     s.RequestedByUserID,
		approvedByUserID:      s.ApprovedByUserID,
		status:                s.Status,
		requestedAt:           s.RequestedAt,
		approvedAt:            s.ApprovedAt,
	}
}

type BankAccountChangeSnapshot struct {
	ID                    BankAccountChangeID
	OrganizationID        OrganizationID
	DestinationEvidenceID string
	RequestedByUserID     string
	ApprovedByUserID      string
	Status                BankAccountChangeStatus
	RequestedAt           kernel.Instant
	ApprovedAt            kernel.Instant
}

func (c *BankAccountChange) Snapshot() BankAccountChangeSnapshot {
	return BankAccountChangeSnapshot{
		ID:                    c.id,
		OrganizationID:        c.organizationID,
		DestinationEvidenceID: c.destinationEvidenceID,
		RequestedByUserID:     c.requestedByUserID,
		ApprovedByUserID:      c.approvedByUserID,
		Status:                c.status,
		RequestedAt:           c.requestedAt,
		ApprovedAt:            c.approvedAt,
	}
}

func (s BankAccountChangeSnapshot) Equal(other BankAccountChangeSnapshot) bool {
	return s == other
}

func (c *BankAccountChange) clone() BankAccountChange {
	return *c
}
