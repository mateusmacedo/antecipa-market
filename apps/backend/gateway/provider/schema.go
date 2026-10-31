package provider

import (
	_ "embed"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// Schema is applied by the composition root through postgres.Migrate: running
// DDL here would need the driver's pool, which a context provider does not hold.
//
//go:embed schema.sql
var Schema string

var Grants = []postgres.Grant{
	postgres.Mutable("connectors"),
	postgres.Mutable("capability_profiles"),
	postgres.Mutable("query_authorizations"),
	postgres.Mutable("external_commands"),
}

// Uniques is empty because every uniqueness the spec declares in this slice is
// an identity, which the primary key answers as Conflict (ADR-066).
var Uniques = []postgres.UniqueSet{}
