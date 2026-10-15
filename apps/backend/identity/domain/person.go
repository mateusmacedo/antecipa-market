package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type Person struct {
	id                     PersonID
	fullName               string
	documentHash           string
	documentHashKeyVersion int
	documentMasked         string
	createdAt              kernel.Instant
}

func NewPerson(id PersonID) *Person {
	return &Person{id: id}
}

func FromPersonSnapshot(s PersonSnapshot) *Person {
	return &Person{
		id:                     s.ID,
		fullName:               s.FullName,
		documentHash:           s.DocumentHash,
		documentHashKeyVersion: s.DocumentHashKeyVersion,
		documentMasked:         s.DocumentMasked,
		createdAt:              s.CreatedAt,
	}
}

type PersonSnapshot struct {
	ID                     PersonID
	FullName               string
	DocumentHash           string
	DocumentHashKeyVersion int
	DocumentMasked         string
	CreatedAt              kernel.Instant
}

func (p *Person) Snapshot() PersonSnapshot {
	return PersonSnapshot{
		ID:                     p.id,
		FullName:               p.fullName,
		DocumentHash:           p.documentHash,
		DocumentHashKeyVersion: p.documentHashKeyVersion,
		DocumentMasked:         p.documentMasked,
		CreatedAt:              p.createdAt,
	}
}

func (s PersonSnapshot) Equal(other PersonSnapshot) bool {
	return s == other
}

func (p *Person) clone() Person {
	return *p
}
