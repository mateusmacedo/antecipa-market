package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// captured dispatches the scripted command and returns the one capture it kept.
func (d *dispatching) captured(t *testing.T, directive partnersim.Directive) (domain.ExternalCommandSnapshot, domain.ExternalResponseCaptured) {
	t.Helper()
	command := d.scripted(t, directive)
	d.dispatched(t, "m-1", usecase.R1D1)
	captures := d.captures()
	if len(captures) != 1 {
		t.Fatalf("captures = %+v, want one", captures)
	}
	return command, captures[0]
}

func TestTheCaptureIsRecordedAsTheOutboundInteractionOnce(t *testing.T) {
	d := newDispatching(t, nil)
	_, capture := d.captured(t, accepting(200))

	if got := d.recordCapture(t, "c-1", capture); got != usecase.R1D1 {
		t.Fatalf("ConsumeResponseCaptured() = %v, want R1×D1", got)
	}
	if got := d.recordCapture(t, "c-1", capture); got != usecase.R2 {
		t.Fatalf("redelivery = %v, want R2 from the message inbox", got)
	}
	if got := d.recordCapture(t, "c-2", capture); got != usecase.R1D1 {
		t.Fatalf("another message of the same capture = %v, want R1×D1 replayed under record.<interactionId>", got)
	}

	i := d.interaction(t, capture.InteractionID)
	if i.Direction != domain.DirectionOutbound || i.Source != domain.InteractionSubmitResponse || i.CommandID != testCommand ||
		i.CapturedStep != domain.StepConfirm || i.ContentType != partnersim.ContentType || i.Status != domain.InteractionStored ||
		i.Request != capture.Request || i.Response != capture.Response {
		t.Fatalf("interaction = %+v, want the outbound response with the versions the store returned", i)
	}
	if handovers := d.handovers(); len(handovers) != 1 || handovers[0].InteractionID != capture.InteractionID {
		t.Fatalf("handovers = %+v, want the interaction handed to the normalization once", handovers)
	}
}

// L878-880: the first revision of a captured suspension concludes in
// schema_mismatch with the break the capture kept, and the kernel gets no
// observation, because the dispatch already applied the step.
func TestACapturedSuspensionConcludesInSchemaMismatchWithoutObservation(t *testing.T) {
	d := newDispatching(t, nil)
	command, capture := d.captured(t, partnersim.Directive{Replies: []partnersim.Reply{{Status: 200, Break: domain.MismatchMissingField}}})
	if capture.Step != domain.StepSuspend || capture.Mismatch.Kind != domain.MismatchMissingField || capture.Mismatch.Detail == "" {
		t.Fatalf("capture = %+v, want the suspension with the break of the contract", capture)
	}
	before := d.command(t, testCommand)
	d.recordCapture(t, "c-1", capture)

	d.normalized(t, "n-1", d.handovers()[0], usecase.R1D1)

	i := d.interaction(t, capture.InteractionID)
	if i.Status != domain.InteractionSchemaMismatch || i.Mismatch != capture.Mismatch {
		t.Fatalf("interaction = %+v, want schema_mismatch with the break of the capture", i)
	}
	mismatches := d.eventsTo(application.Destination)
	last := mismatches[len(mismatches)-1]
	if detected, ok := last.(domain.ExternalSchemaMismatchDetected); !ok || detected.InteractionID != capture.InteractionID || detected.PartnerID != "prt-1" {
		t.Fatalf("last event = %+v, want ExternalSchemaMismatchDetected of the interaction", last)
	}
	if after := d.command(t, testCommand); after != before || after.Status != domain.StatusUnknownOutcome {
		t.Fatalf("command = %+v, want it as the dispatch left it", after)
	}
	if d.observed(t, port.OutboundKey(command.OutboundKey), port.FindingApplied, application.ObservationIDOf(i)) {
		t.Fatalf("the ledger keeps %s, want no observation of a captured submission", application.ObservationIDOf(i))
	}
}

// callback records an inbound interaction whose raw payload the simulator
// wrote, as the edge does for ReceivePartnerInteraction.
func (d *dispatching) callback(t *testing.T, id domain.InteractionID, notice partnersim.Notice) domain.ExternalInteractionStored {
	t.Helper()
	body := d.sim.Notify(notice)
	request, err := d.raw.Put(consuming(t), ports.RawKeyOf(testConnector, id, ports.RawRequest), body)
	if err != nil {
		t.Fatalf("Put() = %v", err)
	}
	d.recorded(t, application.RecordExternalInteraction{
		InteractionID: id, ConnectorKey: testConnector, Operation: "submitEffect", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, ContentType: body.ContentType, PartnerEventID: "pe-" + string(id), Request: request,
	})
	handovers := d.handovers()
	return handovers[len(handovers)-1]
}

// acknowledged is a command the partner accepted under a by_observation
// efficacy, waiting for the callback, with its capture concluded.
func acknowledged(t *testing.T) (*dispatching, domain.ExternalCommandSnapshot) {
	t.Helper()
	d := newDispatching(t, func(o *domain.OperationCapability) { o.Efficacy = domain.EfficacyByObservation })
	command, capture := d.captured(t, accepting(202))
	d.recordCapture(t, "c-1", capture)
	d.normalized(t, "n-c", d.handovers()[0], usecase.R1D1)
	if s := d.command(t, testCommand); s.Status != domain.StatusAcknowledged {
		t.Fatalf("command = %+v, want acknowledged", s)
	}
	return d, command
}

// L882-884: the callback confirms the command through Resolve, under the id
// of the first revision of its interaction.
func TestACallbackConfirmsAnAcknowledgedCommandUnderItsFirstRevision(t *testing.T) {
	d, command := acknowledged(t)
	stored := d.callback(t, "cb-1", partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied, ExternalRef: "ext-9"})

	d.normalized(t, "n-1", stored, usecase.R1D1)

	i := d.interaction(t, "cb-1")
	if i.Status != domain.InteractionNormalized || i.CommandID != testCommand || i.Finding != domain.FindingApplied || i.ReadingHash == "" {
		t.Fatalf("interaction = %+v, want normalized with the finding about the command", i)
	}
	s := d.command(t, testCommand)
	if s.Status != domain.StatusConfirmed || s.ExternalReferenceID != "ext-9" {
		t.Fatalf("command = %+v, want confirmed with the partner's reference", s)
	}
	events := d.eventsTo(application.Destination)
	confirmed, ok := events[len(events)-1].(domain.ExternalCommandConfirmed)
	if !ok || confirmed.ConfirmationSource != domain.SourceCallback || confirmed.InteractionID != "cb-1" {
		t.Fatalf("last event = %+v, want ExternalCommandConfirmed by the callback", events[len(events)-1])
	}
	if !d.observed(t, port.OutboundKey(command.OutboundKey), port.FindingApplied, "cb-1.r1") {
		t.Fatal("the ledger does not keep cb-1.r1, want the observation of the first revision")
	}
}

func pending(source domain.InteractionSource, exchange ports.RawExchange) (ports.Reading, error) {
	reading, err := partnersim.Normalizer{}.Normalize(source, exchange)
	reading.Finding = port.FindingPending
	return reading, err
}

func applied(source domain.InteractionSource, exchange ports.RawExchange) (ports.Reading, error) {
	reading, err := partnersim.Normalizer{}.Normalize(source, exchange)
	reading.Finding, reading.ReadingHash = port.FindingApplied, reading.ReadingHash+"-corrected"
	return reading, err
}

// Decision 18: a redelivery of a concluded revision does not read the payload
// again, so a normalizer replaced in between never contradicts it.
func TestARedeliveryAfterAnotherNormalizerObservesNothingContradictory(t *testing.T) {
	d, command := acknowledged(t)
	stored := d.callback(t, "cb-1", partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied})
	d.partners.normalizeWith(pending)
	d.normalized(t, "n-1", stored, usecase.R1D1)
	events, readings := len(d.eventsTo(application.Destination)), d.partners.readings()

	d.partners.normalizeWith(applied)
	d.normalized(t, "n-1", stored, usecase.R2)
	d.normalized(t, "n-2", stored, usecase.R2)

	if got := d.partners.readings(); got != readings {
		t.Fatalf("the normalizer read %d times more, want no new reading of a concluded revision", got-readings)
	}
	if s := d.command(t, testCommand); s.Status != domain.StatusAcknowledged || len(d.eventsTo(application.Destination)) != events {
		t.Fatalf("command = %+v, want it left as the recorded finding left it, with no new event", s)
	}
	if i := d.interaction(t, "cb-1"); i.Finding != domain.FindingPending {
		t.Fatalf("interaction = %+v, want the finding of its conclusion", i)
	}
}

// commandsDown is the read of the command a process that crashed after the
// commit of the conclusion never got to.
type commandsDown struct{}

var errCrashed = errors.New("the process crashed after the commit of the conclusion")

func (commandsDown) Load(context.Context, domain.IdempotencyKey) (domain.ExternalCommandSnapshot, port.Version, error) {
	return domain.ExternalCommandSnapshot{}, 0, errCrashed
}

// Decision 18: a crash between the commit of the conclusion and the Resolve
// leaves the observation to the redelivery, which rebuilds it from what the
// conclusion recorded, with the instant and the external reference the reading
// had, not the instant the raw payload came in.
func TestARedeliveryAfterACrashBeforeResolveObservesTheReadingAsItWas(t *testing.T) {
	d, command := acknowledged(t)
	stored := d.callback(t, "cb-1", partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied, ExternalRef: "ext-9"})
	readAt := testOccurred - 7*second
	d.partners.normalizeWith(func(source domain.InteractionSource, exchange ports.RawExchange) (ports.Reading, error) {
		reading, err := partnersim.Normalizer{}.Normalize(source, exchange)
		reading.AsOf = readAt
		return reading, err
	})
	reader := d.service.CommandReader
	d.service.CommandReader = commandsDown{}
	if _, err := d.normalize(t, "n-1", stored); !errors.Is(err, errCrashed) {
		t.Fatalf("ConsumeInteractionStored(n-1) = %v, want the crash before the Resolve", err)
	}
	d.service.CommandReader = reader
	if i := d.interaction(t, "cb-1"); i.Status != domain.InteractionNormalized || i.Finding != domain.FindingApplied {
		t.Fatalf("interaction = %+v, want the conclusion committed before the crash", i)
	}

	d.normalized(t, "n-1", stored, usecase.R2)

	var got []port.Observation
	for _, o := range d.ledger.observations() {
		if o.ID == "cb-1.r1" {
			got = append(got, o)
		}
	}
	if len(got) != 1 || got[0].SourceAt != readAt || got[0].ExternalRef != "ext-9" || got[0].Finding != port.FindingApplied {
		t.Fatalf("observations of cb-1.r1 = %+v, want one with the instant %d and the external reference of the reading", got, readAt)
	}
	if s := d.command(t, testCommand); s.Status != domain.StatusConfirmed || s.ExternalReferenceID != "ext-9" {
		t.Fatalf("command = %+v, want confirmed with the partner's reference", s)
	}
}

// L894-896: the reprocessing is a new observation, r2, and the corrected
// reading confirms the command; a late message of r1 changes nothing.
func TestAReprocessingObservesTheCorrectedReadingAsANewRevision(t *testing.T) {
	d, command := acknowledged(t)
	first := d.callback(t, "cb-1", partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied})
	d.partners.normalizeWith(pending)
	d.normalized(t, "n-1", first, usecase.R1D1)

	d.partners.normalizeWith(applied)
	d.reprocessed(t, "cb-1")
	handovers := d.handovers()
	second := handovers[len(handovers)-1]
	d.normalized(t, "n-2", second, usecase.R1D1)

	if second.NormalizationRevision != 2 || d.command(t, testCommand).Status != domain.StatusConfirmed {
		t.Fatalf("handover = %+v, command = %+v, want revision 2 confirming the command", second, d.command(t, testCommand))
	}
	events := d.eventsTo(application.Destination)
	var reprocessed domain.ExternalInteractionReprocessed
	for _, e := range events {
		if r, ok := e.(domain.ExternalInteractionReprocessed); ok {
			reprocessed = r
		}
	}
	if !reprocessed.OutcomeChanged || reprocessed.NormalizationRevision != 2 {
		t.Fatalf("reprocessed = %+v, want the changed reading announced", reprocessed)
	}
	if !d.observed(t, port.OutboundKey(command.OutboundKey), port.FindingApplied, "cb-1.r2") {
		t.Fatal("the ledger does not keep cb-1.r2, want the reprocessing observed as a new observation")
	}

	before := len(d.eventsTo(application.Destination))
	d.normalized(t, "n-1", first, usecase.R2)
	if len(d.eventsTo(application.Destination)) != before {
		t.Fatal("a late message of revision 1 produced an event")
	}
}

// L886-888: a duplicate keeps its raw payload but is never normalized.
func TestADuplicateIsNeverHandedToTheNormalization(t *testing.T) {
	d, command := acknowledged(t)
	d.callback(t, "cb-1", partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied})
	handovers := len(d.handovers())

	d.recorded(t, application.RecordExternalInteraction{
		InteractionID: "cb-2", ConnectorKey: testConnector, Operation: "submitEffect", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, PartnerEventID: "pe-cb-1", Request: d.interaction(t, "cb-1").Request, DuplicateOf: "cb-1",
	})
	if got := len(d.handovers()); got != handovers {
		t.Fatalf("handovers grew to %d, want no normalization of a duplicate", got)
	}
	readings := d.partners.readings()
	d.normalized(t, "n-x", domain.ExternalInteractionStored{InteractionID: "cb-2", NormalizationRevision: 1}, usecase.R2)
	if d.partners.readings() != readings || d.command(t, testCommand).Status != domain.StatusAcknowledged {
		t.Fatal("a message naming a duplicate read it or moved the command")
	}
}

func TestTheNormalizationFailsClosedOnAMissingAdapterOrStore(t *testing.T) {
	tests := map[string]struct {
		break_ func(*dispatching)
		want   usecase.Disposition
		cat    usecase.Category
	}{
		"no adapter":        {func(d *dispatching) { d.service.Partners = ports.NoPartnerAdapters{} }, usecase.R1D3, usecase.TransientDependency},
		"store unavailable": {func(d *dispatching) { d.raw.failGet(ports.ErrRawStoreUnavailable) }, usecase.R1D3, usecase.TransientDependency},
		"payload defect":    {func(d *dispatching) { d.raw.failGet(ports.ErrRawPayloadDefect) }, usecase.R1D4, usecase.Unexpected},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			d, command := acknowledged(t)
			stored := d.callback(t, "cb-1", partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied})
			tt.break_(d)

			disposition, err := d.normalize(t, "n-1", stored)

			if disposition != tt.want || usecase.CategoryOf(err) != string(tt.cat) {
				t.Fatalf("ConsumeInteractionStored() = %v, %v, want %v by %s", disposition, err, tt.want, tt.cat)
			}
			if i := d.interaction(t, "cb-1"); i.Status != domain.InteractionStored {
				t.Fatalf("interaction = %+v, want it still stored", i)
			}
		})
	}
}

// A reading its conclusion refuses goes to the quarantine with nothing kept,
// so the same delivery concludes once the normalizer is fixed.
func TestAReadingTheConclusionRefusesIsQuarantinedUntilTheNormalizerIsFixed(t *testing.T) {
	d, command := acknowledged(t)
	stored := d.callback(t, "cb-1", partnersim.Notice{Key: port.OutboundKey(command.OutboundKey), Finding: port.FindingApplied})
	d.partners.normalizeWith(func(domain.InteractionSource, ports.RawExchange) (ports.Reading, error) {
		return ports.Reading{Outcome: domain.InteractionSchemaMismatch}, nil
	})

	disposition, err := d.normalize(t, "n-1", stored)

	if disposition != usecase.R1D4 || !errors.Is(err, application.ErrReadingRefused) {
		t.Fatalf("ConsumeInteractionStored() = %v, %v, want the quarantine of a refused reading", disposition, err)
	}
	if i := d.interaction(t, "cb-1"); i.Status != domain.InteractionStored {
		t.Fatalf("interaction = %+v, want it still stored", i)
	}
	d.partners.normalizeWith(nil)
	d.normalized(t, "n-1", stored, usecase.R1D1)
	if s := d.command(t, testCommand); s.Status != domain.StatusConfirmed {
		t.Fatalf("command = %+v, want confirmed by the fixed reading", s)
	}
}

// Decision 13: a conflict leaves the command as it is and is telemetry, not a
// failure of the delivery.
func TestAConflictingCallbackIsTelemetryWithoutFailure(t *testing.T) {
	d, command := acknowledged(t)
	key := port.OutboundKey(command.OutboundKey)
	if d.observed(t, key, port.FindingApplied, "ops-1") {
		t.Fatal("ops-1 was already kept")
	}
	stored := d.callback(t, "cb-1", partnersim.Notice{Key: key, Finding: port.FindingRejected})

	d.normalized(t, "n-1", stored, usecase.R1D1)

	if s := d.command(t, testCommand); s.Status != domain.StatusAcknowledged {
		t.Fatalf("command = %+v, want unchanged by a conflicting finding", s)
	}
	if warned := d.warnings.logged(); !slices.Contains(warned, "the observation of an interaction was not applied") {
		t.Fatalf("warnings = %v, want the conflict logged", warned)
	}
}
