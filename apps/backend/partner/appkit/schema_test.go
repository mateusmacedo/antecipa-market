//go:build integration

package appkit_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
)

func TestTheDatabaseHoldsExactlyTheTablesOfThisContext(t *testing.T) {
	got := pg.Tables(t, appkit.OpenPool(t))

	want := []string{"capabilities", "financial_products", "inbox", "integration_profiles", "operational_policies", "outbox", "partner_agreements", "partners", "quarantine"}
	if !slices.Equal(got, want) {
		t.Fatalf("tables = %v, want %v", got, want)
	}
}

func TestTheHarnessResetsEveryTableOfThisContext(t *testing.T) {
	for _, table := range []string{"partners", "partner_agreements", "integration_profiles", "operational_policies", "capabilities", "financial_products"} {
		if !slices.Contains(appkit.Tables, table) {
			t.Fatalf("Tables = %v, want %s reset between cases", appkit.Tables, table)
		}
	}
}
