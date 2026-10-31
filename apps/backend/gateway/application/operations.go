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

func (RegisterConnector) isOperation()                    {}
func (SuspendOperation) isOperation()                     {}
func (EnableOperation) isOperation()                      {}
func (DeclareCapabilityProfile) isOperation()             {}
func (GrantQueryAuthorization) isOperation()              {}
func (RevokeQueryAuthorization) isOperation()             {}
func (CreateExternalCommand) isOperation()                {}
func (CancelExternalCommand) isOperation()                {}
func (RequestResend) isOperation()                        {}
func (ExpireAmbiguity) isOperation()                      {}
func (ResolveUnknownOutcome) isOperation()                {}
func (SweepExternalCommands) isOperation()                {}
func (ConsumeCapabilityEnabled) isOperation()             {}
func (ConsumeCapabilitySuspended) isOperation()           {}
func (FindConnector) isOperation()                        {}
func (FindEffectiveCapabilityProfile) isOperation()       {}
func (FindCapabilityProfile) isOperation()                {}
func (FindQueryAuthorizationByOrganization) isOperation() {}
func (FindExternalCommand) isOperation()                  {}
func (FindExternalCommandsByBusinessKey) isOperation()    {}
func (FindExternalCommandsByStatus) isOperation()         {}
