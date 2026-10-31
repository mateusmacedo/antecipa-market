//go:build integration

package appkit_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
)

// The attempt ledger is the kernel's table, migrated here because the api
// records the human resolution of an unknown outcome in it.
func TestTheDatabaseHoldsExactlyTheTablesOfThisContext(t *testing.T) {
	got := pg.Tables(t, appkit.OpenPool(t))

	want := []string{
		"capability_profiles", "connectors", "external_attempt", "external_commands", "inbox", "outbox", "quarantine", "query_authorizations",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestTheHarnessResetsEveryTableOfThisContext(t *testing.T) {
	for _, table := range []string{"connectors", "capability_profiles", "query_authorizations", "external_commands"} {
		if !slices.Contains(appkit.Tables, table) {
			t.Fatalf("Tables = %v, want %s reset between cases", appkit.Tables, table)
		}
	}
}
