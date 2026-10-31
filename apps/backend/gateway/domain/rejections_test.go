package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// The codes are published on the wire: a rename of the Go constant must never
// change the value a caller matches on.
func TestRejectionCodesKeepTheirWireValues(t *testing.T) {
	codes := map[kernel.Code]string{
		domain.CodeConnectorCredentialRefEmpty:                "receivables-gateway/connector/credential-ref-empty",
		domain.CodeCapabilityProfileOperationsEmpty:           "receivables-gateway/capability-profile/operations-empty",
		domain.CodeCapabilityProfileAlreadyDeclared:           "receivables-gateway/capability-profile/already-declared",
		domain.CodeCapabilityProfileExternalProfileInvalid:    "receivables-gateway/capability-profile/external-profile-invalid",
		domain.CodeCapabilityProfileAmbiguityDeadlineMissing:  "receivables-gateway/capability-profile/ambiguity-deadline-missing",
		domain.CodeCapabilityProfileQueryIntervalMissing:      "receivables-gateway/capability-profile/query-interval-missing",
		domain.CodeExternalCommandCapabilityNotEnabled:        "receivables-gateway/external-command/capability-not-enabled",
		domain.CodeExternalCommandOperationSuspended:          "receivables-gateway/external-command/operation-suspended",
		domain.CodeExternalCommandBusinessKeyEmpty:            "receivables-gateway/external-command/business-key-empty",
		domain.CodeExternalCommandPayloadEmpty:                "receivables-gateway/external-command/payload-empty",
		domain.CodeExternalCommandIdempotencyKeyConflict:      "receivables-gateway/external-command/idempotency-key-conflict",
		domain.CodeExternalCommandContentChangedAfterSend:     "receivables-gateway/external-command/content-changed-after-send",
		domain.CodeExternalCommandNotInFlight:                 "receivables-gateway/external-command/not-in-flight",
		domain.CodeExternalCommandNotResendable:               "receivables-gateway/external-command/not-resendable",
		domain.CodeExternalCommandQuiescenceNotElapsed:        "receivables-gateway/external-command/quiescence-not-elapsed",
		domain.CodeExternalCommandNotWaiting:                  "receivables-gateway/external-command/not-waiting",
		domain.CodeExternalCommandAmbiguityDeadlineNotReached: "receivables-gateway/external-command/ambiguity-deadline-not-reached",
		domain.CodeExternalCommandJustificationMissing:        "receivables-gateway/external-command/justification-missing",
		domain.CodeExternalCommandQueryEvidenceMissing:        "receivables-gateway/external-command/query-evidence-missing",
		domain.CodeExternalCommandPartialResultNotSupported:   "receivables-gateway/external-command/partial-result-not-supported",
		domain.CodeExternalCommandAlreadySent:                 "receivables-gateway/external-command/already-sent",
	}
	for code, wire := range codes {
		if string(code) != wire {
			t.Errorf("code = %q, want %q", code, wire)
		}
	}
}

func TestAlreadyDeclaredIsTheStaticRejectionOfTheProfileIdentity(t *testing.T) {
	if got := domain.RejectCapabilityProfileAlreadyDeclared.Code(); got != domain.CodeCapabilityProfileAlreadyDeclared {
		t.Fatalf("Code() = %q, want %q", got, domain.CodeCapabilityProfileAlreadyDeclared)
	}
}
