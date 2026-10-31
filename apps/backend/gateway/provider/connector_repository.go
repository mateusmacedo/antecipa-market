package provider

import (
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

type connectorState struct {
	PartnerID          string            `json:"partnerId"`
	Environment        string            `json:"environment"`
	CredentialRef      string            `json:"credentialRef"`
	IntegrationVersion int               `json:"integrationVersion"`
	Capabilities       []capabilityState `json:"capabilities"`
	Suspensions        []suspensionState `json:"suspensions"`
	RegisteredAt       int64             `json:"registeredAt"`
}

type capabilityState struct {
	CapabilityID        string `json:"capabilityId"`
	Operation           string `json:"operation"`
	AssetType           string `json:"assetType"`
	StateQuerySupported bool   `json:"stateQuerySupported"`
	Version             int    `json:"version"`
	At                  int64  `json:"at"`
}

type suspensionState struct {
	Operation    string `json:"operation"`
	AssetType    string `json:"assetType"`
	Cause        string `json:"cause"`
	CapabilityID string `json:"capabilityId"`
	Version      int    `json:"version"`
	Since        int64  `json:"since"`
}

// connectorTable keeps the whole state in the snapshot: FindConnector reads
// by the connector key alone.
var connectorTable = postgres.SnapshotTable("connectors", "connector_id", stateOfConnector, connectorOf,
	func(s domain.ConnectorSnapshot, key domain.ConnectorKey) domain.ConnectorSnapshot {
		s.Key = key
		return s
	})

func NewConnectorRepository(tx *postgres.Tx) ports.ConnectorRepository {
	return connectorTable.Repository(tx)
}

func NewConnectorReader(pool postgres.ReadPool) ports.ConnectorReader {
	return connectorTable.Reader(pool)
}

func stateOfConnector(s domain.ConnectorSnapshot) connectorState {
	capabilities := make([]capabilityState, 0, len(s.Capabilities))
	for _, c := range s.Capabilities {
		capabilities = append(capabilities, capabilityState{
			CapabilityID: string(c.CapabilityID), Operation: c.Operation, AssetType: c.AssetType,
			StateQuerySupported: c.StateQuerySupported, Version: c.Version, At: int64(c.At),
		})
	}
	suspensions := make([]suspensionState, 0, len(s.Suspensions))
	for _, x := range s.Suspensions {
		suspensions = append(suspensions, suspensionState{
			Operation: x.Operation, AssetType: x.AssetType, Cause: string(x.Cause),
			CapabilityID: string(x.CapabilityID), Version: x.Version, Since: int64(x.Since),
		})
	}
	return connectorState{
		PartnerID: string(s.PartnerID), Environment: string(s.Environment), CredentialRef: s.CredentialRef,
		IntegrationVersion: s.IntegrationVersion, Capabilities: capabilities, Suspensions: suspensions, RegisteredAt: int64(s.RegisteredAt),
	}
}

func connectorOf(state connectorState) domain.ConnectorSnapshot {
	capabilities := make([]domain.EnabledCapability, 0, len(state.Capabilities))
	for _, c := range state.Capabilities {
		capabilities = append(capabilities, domain.EnabledCapability{
			CapabilityID: domain.CapabilityID(c.CapabilityID), Operation: c.Operation, AssetType: c.AssetType,
			StateQuerySupported: c.StateQuerySupported, Version: c.Version, At: kernel.Instant(c.At),
		})
	}
	suspensions := make([]domain.OperationSuspension, 0, len(state.Suspensions))
	for _, x := range state.Suspensions {
		suspensions = append(suspensions, domain.OperationSuspension{
			Operation: x.Operation, AssetType: x.AssetType, Cause: domain.SuspensionCause(x.Cause),
			CapabilityID: domain.CapabilityID(x.CapabilityID), Version: x.Version, Since: kernel.Instant(x.Since),
		})
	}
	return domain.ConnectorSnapshot{
		PartnerID: domain.PartnerID(state.PartnerID), Environment: domain.Environment(state.Environment), CredentialRef: state.CredentialRef,
		IntegrationVersion: state.IntegrationVersion, Capabilities: capabilities, Suspensions: suspensions, RegisteredAt: kernel.Instant(state.RegisteredAt),
	}
}
