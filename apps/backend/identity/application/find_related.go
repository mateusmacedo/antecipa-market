package application

import (
	"context"
	"errors"
	"fmt"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// findRelated is the query of a relation, fixed once for every collection of
// this context: a CrossTenantAccess answers the empty collection nobody holds
// (IDN-13) and still reaches the instrumentation, which records it (IDN-12).
func findRelated[S any](ctx context.Context, s Service, operation string, input Operation, what string, load func(context.Context) ([]S, error)) ([]S, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, operation)

	if err := s.Authorize(ctx, input); err != nil {
		end(port.AuthorizationResult(err))
		return nil, err
	}

	found, err := load(ctx)
	var access port.CrossTenantAccess
	switch {
	case errors.As(err, &access):
		end(port.Result{Outcome: port.OutcomeFailed, Err: access})
		return []S{}, nil
	case err != nil:
		failed := fmt.Errorf("application: %s: %w", what, err)
		end(port.Result{Outcome: port.OutcomeFailed, Err: failed})
		return nil, failed
	}

	end(port.Result{Outcome: port.OutcomeAccepted})
	return found, nil
}
