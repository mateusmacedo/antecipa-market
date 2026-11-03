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
	// RequestResend, ExpireAmbiguity and the state query have no method of
	// their own: the sweep runs them under its execution, so they answer for
	// its permission.
	case application.SweepExternalCommands, application.RequestResend, application.ExpireAmbiguity, application.QueryExternalState:
		return "gateway:sweep-external-commands"
	// HoldDispatch has no method either: the dispatch handler runs it while it
	// consumes the intent. The record, the conclusion and the link a
	// normalization finds are steps of the interaction cycle, run by its
	// consumers, the sweep and the edge of a partner interaction.
	case application.ConsumeCapabilityEnabled, application.ConsumeCapabilitySuspended, application.ConsumeSchemaMismatchDetected, application.HoldDispatch,
		application.RecordExternalInteraction, application.ConcludeInteraction, application.LinkInteractionReference,
		application.DispatchExternalCommand, application.ConsumeResponseCaptured, application.ConsumeInteractionStored:
		return "gateway:consume"
	case application.ReprocessInteraction:
		return "gateway:reprocess-interaction"
	case application.LinkExternalReference:
		return "gateway:link-external-reference"
	case application.ReceivePartnerInteraction:
		return "gateway:receive-partner-interaction"
	case application.ReadExternal:
		return "gateway:read-external"
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
	case application.FindExternalInteraction:
		return "gateway:find-external-interaction"
	case application.FindExternalInteractionByPartnerEvent:
		return "gateway:find-external-interaction-by-partner-event"
	case application.FindExternalInteractionsByCommand:
		return "gateway:find-external-interactions-by-command"
	case application.FindExternalReference:
		return "gateway:find-external-reference"
	case application.FindExternalReferencesBySubject:
		return "gateway:find-external-references-by-subject"
	default:
		return ""
	}
}
