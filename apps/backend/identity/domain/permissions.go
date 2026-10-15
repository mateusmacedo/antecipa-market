package domain

import "slices"

// Checked by the bff on each route (spec section "Role e Permission").
type Permission string

var rolePermissions = map[Role][]Permission{
	RoleOperator: {
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
	},
	RoleAuditor: {
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
	},
}

func (r Role) Permissions() []Permission {
	return slices.Clone(rolePermissions[r])
}

func EffectivePermissions(roles ...Role) []Permission {
	union := []Permission{}
	for _, role := range roles {
		union = append(union, rolePermissions[role]...)
	}
	slices.Sort(union)
	return slices.Compact(union)
}
