package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type SuspensionCause string

const (
	CausePartner        SuspensionCause = "partner"
	CauseSchemaMismatch SuspensionCause = "schema_mismatch"
)

type CapabilityGate string

const (
	GateAllowed    CapabilityGate = "allowed"
	GateNotEnabled CapabilityGate = "not_enabled"
	GateSuspended  CapabilityGate = "suspended"
)

// EnabledCapability projects a CapabilityEnabled of the Partner; an empty
// AssetType enables the operation for every asset type.
type EnabledCapability struct {
	CapabilityID        CapabilityID
	Operation           string
	AssetType           string
	StateQuerySupported bool
	Version             int
	At                  kernel.Instant
}

// OperationSuspension blocks new commands and reads of an operation; an empty
// Operation covers the whole connector and an empty AssetType every asset type.
type OperationSuspension struct {
	Operation    string
	AssetType    string
	Cause        SuspensionCause
	CapabilityID CapabilityID
	Version      int
	Since        kernel.Instant
}

func (s OperationSuspension) covers(operation, assetType string) bool {
	return (s.Operation == "" || s.Operation == operation) && (s.AssetType == "" || s.AssetType == assetType)
}

type Connector struct {
	key                ConnectorKey
	partnerID          PartnerID
	environment        Environment
	credentialRef      string
	integrationVersion int
	capabilities       []EnabledCapability
	suspensions        []OperationSuspension
	registeredAt       kernel.Instant
}

func NewConnector(key ConnectorKey) *Connector {
	return &Connector{key: key}
}

func FromConnectorSnapshot(s ConnectorSnapshot) *Connector {
	return &Connector{
		key:                s.Key,
		partnerID:          s.PartnerID,
		environment:        s.Environment,
		credentialRef:      s.CredentialRef,
		integrationVersion: s.IntegrationVersion,
		capabilities:       slices.Clone(s.Capabilities),
		suspensions:        slices.Clone(s.Suspensions),
		registeredAt:       s.RegisteredAt,
	}
}

type ConnectorSnapshot struct {
	Key                ConnectorKey
	PartnerID          PartnerID
	Environment        Environment
	CredentialRef      string
	IntegrationVersion int
	Capabilities       []EnabledCapability
	Suspensions        []OperationSuspension
	RegisteredAt       kernel.Instant
}

func (c *Connector) Snapshot() ConnectorSnapshot {
	return ConnectorSnapshot{
		Key:                c.key,
		PartnerID:          c.partnerID,
		Environment:        c.environment,
		CredentialRef:      c.credentialRef,
		IntegrationVersion: c.integrationVersion,
		Capabilities:       slices.Clone(c.capabilities),
		Suspensions:        slices.Clone(c.suspensions),
		RegisteredAt:       c.registeredAt,
	}
}

func (s ConnectorSnapshot) Equal(other ConnectorSnapshot) bool {
	return s.Key == other.Key &&
		s.PartnerID == other.PartnerID &&
		s.Environment == other.Environment &&
		s.CredentialRef == other.CredentialRef &&
		s.IntegrationVersion == other.IntegrationVersion &&
		slices.Equal(s.Capabilities, other.Capabilities) &&
		slices.Equal(s.Suspensions, other.Suspensions) &&
		s.RegisteredAt == other.RegisteredAt
}

func (c *Connector) clone() Connector {
	return Connector{
		key:                c.key,
		partnerID:          c.partnerID,
		environment:        c.environment,
		credentialRef:      c.credentialRef,
		integrationVersion: c.integrationVersion,
		capabilities:       slices.Clone(c.capabilities),
		suspensions:        slices.Clone(c.suspensions),
		registeredAt:       c.registeredAt,
	}
}

// Gate is the fail-closed check of 13 §4: an operation never enabled is
// refused before a suspension is even looked at.
func (s ConnectorSnapshot) Gate(operation, assetType string) (CapabilityGate, EnabledCapability) {
	i := slices.IndexFunc(s.Capabilities, func(c EnabledCapability) bool {
		return c.Operation == operation && (c.AssetType == "" || c.AssetType == assetType)
	})
	if i < 0 {
		return GateNotEnabled, EnabledCapability{}
	}
	if slices.ContainsFunc(s.Suspensions, func(x OperationSuspension) bool { return x.covers(operation, assetType) }) {
		return GateSuspended, s.Capabilities[i]
	}
	return GateAllowed, s.Capabilities[i]
}

// appliedVersion is the newest capability version the Partner sent for the
// operation and asset type, enabled or suspended.
func (c *Connector) appliedVersion(operation, assetType string) int {
	applied := 0
	for _, enabled := range c.capabilities {
		if enabled.Operation == operation && enabled.AssetType == assetType {
			applied = max(applied, enabled.Version)
		}
	}
	for _, suspension := range c.suspensions {
		if suspension.Cause == CausePartner && suspension.Operation == operation && suspension.AssetType == assetType {
			applied = max(applied, suspension.Version)
		}
	}
	return applied
}
