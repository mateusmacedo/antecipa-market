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
