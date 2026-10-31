package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	partnerID    = domain.PartnerID("prt-1")
	connectorKey = domain.ConnectorKey("prt-1:production")
	at           = kernel.Instant(1_755_432_000_000_000_000)
	second       = kernel.Instant(1_000_000_000)
)

func newRegisteredConnector(t *testing.T) *domain.Connector {
	t.Helper()
	c := domain.NewConnector(connectorKey)
	if _, rej := c.Register(domain.RegisterConnector{
		PartnerID: partnerID, Environment: domain.EnvironmentProduction, CredentialRef: "vault://prt-1/production", IntegrationVersion: 1, At: at,
	}); rej != nil {
		t.Fatalf("setup: Register rejected: %v", rej)
	}
	return c
}

func enable(t *testing.T, c *domain.Connector, operation, assetType string, version int) {
	t.Helper()
	if _, rej := c.EnableOperation(domain.EnableOperation{
		CapabilityID: domain.CapabilityID("cap-" + operation), Operation: operation, AssetType: assetType,
		StateQuerySupported: true, Version: version, At: at,
	}); rej != nil {
		t.Fatalf("setup: EnableOperation rejected: %v", rej)
	}
}

func suspend(t *testing.T, c *domain.Connector, cmd domain.SuspendOperation) {
	t.Helper()
	if _, rej := c.SuspendOperation(cmd); rej != nil {
		t.Fatalf("setup: SuspendOperation rejected: %v", rej)
	}
}

func requireNoEvents[R any](t *testing.T, acc kernel.Accepted[R]) {
	t.Helper()
	if events := acc.Events(); len(events) != 0 {
		t.Fatalf("Events() = %v, want none", events)
	}
}
