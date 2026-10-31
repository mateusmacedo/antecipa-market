package app

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

// edgeOperations are the ones a caller reaches through a method of the
// ServiceDesc; each answers for a permission of its own, as in partner and
// identity.
var edgeOperations = []application.Operation{
	application.RegisterConnector{}, application.SuspendOperation{}, application.EnableOperation{},
	application.DeclareCapabilityProfile{},
	application.GrantQueryAuthorization{}, application.RevokeQueryAuthorization{},
	application.CreateExternalCommand{}, application.CancelExternalCommand{}, application.ResolveUnknownOutcome{},
	application.SweepExternalCommands{},
	application.FindConnector{}, application.FindEffectiveCapabilityProfile{}, application.FindCapabilityProfile{},
	application.FindQueryAuthorizationByOrganization{},
	application.FindExternalCommand{}, application.FindExternalCommandsByBusinessKey{},
	application.FindExternalCommandsByStatus{},
}

// internalSteps have no method: the sweep and the consumer run them under the
// execution of their own call, so they answer for the caller's permission.
var internalSteps = map[application.Operation]port.Permission{
	application.RequestResend{}:              "gateway:sweep-external-commands",
	application.ExpireAmbiguity{}:            "gateway:sweep-external-commands",
	application.ConsumeCapabilityEnabled{}:   "gateway:consume",
	application.ConsumeCapabilitySuspended{}: "gateway:consume",
}

func TestEveryEdgeOperationDeclaresItsPermission(t *testing.T) {
	cases := map[string]struct {
		op   application.Operation
		want port.Permission
	}{
		"RegisterConnector": {application.RegisterConnector{}, "gateway:register-connector"},
		"SuspendOperation":  {application.SuspendOperation{}, "gateway:suspend-operation"},
		"EnableOperation":   {application.EnableOperation{}, "gateway:enable-operation"},

		"DeclareCapabilityProfile": {application.DeclareCapabilityProfile{}, "gateway:declare-capability-profile"},

		"GrantQueryAuthorization":  {application.GrantQueryAuthorization{}, "gateway:grant-query-authorization"},
		"RevokeQueryAuthorization": {application.RevokeQueryAuthorization{}, "gateway:revoke-query-authorization"},

		"CreateExternalCommand": {application.CreateExternalCommand{}, "gateway:create-external-command"},
		"CancelExternalCommand": {application.CancelExternalCommand{}, "gateway:cancel-external-command"},
		"ResolveUnknownOutcome": {application.ResolveUnknownOutcome{}, "gateway:resolve-unknown-outcome"},
		"SweepExternalCommands": {application.SweepExternalCommands{}, "gateway:sweep-external-commands"},

		"FindConnector":                  {application.FindConnector{}, "gateway:find-connector"},
		"FindEffectiveCapabilityProfile": {application.FindEffectiveCapabilityProfile{}, "gateway:find-effective-capability-profile"},
		"FindCapabilityProfile":          {application.FindCapabilityProfile{}, "gateway:find-capability-profile"},

		"FindQueryAuthorizationByOrganization": {application.FindQueryAuthorizationByOrganization{}, "gateway:find-query-authorization-by-organization"},

		"FindExternalCommand":               {application.FindExternalCommand{}, "gateway:find-external-command"},
		"FindExternalCommandsByBusinessKey": {application.FindExternalCommandsByBusinessKey{}, "gateway:find-external-commands-by-business-key"},
		"FindExternalCommandsByStatus":      {application.FindExternalCommandsByStatus{}, "gateway:find-external-commands-by-status"},
	}
	if len(cases) != len(edgeOperations) {
		t.Fatalf("%d cases for %d edge operations: an operation lacks its expected permission", len(cases), len(edgeOperations))
	}
	for name, c := range cases {
		if got := permissionOf(c.op); got != c.want {
			t.Errorf("%s: permissionOf() = %q, want %q", name, got, c.want)
		}
	}
}

func TestNoTwoEdgeOperationsShareAPermission(t *testing.T) {
	seen := map[port.Permission]application.Operation{}
	for _, op := range edgeOperations {
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

func TestTheInternalStepsRunUnderThePermissionOfTheirCaller(t *testing.T) {
	for op, want := range internalSteps {
		if got := permissionOf(op); got != want {
			t.Errorf("permissionOf(%T) = %q, want %q", op, got, want)
		}
	}
}

func TestAuthorizationDeniesASubjectWithoutThePermission(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background())

	if err := Authorization()(ctx, application.RegisterConnector{}); !errors.Is(err, port.ErrDenied) {
		t.Fatalf("Authorization() = %v, want ErrDenied", err)
	}
}

func TestAuthorizationAdmitsTheTenantScopedCallTheGRPCHopMounts(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background(), execctx.WithoutSubject(), execctx.WithTenant("acme"))

	every := append([]application.Operation(nil), edgeOperations...)
	for op := range internalSteps {
		every = append(every, op)
	}
	for _, op := range every {
		if err := Authorization()(ctx, op); err != nil {
			t.Fatalf("Authorization(%T) = %v, want nil: the permission is checked at the bff", op, err)
		}
	}
}
