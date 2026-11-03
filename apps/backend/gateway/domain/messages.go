package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type RegisterConnector struct {
	PartnerID          PartnerID
	Environment        Environment
	CredentialRef      string
	IntegrationVersion int
	At                 kernel.Instant
}

type SuspendOperation struct {
	Operation    string
	AssetType    string
	Cause        SuspensionCause
	CapabilityID CapabilityID
	Version      int
	At           kernel.Instant
}

type EnableOperation struct {
	CapabilityID        CapabilityID
	Operation           string
	AssetType           string
	StateQuerySupported bool
	Version             int
	At                  kernel.Instant
}

type DeclareCapabilityProfile struct {
	ConnectorKey    ConnectorKey
	Version         int
	EffectiveFrom   kernel.Instant
	Operations      []OperationCapability
	InvalidProfiles []InvalidProfile
}

type GrantQueryAuthorization struct {
	OrganizationID  OrganizationID
	ConsentVersion  int
	AssetTypes      []string
	PeriodFromDate  string
	PeriodUntilDate string
	At              kernel.Instant
}

type RevokeQueryAuthorization struct {
	EffectiveAt kernel.Instant
}

// CreateExternalCommand carries as facts what the use case read in the unit of
// work: the gate of the connector and the effective profile, and whether the
// series is open to a new version.
type CreateExternalCommand struct {
	TenantID       TenantID
	ConnectorKey   ConnectorKey
	Operation      string
	AssetType      string
	BusinessKey    string
	PayloadVersion int
	Payload        string
	PayloadHash    string
	Profile        CommandProfile
	OutboundKey    string
	CapabilityGate CapabilityGate
	SeriesOpen     bool
	At             kernel.Instant
}

type CancelExternalCommand struct {
	At kernel.Instant
}

type RequestResend struct {
	At kernel.Instant
}

type HoldDispatch struct {
	At kernel.Instant
}

// ExpireAmbiguity carries the partner of the connector, read in the unit of
// work, so the escalation event is self-contained (92 §2).
type ExpireAmbiguity struct {
	PartnerID PartnerID
	At        kernel.Instant
}

// ApplyExternalStep is one step of the kernel's cycle. ClassCategory is the
// category of the class the ledger recorded for the attempt.
type ApplyExternalStep struct {
	Step                Step
	Origin              StepOrigin
	ClassCategory       FailureCategory
	Finding             Finding
	Source              ConfirmationSource
	InteractionID       InteractionID
	ExternalReferenceID ExternalReferenceID
	PartnerCode         string
	PartnerMessage      string
	Partial             bool
	At                  kernel.Instant
}

type Resolution string

const (
	ResolutionConfirmed    Resolution = "confirmed"
	ResolutionFinalFailure Resolution = "final_failure"
	ResolutionNotReceived  Resolution = "not_received"
)

type ResolveUnknownOutcome struct {
	Resolution          Resolution
	QueryInteractionID  InteractionID
	ExternalReferenceID ExternalReferenceID
	PartnerCode         string
	PartnerMessage      string
	Justification       string
	Partial             bool
}

// RecordExternalInteraction carries the references the store returned for the
// raw payload, written before the record; DuplicateOf is set by the caller that
// found the first reception of the same partner event.
type RecordExternalInteraction struct {
	ConnectorKey   ConnectorKey
	Operation      string
	Direction      Direction
	Source         InteractionSource
	ContentType    string
	CommandID      IdempotencyKey
	PartnerEventID string
	BusinessDate   string
	Request        RawPayload
	Response       RawPayload
	CapturedStep   Step
	Mismatch       Mismatch
	DuplicateOf    InteractionID
	At             kernel.Instant
}

// ConcludeInteraction carries the reading of the revision it concludes and, as
// facts read in the unit of work, whether the linked command is terminal and
// what the events take from the connector and the command (92 §2).
type ConcludeInteraction struct {
	Revision        int
	Outcome         InteractionStatus
	CommandID       IdempotencyKey
	ReadingHash     string
	Finding         Finding
	Mismatch        Mismatch
	CommandTerminal bool
	PartnerID       PartnerID
	Environment     Environment
	AssetType       string
	At              kernel.Instant
}

type ReprocessInteraction struct {
	Justification string
	At            kernel.Instant
}

type LinkExternalReference struct {
	ProviderID          PartnerID
	System              ExternalSystem
	ReferenceType       ReferenceType
	Reference           string
	NormalizedReference string
	SubjectType         string
	SubjectID           SubjectID
	AsOf                kernel.Instant
	ObservedVersion     string
}

type ConnectorResponse struct {
	ConnectorKey ConnectorKey
}

type ProfileResponse struct {
	ProfileKey ProfileKey
}

type AuthorizationResponse struct {
	ConsentID ConsentID
}

type CommandResponse struct {
	IdempotencyKey IdempotencyKey
}

type CommandStatusResponse struct {
	IdempotencyKey IdempotencyKey
	Status         CommandStatus
}

type StepApplied struct {
	Status CommandStatus
}

type ResolutionAccepted struct {
	Finding Finding
}

type InteractionResponse struct {
	InteractionID InteractionID
	Status        InteractionStatus
}

type ReprocessResponse struct {
	InteractionID         InteractionID
	NormalizationRevision int
}

type ReferenceResponse struct {
	ReferenceKey ReferenceKey
}

// ExternalCommandDispatchRequested is the dispatch intent, internal to the
// context: the payload does not travel, the handler reads the command by key.
type ExternalCommandDispatchRequested struct {
	IdempotencyKey IdempotencyKey
	ConnectorKey   ConnectorKey
	Operation      string
	At             kernel.Instant
}

func (ExternalCommandDispatchRequested) EventName() string {
	return "gateway.external-command-dispatch-requested"
}

// CommandHeader is what every event of a command carries; the tenant is data,
// not scope, in phase 1 (decision 1).
type CommandHeader struct {
	IdempotencyKey IdempotencyKey
	TenantID       TenantID
	ConnectorKey   ConnectorKey
	Operation      string
	BusinessKey    string
}

type ExternalCommandSubmitted struct {
	CommandHeader
	Submission int
	At         kernel.Instant
}

func (ExternalCommandSubmitted) EventName() string { return "gateway.external-command-submitted" }

type ExternalCommandAcknowledged struct {
	CommandHeader
	ExternalReferenceID ExternalReferenceID
	At                  kernel.Instant
}

func (ExternalCommandAcknowledged) EventName() string { return "gateway.external-command-acknowledged" }

type ExternalCommandConfirmed struct {
	CommandHeader
	ExternalReferenceID ExternalReferenceID
	ConfirmationSource  ConfirmationSource
	Partial             bool
	InteractionID       InteractionID
	ConfirmedAt         kernel.Instant
}

func (ExternalCommandConfirmed) EventName() string { return "gateway.external-command-confirmed" }

type ExternalCommandFailed struct {
	CommandHeader
	Category       FailureCategory
	PartnerCode    string
	PartnerMessage string
	InteractionID  InteractionID
	At             kernel.Instant
}

func (ExternalCommandFailed) EventName() string { return "gateway.external-command-failed" }

type ExternalOutcomeUnknown struct {
	CommandHeader
	FailureCategory         FailureCategory
	Submissions             int
	LastSubmittedAt         kernel.Instant
	StateQuerySupported     bool
	RequiresHumanResolution bool
	UnknownSince            kernel.Instant
}

func (ExternalOutcomeUnknown) EventName() string { return "gateway.external-outcome-unknown" }

type ExternalCommandAmbiguityExpired struct {
	CommandHeader
	PartnerID           PartnerID
	Status              CommandStatus
	WaitingSince        kernel.Instant
	AmbiguityDeadline   int
	StateQuerySupported bool
	At                  kernel.Instant
}

func (ExternalCommandAmbiguityExpired) EventName() string {
	return "gateway.external-command-ambiguity-expired"
}

// ExternalResponseCaptured is the capture of the response of a submission,
// internal to the context: the consumer records the outbound interaction from
// it, with the step the dispatch applied.
type ExternalResponseCaptured struct {
	InteractionID InteractionID
	CommandID     IdempotencyKey
	ConnectorKey  ConnectorKey
	Operation     string
	Request       RawPayload
	Response      RawPayload
	Step          Step
	Mismatch      Mismatch
	At            kernel.Instant
}

func (ExternalResponseCaptured) EventName() string { return "gateway.external-response-captured" }

// ExternalInteractionStored hands a revision of an interaction to the
// normalization, internal to the context: the raw payload does not travel.
type ExternalInteractionStored struct {
	InteractionID         InteractionID
	ConnectorKey          ConnectorKey
	Operation             string
	Direction             Direction
	Source                InteractionSource
	CommandID             IdempotencyKey
	NormalizationRevision int
	At                    kernel.Instant
}

func (ExternalInteractionStored) EventName() string { return "gateway.external-interaction-stored" }

// ExternalSchemaMismatchDetected carries the partner and the environment of the
// connector, so the Partner and the Backoffice act on it alone (92 §2).
type ExternalSchemaMismatchDetected struct {
	InteractionID InteractionID
	ConnectorKey  ConnectorKey
	PartnerID     PartnerID
	Environment   Environment
	Operation     string
	AssetType     string
	CommandID     IdempotencyKey
	Mismatch      Mismatch
	At            kernel.Instant
}

func (ExternalSchemaMismatchDetected) EventName() string {
	return "gateway.external-schema-mismatch-detected"
}

type ExternalInteractionUncorrelated struct {
	InteractionID  InteractionID
	ConnectorKey   ConnectorKey
	PartnerID      PartnerID
	Environment    Environment
	Operation      string
	PartnerEventID string
	BusinessDate   string
	ReceivedAt     kernel.Instant
	At             kernel.Instant
}

func (ExternalInteractionUncorrelated) EventName() string {
	return "gateway.external-interaction-uncorrelated"
}

type ExternalInteractionReprocessed struct {
	InteractionID           InteractionID
	ConnectorKey            ConnectorKey
	PartnerID               PartnerID
	Operation               string
	CommandID               IdempotencyKey
	PreviousRevision        int
	NormalizationRevision   int
	PreviousOutcome         InteractionStatus
	Outcome                 InteractionStatus
	OutcomeChanged          bool
	TerminalCommandAffected bool
	At                      kernel.Instant
}

func (ExternalInteractionReprocessed) EventName() string {
	return "gateway.external-interaction-reprocessed"
}

var (
	_ kernel.DomainEvent = ExternalCommandDispatchRequested{}
	_ kernel.DomainEvent = ExternalCommandSubmitted{}
	_ kernel.DomainEvent = ExternalCommandAcknowledged{}
	_ kernel.DomainEvent = ExternalCommandConfirmed{}
	_ kernel.DomainEvent = ExternalCommandFailed{}
	_ kernel.DomainEvent = ExternalOutcomeUnknown{}
	_ kernel.DomainEvent = ExternalCommandAmbiguityExpired{}
	_ kernel.DomainEvent = ExternalResponseCaptured{}
	_ kernel.DomainEvent = ExternalInteractionStored{}
	_ kernel.DomainEvent = ExternalSchemaMismatchDetected{}
	_ kernel.DomainEvent = ExternalInteractionUncorrelated{}
	_ kernel.DomainEvent = ExternalInteractionReprocessed{}
)
