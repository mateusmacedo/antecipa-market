package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

const (
	CodeConnectorCredentialRefEmpty kernel.Code = "receivables-gateway/connector/credential-ref-empty"

	CodeCapabilityProfileOperationsEmpty          kernel.Code = "receivables-gateway/capability-profile/operations-empty"
	CodeCapabilityProfileAlreadyDeclared          kernel.Code = "receivables-gateway/capability-profile/already-declared"
	CodeCapabilityProfileExternalProfileInvalid   kernel.Code = "receivables-gateway/capability-profile/external-profile-invalid"
	CodeCapabilityProfileAmbiguityDeadlineMissing kernel.Code = "receivables-gateway/capability-profile/ambiguity-deadline-missing"
	CodeCapabilityProfileQueryIntervalMissing     kernel.Code = "receivables-gateway/capability-profile/query-interval-missing"
	CodeCapabilityProfileMaxSubmissionsMissing    kernel.Code = "receivables-gateway/capability-profile/max-submissions-missing"

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
	CodeExternalCommandNotHoldable                 kernel.Code = "receivables-gateway/external-command/not-holdable"
	CodeExternalCommandObservationConflict         kernel.Code = "receivables-gateway/external-command/observation-conflict"
	CodeExternalCommandObservationSaturated        kernel.Code = "receivables-gateway/external-command/observation-saturated"

	CodeExternalInteractionRawPayloadMissing     kernel.Code = "receivables-gateway/external-interaction/raw-payload-missing"
	CodeExternalInteractionAlreadyConcluded      kernel.Code = "receivables-gateway/external-interaction/already-concluded"
	CodeExternalInteractionMismatchDetailMissing kernel.Code = "receivables-gateway/external-interaction/mismatch-detail-missing"
	CodeExternalInteractionNotConcluded          kernel.Code = "receivables-gateway/external-interaction/not-concluded"
	CodeExternalInteractionJustificationMissing  kernel.Code = "receivables-gateway/external-interaction/justification-missing"
	CodeExternalInteractionPayloadTooLarge       kernel.Code = "receivables-gateway/external-interaction/payload-too-large"

	CodeExternalReferenceReferenceEmpty kernel.Code = "receivables-gateway/external-reference/reference-empty"
	CodeExternalReferenceAlreadyLinked  kernel.Code = "receivables-gateway/external-reference/already-linked"
)

// RejectExternalInteractionPayloadTooLarge is the refusal of a raw body above
// the maxPayloadBytes of the operation: the edge answers it before writing
// anything to the store, so no UPR produces it.
var RejectExternalInteractionPayloadTooLarge = kernel.Reject(CodeExternalInteractionPayloadTooLarge,
	"the raw payload exceeds the maxPayloadBytes of the operation")

// RejectCapabilityProfileAlreadyDeclared is the refusal of a version that
// already exists: the profile key is the identity, so no UPR produces it and
// the use case answers it when the reader finds the key taken.
var RejectCapabilityProfileAlreadyDeclared = kernel.Reject(CodeCapabilityProfileAlreadyDeclared,
	"this version of the capability profile is already declared")

// RejectExternalCommandObservationConflict is the refusal of an observation
// whose terminal finding contradicts the one the kernel's ledger holds for the
// attempt: the ledger decides it, so no UPR produces it.
var RejectExternalCommandObservationConflict = kernel.Reject(CodeExternalCommandObservationConflict,
	"the observation contradicts the terminal finding the ledger holds for the attempt")

// RejectExternalCommandObservationSaturated is the refusal of an observation
// the ledger has no room left to keep for the attempt.
var RejectExternalCommandObservationSaturated = kernel.Reject(CodeExternalCommandObservationSaturated,
	"the ledger keeps no more observations for the attempt")
