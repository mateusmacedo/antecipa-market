package application_test

import (
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func receipt(id port.MessageID, hash string) port.Receipt {
	return port.Receipt{MessageID: id, MessageType: "com.company.partner.capability-suspended.v1", PayloadHash: hash, ReceivedAt: testOccurred}
}

func suspended(id port.MessageID, hash string, version int) application.ConsumeCapabilitySuspended {
	return application.ConsumeCapabilitySuspended{
		Receipt: receipt(id, hash),
		Suspend: application.SuspendOperation{
			ConnectorKey: testConnector, Operation: "submitEffect", Cause: domain.CausePartner, CapabilityID: "cap-submitEffect", Version: version,
		},
	}
}

func consumed(t *testing.T, disposition usecase.Disposition, err error, want usecase.Disposition) {
	t.Helper()
	if disposition != want {
		t.Fatalf("disposition = %v (%v), want %v", disposition, err, want)
	}
}

func TestConsumeCapabilitySuspendedSuspendsOnceAndAcknowledgesTheRedelivery(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.enabled(t, "submitEffect", true, 3)

	disposition, err := h.service.ConsumeCapabilitySuspended(withExecution(t), suspended("evt-1", "h1", 3))
	consumed(t, disposition, err, usecase.R1D1)
	if gate, _ := h.connector(t).Gate("submitEffect", "duplicata"); gate != domain.GateSuspended {
		t.Fatalf("Gate() = %q, want suspended", gate)
	}

	disposition, err = h.service.ConsumeCapabilitySuspended(withExecution(t), suspended("evt-1", "h1", 3))
	consumed(t, disposition, err, usecase.R2)
	disposition, err = h.service.ConsumeCapabilitySuspended(withExecution(t), suspended("evt-1", "h-other", 3))
	consumed(t, disposition, err, usecase.R4)
}

func TestConsumeCapabilitySuspendedDiscardsAnOlderVersion(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.enabled(t, "submitEffect", true, 3)
	before := h.connector(t)

	disposition, err := h.service.ConsumeCapabilitySuspended(withExecution(t), suspended("evt-1", "h1", 2))

	consumed(t, disposition, err, usecase.R1D1)
	if !before.Equal(h.connector(t)) {
		t.Fatalf("connector = %+v, want it unchanged by version 2 after version 3", h.connector(t))
	}
}

func TestConsumeCapabilityEnabledEnablesTheOperation(t *testing.T) {
	h := newHarness(t)
	h.registered(t)

	disposition, err := h.service.ConsumeCapabilityEnabled(withExecution(t), application.ConsumeCapabilityEnabled{
		Receipt: receipt("evt-2", "h2"),
		Enable:  application.EnableOperation{ConnectorKey: testConnector, CapabilityID: "cap-1", Operation: "submitEffect", StateQuerySupported: true, Version: 1},
	})

	consumed(t, disposition, err, usecase.R1D1)
	if gate, capability := h.connector(t).Gate("submitEffect", "duplicata"); gate != domain.GateAllowed || !capability.StateQuerySupported {
		t.Fatalf("Gate() = %q %+v, want allowed with the homologated state query", gate, capability)
	}
}

// A suspension dropped before its connector exists would leave the operation
// open later, so the delivery is retried until the consumer quarantines it.
func TestConsumeOverAnUnknownConnectorAsksForTheRedelivery(t *testing.T) {
	h := newHarness(t)

	disposition, err := h.service.ConsumeCapabilitySuspended(withExecution(t), suspended("evt-1", "h1", 1))

	consumed(t, disposition, err, usecase.R1D3)
	if h.store.InboxRows() != 0 {
		t.Fatalf("the inbox kept %d rows, want the reception rolled back", h.store.InboxRows())
	}
}

func mismatchDelivery(id port.MessageID, hash string) application.ConsumeSchemaMismatchDetected {
	return application.ConsumeSchemaMismatchDetected{
		Receipt: port.Receipt{
			MessageID: id, MessageType: "com.company.gateway.external-schema-mismatch-detected.v1", PayloadHash: hash, ReceivedAt: testOccurred,
		},
		ConnectorKey: testConnector, Operation: "submitEffect",
	}
}

// L570-572: the conclusion only writes the fact; its consumption is the unit of
// work that suspends the operation, for every asset type, with cause
// schema_mismatch.
func TestTheMismatchThisContextDetectedSuspendsTheOperationOnceConsumed(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.recorded(t, application.RecordExternalInteraction{
		InteractionID: testInteraction, ConnectorKey: testConnector, Operation: "submitEffect", Direction: domain.DirectionOutbound,
		Source: domain.InteractionSubmitResponse, CommandID: testCommand, Request: rawRequest, Response: rawResponse,
		CapturedStep: domain.StepSuspend, Mismatch: brokenField,
	})
	h.concluded(t, application.ConcludeInteraction{
		InteractionID: testInteraction, Revision: 1, Outcome: domain.InteractionSchemaMismatch, Mismatch: brokenField,
	})
	if gate, _ := h.connector(t).Gate("submitEffect", "duplicata"); gate != domain.GateAllowed {
		t.Fatalf("Gate() = %q after the conclusion, want allowed: the conclusion suspends nothing in a second unit of work", gate)
	}
	events := h.eventsTo(application.Destination)
	detected, ok := events[len(events)-1].(domain.ExternalSchemaMismatchDetected)
	if !ok || detected.AssetType != "duplicata" {
		t.Fatalf("last event = %+v, want the mismatch with the asset type of its command", events[len(events)-1])
	}

	disposition, err := h.service.ConsumeSchemaMismatchDetected(withExecution(t), application.ConsumeSchemaMismatchDetected{
		Receipt: mismatchDelivery("evt-m", "hm").Receipt, ConnectorKey: detected.ConnectorKey, Operation: detected.Operation,
	})

	consumed(t, disposition, err, usecase.R1D1)
	want := []domain.OperationSuspension{{Operation: "submitEffect", Cause: domain.CauseSchemaMismatch, Since: testOccurred}}
	if got := h.connector(t).Suspensions; !slices.Equal(got, want) {
		t.Fatalf("suspensions = %+v, want %+v: every asset type of the operation", got, want)
	}
	for _, assetType := range []string{"duplicata", "cartao"} {
		if gate, _ := h.connector(t).Gate("submitEffect", assetType); gate != domain.GateSuspended {
			t.Fatalf("Gate(%s) = %q, want suspended", assetType, gate)
		}
	}
	disposition, err = h.service.ConsumeSchemaMismatchDetected(withExecution(t), mismatchDelivery("evt-m", "hm"))
	consumed(t, disposition, err, usecase.R2)
}

// The mismatch registers in the inbox of gateway.events, apart from the one of
// partner.events: the same message id there is another message.
func TestTheMismatchRegistersInAnInboxApartFromPartnerEvents(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.enabled(t, "submitEffect", true, 1)

	disposition, err := h.service.ConsumeSchemaMismatchDetected(withExecution(t), mismatchDelivery("evt-1", "h-mismatch"))
	consumed(t, disposition, err, usecase.R1D1)
	disposition, err = h.service.ConsumeCapabilitySuspended(withExecution(t), suspended("evt-1", "h-partner", 1))
	consumed(t, disposition, err, usecase.R1D1)
}
