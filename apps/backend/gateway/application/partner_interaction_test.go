package application_test

import (
	"errors"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

const receiveLimit = 1 << 12

// receiving is the dispatching harness whose submitEffect bounds a partner
// interaction at receiveLimit bytes.
func receiving(t *testing.T, change func(*domain.OperationCapability)) *dispatching {
	t.Helper()
	return newDispatching(t, func(o *domain.OperationCapability) {
		o.MaxPayloadBytes = receiveLimit
		if change != nil {
			change(o)
		}
	})
}

func partnerReceipt(event string, body []byte) application.ReceivePartnerInteraction {
	return application.ReceivePartnerInteraction{
		ConnectorKey: testConnector, Operation: "submitEffect", Source: domain.InteractionCallback, PartnerEventID: event,
		BusinessDate: "2026-09-02", ContentType: partnersim.ContentType, Body: body,
	}
}

func (d *dispatching) receive(t *testing.T, key string, cmd application.ReceivePartnerInteraction) domain.InteractionResponse {
	t.Helper()
	return must(d.service.ReceivePartnerInteraction(withKey(t, key), cmd)).accepted(t)
}

func (d *dispatching) inboundID(t *testing.T, event string) domain.InteractionID {
	t.Helper()
	id, err := d.service.DerivedKeys.InboundInteractionID(testConnector, event)
	if err != nil {
		t.Fatalf("InboundInteractionID() = %v", err)
	}
	return id
}

func TestAReceivedCallbackIsKeptAndRecordedUnderTheIdOfItsPartnerEvent(t *testing.T) {
	d := receiving(t, nil)
	body := d.sim.Notify(partnersim.Notice{ExternalRef: "ext-1"}).Body

	got := d.receive(t, "rcv-1", partnerReceipt("evt-p-1", body))

	id := d.inboundID(t, "evt-p-1")
	if got.InteractionID != id || got.Status != domain.InteractionStored {
		t.Fatalf("ReceivePartnerInteraction() = %+v, want stored under %s", got, id)
	}
	i := d.interaction(t, id)
	if i.Direction != domain.DirectionInbound || i.Source != domain.InteractionCallback || i.PartnerEventID != "evt-p-1" ||
		i.BusinessDate != "2026-09-02" || i.ContentType != partnersim.ContentType || i.Request.Ref != ports.RawKeyOf(testConnector, id, ports.RawRequest) ||
		i.Request.VersionID != "v1" || i.Response.Ref != "" || i.Status != domain.InteractionStored {
		t.Fatalf("interaction = %+v, want the inbound callback with its raw request", i)
	}
	if handovers := d.handovers(); len(handovers) != 1 || handovers[0].InteractionID != id {
		t.Fatalf("handovers = %+v, want the callback handed to the normalization", handovers)
	}
}

// L890-892: the body is measured against the operation before any write.
func TestABodyAboveMaxPayloadBytesIsRefusedWithNothingKept(t *testing.T) {
	d := receiving(t, nil)
	exact := make([]byte, receiveLimit)
	over := make([]byte, receiveLimit+1)

	must(d.service.ReceivePartnerInteraction(withKey(t, "rcv-1"), partnerReceipt("evt-p-1", over))).refused(t, string(domain.CodeExternalInteractionPayloadTooLarge))

	if n := d.raw.versions(ports.RawKeyOf(testConnector, d.inboundID(t, "evt-p-1"), ports.RawRequest)); n != 0 || len(d.handovers()) != 0 {
		t.Fatalf("versions = %d, handovers = %+v; want nothing written", n, d.handovers())
	}
	if got := d.receive(t, "rcv-2", partnerReceipt("evt-p-2", exact)); got.Status != domain.InteractionStored {
		t.Fatalf("ReceivePartnerInteraction(exactly the limit) = %+v, want stored", got)
	}
}

// L886-888: the second receipt of a partner event keeps its own raw version
// and is recorded as a duplicate of the first, with no normalization and no
// change to any command.
func TestAPartnerEventReceivedAgainIsADuplicateOfTheFirst(t *testing.T) {
	d := receiving(t, nil)
	d.scripted(t, accepting(200))
	before := d.command(t, testCommand)
	body := d.sim.Notify(partnersim.Notice{ExternalRef: "ext-1"}).Body
	first := d.receive(t, "rcv-1", partnerReceipt("evt-p-1", body))

	again := d.receive(t, "rcv-2", partnerReceipt("evt-p-1", body))

	if again.Status != domain.InteractionDuplicate || again.InteractionID == first.InteractionID {
		t.Fatalf("second ReceivePartnerInteraction() = %+v, want a duplicate under an id of its own", again)
	}
	duplicate := d.interaction(t, again.InteractionID)
	key := ports.RawKeyOf(testConnector, first.InteractionID, ports.RawRequest)
	if duplicate.DuplicateOf != first.InteractionID || duplicate.Request.Ref != key || duplicate.Request.VersionID != "v2" ||
		duplicate.PartnerEventID != "evt-p-1" || duplicate.Direction != domain.DirectionInbound {
		t.Fatalf("duplicate = %+v, want the duplicate of %s with its own version of the raw", duplicate, first.InteractionID)
	}
	if n := d.raw.versions(key); n != 2 {
		t.Fatalf("versions = %d, want both receipts kept", n)
	}
	if handovers := d.handovers(); len(handovers) != 1 || handovers[0].InteractionID != first.InteractionID {
		t.Fatalf("handovers = %+v, want only the first receipt normalized", handovers)
	}
	if after := d.command(t, testCommand); after != before {
		t.Fatalf("command = %+v, want it untouched by a duplicate", after)
	}
	found, err := d.service.FindExternalInteractionByPartnerEvent(withExecution(t), testConnector, "evt-p-1")
	if err != nil || found.ID != first.InteractionID {
		t.Fatalf("FindExternalInteractionByPartnerEvent() = %+v, %v; want the first receipt", found, err)
	}
}

// A retry of the bff under the same key answers the first receipt; the body it
// wrote again stays an orphan version (decision 16).
func TestAReplayOfTheKeyAnswersTheFirstReceipt(t *testing.T) {
	d := receiving(t, nil)
	body := d.sim.Notify(partnersim.Notice{}).Body
	first := d.receive(t, "rcv-1", partnerReceipt("evt-p-1", body))

	again := d.receive(t, "rcv-1", partnerReceipt("evt-p-1", body))

	if again != first {
		t.Fatalf("replay = %+v, want %+v", again, first)
	}
	if n := d.raw.versions(ports.RawKeyOf(testConnector, first.InteractionID, ports.RawRequest)); n != 2 || len(d.handovers()) != 1 {
		t.Fatalf("versions = %d, handovers = %d; want an orphan version and one interaction", n, len(d.handovers()))
	}
}

func TestAReceiptWithoutPartnerEventIsNeverDeduplicated(t *testing.T) {
	d := receiving(t, nil)
	body := d.sim.Notify(partnersim.Notice{}).Body
	file := partnerReceipt("", body)
	file.Source = domain.InteractionFile

	first, second := d.receive(t, "rcv-1", file), d.receive(t, "rcv-2", file)

	if first.InteractionID == second.InteractionID || first.Status != domain.InteractionStored || second.Status != domain.InteractionStored {
		t.Fatalf("receipts = %+v and %+v, want two stored interactions", first, second)
	}
	if i := d.interaction(t, second.InteractionID); i.Source != domain.InteractionFile || i.PartnerEventID != "" || i.DuplicateOf != "" {
		t.Fatalf("interaction = %+v, want a file without partner event", i)
	}
}

func TestAReceiptTheProfileDoesNotBoundIsRefused(t *testing.T) {
	d := receiving(t, nil)
	cmd := partnerReceipt("evt-p-1", []byte(`{}`))
	cmd.Operation = "requestRelease"

	must(d.service.ReceivePartnerInteraction(withKey(t, "rcv-1"), cmd)).refused(t, string(domain.CodeConnectorCapabilityNotEnabled))

	if n := d.raw.versions(ports.RawKeyOf(testConnector, d.inboundID(t, "evt-p-1"), ports.RawRequest)); n != 0 {
		t.Fatalf("versions = %d, want nothing written", n)
	}
}

func TestAReceiptFailsAsADependencyWhileTheRawStoreIsUnavailable(t *testing.T) {
	d := receiving(t, nil)
	d.raw.failPut(ports.RawRequest, ports.ErrRawStoreUnavailable)

	_, err := d.service.ReceivePartnerInteraction(withKey(t, "rcv-1"), partnerReceipt("evt-p-1", []byte(`{}`)))

	if failure, ok := errors.AsType[*usecase.Failure](err); !ok || failure.Category() != usecase.TransientDependency || !errors.Is(err, ports.ErrRawStoreUnavailable) {
		t.Fatalf("ReceivePartnerInteraction() = %v, want a transient dependency", err)
	}
	if len(d.handovers()) != 0 {
		t.Fatalf("handovers = %+v, want nothing recorded", d.handovers())
	}
}

func TestAReceiptOfASourceThePartnerDoesNotSendIsInvalid(t *testing.T) {
	d := receiving(t, nil)
	cmd := partnerReceipt("evt-p-1", []byte(`{}`))
	cmd.Source = domain.InteractionStateQuery

	_, err := d.service.ReceivePartnerInteraction(withKey(t, "rcv-1"), cmd)

	if failure, ok := errors.AsType[*usecase.Failure](err); !ok || failure.Category() != usecase.Validation {
		t.Fatalf("ReceivePartnerInteraction(state_query) = %v, want a validation failure", err)
	}
}

// L882-884 end to end in process: the callback the edge receives is
// normalized and confirms the acknowledged command under the first revision.
func TestAReceivedCallbackConfirmsAnAcknowledgedCommand(t *testing.T) {
	d := receiving(t, func(o *domain.OperationCapability) { o.Efficacy = domain.EfficacyByObservation })
	command, capture := d.captured(t, accepting(202))
	d.recordCapture(t, "c-1", capture)
	d.normalized(t, "n-c", d.handovers()[0], usecase.R1D1)
	if s := d.command(t, testCommand); s.Status != domain.StatusAcknowledged {
		t.Fatalf("command = %+v, want acknowledged", s)
	}
	body := d.sim.Notify(partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied, ExternalRef: "ext-9"}).Body

	received := d.receive(t, "rcv-1", partnerReceipt("evt-p-9", body))
	handovers := d.handovers()
	d.normalized(t, "n-1", handovers[len(handovers)-1], usecase.R1D1)

	i := d.interaction(t, received.InteractionID)
	if i.Status != domain.InteractionNormalized || i.CommandID != testCommand || i.Finding != domain.FindingApplied {
		t.Fatalf("interaction = %+v, want normalized with the finding about the command", i)
	}
	if s := d.command(t, testCommand); s.Status != domain.StatusConfirmed || s.ExternalReferenceID != "ext-9" {
		t.Fatalf("command = %+v, want confirmed by the callback", s)
	}
	observation := string(received.InteractionID) + ".r1"
	if !d.observed(t, port.OutboundKey(command.OutboundKey), port.FindingApplied, observation) {
		t.Fatalf("the ledger does not keep %s, want the observation of the first revision", observation)
	}
	events := d.eventsTo(application.Destination)
	if confirmed, ok := events[len(events)-1].(domain.ExternalCommandConfirmed); !ok || confirmed.ConfirmationSource != domain.SourceCallback ||
		confirmed.InteractionID != received.InteractionID {
		t.Fatalf("last event = %+v, want ExternalCommandConfirmed by the callback", events[len(events)-1])
	}
}
