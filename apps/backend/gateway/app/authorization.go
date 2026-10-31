package app

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

// Authorization is step 1 of this context: each operation declares the
// permission it requires here, next to its definition (IDN-16).
func Authorization() usecase.Authorize[application.Operation] {
	return usecase.Permitted(permissionOf)
}

// An operation absent from the switch declares none, which Permitted denies
// (IDN-17): adding an operation without a line here closes it, never opens it.
func permissionOf(op application.Operation) port.Permission {
	switch op.(type) {
	case application.RegisterConnector:
		return "gateway:register-connector"
	case application.SuspendOperation:
		return "gateway:suspend-operation"
	case application.EnableOperation:
		return "gateway:enable-operation"
	case application.DeclareCapabilityProfile:
		return "gateway:declare-capability-profile"
	case application.GrantQueryAuthorization:
		return "gateway:grant-query-authorization"
	case application.RevokeQueryAuthorization:
		return "gateway:revoke-query-authorization"
	case application.CreateExternalCommand:
		return "gateway:create-external-command"
	case application.CancelExternalCommand:
		return "gateway:cancel-external-command"
	case application.ResolveUnknownOutcome:
		return "gateway:resolve-unknown-outcome"
	// RequestResend and ExpireAmbiguity have no method of their own: the sweep
	// runs them under its execution, so they answer for its permission.
	case application.SweepExternalCommands, application.RequestResend, application.ExpireAmbiguity:
		return "gateway:sweep-external-commands"
	case application.ConsumeCapabilityEnabled, application.ConsumeCapabilitySuspended:
		return "gateway:consume"
	case application.FindConnector:
		return "gateway:find-connector"
	case application.FindEffectiveCapabilityProfile:
		return "gateway:find-effective-capability-profile"
	case application.FindCapabilityProfile:
		return "gateway:find-capability-profile"
	case application.FindQueryAuthorizationByOrganization:
		return "gateway:find-query-authorization-by-organization"
	case application.FindExternalCommand:
		return "gateway:find-external-command"
	case application.FindExternalCommandsByBusinessKey:
		return "gateway:find-external-commands-by-business-key"
	case application.FindExternalCommandsByStatus:
		return "gateway:find-external-commands-by-status"
	default:
		return ""
	}
}
