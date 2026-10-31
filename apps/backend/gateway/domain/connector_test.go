package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func TestRegisterConnectorCreatesAnAbsentConnectorWithoutSuspensions(t *testing.T) {
	c := domain.NewConnector(connectorKey)

	acc, rej := c.Register(domain.RegisterConnector{
		PartnerID: partnerID, Environment: domain.EnvironmentProduction, CredentialRef: "vault://prt-1/production", IntegrationVersion: 1, At: at,
	})

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	if got, want := acc.Response(), (domain.ConnectorResponse{ConnectorKey: connectorKey}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	s := c.Snapshot()
	if s.RegisteredAt != at || s.PartnerID != partnerID || s.Environment != domain.EnvironmentProduction || s.CredentialRef != "vault://prt-1/production" || s.IntegrationVersion != 1 {
		t.Fatalf("Snapshot() = %+v", s)
	}
	if s.Capabilities == nil || len(s.Capabilities) != 0 || s.Suspensions == nil || len(s.Suspensions) != 0 {
		t.Fatalf("capabilities %v and suspensions %v, want both empty", s.Capabilities, s.Suspensions)
	}
}

func TestRegisterConnectorRejectsAnEmptyCredentialRef(t *testing.T) {
	c := domain.NewConnector(connectorKey)
	before := c.Snapshot()

	acc, rej := c.Register(domain.RegisterConnector{PartnerID: partnerID, Environment: domain.EnvironmentProduction, IntegrationVersion: 1, At: at})

	tb.RequireRejected(t, acc, rej, domain.CodeConnectorCredentialRefEmpty)
	if !before.Equal(c.Snapshot()) {
		t.Fatalf("snapshot changed on rejection: %+v", c.Snapshot())
	}
}

func TestRegisterConnectorSwapsTheCredentialOnANewerIntegrationVersion(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "", 3)
	suspend(t, c, domain.SuspendOperation{Operation: "requestRelease", Cause: domain.CauseSchemaMismatch, At: at})
	before := c.Snapshot()

	_, rej := c.Register(domain.RegisterConnector{
		PartnerID: partnerID, Environment: domain.EnvironmentProduction, CredentialRef: "vault://prt-1/production-v2", IntegrationVersion: 2, At: at + second,
	})

	tb.RequireAccepted(t, rej)
	after := c.Snapshot()
	if after.CredentialRef != "vault://prt-1/production-v2" || after.IntegrationVersion != 2 {
		t.Fatalf("credential %q version %d, want the new pair", after.CredentialRef, after.IntegrationVersion)
	}
	after.CredentialRef, after.IntegrationVersion = before.CredentialRef, before.IntegrationVersion
	if !before.Equal(after) {
		t.Fatalf("only the credential and the version may change\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestRegisterConnectorKeepsTheCredentialOnAnOlderOrEqualIntegrationVersion(t *testing.T) {
	for _, version := range []int{0, 1} {
		c := newRegisteredConnector(t)
		before := c.Snapshot()

		_, rej := c.Register(domain.RegisterConnector{
			PartnerID: partnerID, Environment: domain.EnvironmentProduction, CredentialRef: "vault://other", IntegrationVersion: version, At: at + second,
		})

		tb.RequireAccepted(t, rej)
		if !before.Equal(c.Snapshot()) {
			t.Fatalf("version %d changed the connector: %+v", version, c.Snapshot())
		}
	}
}

func TestSuspendOperationSuspendsOnlyTheOperationNamed(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "", 3)
	enable(t, c, "requestRelease", "", 1)

	acc, rej := c.SuspendOperation(domain.SuspendOperation{
		Operation: "submitEffect", Cause: domain.CausePartner, CapabilityID: "cap-submitEffect", Version: 3, At: at + second,
	})

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	got := c.Snapshot().Suspensions
	want := []domain.OperationSuspension{{Operation: "submitEffect", Cause: domain.CausePartner, CapabilityID: "cap-submitEffect", Version: 3, Since: at + second}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("Suspensions = %+v, want %+v", got, want)
	}
	if gate, _ := c.Snapshot().Gate("requestRelease", "duplicata"); gate != domain.GateAllowed {
		t.Fatalf("Gate(requestRelease) = %q, want the other operations to go on", gate)
	}
	if gate, _ := c.Snapshot().Gate("submitEffect", "duplicata"); gate != domain.GateSuspended {
		t.Fatalf("Gate(submitEffect) = %q, want suspended", gate)
	}
}

func TestSuspendOperationAcceptsARepeatedSuspensionUnchanged(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "", 3)
	suspend(t, c, domain.SuspendOperation{Operation: "submitEffect", Cause: domain.CausePartner, CapabilityID: "cap-submitEffect", Version: 3, At: at})
	before := c.Snapshot()

	_, rej := c.SuspendOperation(domain.SuspendOperation{Operation: "submitEffect", Cause: domain.CausePartner, CapabilityID: "cap-submitEffect", Version: 3, At: at + second})

	tb.RequireAccepted(t, rej)
	if !before.Equal(c.Snapshot()) {
		t.Fatalf("a repeated suspension changed the connector: %+v", c.Snapshot())
	}
}

func TestSuspendOperationDiscardsAPartnerSuspensionOfAnOlderVersion(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "", 3)
	before := c.Snapshot()

	_, rej := c.SuspendOperation(domain.SuspendOperation{Operation: "submitEffect", Cause: domain.CausePartner, CapabilityID: "cap-old", Version: 2, At: at + second})

	tb.RequireAccepted(t, rej)
	if !before.Equal(c.Snapshot()) {
		t.Fatalf("version 2 after version 3 changed the connector: %+v", c.Snapshot())
	}
}

func TestEnableOperationLiftsTheSchemaMismatchSuspension(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "", 3)
	suspend(t, c, domain.SuspendOperation{Operation: "submitEffect", Cause: domain.CauseSchemaMismatch, At: at})
	suspend(t, c, domain.SuspendOperation{Operation: "requestRelease", Cause: domain.CauseSchemaMismatch, At: at})

	acc, rej := c.EnableOperation(domain.EnableOperation{
		CapabilityID: "cap-submitEffect-4", Operation: "submitEffect", StateQuerySupported: false, Version: 4, At: at + second,
	})

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	s := c.Snapshot()
	want := domain.EnabledCapability{CapabilityID: "cap-submitEffect-4", Operation: "submitEffect", Version: 4, At: at + second}
	if len(s.Capabilities) != 1 || s.Capabilities[0] != want {
		t.Fatalf("Capabilities = %+v, want only %+v", s.Capabilities, want)
	}
	if len(s.Suspensions) != 1 || s.Suspensions[0].Operation != "requestRelease" {
		t.Fatalf("Suspensions = %+v, want only the one of requestRelease", s.Suspensions)
	}
}

func TestEnableOperationLiftsOnlyTheSuspensionsOfItsAssetType(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "duplicata", 1)
	suspend(t, c, domain.SuspendOperation{Operation: "submitEffect", AssetType: "cartao", Cause: domain.CausePartner, CapabilityID: "cap-cartao", Version: 1, At: at})
	suspend(t, c, domain.SuspendOperation{Operation: "submitEffect", Cause: domain.CauseSchemaMismatch, At: at})

	_, rej := c.EnableOperation(domain.EnableOperation{CapabilityID: "cap-dup-2", Operation: "submitEffect", AssetType: "duplicata", Version: 2, At: at + second})

	tb.RequireAccepted(t, rej)
	got := c.Snapshot().Suspensions
	if len(got) != 1 || got[0].AssetType != "cartao" {
		t.Fatalf("Suspensions = %+v, want only the one of cartao", got)
	}
}

func TestEnableOperationDiscardsAnOlderVersion(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "", 3)
	before := c.Snapshot()

	_, rej := c.EnableOperation(domain.EnableOperation{CapabilityID: "cap-old", Operation: "submitEffect", Version: 2, At: at + second})

	tb.RequireAccepted(t, rej)
	if !before.Equal(c.Snapshot()) {
		t.Fatalf("version 2 after version 3 changed the connector: %+v", c.Snapshot())
	}
}

func TestGateFailsClosed(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "duplicata", 1)
	enable(t, c, "requestRelease", "", 1)
	suspend(t, c, domain.SuspendOperation{Cause: domain.CauseSchemaMismatch, AssetType: "cartao", At: at})

	tests := []struct {
		operation, assetType string
		want                 domain.CapabilityGate
	}{
		{"submitEffect", "duplicata", domain.GateAllowed},
		{"submitEffect", "cartao", domain.GateNotEnabled},
		{"activateOptIn", "duplicata", domain.GateNotEnabled},
		{"requestRelease", "duplicata", domain.GateAllowed},
		{"requestRelease", "cartao", domain.GateSuspended},
	}
	for _, tc := range tests {
		if got, _ := c.Snapshot().Gate(tc.operation, tc.assetType); got != tc.want {
			t.Errorf("Gate(%s, %s) = %q, want %q", tc.operation, tc.assetType, got, tc.want)
		}
	}
}

func TestConnectorSnapshotRoundTrips(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "duplicata", 1)
	suspend(t, c, domain.SuspendOperation{Operation: "submitEffect", Cause: domain.CauseSchemaMismatch, At: at})

	s := c.Snapshot()
	if !domain.FromConnectorSnapshot(s).Snapshot().Equal(s) {
		t.Fatalf("round trip lost state: %+v", s)
	}
	other := s
	other.Suspensions = nil
	if s.Equal(other) {
		t.Fatalf("Equal ignored the suspensions")
	}
}

func TestARefusedDecisionLeavesTheConnectorCollectionsUntouched(t *testing.T) {
	c := newRegisteredConnector(t)
	enable(t, c, "submitEffect", "", 1)
	held := c.Snapshot()

	if _, rej := c.Register(domain.RegisterConnector{PartnerID: partnerID, Environment: domain.EnvironmentProduction, IntegrationVersion: 9, At: at}); rej == nil {
		t.Fatalf("Register without credential accepted")
	}
	held.Capabilities[0].Version = 99
	if got := c.Snapshot().Capabilities[0].Version; got != 1 {
		t.Fatalf("the snapshot shares its slice with the aggregate: version %d", got)
	}
}
