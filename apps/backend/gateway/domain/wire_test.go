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
		{string(domain.CausePartner), "partner"},
		{string(domain.CauseSchemaMismatch), "schema_mismatch"},
		{string(domain.AuthorizationGranted), "granted"},
		{string(domain.AuthorizationRevoked), "revoked"},
		{string(domain.EnvironmentProduction), "production"},
		{string(domain.IdempotencyKeyHeader), "key_header"},
		{string(domain.EfficacyByObservation), "by_observation"},
		{string(domain.SourceHuman), "human"},
	}
	for _, v := range values {
		if v.got != v.want {
			t.Errorf("value = %q, want %q", v.got, v.want)
		}
	}
}
