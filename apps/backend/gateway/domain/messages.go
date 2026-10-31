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

var (
	_ kernel.DomainEvent = ExternalCommandDispatchRequested{}
	_ kernel.DomainEvent = ExternalCommandSubmitted{}
	_ kernel.DomainEvent = ExternalCommandAcknowledged{}
	_ kernel.DomainEvent = ExternalCommandConfirmed{}
	_ kernel.DomainEvent = ExternalCommandFailed{}
	_ kernel.DomainEvent = ExternalOutcomeUnknown{}
	_ kernel.DomainEvent = ExternalCommandAmbiguityExpired{}
)
