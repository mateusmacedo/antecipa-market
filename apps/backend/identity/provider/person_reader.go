package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

func NewPersonReader(pool postgres.ReadPool) ports.PersonReader {
	return personTable.Reader(pool)
}

var personsByDocumentHash = personTable.Relation("document_hash")

type PersonsByDocumentReader struct{ pool postgres.ReadPool }

func NewPersonsByDocumentReader(pool postgres.ReadPool) *PersonsByDocumentReader {
	return &PersonsByDocumentReader{pool: pool}
}

func (r *PersonsByDocumentReader) LoadByDocumentHash(ctx context.Context, documentHash string) ([]domain.PersonSnapshot, error) {
	return personsByDocumentHash.Query(ctx, r.pool, documentHash)
}

var _ ports.PersonsByDocumentReader = (*PersonsByDocumentReader)(nil)
