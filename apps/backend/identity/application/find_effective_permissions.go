package application

import (
	"context"
	"errors"
	"fmt"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

// FindEffectivePermissions reads the memberships before the account, so the
// suspension is the last check (fail-closed): a suspension committed between
// the two reads, which share no snapshot, still empties the answer.
func (s Service) FindEffectivePermissions(ctx context.Context, user domain.UserID) ([]domain.Permission, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindEffectivePermissions)

	if err := s.Authorize(ctx, FindEffectivePermissions{User: user}); err != nil {
		end(port.AuthorizationResult(err))
		return nil, err
	}
	fail := func(err error) ([]domain.Permission, error) {
		failed := fmt.Errorf("application: find effective permissions %s: %w", user, err)
		end(port.Result{Outcome: port.OutcomeFailed, Err: failed})
		return nil, failed
	}

	memberships, membershipsErr := s.MembershipsByUser.LoadByUser(ctx, user)
	if membershipsErr != nil && !absent(membershipsErr) {
		return fail(membershipsErr)
	}
	account, _, accountErr := s.UserAccountReader.Load(ctx, user)
	if accountErr != nil && !errors.Is(accountErr, port.ErrNotFound) {
		return fail(accountErr)
	}
	end(readResult(membershipsErr, accountErr))

	if accountErr != nil || account.Status != domain.UserAccountActive {
		return []domain.Permission{}, nil
	}
	roles := make([]domain.Role, 0, len(memberships))
	for _, m := range memberships {
		if m.Status == domain.MembershipStatusActive {
			roles = append(roles, m.Role)
		}
	}
	return domain.EffectivePermissions(roles...), nil
}

// readResult hands a CrossTenantAccess of either read to the security record,
// which logs every cross-tenant attempt, while the caller gets the answer of a
// user nobody holds, so the response never reveals the record exists.
func readResult(errs ...error) port.Result {
	for _, err := range errs {
		var access port.CrossTenantAccess
		if errors.As(err, &access) {
			return port.Result{Outcome: port.OutcomeFailed, Err: access}
		}
	}
	return port.Result{Outcome: port.OutcomeAccepted}
}
