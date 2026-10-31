//go:build integration

package provider_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
)

// The attempt ledger is the kernel's, but it lives in this context's database,
// migrated with the ExternalAttempt capability; KIT-04 certifies it there.
func TestTheAttemptLedgerConformsToTheKitInTheContextDatabase(t *testing.T) {
	pool := appkit.OpenPool(t)
	tb.Require(t, providerkit.AttemptLedger(func() providerkit.AttemptLedgerSubject {
		pg.ResetTables(t, pool, "external_attempt")
		return providerkit.AttemptLedgerSubject{Ledger: postgres.NewAttemptStore(pool)}
	}))
}
