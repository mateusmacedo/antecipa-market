package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// Event names, statuses and categories are written to the outbox and the
// snapshot: a rename in Go must leave every stored and published value as is.
func TestWireValuesStayFixed(t *testing.T) {
	events := map[string]kernel.DomainEvent{
		"gateway.external-command-dispatch-requested": domain.ExternalCommandDispatchRequested{},
		"gateway.external-command-submitted":          domain.ExternalCommandSubmitted{},
		"gateway.external-command-acknowledged":       domain.ExternalCommandAcknowledged{},
		"gateway.external-command-confirmed":          domain.ExternalCommandConfirmed{},
		"gateway.external-command-failed":             domain.ExternalCommandFailed{},
		"gateway.external-outcome-unknown":            domain.ExternalOutcomeUnknown{},
		"gateway.external-command-ambiguity-expired":  domain.ExternalCommandAmbiguityExpired{},
		"gateway.external-response-captured":          domain.ExternalResponseCaptured{},
		"gateway.external-interaction-stored":         domain.ExternalInteractionStored{},
		"gateway.external-schema-mismatch-detected":   domain.ExternalSchemaMismatchDetected{},
		"gateway.external-interaction-uncorrelated":   domain.ExternalInteractionUncorrelated{},
		"gateway.external-interaction-reprocessed":    domain.ExternalInteractionReprocessed{},
	}
	for name, event := range events {
		if event.EventName() != name {
			t.Errorf("EventName() = %q, want %q", event.EventName(), name)
		}
	}

	values := []struct{ got, want string }{
		{string(domain.StatusCreated), "created"},
		{string(domain.StatusAcknowledged), "acknowledged"},
		{string(domain.StatusConfirmed), "confirmed"},
		{string(domain.StatusRetryableFailure), "retryable_failure"},
		{string(domain.StatusFinalFailure), "final_failure"},
		{string(domain.StatusUnknownOutcome), "unknown_outcome"},
		{string(domain.StatusCancelled), "cancelled"},
		{string(domain.CategoryFinalValidation), "final_validation"},
		{string(domain.CategoryFinalBusiness), "final_business"},
		{string(domain.CategoryFinalAuthorization), "final_authorization"},
		{string(domain.CategoryUnknownSent), "unknown_sent"},
		{string(domain.CategoryUnknownAmbiguous), "unknown_ambiguous"},
		{string(domain.CategorySchemaMismatch), "schema_mismatch"},
		{string(domain.CategoryResendBudgetExhausted), "resend_budget_exhausted"},
		{string(domain.CausePartner), "partner"},
		{string(domain.CauseSchemaMismatch), "schema_mismatch"},
		{string(domain.AuthorizationGranted), "granted"},
		{string(domain.AuthorizationRevoked), "revoked"},
		{string(domain.EnvironmentProduction), "production"},
		{string(domain.IdempotencyKeyHeader), "key_header"},
		{string(domain.EfficacyByObservation), "by_observation"},
		{string(domain.SourceHuman), "human"},
		{string(domain.DirectionOutbound), "outbound"},
		{string(domain.DirectionInbound), "inbound"},
		{string(domain.InteractionSubmitResponse), "submit_response"},
		{string(domain.InteractionStateQuery), "state_query"},
		{string(domain.InteractionCallback), "callback"},
		{string(domain.InteractionFile), "file"},
		{string(domain.InteractionExternalRead), "external_read"},
		{string(domain.InteractionStored), "stored"},
		{string(domain.InteractionDuplicate), "duplicate"},
		{string(domain.InteractionNormalized), "normalized"},
		{string(domain.InteractionUncorrelated), "uncorrelated"},
		{string(domain.InteractionSchemaMismatch), "schema_mismatch"},
		{string(domain.MismatchStructure), "structure"},
		{string(domain.MismatchMissingField), "missing_field"},
		{string(domain.MismatchType), "type"},
		{string(domain.MismatchVersion), "version"},
		{string(domain.FindingApplied), "applied"},
		{string(domain.FindingRejected), "rejected"},
		{string(domain.FindingAbsent), "absent"},
		{string(domain.FindingPending), "pending"},
		{string(domain.FindingInconclusive), "inconclusive"},
		{string(domain.SystemRegistry), "registry"},
		{string(domain.SystemBookkeeper), "bookkeeper"},
		{string(domain.SystemAcquirer), "acquirer"},
		{string(domain.SystemFunder), "funder"},
		{string(domain.SystemDepository), "depository"},
		{string(domain.ReferenceAsset), "asset"},
		{string(domain.ReferenceEffect), "effect"},
		{string(domain.ReferenceAuthorization), "authorization"},
		{string(domain.ReferenceCommand), "command"},
		{string(domain.ReferenceSettlement), "settlement"},
	}
	for _, v := range values {
		if v.got != v.want {
			t.Errorf("value = %q, want %q", v.got, v.want)
		}
	}
}
