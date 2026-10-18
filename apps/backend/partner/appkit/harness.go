//go:build integration

package appkit

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

// Tables of this context, which the kit cannot know: it resets the kernel
// tables and the ones named here.
var Tables = []string{"partners", "partner_agreements", "integration_profiles", "operational_policies", "capabilities", "financial_products"}

// PoolOptions is the database this context's suites run in.
var PoolOptions = pg.Options{
	Project:      "partner",
	Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox},
	Schemas:      []string{provider.Schema},
	Tables:       Tables,
	Grants:       provider.Grants,
}

func OpenPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return pg.OpenPool(t, PoolOptions)
}
