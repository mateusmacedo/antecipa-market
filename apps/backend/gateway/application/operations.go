package application

import (
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// Operation is what the authorization hook decides on: a query is an entry
// point like a command, and authenticating at the edge does not stand for
// permission to read what it returns.
type Operation interface{ isOperation() }

type RegisterConnector struct {
	PartnerID          domain.PartnerID
	Environment        domain.Environment
	CredentialRef      string
	IntegrationVersion int
}

type SuspendOperation struct {
	ConnectorKey domain.ConnectorKey
	Operation    string
	AssetType    string
	Cause        domain.SuspensionCause
	CapabilityID domain.CapabilityID
	Version      int
}

type EnableOperation struct {
	ConnectorKey        domain.ConnectorKey
	CapabilityID        domain.CapabilityID
	Operation           string
	AssetType           string
	StateQuerySupported bool
	Version             int
}

type DeclareCapabilityProfile struct {
	ConnectorKey  domain.ConnectorKey
	Version       int
	EffectiveFrom kernel.Instant
	Operations    []domain.OperationCapability
}

type GrantQueryAuthorization struct {
	ConsentID       domain.ConsentID
	OrganizationID  domain.OrganizationID
	ConsentVersion  int
	AssetTypes      []string
	PeriodFromDate  string
	PeriodUntilDate string
}

type RevokeQueryAuthorization struct {
	ConsentID   domain.ConsentID
	EffectiveAt kernel.Instant
}

type CreateExternalCommand struct {
	IdempotencyKey domain.IdempotencyKey
	TenantID       domain.TenantID
	ConnectorKey   domain.ConnectorKey
	Operation      string
	AssetType      string
	BusinessKey    string
	PayloadVersion int
	Payload        string
	PayloadHash    string
}

type CancelExternalCommand struct {
	IdempotencyKey domain.IdempotencyKey
}

type RequestResend struct {
	IdempotencyKey domain.IdempotencyKey
}

// HoldDispatch names the message of the dispatch intent it holds: the command
// key derives from it, so a redelivery of the same intent replays.
type HoldDispatch struct {
	IdempotencyKey domain.IdempotencyKey
	MessageID      port.MessageID
}

type ExpireAmbiguity struct {
	IdempotencyKey domain.IdempotencyKey
}

type ResolveUnknownOutcome struct {
	IdempotencyKey      domain.IdempotencyKey
	Resolution          domain.Resolution
	QueryInteractionID  domain.InteractionID
	ExternalReferenceID domain.ExternalReferenceID
	PartnerCode         string
	PartnerMessage      string
	Justification       string
	Partial             bool
}

type SweepExternalCommands struct{}

// QueryExternalState is the state query the sweep makes of a command due at
// NextQueryAt: the record of its interaction runs under the sweep's call.
type QueryExternalState struct {
	IdempotencyKey domain.IdempotencyKey
	NextQueryAt    kernel.Instant
}

// RecordExternalInteraction is recorded after its raw payload is in the
// store, under the id its recorder minted, which also derives its command key.
type RecordExternalInteraction struct {
	InteractionID  domain.InteractionID
	ConnectorKey   domain.ConnectorKey
	Operation      string
	Direction      domain.Direction
	Source         domain.InteractionSource
	ContentType    string
	CommandID      domain.IdempotencyKey
	PartnerEventID string
	BusinessDate   string
	Request        domain.RawPayload
	Response       domain.RawPayload
	CapturedStep   domain.Step
	Mismatch       domain.Mismatch
	DuplicateOf    domain.InteractionID
}

// ReceivePartnerInteraction is a callback or a file the bff hands over, with
// the raw body as the partner sent it.
type ReceivePartnerInteraction struct {
	ConnectorKey   domain.ConnectorKey
	Operation      string
	Source         domain.InteractionSource
	PartnerEventID string
	BusinessDate   string
	ContentType    string
	Body           []byte
}

// ReadExternal is a read of a canonical port; OrganizationID and AssetType
// name what the authorization of query of ReceivablesQueryPort guards.
type ReadExternal struct {
	ConnectorKey   domain.ConnectorKey
	Operation      string
	OrganizationID domain.OrganizationID
	AssetType      string
	Parameters     []byte
}

// ConcludeInteraction names the revision it concludes, which derives its
// command key: a reprocessing is concluded under a key of its own. The
// finding comes with the instant and the external reference of its reading,
// which a redelivery observes again.
type ConcludeInteraction struct {
	InteractionID      domain.InteractionID
	Revision           int
	Outcome            domain.InteractionStatus
	CommandID          domain.IdempotencyKey
	ReadingHash        string
	Finding            domain.Finding
	FindingAsOf        port.Instant
	FindingExternalRef string
	Mismatch           domain.Mismatch
}

type ReprocessInteraction struct {
	InteractionID domain.InteractionID
	Justification string
}

type LinkExternalReference struct {
	ProviderID          domain.PartnerID
	System              domain.ExternalSystem
	ReferenceType       domain.ReferenceType
	Reference           string
	NormalizedReference string
	SubjectType         string
	SubjectID           domain.SubjectID
	AsOf                kernel.Instant
	ObservedVersion     string
}

// LinkInteractionReference is a link the normalization found in the
// interaction it names; the interaction derives the command key.
type LinkInteractionReference struct {
	Link          LinkExternalReference
	InteractionID domain.InteractionID
}

// DispatchExternalCommand is one delivery of the dispatch intent: the payload
// does not travel, so the handler reads the command by its key.
type DispatchExternalCommand struct {
	Receipt        port.Receipt
	IdempotencyKey domain.IdempotencyKey
}

// ConsumeResponseCaptured is one delivery of the capture of a submission's
// response, recorded as the outbound interaction.
type ConsumeResponseCaptured struct {
	Receipt port.Receipt
	Capture domain.ExternalResponseCaptured
}

// ConsumeInteractionStored is one delivery of a revision of an interaction
// handed to the normalization.
type ConsumeInteractionStored struct {
	Receipt       port.Receipt
	InteractionID domain.InteractionID
	Revision      int
}

// ConsumeCapabilityEnabled and ConsumeCapabilitySuspended are one delivery of
// partner.events, with the receipt the message inbox registers.
type ConsumeCapabilityEnabled struct {
	Receipt port.Receipt
	Enable  EnableOperation
}

type ConsumeCapabilitySuspended struct {
	Receipt port.Receipt
	Suspend SuspendOperation
}

// ConsumeSchemaMismatchDetected is one delivery of a mismatch this context
// published on gateway.events, naming the connector and the operation it broke.
type ConsumeSchemaMismatchDetected struct {
	Receipt      port.Receipt
	ConnectorKey domain.ConnectorKey
	Operation    string
}

type FindConnector struct {
	ConnectorKey domain.ConnectorKey
}

type FindEffectiveCapabilityProfile struct {
	ConnectorKey domain.ConnectorKey
	EffectiveAt  kernel.Instant
}

type FindCapabilityProfile struct {
	ConnectorKey domain.ConnectorKey
	Version      int
}

type FindQueryAuthorizationByOrganization struct {
	OrganizationID domain.OrganizationID
}

type FindExternalCommand struct {
	IdempotencyKey domain.IdempotencyKey
}

type FindExternalCommandsByBusinessKey struct {
	ConnectorKey domain.ConnectorKey
	BusinessKey  string
}

// FindExternalCommandsByStatus lists the status across connectors when
// ConnectorKey is empty.
type FindExternalCommandsByStatus struct {
	Status       domain.CommandStatus
	ConnectorKey domain.ConnectorKey
}

type FindExternalInteraction struct {
	InteractionID domain.InteractionID
}

type FindExternalInteractionByPartnerEvent struct {
	ConnectorKey   domain.ConnectorKey
	PartnerEventID string
}

type FindExternalInteractionsByCommand struct {
	CommandID domain.IdempotencyKey
}

type FindExternalReference struct {
	ReferenceKey domain.ReferenceKey
}

type FindExternalReferencesBySubject struct {
	SubjectType string
	SubjectID   domain.SubjectID
}

func (RecordExternalInteraction) isOperation()             {}
func (ReceivePartnerInteraction) isOperation()             {}
func (ReadExternal) isOperation()                          {}
func (ConcludeInteraction) isOperation()                   {}
func (ReprocessInteraction) isOperation()                  {}
func (LinkExternalReference) isOperation()                 {}
func (LinkInteractionReference) isOperation()              {}
func (FindExternalInteraction) isOperation()               {}
func (FindExternalInteractionByPartnerEvent) isOperation() {}
func (FindExternalInteractionsByCommand) isOperation()     {}
func (FindExternalReference) isOperation()                 {}
func (FindExternalReferencesBySubject) isOperation()       {}

func (RegisterConnector) isOperation()                    {}
func (SuspendOperation) isOperation()                     {}
func (EnableOperation) isOperation()                      {}
func (DeclareCapabilityProfile) isOperation()             {}
func (GrantQueryAuthorization) isOperation()              {}
func (RevokeQueryAuthorization) isOperation()             {}
func (CreateExternalCommand) isOperation()                {}
func (CancelExternalCommand) isOperation()                {}
func (RequestResend) isOperation()                        {}
func (HoldDispatch) isOperation()                         {}
func (ExpireAmbiguity) isOperation()                      {}
func (ResolveUnknownOutcome) isOperation()                {}
func (SweepExternalCommands) isOperation()                {}
func (QueryExternalState) isOperation()                   {}
func (ConsumeCapabilityEnabled) isOperation()             {}
func (ConsumeCapabilitySuspended) isOperation()           {}
func (ConsumeSchemaMismatchDetected) isOperation()        {}
func (DispatchExternalCommand) isOperation()              {}
func (ConsumeResponseCaptured) isOperation()              {}
func (ConsumeInteractionStored) isOperation()             {}
func (FindConnector) isOperation()                        {}
func (FindEffectiveCapabilityProfile) isOperation()       {}
func (FindCapabilityProfile) isOperation()                {}
func (FindQueryAuthorizationByOrganization) isOperation() {}
func (FindExternalCommand) isOperation()                  {}
func (FindExternalCommandsByBusinessKey) isOperation()    {}
func (FindExternalCommandsByStatus) isOperation()         {}
