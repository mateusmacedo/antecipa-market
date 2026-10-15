package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) FindOrganizationByTaxID(ctx context.Context, q FindOrganizationByTaxID) (domain.OrganizationSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindOrganizationByTaxID)

	if err := s.Authorize(ctx, q); err != nil {
		end(port.AuthorizationResult(err))
		return domain.OrganizationSnapshot{}, err
	}
	fail := func(err error) (domain.OrganizationSnapshot, error) {
		failed := fmt.Errorf("application: find organization by tax id: %w", err)
		end(port.Result{Outcome: port.OutcomeFailed, Err: failed})
		return domain.OrganizationSnapshot{}, failed
	}

	protected, err := s.protectTaxID(ctx, q.TaxIDType, q.TaxID)
	if err != nil {
		return fail(err)
	}
	found, err := s.TaxIDReader.LoadByTaxIDHash(ctx, protected.Hash)
	var access port.CrossTenantAccess
	switch {
	case errors.As(err, &access):
		// IDN-13: the caller gets the answer of a tax id nobody holds; the
		// instrumentation still gets the access, which IDN-12 records.
		end(port.Result{Outcome: port.OutcomeFailed, Err: access})
		return domain.OrganizationSnapshot{}, fmt.Errorf("application: find organization by tax id: %w", port.ErrNotFound)
	case err != nil:
		return fail(err)
	}

	i := slices.IndexFunc(found, inJurisdiction(q.Jurisdiction))
	if i < 0 {
		return fail(port.ErrNotFound)
	}
	end(port.Result{Outcome: port.OutcomeAccepted})
	return found[i], nil
}
