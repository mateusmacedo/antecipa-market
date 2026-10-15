package ports

import (
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

type (
	UserAccountRepository = port.Repository[domain.UserID, domain.UserAccountSnapshot]
	UserAccountReader     = port.Reader[domain.UserID, domain.UserAccountSnapshot]
)
