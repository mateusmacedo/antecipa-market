package provider

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

func NewPersonReader(pool postgres.ReadPool) ports.PersonReader {
	return personTable.Reader(pool)
}
