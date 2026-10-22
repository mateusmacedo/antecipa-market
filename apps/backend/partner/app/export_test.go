package app

import (
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
)

// Bind and UnitOfWork are the bind and the unit of work of the composition
// root, for the suites that edit the Resources it builds.
var (
	Bind       = bind
	UnitOfWork = unitOfWork
)

// LockWaitAsConflict lays the decorator over a unit of work the suite fakes.
func LockWaitAsConflict(u port.UnitOfWork[application.Resources]) port.UnitOfWork[application.Resources] {
	return lockWaitAsConflict{u}
}
