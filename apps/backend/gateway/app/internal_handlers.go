package app

import (
	"context"
	"errors"
	"fmt"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	dispatchRequestedEvent = "com.company.gateway.external-command-dispatch-requested.v1"
	responseCapturedEvent  = "com.company.gateway.external-response-captured.v1"
	interactionStoredEvent = "com.company.gateway.external-interaction-stored.v1"
	schemaMismatchEvent    = "com.company.gateway.external-schema-mismatch-detected.v1"
)

var errIncompleteMessage = errors.New("gateway: the internal message lacks a field its handler keys on")

// DispatchHandler hands the dispatch intent of gateway.dispatch to the
// handler that sends the command (decision 14).
func DispatchHandler(service application.Service) kernelapp.Handler {
	return func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (usecase.Disposition, error) {
		var intent eventv1.ExternalCommandDispatchRequested
		if err := unpackInternal(env, dispatchRequestedEvent, &intent); err != nil {
			return usecase.R1D4, err
		}
		if intent.GetIdempotencyKey() == "" {
			return usecase.R1D4, incomplete("idempotency_key")
		}
		return service.DispatchExternalCommand(ctx, application.DispatchExternalCommand{
			Receipt: receipt, IdempotencyKey: domain.IdempotencyKey(intent.GetIdempotencyKey()),
		})
	}
}

// InteractionHandler hands the capture of gateway.interactions to the record
// of the outbound interaction (decision 17).
func InteractionHandler(service application.Service) kernelapp.Handler {
	return func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (usecase.Disposition, error) {
		var captured eventv1.ExternalResponseCaptured
		if err := unpackInternal(env, responseCapturedEvent, &captured); err != nil {
			return usecase.R1D4, err
		}
		capture, err := captureOf(&captured)
		if err != nil {
			return usecase.R1D4, err
		}
		return service.ConsumeResponseCaptured(ctx, application.ConsumeResponseCaptured{Receipt: receipt, Capture: capture})
	}
}

// NormalizationHandler hands a revision of gateway.normalizations to the
// normalization that concludes it (decision 18).
func NormalizationHandler(service application.Service) kernelapp.Handler {
	return func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (usecase.Disposition, error) {
		var stored eventv1.ExternalInteractionStored
		if err := unpackInternal(env, interactionStoredEvent, &stored); err != nil {
			return usecase.R1D4, err
		}
		switch {
		case stored.GetInteractionId() == "":
			return usecase.R1D4, incomplete("interaction_id")
		case stored.GetNormalizationRevision() < 1:
			return usecase.R1D4, incomplete("normalization_revision")
		}
		return service.ConsumeInteractionStored(ctx, application.ConsumeInteractionStored{
			Receipt: receipt, InteractionID: domain.InteractionID(stored.GetInteractionId()), Revision: int(stored.GetNormalizationRevision()),
		})
	}
}

// SchemaMismatchHandler hands a mismatch this context published on
// gateway.events to the suspension of the operation it broke (decision 2). An
// operation the event does not name is refused: suspending the whole connector
// is never what a broken response of one operation asks for.
func SchemaMismatchHandler(service application.Service) kernelapp.Handler {
	return func(ctx context.Context, receipt ports.Receipt, env envelope.Envelope) (usecase.Disposition, error) {
		var detected eventv1.ExternalSchemaMismatchDetected
		if err := unpackInternal(env, schemaMismatchEvent, &detected); err != nil {
			return usecase.R1D4, err
		}
		switch {
		case detected.GetConnectorKey() == "":
			return usecase.R1D4, incomplete("connector_key")
		case detected.GetOperation() == "":
			return usecase.R1D4, incomplete("operation")
		}
		return service.ConsumeSchemaMismatchDetected(ctx, application.ConsumeSchemaMismatchDetected{
			Receipt: receipt, ConnectorKey: domain.ConnectorKey(detected.GetConnectorKey()), Operation: detected.GetOperation(),
		})
	}
}

// unpackInternal refuses a message outside its contract as terminal
// (Validation, ERR-11), before any unit of work opens.
func unpackInternal(env envelope.Envelope, eventType string, into proto.Message) error {
	if env.Type != eventType {
		return usecase.NewFailure(usecase.Validation, false, fmt.Errorf("gateway: event type %q is not consumed here", env.Type))
	}
	if err := envelope.Unpack(env, into); err != nil {
		return usecase.NewFailure(usecase.Validation, false, err)
	}
	return nil
}

func incomplete(field string) error {
	return usecase.NewFailure(usecase.Validation, false, fmt.Errorf("%w: %s", errIncompleteMessage, field))
}

// A step the wire does not know is no step this context applied: the capture
// is refused rather than recorded with a guess.
var stepsFromTheWire = map[eventv1.Step]domain.Step{
	eventv1.Step_STEP_CONFIRM:   domain.StepConfirm,
	eventv1.Step_STEP_AWAIT:     domain.StepAwait,
	eventv1.Step_STEP_RETRY:     domain.StepRetry,
	eventv1.Step_STEP_FAIL:      domain.StepFail,
	eventv1.Step_STEP_QUERY:     domain.StepQuery,
	eventv1.Step_STEP_RECONCILE: domain.StepReconcile,
	eventv1.Step_STEP_SUSPEND:   domain.StepSuspend,
}

var mismatchKindsFromTheWire = map[eventv1.MismatchKind]domain.MismatchKind{
	eventv1.MismatchKind_MISMATCH_KIND_STRUCTURE:     domain.MismatchStructure,
	eventv1.MismatchKind_MISMATCH_KIND_MISSING_FIELD: domain.MismatchMissingField,
	eventv1.MismatchKind_MISMATCH_KIND_TYPE:          domain.MismatchType,
	eventv1.MismatchKind_MISMATCH_KIND_VERSION:       domain.MismatchVersion,
}

func captureOf(m *eventv1.ExternalResponseCaptured) (domain.ExternalResponseCaptured, error) {
	switch {
	case m.GetInteractionId() == "":
		return domain.ExternalResponseCaptured{}, incomplete("interaction_id")
	case m.GetCommandId() == "":
		return domain.ExternalResponseCaptured{}, incomplete("command_id")
	}
	step, known := stepsFromTheWire[m.GetStep()]
	if !known {
		return domain.ExternalResponseCaptured{}, incomplete("step")
	}
	return domain.ExternalResponseCaptured{
		InteractionID: domain.InteractionID(m.GetInteractionId()), CommandID: domain.IdempotencyKey(m.GetCommandId()),
		ConnectorKey: domain.ConnectorKey(m.GetConnectorId()), Operation: m.GetOperation(),
		Request:     domain.RawPayload{Ref: m.GetRequestRef(), VersionID: m.GetRequestVersionId(), Hash: m.GetRequestHash()},
		Response:    domain.RawPayload{Ref: m.GetResponseRef(), VersionID: m.GetResponseVersionId(), Hash: m.GetResponseHash()},
		ContentType: m.GetContentType(), Step: step,
		Mismatch: domain.Mismatch{
			Kind: mismatchKindsFromTheWire[m.GetMismatchKind()], Detail: m.GetMismatchDetail(), PartnerDeclaredVersion: m.GetPartnerDeclaredVersion(),
		},
		At: kernel.Instant(m.GetAt().AsTime().UnixNano()),
	}, nil
}
