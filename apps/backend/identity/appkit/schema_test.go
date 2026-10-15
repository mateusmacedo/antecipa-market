//go:build integration

package appkit_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
)

func TestTheDatabaseHoldsExactlyTheTablesOfThisContext(t *testing.T) {
	got := pg.Tables(t, appkit.OpenPool(t))

	want := []string{"bank_account_changes", "establishments", "inbox", "memberships", "organizations", "outbox", "persons", "quarantine", "representations", "user_accounts"}
	if !slices.Equal(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestTheHarnessResetsEveryTableOfThisContext(t *testing.T) {
	for _, table := range []string{"organizations", "establishments", "persons", "representations", "user_accounts", "memberships", "bank_account_changes"} {
		if !slices.Contains(appkit.Tables, table) {
			t.Fatalf("Tables = %v, want %s reset between cases", appkit.Tables, table)
		}
	}
}
