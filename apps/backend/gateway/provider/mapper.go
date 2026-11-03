package provider

import (
	"fmt"
	"time"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	dispatchRequestedType = "com.company.gateway.external-command-dispatch-requested.v1"
	submittedType         = "com.company.gateway.external-command-submitted.v1"
	acknowledgedType      = "com.company.gateway.external-command-acknowledged.v1"
	confirmedType         = "com.company.gateway.external-command-confirmed.v1"
	failedType            = "com.company.gateway.external-command-failed.v1"
	outcomeUnknownType    = "com.company.gateway.external-outcome-unknown.v1"
	ambiguityExpiredType  = "com.company.gateway.external-command-ambiguity-expired.v1"
	responseCapturedType  = "com.company.gateway.external-response-captured.v1"
	interactionStoredType = "com.company.gateway.external-interaction-stored.v1"
	schemaMismatchType    = "com.company.gateway.external-schema-mismatch-detected.v1"
	uncorrelatedType      = "com.company.gateway.external-interaction-uncorrelated.v1"
	reprocessedType       = "com.company.gateway.external-interaction-reprocessed.v1"
)

type Mapper struct{}

func (Mapper) Map(event kernel.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case domain.ExternalCommandDispatchRequested:
		return postgres.Mapped{Type: dispatchRequestedType, Message: &eventv1.ExternalCommandDispatchRequested{
			IdempotencyKey: string(e.IdempotencyKey), ConnectorId: string(e.ConnectorKey), Operation: e.Operation, At: timestamp(e.At),
		}}, nil
	case domain.ExternalCommandSubmitted:
		h := e.CommandHeader
		return postgres.Mapped{Type: submittedType, Message: &eventv1.ExternalCommandSubmitted{
			IdempotencyKey: string(h.IdempotencyKey), TenantId: string(h.TenantID), ConnectorId: string(h.ConnectorKey),
			Operation: h.Operation, BusinessKey: h.BusinessKey, Submission: int32(e.Submission), At: timestamp(e.At),
		}}, nil
	case domain.ExternalCommandAcknowledged:
		h := e.CommandHeader
		return postgres.Mapped{Type: acknowledgedType, Message: &eventv1.ExternalCommandAcknowledged{
			IdempotencyKey: string(h.IdempotencyKey), TenantId: string(h.TenantID), ConnectorId: string(h.ConnectorKey),
			Operation: h.Operation, BusinessKey: h.BusinessKey, ExternalReferenceId: string(e.ExternalReferenceID), At: timestamp(e.At),
		}}, nil
	case domain.ExternalCommandConfirmed:
		h := e.CommandHeader
		return postgres.Mapped{Type: confirmedType, Message: &eventv1.ExternalCommandConfirmed{
			IdempotencyKey: string(h.IdempotencyKey), TenantId: string(h.TenantID), ConnectorId: string(h.ConnectorKey),
			Operation: h.Operation, BusinessKey: h.BusinessKey, ExternalReferenceId: string(e.ExternalReferenceID),
			ConfirmationSource: confirmationSource(e.ConfirmationSource), Partial: e.Partial, InteractionId: string(e.InteractionID),
			ConfirmedAt: timestamp(e.ConfirmedAt),
		}}, nil
	case domain.ExternalCommandFailed:
		h := e.CommandHeader
		return postgres.Mapped{Type: failedType, Message: &eventv1.ExternalCommandFailed{
			IdempotencyKey: string(h.IdempotencyKey), TenantId: string(h.TenantID), ConnectorId: string(h.ConnectorKey),
			Operation: h.Operation, BusinessKey: h.BusinessKey, Category: failureCategory(e.Category),
			PartnerCode: e.PartnerCode, PartnerMessage: e.PartnerMessage, InteractionId: string(e.InteractionID), At: timestamp(e.At),
		}}, nil
	case domain.ExternalOutcomeUnknown:
		h := e.CommandHeader
		return postgres.Mapped{Type: outcomeUnknownType, Message: &eventv1.ExternalOutcomeUnknown{
			IdempotencyKey: string(h.IdempotencyKey), TenantId: string(h.TenantID), ConnectorId: string(h.ConnectorKey),
			Operation: h.Operation, BusinessKey: h.BusinessKey, FailureCategory: failureCategory(e.FailureCategory),
			Submissions: int32(e.Submissions), LastSubmittedAt: timestamp(e.LastSubmittedAt), StateQuerySupported: e.StateQuerySupported,
			RequiresHumanResolution: e.RequiresHumanResolution, UnknownSince: timestamp(e.UnknownSince),
		}}, nil
	case domain.ExternalCommandAmbiguityExpired:
		h := e.CommandHeader
		return postgres.Mapped{Type: ambiguityExpiredType, Message: &eventv1.ExternalCommandAmbiguityExpired{
			IdempotencyKey: string(h.IdempotencyKey), TenantId: string(h.TenantID), ConnectorId: string(h.ConnectorKey),
			PartnerId: string(e.PartnerID), Operation: h.Operation, BusinessKey: h.BusinessKey, Status: waitingStatus(e.Status),
			WaitingSince: timestamp(e.WaitingSince), AmbiguityDeadline: int32(e.AmbiguityDeadline),
			StateQuerySupported: e.StateQuerySupported, At: timestamp(e.At),
		}}, nil
	default:
		return mapInteraction(event)
	}
}

func mapInteraction(event kernel.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case domain.ExternalResponseCaptured:
		return postgres.Mapped{Type: responseCapturedType, Message: &eventv1.ExternalResponseCaptured{
			InteractionId: string(e.InteractionID), CommandId: string(e.CommandID), ConnectorId: string(e.ConnectorKey), Operation: e.Operation,
			RequestRef: e.Request.Ref, RequestVersionId: e.Request.VersionID, RequestHash: e.Request.Hash,
			ResponseRef: e.Response.Ref, ResponseVersionId: e.Response.VersionID, ResponseHash: e.Response.Hash, ContentType: e.ContentType,
			Step: step(e.Step), MismatchKind: mismatchKind(e.Mismatch.Kind), MismatchDetail: e.Mismatch.Detail,
			PartnerDeclaredVersion: e.Mismatch.PartnerDeclaredVersion, At: timestamp(e.At),
		}}, nil
	case domain.ExternalInteractionStored:
		return postgres.Mapped{Type: interactionStoredType, Message: &eventv1.ExternalInteractionStored{
			InteractionId: string(e.InteractionID), ConnectorId: string(e.ConnectorKey), Operation: e.Operation,
			Direction: direction(e.Direction), Source: interactionSource(e.Source), CommandId: string(e.CommandID),
			NormalizationRevision: int32(e.NormalizationRevision), At: timestamp(e.At),
		}}, nil
	case domain.ExternalSchemaMismatchDetected:
		return postgres.Mapped{Type: schemaMismatchType, Message: &eventv1.ExternalSchemaMismatchDetected{
			InteractionId: string(e.InteractionID), ConnectorId: string(e.ConnectorKey), ConnectorKey: string(e.ConnectorKey),
			PartnerId: string(e.PartnerID), Environment: environment(e.Environment), Operation: e.Operation, AssetType: e.AssetType,
			CommandId: string(e.CommandID), MismatchKind: mismatchKind(e.Mismatch.Kind), MismatchDetail: e.Mismatch.Detail,
			PartnerDeclaredVersion: e.Mismatch.PartnerDeclaredVersion, At: timestamp(e.At),
		}}, nil
	case domain.ExternalInteractionUncorrelated:
		return postgres.Mapped{Type: uncorrelatedType, Message: &eventv1.ExternalInteractionUncorrelated{
			InteractionId: string(e.InteractionID), ConnectorId: string(e.ConnectorKey), ConnectorKey: string(e.ConnectorKey),
			PartnerId: string(e.PartnerID), Environment: environment(e.Environment), Operation: e.Operation,
			PartnerEventId: e.PartnerEventID, BusinessDate: e.BusinessDate, ReceivedAt: timestamp(e.ReceivedAt), At: timestamp(e.At),
		}}, nil
	case domain.ExternalInteractionReprocessed:
		return postgres.Mapped{Type: reprocessedType, Message: &eventv1.ExternalInteractionReprocessed{
			InteractionId: string(e.InteractionID), ConnectorId: string(e.ConnectorKey), PartnerId: string(e.PartnerID), Operation: e.Operation,
			CommandId: string(e.CommandID), PreviousRevision: int32(e.PreviousRevision), NormalizationRevision: int32(e.NormalizationRevision),
			PreviousOutcome: interactionOutcome(e.PreviousOutcome), Outcome: interactionOutcome(e.Outcome),
			OutcomeChanged: e.OutcomeChanged, TerminalCommandAffected: e.TerminalCommandAffected, At: timestamp(e.At),
		}}, nil
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}

// A zero instant is an absent one, never the Unix epoch.
func timestamp(at kernel.Instant) *timestamppb.Timestamp {
	if at == 0 {
		return nil
	}
	return timestamppb.New(time.Unix(0, int64(at)))
}

func failureCategory(c domain.FailureCategory) eventv1.FailureCategory {
	switch c {
	case domain.CategoryFinalValidation:
		return eventv1.FailureCategory_FAILURE_CATEGORY_FINAL_VALIDATION
	case domain.CategoryFinalBusiness:
		return eventv1.FailureCategory_FAILURE_CATEGORY_FINAL_BUSINESS
	case domain.CategoryFinalAuthorization:
		return eventv1.FailureCategory_FAILURE_CATEGORY_FINAL_AUTHORIZATION
	case domain.CategoryUnknownSent:
		return eventv1.FailureCategory_FAILURE_CATEGORY_UNKNOWN_SENT
	case domain.CategoryUnknownAmbiguous:
		return eventv1.FailureCategory_FAILURE_CATEGORY_UNKNOWN_AMBIGUOUS
	case domain.CategorySchemaMismatch:
		return eventv1.FailureCategory_FAILURE_CATEGORY_SCHEMA_MISMATCH
	case domain.CategoryResendBudgetExhausted:
		return eventv1.FailureCategory_FAILURE_CATEGORY_RESEND_BUDGET_EXHAUSTED
	default:
		return eventv1.FailureCategory_FAILURE_CATEGORY_UNSPECIFIED
	}
}

func confirmationSource(s domain.ConfirmationSource) eventv1.ConfirmationSource {
	switch s {
	case domain.SourceSubmitResponse:
		return eventv1.ConfirmationSource_CONFIRMATION_SOURCE_SUBMIT_RESPONSE
	case domain.SourceStateQuery:
		return eventv1.ConfirmationSource_CONFIRMATION_SOURCE_STATE_QUERY
	case domain.SourceCallback:
		return eventv1.ConfirmationSource_CONFIRMATION_SOURCE_CALLBACK
	case domain.SourceFile:
		return eventv1.ConfirmationSource_CONFIRMATION_SOURCE_FILE
	case domain.SourceHuman:
		return eventv1.ConfirmationSource_CONFIRMATION_SOURCE_HUMAN
	default:
		return eventv1.ConfirmationSource_CONFIRMATION_SOURCE_UNSPECIFIED
	}
}

var (
	environmentsOnTheWire = map[domain.Environment]eventv1.Environment{
		domain.EnvironmentDevelopment:  eventv1.Environment_ENVIRONMENT_DEVELOPMENT,
		domain.EnvironmentIntegration:  eventv1.Environment_ENVIRONMENT_INTEGRATION,
		domain.EnvironmentHomologation: eventv1.Environment_ENVIRONMENT_HOMOLOGATION,
		domain.EnvironmentProduction:   eventv1.Environment_ENVIRONMENT_PRODUCTION,
	}
	mismatchKindsOnTheWire = map[domain.MismatchKind]eventv1.MismatchKind{
		domain.MismatchStructure:    eventv1.MismatchKind_MISMATCH_KIND_STRUCTURE,
		domain.MismatchMissingField: eventv1.MismatchKind_MISMATCH_KIND_MISSING_FIELD,
		domain.MismatchType:         eventv1.MismatchKind_MISMATCH_KIND_TYPE,
		domain.MismatchVersion:      eventv1.MismatchKind_MISMATCH_KIND_VERSION,
	}
	outcomesOnTheWire = map[domain.InteractionStatus]eventv1.InteractionOutcome{
		domain.InteractionNormalized:     eventv1.InteractionOutcome_INTERACTION_OUTCOME_NORMALIZED,
		domain.InteractionUncorrelated:   eventv1.InteractionOutcome_INTERACTION_OUTCOME_UNCORRELATED,
		domain.InteractionSchemaMismatch: eventv1.InteractionOutcome_INTERACTION_OUTCOME_SCHEMA_MISMATCH,
	}
	directionsOnTheWire = map[domain.Direction]eventv1.InteractionDirection{
		domain.DirectionOutbound: eventv1.InteractionDirection_INTERACTION_DIRECTION_OUTBOUND,
		domain.DirectionInbound:  eventv1.InteractionDirection_INTERACTION_DIRECTION_INBOUND,
	}
	sourcesOnTheWire = map[domain.InteractionSource]eventv1.InteractionSource{
		domain.InteractionSubmitResponse: eventv1.InteractionSource_INTERACTION_SOURCE_SUBMIT_RESPONSE,
		domain.InteractionStateQuery:     eventv1.InteractionSource_INTERACTION_SOURCE_STATE_QUERY,
		domain.InteractionCallback:       eventv1.InteractionSource_INTERACTION_SOURCE_CALLBACK,
		domain.InteractionFile:           eventv1.InteractionSource_INTERACTION_SOURCE_FILE,
		domain.InteractionExternalRead:   eventv1.InteractionSource_INTERACTION_SOURCE_EXTERNAL_READ,
	}
	stepsOnTheWire = map[domain.Step]eventv1.Step{
		domain.StepConfirm:   eventv1.Step_STEP_CONFIRM,
		domain.StepAwait:     eventv1.Step_STEP_AWAIT,
		domain.StepRetry:     eventv1.Step_STEP_RETRY,
		domain.StepFail:      eventv1.Step_STEP_FAIL,
		domain.StepQuery:     eventv1.Step_STEP_QUERY,
		domain.StepReconcile: eventv1.Step_STEP_RECONCILE,
		domain.StepSuspend:   eventv1.Step_STEP_SUSPEND,
	}
)

// A value the maps do not know travels as the zero, UNSPECIFIED, never as a guess.
func environment(e domain.Environment) eventv1.Environment { return environmentsOnTheWire[e] }

func mismatchKind(k domain.MismatchKind) eventv1.MismatchKind { return mismatchKindsOnTheWire[k] }

func interactionOutcome(s domain.InteractionStatus) eventv1.InteractionOutcome {
	return outcomesOnTheWire[s]
}

func direction(d domain.Direction) eventv1.InteractionDirection { return directionsOnTheWire[d] }

func interactionSource(s domain.InteractionSource) eventv1.InteractionSource {
	return sourcesOnTheWire[s]
}

func step(s domain.Step) eventv1.Step { return stepsOnTheWire[s] }

func waitingStatus(s domain.CommandStatus) eventv1.WaitingStatus {
	switch s {
	case domain.StatusAcknowledged:
		return eventv1.WaitingStatus_WAITING_STATUS_ACKNOWLEDGED
	case domain.StatusUnknownOutcome:
		return eventv1.WaitingStatus_WAITING_STATUS_UNKNOWN_OUTCOME
	default:
		return eventv1.WaitingStatus_WAITING_STATUS_UNSPECIFIED
	}
}
