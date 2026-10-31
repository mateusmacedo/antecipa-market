package domain

import "strconv"

type (
	PartnerID           string
	CapabilityID        string
	ConsentID           string
	OrganizationID      string
	TenantID            string
	InteractionID       string
	ExternalReferenceID string
)

// ConnectorKey is "<partnerId>:<environment>", the natural identity of a
// Connector and the connectorId every other aggregate refers to it by.
type ConnectorKey string

func ConnectorKeyOf(partner PartnerID, environment Environment) ConnectorKey {
	return ConnectorKey(string(partner) + ":" + string(environment))
}

// ProfileKey is "<connectorKey>:<version>": one version is declared once.
type ProfileKey string

func ProfileKeyOf(connector ConnectorKey, version int) ProfileKey {
	return ProfileKey(string(connector) + ":" + strconv.Itoa(version))
}

// IdempotencyKey is the natural identity of an ExternalCommand, derived by
// the caller from stable business data; it never reaches the partner.
type IdempotencyKey string

// SeriesKeyOf is the key of the versions of one business object sent to one
// operation of one connector: two versions of a series never reach the partner.
func SeriesKeyOf(connector ConnectorKey, operation, businessKey string) string {
	return string(connector) + ":" + operation + ":" + businessKey
}

type Environment string

const (
	EnvironmentDevelopment  Environment = "development"
	EnvironmentIntegration  Environment = "integration"
	EnvironmentHomologation Environment = "homologation"
	EnvironmentProduction   Environment = "production"
)

// IsWriteOperation tells the operations of the ports that change the partner's
// state, the ones an ExternalCommand carries.
func IsWriteOperation(operation string) bool {
	switch operation {
	case "activateOptIn", "revokeOptIn", "submitEffect", "requestRelease":
		return true
	default:
		return false
	}
}

const nanosPerSecond = 1_000_000_000
