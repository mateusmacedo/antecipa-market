package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

// LockWaitAsConflict lays the decorator over a unit of work the suite fakes.
func LockWaitAsConflict(u port.UnitOfWork[application.Resources]) port.UnitOfWork[application.Resources] {
	return lockWaitAsConflict{u}
}

// UnitOfWork is the one the composition root mounts, over a bind the suite
// edits, so a guarded race runs under the same decorator production does.
func UnitOfWork(pool *pgxpool.Pool, bind func(*postgres.Tx) application.Resources) port.UnitOfWork[application.Resources] {
	return unitOfWork(pool, bind)
}
