package provider

import _ "embed"

// Schema is applied by the composition root through postgres.Migrate: running
// schema statements here would need the driver's pool, which a context provider
// does not hold.
//
//go:embed schema.sql
var Schema string
