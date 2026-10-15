package app

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
)

// Authorization is step 1 of this context. Over gRPC the interceptor mounts no
// subject, so Permitted admits by the tenant alone: the bff checks the
// permission against FindEffectivePermissions before the hop (CTX-12, IDN-10).
func Authorization() usecase.Authorize[application.Operation] {
	return usecase.Permitted(permissionOf)
}

// An operation absent from the switch declares none, which Permitted denies
// (IDN-17): adding an operation without a line here closes it, never opens it.
func permissionOf(op application.Operation) port.Permission {
	switch op.(type) {
	case application.RegisterOrganization:
		return "identity:register-organization"
	case application.FindOrganization:
		return "identity:find-organization"
	case application.FindOrganizationByTaxID:
		return "identity:find-organization-by-tax-id"
	case application.AddEstablishment:
		return "identity:add-establishment"
	case application.RegisterPerson:
		return "identity:register-person"
	case application.RegisterRepresentation:
		return "identity:register-representation"
	case application.FindEstablishmentsByOrganization:
		return "identity:find-establishments-by-organization"
	case application.FindPerson:
		return "identity:find-person"
	case application.FindRepresentation:
		return "identity:find-representation"
	case application.FindRepresentationsByOrganization:
		return "identity:find-representations-by-organization"
	case application.ProvisionUserAccount:
		return "identity:provision-user-account"
	case application.SuspendUser:
		return "identity:suspend-user"
	case application.InviteUser:
		return "identity:invite-user"
	case application.GrantMembership:
		return "identity:grant-membership"
	case application.RevokeMembership:
		return "identity:revoke-membership"
	case application.FindMembershipsByUser:
		return "identity:find-memberships-by-user"
	case application.FindMembershipsByOrganization:
		return "identity:find-memberships-by-organization"
	case application.FindEffectivePermissions:
		return "identity:find-effective-permissions"
	case application.RequestBankAccountChange:
		return "identity:request-bank-account-change"
	case application.ApproveBankAccountChange:
		return "identity:approve-bank-account-change"
	case application.FindBankAccountChange:
		return "identity:find-bank-account-change"
	case application.FindBankAccountChangesByOrganization:
		return "identity:find-bank-account-changes-by-organization"
	default:
		return ""
	}
}
