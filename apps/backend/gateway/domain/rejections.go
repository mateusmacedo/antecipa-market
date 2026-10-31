package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

const (
	CodeConnectorCredentialRefEmpty kernel.Code = "receivables-gateway/connector/credential-ref-empty"

	CodeCapabilityProfileOperationsEmpty          kernel.Code = "receivables-gateway/capability-profile/operations-empty"
	CodeCapabilityProfileAlreadyDeclared          kernel.Code = "receivables-gateway/capability-profile/already-declared"
	CodeCapabilityProfileExternalProfileInvalid   kernel.Code = "receivables-gateway/capability-profile/external-profile-invalid"
	CodeCapabilityProfileAmbiguityDeadlineMissing kernel.Code = "receivables-gateway/capability-profile/ambiguity-deadline-missing"
	CodeCapabilityProfileQueryIntervalMissing     kernel.Code = "receivables-gateway/capability-profile/query-interval-missing"

	CodeExternalCommandCapabilityNotEnabled        kernel.Code = "receivables-gateway/external-command/capability-not-enabled"
	CodeExternalCommandOperationSuspended          kernel.Code = "receivables-gateway/external-command/operation-suspended"
	CodeExternalCommandBusinessKeyEmpty            kernel.Code = "receivables-gateway/external-command/business-key-empty"
	CodeExternalCommandPayloadEmpty                kernel.Code = "receivables-gateway/external-command/payload-empty"
	CodeExternalCommandIdempotencyKeyConflict      kernel.Code = "receivables-gateway/external-command/idempotency-key-conflict"
	CodeExternalCommandContentChangedAfterSend     kernel.Code = "receivables-gateway/external-command/content-changed-after-send"
	CodeExternalCommandNotInFlight                 kernel.Code = "receivables-gateway/external-command/not-in-flight"
	CodeExternalCommandNotResendable               kernel.Code = "receivables-gateway/external-command/not-resendable"
	CodeExternalCommandQuiescenceNotElapsed        kernel.Code = "receivables-gateway/external-command/quiescence-not-elapsed"
	CodeExternalCommandNotWaiting                  kernel.Code = "receivables-gateway/external-command/not-waiting"
	CodeExternalCommandAmbiguityDeadlineNotReached kernel.Code = "receivables-gateway/external-command/ambiguity-deadline-not-reached"
	CodeExternalCommandJustificationMissing        kernel.Code = "receivables-gateway/external-command/justification-missing"
	CodeExternalCommandQueryEvidenceMissing        kernel.Code = "receivables-gateway/external-command/query-evidence-missing"
	CodeExternalCommandPartialResultNotSupported   kernel.Code = "receivables-gateway/external-command/partial-result-not-supported"
	CodeExternalCommandAlreadySent                 kernel.Code = "receivables-gateway/external-command/already-sent"
)

// RejectCapabilityProfileAlreadyDeclared is the refusal of a version that
// already exists: the profile key is the identity, so no UPR produces it and
// the use case answers it when the reader finds the key taken.
var RejectCapabilityProfileAlreadyDeclared = kernel.Reject(CodeCapabilityProfileAlreadyDeclared,
	"this version of the capability profile is already declared")
