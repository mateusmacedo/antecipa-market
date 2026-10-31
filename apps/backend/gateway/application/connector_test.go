package application_test

import (
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func registerConnector() application.RegisterConnector {
	return application.RegisterConnector{PartnerID: "prt-1", Environment: domain.EnvironmentProduction, CredentialRef: "vault://prt-1/production", IntegrationVersion: 1}
}

func (h *harness) registered(t *testing.T) {
	t.Helper()
	outcome, err := h.service.RegisterConnector(withExecution(t), registerConnector())
	must(outcome, err).accepted(t)
}

func (h *harness) enabled(t *testing.T, operation string, stateQuery bool, version int) {
	t.Helper()
	outcome, err := h.service.EnableOperation(withExecution(t), application.EnableOperation{
		ConnectorKey: testConnector, CapabilityID: domain.CapabilityID("cap-" + operation), Operation: operation,
		StateQuerySupported: stateQuery, Version: version,
	})
	must(outcome, err).accepted(t)
}

func TestRegisterConnectorRegistersUnderTheConnectorKey(t *testing.T) {
	h := newHarness(t)

	got := must(h.service.RegisterConnector(withExecution(t), registerConnector())).accepted(t)

	if got.ConnectorKey != testConnector {
		t.Fatalf("ConnectorKey = %q, want %q", got.ConnectorKey, testConnector)
	}
	if s := h.connector(t); s.RegisteredAt != testOccurred || s.CredentialRef != "vault://prt-1/production" {
		t.Fatalf("connector = %+v", s)
	}
	if n := len(h.entries()); n != 0 {
		t.Fatalf("outbox has %d entries, want none: the connector emits no event", n)
	}
}

func TestRegisterConnectorRefusesAnEmptyCredentialRef(t *testing.T) {
	h := newHarness(t)
	cmd := registerConnector()
	cmd.CredentialRef = ""

	outcome, err := h.service.RegisterConnector(withExecution(t), cmd)

	must(outcome, err).refused(t, string(domain.CodeConnectorCredentialRefEmpty))
}

func TestSuspendAndEnableOperationMoveTheGate(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.enabled(t, "submitEffect", true, 3)

	must(h.service.SuspendOperation(withExecution(t), application.SuspendOperation{
		ConnectorKey: testConnector, Operation: "submitEffect", Cause: domain.CauseSchemaMismatch,
	})).accepted(t)
	if gate, _ := h.connector(t).Gate("submitEffect", "duplicata"); gate != domain.GateSuspended {
		t.Fatalf("Gate() = %q after the suspension, want suspended", gate)
	}

	h.enabled(t, "submitEffect", true, 4)
	if gate, _ := h.connector(t).Gate("submitEffect", "duplicata"); gate != domain.GateAllowed {
		t.Fatalf("Gate() = %q after the re-homologation, want allowed", gate)
	}
}

func TestSuspendOperationOfAnUnknownConnectorFails(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.SuspendOperation(withExecution(t), application.SuspendOperation{ConnectorKey: testConnector, Operation: "submitEffect", Cause: domain.CausePartner})

	if err == nil {
		t.Fatalf("SuspendOperation() over no connector succeeded")
	}
}
