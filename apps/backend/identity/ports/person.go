package ports

import (
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type (
	PersonRepository = port.Repository[domain.PersonID, domain.PersonSnapshot]
	PersonReader     = port.Reader[domain.PersonID, domain.PersonSnapshot]
)
