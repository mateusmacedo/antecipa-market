package app

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

var everyOperation = []application.Operation{
	application.RegisterConnector{}, application.SuspendOperation{}, application.EnableOperation{},
	application.DeclareCapabilityProfile{},
	application.GrantQueryAuthorization{}, application.RevokeQueryAuthorization{},
	application.CreateExternalCommand{}, application.CancelExternalCommand{}, application.ResolveUnknownOutcome{},
	application.SweepExternalCommands{}, application.RequestResend{}, application.ExpireAmbiguity{},
	application.ConsumeCapabilityEnabled{}, application.ConsumeCapabilitySuspended{},
	application.FindConnector{}, application.FindEffectiveCapabilityProfile{}, application.FindCapabilityProfile{},
	application.FindQueryAuthorizationByOrganization{},
	application.FindExternalCommand{}, application.FindExternalCommandsByBusinessKey{},
	application.FindExternalCommandsByStatus{},
}

func TestEveryOperationDeclaresItsPermission(t *testing.T) {
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
		"RequestResend":         {application.RequestResend{}, "gateway:sweep-external-commands"},
		"ExpireAmbiguity":       {application.ExpireAmbiguity{}, "gateway:sweep-external-commands"},

		"ConsumeCapabilityEnabled":   {application.ConsumeCapabilityEnabled{}, "gateway:consume"},
		"ConsumeCapabilitySuspended": {application.ConsumeCapabilitySuspended{}, "gateway:consume"},

		"FindConnector":                  {application.FindConnector{}, "gateway:read-connector"},
		"FindEffectiveCapabilityProfile": {application.FindEffectiveCapabilityProfile{}, "gateway:read-connector"},
		"FindCapabilityProfile":          {application.FindCapabilityProfile{}, "gateway:read-connector"},

		"FindQueryAuthorizationByOrganization": {application.FindQueryAuthorizationByOrganization{}, "gateway:read-query-authorization"},

		"FindExternalCommand":               {application.FindExternalCommand{}, "gateway:read-external-command"},
		"FindExternalCommandsByBusinessKey": {application.FindExternalCommandsByBusinessKey{}, "gateway:read-external-command"},
		"FindExternalCommandsByStatus":      {application.FindExternalCommandsByStatus{}, "gateway:read-external-command"},
	}
	if len(cases) != len(everyOperation) {
		t.Fatalf("%d cases for %d operations: an operation lacks its expected permission", len(cases), len(everyOperation))
	}
	for name, c := range cases {
		if got := permissionOf(c.op); got != c.want {
			t.Errorf("%s: permissionOf() = %q, want %q", name, got, c.want)
		}
	}
}

func TestEveryOperationDeclaresSomePermission(t *testing.T) {
	for _, op := range everyOperation {
		if permissionOf(op) == "" {
			t.Errorf("%T declares no permission, which Permitted denies to every caller", op)
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

	for _, op := range everyOperation {
		if err := Authorization()(ctx, op); err != nil {
			t.Fatalf("Authorization(%T) = %v, want nil: the permission is checked at the bff", op, err)
		}
	}
}
