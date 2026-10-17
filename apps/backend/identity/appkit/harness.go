//go:build integration

package appkit

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

// Tables of this context, which the kit cannot know: it resets the kernel
// tables and the ones named here.
var Tables = []string{"organizations", "establishments", "persons", "representations", "user_accounts", "memberships", "bank_account_changes"}

// PoolOptions is the database this context's suites run in.
var PoolOptions = pg.Options{
	Project:      "identity",
	Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox},
	Schemas:      []string{provider.Schema},
	Tables:       Tables,
	Grants:       provider.Grants,
}

func OpenPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return pg.OpenPool(t, PoolOptions)
}
