package application

import (
	"context"
	"errors"
	"slices"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// ErrTenantUnresolved refuses to protect an identifier without a tenant: the
// tenant is an input of the hash, and no placeholder tenant such as "system"
// or "default" is ever invented to fill its absence.
var ErrTenantUnresolved = errors.New("application: the execution context resolved no tenant")

func (s Service) protectTaxID(ctx context.Context, kind domain.TaxIDType, value string) (ports.ProtectedIdentifier, error) {
	execution, err := port.RequireExecutionContext(ctx)
	if err != nil {
		return ports.ProtectedIdentifier{}, err
	}
	tenant, ok := execution.Tenant()
	if !ok {
		return ports.ProtectedIdentifier{}, ErrTenantUnresolved
	}
	return s.Protector.Protect(ctx, tenant, kind, value)
}

// taxIDRegistered is the consultative per-tenant uniqueness read: it runs on the
// pool, apart from the command transaction, so the unique constraint stays the
// authority and a race ends as a technical failure of the loser.
func (s Service) taxIDRegistered(ctx context.Context, taxIDHash, jurisdiction string) (bool, error) {
	found, err := s.TaxIDReader.LoadByTaxIDHash(ctx, taxIDHash)
	if err != nil && !absent(err) {
		return false, err
	}
	return slices.ContainsFunc(found, inJurisdiction(jurisdiction)), nil
}

func inJurisdiction(jurisdiction string) func(domain.OrganizationSnapshot) bool {
	return func(s domain.OrganizationSnapshot) bool { return s.Jurisdiction == jurisdiction }
}
