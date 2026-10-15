package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type ProtectedIdentifier struct {
	Hash       string
	KeyVersion int
}

// IdentifierProtector derives the keyed hash of an identifier. It is a port
// because the key and crypto/* live only in the provider, and the tenant is an
// input so the same identifier hashes apart in each tenant.
type IdentifierProtector interface {
	Protect(ctx context.Context, tenant port.TenantID, kind domain.TaxIDType, value string) (ProtectedIdentifier, error)
}
