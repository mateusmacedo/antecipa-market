package application

import (
	"context"
	"errors"
	"slices"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// organizationStatus is the precondition read of another aggregate, inside the
// unit of work but outside the optimistic lock. An organization not found, or
// held by another tenant, is OrganizationNew, which the domain decision refuses
// as not active.
func organizationStatus(ctx context.Context, organizations ports.OrganizationRepository, id domain.OrganizationID) (domain.OrganizationStatus, error) {
	snapshot, err := organizationOf(ctx, organizations, id)
	return snapshot.Status, err
}

func personFound(ctx context.Context, persons ports.PersonRepository, id domain.PersonID) (bool, error) {
	_, _, err := persons.Load(ctx, id)
	switch {
	case errors.Is(err, port.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// The kernel Relation filters by one column, so the organization half of the
// key is refined in memory over the rows of the filtered column.
func (s Service) establishmentTaxIDRegistered(ctx context.Context, taxIDHash string, organizationID domain.OrganizationID) (bool, error) {
	found, err := s.EstablishmentTaxIDReader.LoadByTaxIDHash(ctx, taxIDHash)
	if absent(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(found, func(e domain.EstablishmentSnapshot) bool { return e.OrganizationID == organizationID }), nil
}

func (s Service) documentRegistered(ctx context.Context, documentHash string) (bool, error) {
	found, err := s.DocumentReader.LoadByDocumentHash(ctx, documentHash)
	if absent(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(found) > 0, nil
}

// absent tells a read that met only another tenant's rows: nothing of them is
// visible in this tenant, so the uniqueness reads find the value free and
// FindEffectivePermissions finds no membership.
func absent(err error) bool {
	var access port.CrossTenantAccess
	return errors.As(err, &access)
}

// organizationOf is the precondition read of another aggregate when the command
// needs more than the status: an organization not found, or held by another
// tenant, is the zero snapshot, whose OrganizationNew status the domain decision
// refuses as not active.
func organizationOf(ctx context.Context, organizations ports.OrganizationRepository, id domain.OrganizationID) (domain.OrganizationSnapshot, error) {
	snapshot, _, err := organizations.Load(ctx, id)
	switch {
	case errors.Is(err, port.ErrNotFound):
		return domain.OrganizationSnapshot{}, nil
	case err != nil:
		return domain.OrganizationSnapshot{}, err
	}
	return snapshot, nil
}

func userAccountStatus(ctx context.Context, accounts ports.UserAccountRepository, id domain.UserID) (domain.UserAccountStatus, error) {
	snapshot, _, err := accounts.Load(ctx, id)
	switch {
	case errors.Is(err, port.ErrNotFound):
		return domain.UserAccountNew, nil
	case err != nil:
		return domain.UserAccountNew, err
	}
	return snapshot.Status, nil
}
