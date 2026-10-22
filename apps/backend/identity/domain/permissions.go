package domain

import "slices"

// Checked by the backend-for-frontend on each route (spec section "Role e
// Permission").
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
	RolePlatformOps: {
		"partner:activate-agreement",
		"partner:change-partner-stage",
		"partner:change-rollout",
		"partner:declare-capability",
		"partner:define-operational-policy",
		"partner:define-product",
		"partner:enable-capability",
		"partner:find-agreements-by-partner",
		"partner:find-capabilities-by-partner",
		"partner:find-certificates-expiring",
		"partner:find-effective-capability",
		"partner:find-eligible-partners",
		"partner:find-integration-profile",
		"partner:find-operational-policy",
		"partner:find-partner",
		"partner:find-partners-by-organization",
		"partner:find-products-by-partner",
		"partner:raise-certificate-expiry-alert",
		"partner:record-homologation",
		"partner:register-agreement",
		"partner:register-integration-profile",
		"partner:register-partner",
		"partner:retire-product",
		"partner:suspend-capability",
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
