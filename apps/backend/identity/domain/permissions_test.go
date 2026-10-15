package domain_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

var operatorPermissions = []domain.Permission{
	"eligibility:request-eligibility-evaluation",
	"identity:find-bank-account-change",
	"identity:find-bank-account-changes-by-organization",
	"identity:request-bank-account-change",
	"kyb:find-business-profile",
	"kyb:find-disclosures-by-case",
	"kyb:find-ownership-structure",
	"kyb:find-verification-case",
	"kyb:find-verification-cases-by-organization",
	"marketplace:open-funding-request",
	"portfolio:find-availability-by-snapshot",
	"portfolio:find-latest-published-snapshot",
	"portfolio:find-portfolio",
	"portfolio:find-snapshots-by-portfolio",
	"portfolio:group-assets-by-snapshot",
	"portfolio:list-assets-by-snapshot",
	"portfolio:request-discovery",
}

var auditorPermissions = []domain.Permission{
	"kyb:find-business-profile",
	"kyb:find-disclosures-by-case",
	"kyb:find-ownership-structure",
	"kyb:find-verification-case",
	"kyb:find-verification-cases-by-organization",
	"ledger:access-evidence-bundle",
	"ledger:find-evidence-bundle",
	"ledger:request-evidence-bundle",
	"portfolio:find-availability-by-snapshot",
	"portfolio:find-latest-published-snapshot",
	"portfolio:find-portfolio",
	"portfolio:find-snapshots-by-portfolio",
	"portfolio:group-assets-by-snapshot",
	"portfolio:list-assets-by-snapshot",
}

// The catalogue is what the bff checks on every route: a permission renamed or
// dropped here reaches the wire, so the frozen table of the spec is pinned.
func TestTheRolesKeepTheFrozenPermissionsOfTheSpec(t *testing.T) {
	for _, tc := range []struct {
		role  domain.Role
		want  []domain.Permission
		count int
	}{
		{domain.RoleOperator, operatorPermissions, 17},
		{domain.RoleAuditor, auditorPermissions, 14},
	} {
		got := tc.role.Permissions()
		if len(got) != tc.count || !slices.Equal(got, tc.want) {
			t.Errorf("%s.Permissions() = %v (%d), want %v (%d)", tc.role, got, len(got), tc.want, tc.count)
		}
		if !slices.IsSorted(got) {
			t.Errorf("%s.Permissions() = %v, want ascending lexicographic order", tc.role, got)
		}
	}
}

func TestARoleOutsideTheCatalogueHasNoPermission(t *testing.T) {
	for _, role := range []domain.Role{"", "approver", "org_admin", "platform_ops"} {
		if got := role.Permissions(); len(got) != 0 {
			t.Errorf("%q.Permissions() = %v, want none", role, got)
		}
	}
}

func TestPermissionsAnswersACopyOfTheCatalogue(t *testing.T) {
	domain.RoleOperator.Permissions()[0] = "changed:by-the-caller"

	if got := domain.RoleOperator.Permissions(); !slices.Equal(got, operatorPermissions) {
		t.Fatalf("Permissions() = %v: a slice held outside reached the catalogue", got)
	}
}

func TestEffectivePermissionsIsTheSortedUnionWithoutRepetition(t *testing.T) {
	union := []domain.Permission{
		"eligibility:request-eligibility-evaluation",
		"identity:find-bank-account-change",
		"identity:find-bank-account-changes-by-organization",
		"identity:request-bank-account-change",
		"kyb:find-business-profile",
		"kyb:find-disclosures-by-case",
		"kyb:find-ownership-structure",
		"kyb:find-verification-case",
		"kyb:find-verification-cases-by-organization",
		"ledger:access-evidence-bundle",
		"ledger:find-evidence-bundle",
		"ledger:request-evidence-bundle",
		"marketplace:open-funding-request",
		"portfolio:find-availability-by-snapshot",
		"portfolio:find-latest-published-snapshot",
		"portfolio:find-portfolio",
		"portfolio:find-snapshots-by-portfolio",
		"portfolio:group-assets-by-snapshot",
		"portfolio:list-assets-by-snapshot",
		"portfolio:request-discovery",
	}
	for _, tc := range []struct {
		name  string
		roles []domain.Role
		want  []domain.Permission
	}{
		{"operator", []domain.Role{domain.RoleOperator}, operatorPermissions},
		{"auditor", []domain.Role{domain.RoleAuditor}, auditorPermissions},
		{"operator and auditor", []domain.Role{domain.RoleOperator, domain.RoleAuditor}, union},
		{"auditor and operator", []domain.Role{domain.RoleAuditor, domain.RoleOperator}, union},
		{"the same role twice", []domain.Role{domain.RoleAuditor, domain.RoleAuditor}, auditorPermissions},
		{"a role outside the catalogue", []domain.Role{"approver", domain.RoleAuditor}, auditorPermissions},
		{"no role", nil, []domain.Permission{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.EffectivePermissions(tc.roles...)

			if got == nil || !slices.Equal(got, tc.want) {
				t.Fatalf("EffectivePermissions(%v) = %v, want %v", tc.roles, got, tc.want)
			}
		})
	}
	if got := len(union); got != 20 {
		t.Fatalf("the union of operator and auditor has %d permissions, want 20", got)
	}
}

func TestEffectivePermissionsLeavesTheCatalogueUntouched(t *testing.T) {
	domain.EffectivePermissions(domain.RoleOperator, domain.RoleAuditor)[0] = "changed:by-the-caller"

	if got := domain.RoleAuditor.Permissions(); !slices.Equal(got, auditorPermissions) {
		t.Fatalf("Permissions() = %v: the union wrote into the catalogue", got)
	}
}
