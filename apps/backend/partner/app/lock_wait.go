package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
)

const lockNotAvailable = "55P03"

// lockWaitAsConflict answers a lock wait past the lock_timeout of the session
// as the conflict a deadlock on the guard already is: the series was only
// contended, and the edge answers Aborted, a call to replay, not Internal.
// WORKAROUND(DEVS-157): the kernel rc.7 leaves the 55P03 of a guarded read or a
// Save unclassified; the inbox wraps its own as ErrRegisterTimeout.
type lockWaitAsConflict struct {
	port.UnitOfWork[application.Resources]
}

func (u lockWaitAsConflict) Within(ctx context.Context, fn func(context.Context, application.Resources) error) error {
	err := u.UnitOfWork.Within(ctx, fn)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == lockNotAvailable && !errors.Is(err, port.ErrRegisterTimeout) {
		return usecase.NewFailure(usecase.Conflict, true, err)
	}
	return err
}

func unitOfWork(transactions *pgxpool.Pool, bind func(*postgres.Tx) application.Resources) port.UnitOfWork[application.Resources] {
	return lockWaitAsConflict{postgres.NewUnitOfWork(transactions, bind)}
}
