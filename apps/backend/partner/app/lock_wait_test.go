package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
)

type failingWork struct{ err error }

func (w failingWork) Within(context.Context, func(context.Context, application.Resources) error) error {
	return w.err
}

func TestALockWaitPastTheTimeoutIsAnsweredAsAConflict(t *testing.T) {
	lockWait := fmt.Errorf("application: change-partner-stage prt-1: postgres: guarded read: %w", &pgconn.PgError{Code: "55P03"})

	err := app.LockWaitAsConflict(failingWork{err: lockWait}).Within(context.Background(), nil)

	if got := status.Code(kernelgrpc.StatusOf(err)); got != codes.Aborted {
		t.Fatalf("StatusOf(%v) = %s, want Aborted", err, got)
	}
	if !errors.Is(err, lockWait) {
		t.Fatalf("Within() = %v, want the lock wait kept in the chain", err)
	}
}

func TestEveryOtherFailureOfTheUnitOfWorkPassesUntouched(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"none", nil},
		{"inbox register timeout", fmt.Errorf("%w: %w", port.ErrRegisterTimeout, &pgconn.PgError{Code: "55P03"})},
		{"another sqlstate", &pgconn.PgError{Code: "57014"}},
		{"version conflict", port.ErrVersionConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := app.LockWaitAsConflict(failingWork{err: tc.err}).Within(context.Background(), nil); err != tc.err {
				t.Fatalf("Within() = %v, want %v untouched", err, tc.err)
			}
		})
	}
}
