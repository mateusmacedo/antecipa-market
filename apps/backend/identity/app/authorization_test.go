package app

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestEveryOperationDeclaresItsPermission(t *testing.T) {
	cases := map[string]struct {
		op   application.Operation
		want port.Permission
	}{
		"RegisterOrganization":              {application.RegisterOrganization{}, "identity:register-organization"},
		"FindOrganization":                  {application.FindOrganization{}, "identity:find-organization"},
		"FindOrganizationByTaxID":           {application.FindOrganizationByTaxID{}, "identity:find-organization-by-tax-id"},
		"AddEstablishment":                  {application.AddEstablishment{}, "identity:add-establishment"},
		"RegisterPerson":                    {application.RegisterPerson{}, "identity:register-person"},
		"RegisterRepresentation":            {application.RegisterRepresentation{}, "identity:register-representation"},
		"FindEstablishmentsByOrganization":  {application.FindEstablishmentsByOrganization{}, "identity:find-establishments-by-organization"},
		"FindPerson":                        {application.FindPerson{}, "identity:find-person"},
		"FindRepresentation":                {application.FindRepresentation{}, "identity:find-representation"},
		"FindRepresentationsByOrganization": {application.FindRepresentationsByOrganization{}, "identity:find-representations-by-organization"},
		"ProvisionUserAccount":              {application.ProvisionUserAccount{}, "identity:provision-user-account"},
		"SuspendUser":                       {application.SuspendUser{}, "identity:suspend-user"},
		"InviteUser":                        {application.InviteUser{}, "identity:invite-user"},
		"GrantMembership":                   {application.GrantMembership{}, "identity:grant-membership"},
		"RevokeMembership":                  {application.RevokeMembership{}, "identity:revoke-membership"},
		"FindMembershipsByUser":             {application.FindMembershipsByUser{}, "identity:find-memberships-by-user"},
		"FindMembershipsByOrganization":     {application.FindMembershipsByOrganization{}, "identity:find-memberships-by-organization"},
		"FindEffectivePermissions":          {application.FindEffectivePermissions{}, "identity:find-effective-permissions"},
		"RequestBankAccountChange":          {application.RequestBankAccountChange{}, "identity:request-bank-account-change"},
		"ApproveBankAccountChange":          {application.ApproveBankAccountChange{}, "identity:approve-bank-account-change"},
		"FindBankAccountChange":             {application.FindBankAccountChange{}, "identity:find-bank-account-change"},
		"FindBankAccountChangesByOrganization": {
			application.FindBankAccountChangesByOrganization{}, "identity:find-bank-account-changes-by-organization",
		},
	}
	for name, c := range cases {
		if got := permissionOf(c.op); got != c.want {
			t.Errorf("%s: permissionOf() = %q, want %q", name, got, c.want)
		}
	}
}

func TestAuthorizationDeniesASubjectWithoutThePermission(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background())

	if err := Authorization()(ctx, application.FindOrganization{}); !errors.Is(err, port.ErrDenied) {
		t.Fatalf("Authorization() = %v, want ErrDenied (IDN-08)", err)
	}
}

func TestAuthorizationAdmitsTheTenantScopedCallTheGRPCHopMounts(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background(), execctx.WithoutSubject(), execctx.WithTenant("acme"))

	for _, op := range everyOperation {
		if err := Authorization()(ctx, op); err != nil {
			t.Fatalf("Authorization(%T) = %v, want nil: subject and permissions stop at the bff (CTX-12)", op, err)
		}
	}
}

var everyOperation = []application.Operation{
	application.RegisterOrganization{}, application.FindOrganization{}, application.FindOrganizationByTaxID{},
	application.AddEstablishment{}, application.RegisterPerson{}, application.RegisterRepresentation{},
	application.FindEstablishmentsByOrganization{}, application.FindPerson{}, application.FindRepresentation{},
	application.FindRepresentationsByOrganization{}, application.ProvisionUserAccount{}, application.SuspendUser{},
	application.InviteUser{}, application.GrantMembership{}, application.RevokeMembership{},
	application.FindMembershipsByUser{}, application.FindMembershipsByOrganization{}, application.FindEffectivePermissions{},
	application.RequestBankAccountChange{}, application.ApproveBankAccountChange{}, application.FindBankAccountChange{},
	application.FindBankAccountChangesByOrganization{},
}

func TestNoTwoOperationsShareAPermission(t *testing.T) {
	seen := map[port.Permission]application.Operation{}
	for _, op := range everyOperation {
		permission := permissionOf(op)
		if permission == "" {
			t.Fatalf("%T declares no permission, which Permitted denies to every caller", op)
		}
		if other, taken := seen[permission]; taken {
			t.Fatalf("%T and %T share %q", op, other, permission)
		}
		seen[permission] = op
	}
}

// The backend-for-frontend gates each route by the permission of the frozen
// table (spec, Role e Permission): one that drifts from it here closes the
// route to every operator.
func TestThePermissionsOfTheBankAccountChangeAreThoseOfTheFrozenTable(t *testing.T) {
	granted := map[port.Permission]bool{}
	for _, p := range domain.RoleOperator.Permissions() {
		granted[port.Permission(p)] = true
	}
	for _, op := range []application.Operation{
		application.RequestBankAccountChange{}, application.FindBankAccountChange{}, application.FindBankAccountChangesByOrganization{},
	} {
		if !granted[permissionOf(op)] {
			t.Errorf("%T declares %q, which the operator of the frozen table does not hold", op, permissionOf(op))
		}
	}
	for _, role := range []domain.Role{domain.RoleOperator, domain.RoleAuditor} {
		for _, p := range role.Permissions() {
			if port.Permission(p) == permissionOf(application.ApproveBankAccountChange{}) {
				t.Errorf("%s holds %q: no role of this version both proposes and approves the change", role, p)
			}
		}
	}
}
