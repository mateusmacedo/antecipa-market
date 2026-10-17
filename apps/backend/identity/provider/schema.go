package provider

import (
	_ "embed"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// Schema is applied by the composition root through postgres.Migrate: running
// schema statements here would need the driver's pool, which a context provider
// does not hold.
//
//go:embed schema.sql
var Schema string

var Grants = []postgres.Grant{
	postgres.Mutable("organizations"),
	postgres.Mutable("establishments"),
	postgres.Mutable("persons"),
	postgres.Mutable("representations"),
	postgres.Mutable("user_accounts"),
	postgres.Mutable("memberships"),
	postgres.Mutable("bank_account_changes"),
}
