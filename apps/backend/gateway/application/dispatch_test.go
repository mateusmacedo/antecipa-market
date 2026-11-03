package application_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

func accepting(status int) partnersim.Directive {
	return partnersim.Directive{Replies: []partnersim.Reply{{Status: status, ExternalRef: "ext-1"}}}
}

func TestTheDispatchSendsTheCommandAndCapturesBothRawPayloads(t *testing.T) {
	d := newDispatching(t, nil)
	command := d.scripted(t, accepting(200))

	d.dispatched(t, "m-1", usecase.R1D1)

	if s := d.command(t, testCommand); s.Status != domain.StatusConfirmed || s.Submissions != 1 {
		t.Fatalf("command = %+v, want confirmed after one submission", s)
	}
	events := d.eventsTo(application.Destination)
	requireNames(t, events, "gateway.external-command-submitted", "gateway.external-command-confirmed")
	captures := d.captures()
	if len(captures) != 1 {
		t.Fatalf("captures = %+v, want the one of the submission on %s", captures, application.InteractionDestination)
	}
	c := captures[0]
	if c.CommandID != testCommand || c.ConnectorKey != testConnector || c.Operation != "submitEffect" || c.Step != domain.StepConfirm ||
		c.ContentType != partnersim.ContentType || c.Request.VersionID == "" || c.Response.VersionID == "" {
		t.Fatalf("capture = %+v, want both versions the store returned, the step and the content type", c)
	}
	if c.Request.Ref != ports.RawKeyOf(testConnector, c.InteractionID, ports.RawRequest) ||
		d.raw.versions(c.Request.Ref) != 1 || d.raw.versions(c.Response.Ref) != 1 {
		t.Fatalf("capture = %+v, want the request and the response kept under the key of decision 16", c)
	}
	if confirmed := events[1].(domain.ExternalCommandConfirmed); confirmed.InteractionID != c.InteractionID {
		t.Fatalf("confirmed under interaction %q, want the captured %q", confirmed.InteractionID, c.InteractionID)
	}
	if submitted := d.sim.Partner().Submitted(); !slices.Equal(submitted, []port.OutboundKey{port.OutboundKey(command.OutboundKey)}) {
		t.Fatalf("partner received %v, want one submission under the outbound key", submitted)
	}
}

// Decision 17: without the request kept, the partner is never called.
func TestTheDispatchCallsNoPartnerWhenTheStoreCannotKeepTheRequest(t *testing.T) {
	d := newDispatching(t, nil)
	command := d.scripted(t, accepting(200))
	d.raw.failPut(ports.RawRequest, ports.ErrRawStoreUnavailable)

	disposition, err := d.dispatch(t, "m-1")

	if disposition != usecase.R1D3 || usecase.CategoryOf(err) != string(usecase.TransientDependency) {
		t.Fatalf("DispatchExternalCommand() = %v, %v, want a redelivery as a transient dependency", disposition, err)
	}
	if submitted := d.sim.Partner().Submitted(); len(submitted) != 0 {
		t.Fatalf("partner received %v, want no external call", submitted)
	}
	if s := d.command(t, testCommand); s.Status != domain.StatusCreated || len(d.eventsTo(application.Destination)) != 0 || len(d.captures()) != 0 {
		t.Fatalf("command = %+v, want created with no event and no capture", s)
	}

	d.raw.failPut(ports.RawRequest, nil)
	d.dispatched(t, "m-1", usecase.R1D1)
	if submitted := d.sim.Partner().Submitted(); !slices.Equal(submitted, []port.OutboundKey{port.OutboundKey(command.OutboundKey)}) {
		t.Fatalf("partner received %v, want the redelivery under the same outbound key", submitted)
	}
}

func TestTheDispatchDiscardsTheIntentOfACommandItCannotSend(t *testing.T) {
	statuses := map[string]func(*domain.ExternalCommandSnapshot){
		"cancelled":     func(s *domain.ExternalCommandSnapshot) { s.Status = domain.StatusCancelled },
		"confirmed":     func(s *domain.ExternalCommandSnapshot) { s.Status, s.Submissions = domain.StatusConfirmed, 1 },
		"acknowledged":  func(s *domain.ExternalCommandSnapshot) { s.Status, s.Submissions = domain.StatusAcknowledged, 1 },
		"unknown":       func(s *domain.ExternalCommandSnapshot) { s.Status, s.Submissions = domain.StatusUnknownOutcome, 1 },
		"final failure": func(s *domain.ExternalCommandSnapshot) { s.Status, s.Submissions = domain.StatusFinalFailure, 1 },
	}
	for name, change := range statuses {
		t.Run(name, func(t *testing.T) {
			d := newDispatching(t, nil)
			d.scripted(t, accepting(200))
			d.force(t, testCommand, change)
			before := len(d.entries())

			d.dispatched(t, "m-1", usecase.R1D2)

			if submitted := d.sim.Partner().Submitted(); len(submitted) != 0 || len(d.entries()) != before {
				t.Fatalf("partner received %v and the outbox grew to %d, want no call and nothing written", submitted, len(d.entries()))
			}
		})
	}
}

// L762-764: the cancellation reaches the handler before the send.
func TestTheDispatchDiscardsTheIntentOfACancelledCommand(t *testing.T) {
	d := newDispatching(t, nil)
	d.scripted(t, accepting(200))
	must(d.service.CancelExternalCommand(withExecution(t), application.CancelExternalCommand{IdempotencyKey: testCommand})).accepted(t)

	d.dispatched(t, "m-1", usecase.R1D2)

	if s := d.command(t, testCommand); s.Status != domain.StatusCancelled || len(d.sim.Partner().Submitted()) != 0 {
		t.Fatalf("command = %+v, partner = %v, want cancelled with no external call", s, d.sim.Partner().Submitted())
	}
}

// Decision 14: the suspension lasts until the rehomologation, so the intent is
// held and acknowledged, never redelivered into the quarantine.
func TestTheDispatchHoldsACommandWhoseOperationIsSuspended(t *testing.T) {
	d := newDispatching(t, nil)
	command := d.scripted(t, accepting(200))
	must(d.service.SuspendOperation(withExecution(t), application.SuspendOperation{
		ConnectorKey: testConnector, Operation: "submitEffect", Cause: domain.CauseSchemaMismatch,
	})).accepted(t)
	d.clock.advance(second)

	d.dispatched(t, "m-1", usecase.R1D1)
	d.clock.advance(second)
	d.dispatched(t, "m-1", usecase.R1D1)

	if s := d.command(t, testCommand); s.Status != domain.StatusCreated || s.DispatchHeldSince != testOccurred+second {
		t.Fatalf("command = %+v, want created and held since the first delivery", s)
	}
	if submitted := d.sim.Partner().Submitted(); len(submitted) != 0 {
		t.Fatalf("partner received %v, want no external call while suspended", submitted)
	}

	d.enabled(t, "submitEffect", true, 2)
	if report := d.sweep(t); report.ResendRequested != 1 {
		t.Fatalf("sweep = %+v, want the release of the held dispatch", report)
	}
	d.dispatched(t, "m-2", usecase.R1D1)
	if s := d.command(t, testCommand); s.Status != domain.StatusConfirmed || s.DispatchHeldSince != 0 {
		t.Fatalf("command = %+v, want sent once released", s)
	}
	if submitted := d.sim.Partner().Submitted(); !slices.Equal(submitted, []port.OutboundKey{port.OutboundKey(command.OutboundKey)}) {
		t.Fatalf("partner received %v, want one submission after the release", submitted)
	}
}

func (d *dispatching) sweep(t *testing.T) application.SweepReport {
	t.Helper()
	report, err := d.service.SweepExternalCommands(withExecution(t))
	if err != nil {
		t.Fatalf("SweepExternalCommands() = %v", err)
	}
	return report
}

// IDM-02: a command key of 100 characters and a long message id do not fit
// hold.<key>.<message>, so the policy shortens it, and a redelivery replays.
func TestHoldDispatchFitsTheWorstCaseOfTheKeyAndTheMessage(t *testing.T) {
	h := ready(t)
	longest := domain.IdempotencyKey(strings.Repeat("k", 100))
	h.created(t, longest, 1)
	message := port.MessageID("gateway-" + strings.Repeat("9", 120))
	if plain := "hold." + string(longest) + "." + string(message); port.ValidIdempotencyKey(plain) {
		t.Fatalf("the plain key %q fits IDM-02: the case is not the worst one", plain)
	}

	first := must(h.service.HoldDispatch(withExecution(t), application.HoldDispatch{IdempotencyKey: longest, MessageID: message})).accepted(t)
	h.clock.advance(second)
	again := must(h.service.HoldDispatch(withExecution(t), application.HoldDispatch{IdempotencyKey: longest, MessageID: message})).accepted(t)

	if first != again || h.command(t, longest).DispatchHeldSince != testOccurred {
		t.Fatalf("hold = %+v then %+v, want the replay of the first under the derived key", first, again)
	}
	key, err := h.service.DerivedKeys.Key("hold", string(longest), string(message))
	if err != nil || !port.ValidIdempotencyKey(key) {
		t.Fatalf("Key(hold) = %q, %v, want a key within IDM-02", key, err)
	}
}

// L666-668: a retryable class ends the attempt before any step, so the command
// stays created, cancellable, while each response is still recorded.
func TestAnIntentThatExhaustsTheRedeliveryLeavesTheCommandCancellable(t *testing.T) {
	d := newDispatching(t, nil)
	upstream := partnersim.Reply{Status: 503}
	command := d.scripted(t, partnersim.Directive{Replies: []partnersim.Reply{upstream, upstream, upstream}})

	for range 3 {
		disposition, err := d.dispatch(t, "m-1")
		if disposition != usecase.R1D3 || usecase.CategoryOf(err) != string(usecase.TransientDependency) {
			t.Fatalf("DispatchExternalCommand() = %v, %v, want a redelivery of the same message", disposition, err)
		}
	}

	if s := d.command(t, testCommand); s.Status != domain.StatusCreated || s.Submissions != 0 || len(d.eventsTo(application.Destination)) != 0 {
		t.Fatalf("command = %+v, want created with no event: no request produced an effect", s)
	}
	if submitted := d.sim.Partner().Submitted(); len(submitted) != 3 || submitted[2] != port.OutboundKey(command.OutboundKey) {
		t.Fatalf("partner received %v, want three submissions under the same outbound key", submitted)
	}
	recorded := d.handovers()
	if len(recorded) != 3 || len(d.captures()) != 0 {
		t.Fatalf("handovers = %+v, captures = %+v, want each response recorded directly", recorded, d.captures())
	}
	for _, r := range recorded {
		i := d.interaction(t, r.InteractionID)
		if i.Direction != domain.DirectionOutbound || i.Source != domain.InteractionSubmitResponse || i.CommandID != testCommand ||
			i.CapturedStep != domain.StepRetry || i.Response.VersionID == "" {
			t.Fatalf("interaction = %+v, want the outbound response with the retry step", i)
		}
	}

	must(d.service.CancelExternalCommand(withExecution(t), application.CancelExternalCommand{IdempotencyKey: testCommand})).accepted(t)
	d.dispatched(t, "m-1", usecase.R1D2)
	if submitted := d.sim.Partner().Submitted(); len(submitted) != 3 {
		t.Fatalf("partner received %v after the cancellation, want no fourth call", submitted)
	}
}

// Decision 17: a response the store lost leaves the outcome as the partner
// gave it, with no capture, and the capture operation fails in the metrics.
func TestALostResponseKeepsTheOutcomeWithoutACapture(t *testing.T) {
	d := newDispatching(t, nil)
	d.scripted(t, accepting(200))
	d.raw.failPut(ports.RawResponse, ports.ErrRawStoreUnavailable)

	d.dispatched(t, "m-1", usecase.R1D1)

	if s := d.command(t, testCommand); s.Status != domain.StatusConfirmed {
		t.Fatalf("command = %+v, want the outcome of the submission", s)
	}
	if len(d.captures()) != 0 || len(d.handovers()) != 0 {
		t.Fatalf("captures = %+v, handovers = %+v, want no outbound interaction", d.captures(), d.handovers())
	}
	if got := d.telemetry.outcomes(application.OperationCaptureResponse); !slices.Equal(got, []port.OutcomeCategory{port.OutcomeFailed}) {
		t.Fatalf("capture operation ended %v, want one failure counted", got)
	}
}

// Decision 21: with no adapter the dispatch fails closed as a dependency, and
// no attempt is begun.
func TestTheDispatchFailsClosedWithoutAPartnerAdapter(t *testing.T) {
	d := newDispatching(t, nil)
	d.scripted(t, accepting(200))
	d.service.Partners = ports.NoPartnerAdapters{}

	disposition, err := d.dispatch(t, "m-1")

	if disposition != usecase.R1D3 || !errors.Is(err, ports.ErrPartnerAdapterUnavailable) ||
		usecase.CategoryOf(err) != string(usecase.TransientDependency) {
		t.Fatalf("DispatchExternalCommand() = %v, %v, want an unavailable dependency", disposition, err)
	}
	stale, err := d.service.Attempts.Stale(consuming(t), d.clock.Now()+staleAfter, port.AttemptCursor{}, 10)
	if err != nil || len(stale) != 0 {
		t.Fatalf("attempts = %+v, %v, want none begun", stale, err)
	}
}
