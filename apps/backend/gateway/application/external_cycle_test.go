package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	partnerCode    = "E42"
	partnerMessage = "registro recusado pela registradora"
	staleAfter     = 10 * second
)

// cycle drives one command through the kernel's external cycle against the
// scripted partner of the kernel (04:G12, KIT-12): Dispatch stands in for the
// dispatch handler, which is the slice that captures the raw response.
type cycle struct {
	h       *harness
	partner *serviceskit.Partner
	key     port.OutboundKey
}

func newCycle(t *testing.T, change func(*domain.OperationCapability), homologatedQuery bool) *cycle {
	t.Helper()
	operation := simulatorOperation("submitEffect")
	if change != nil {
		change(&operation)
	}
	h := newHarness(t)
	h.registered(t)
	h.enabled(t, "submitEffect", homologatedQuery, 1)
	h.declared(t, operation)
	command := h.created(t, testCommand, 1)
	return &cycle{h: h, partner: serviceskit.NewPartner(h.clock, &memory.SequenceIDs{Prefix: "obs-"}), key: port.OutboundKey(command.OutboundKey)}
}

func (c *cycle) script(replies ...serviceskit.Reply) *cycle {
	c.partner.Script(c.key, replies...)
	return c
}

func (c *cycle) dispatch(t *testing.T, message port.MessageID) (usecase.Disposition, error) {
	t.Helper()
	command := c.h.command(t, testCommand)
	ext := application.CycleOf(command)
	return usecase.Dispatch(withExecution(t), usecase.ExternalDispatch[application.Resources]{
		UoW:    c.h.service.UoW,
		Store:  memory.NewAttemptStore(c.h.store),
		Ledger: application.LedgerOf,
		Inbox:  func(res application.Resources) port.Inbox { return res.Inbox },
		Receipt: port.Receipt{
			Consumer: "gateway", MessageID: message, MessageType: "com.company.gateway.external-command-dispatch-requested.v1",
			PayloadHash: "h-" + string(message), ReceivedAt: c.h.clock.Now(),
		},
		Command:         c.partner,
		Key:             ext.Key,
		Payload:         []byte(command.Payload),
		Profile:         ext.Profile,
		QueryAvailable:  ext.QueryAvailable && c.partner.QueryAvailable(),
		StaleAfter:      int64(staleAfter),
		Clock:           c.h.clock,
		SettleContext:   func(ctx context.Context) (context.Context, context.CancelFunc) { return context.WithCancel(ctx) },
		Instrumentation: port.NoInstrumentation(),
		Apply:           c.h.service.SubmissionStep(testCommand, partnerCode, partnerMessage),
	})
}

func (c *cycle) dispatched(t *testing.T, message port.MessageID) {
	t.Helper()
	disposition, err := c.dispatch(t, message)
	if err != nil || disposition != usecase.R1D1 {
		t.Fatalf("Dispatch(%s) = %v, %v, want R1×D1", message, disposition, err)
	}
}

func (c *cycle) resolve(t *testing.T, key string, resolution domain.Resolution) result[domain.CommandStatusResponse] {
	t.Helper()
	return must(c.h.service.ResolveUnknownOutcome(withKey(t, key), application.ResolveUnknownOutcome{
		IdempotencyKey: testCommand, Resolution: resolution, QueryInteractionID: "int-9", ExternalReferenceID: "ext-9",
		Justification: "registry confirmed it by the incident channel",
	}))
}

func (c *cycle) sweep(t *testing.T) application.SweepReport {
	t.Helper()
	report, err := c.h.service.SweepExternalCommands(withExecution(t))
	if err != nil {
		t.Fatalf("SweepExternalCommands() = %v", err)
	}
	return report
}

// events lists the facts of gateway.events in outbox order.
func (c *cycle) events() []kernel.DomainEvent {
	var out []kernel.DomainEvent
	for _, entry := range c.h.entries() {
		if entry.Intent.Destination == application.Destination {
			out = append(out, entry.Event)
		}
	}
	return out
}

func (c *cycle) intents() []domain.ExternalCommandDispatchRequested {
	var out []domain.ExternalCommandDispatchRequested
	for _, entry := range c.h.entries() {
		if entry.Intent.Destination == application.DispatchDestination {
			out = append(out, entry.Event.(domain.ExternalCommandDispatchRequested))
		}
	}
	return out
}

func names(events []kernel.DomainEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.EventName())
	}
	return out
}

func requireNames(t *testing.T, got []kernel.DomainEvent, want ...string) {
	t.Helper()
	if !slices.Equal(names(got), want) {
		t.Fatalf("events = %v, want %v", names(got), want)
	}
}

func onSubmit(o *domain.OperationCapability)      { o.Efficacy = domain.EfficacyOnSubmit }
func byObservation(o *domain.OperationCapability) { o.Efficacy = domain.EfficacyByObservation }

func TestASubmissionAcceptedOnSubmitConfirmsWithTwoFacts(t *testing.T) {
	c := newCycle(t, onSubmit, true).script(serviceskit.Reply{Class: port.ClassAccepted, ExternalRef: "ext-1", Applied: true})

	c.dispatched(t, "m-1")

	s := c.h.command(t, testCommand)
	if s.Status != domain.StatusConfirmed || s.Submissions != 1 {
		t.Fatalf("command = %+v, want confirmed after one submission", s)
	}
	requireNames(t, c.events(), "gateway.external-command-submitted", "gateway.external-command-confirmed")
}

func TestASubmissionAcceptedByObservationOnlyAcknowledges(t *testing.T) {
	c := newCycle(t, byObservation, true).script(serviceskit.Reply{Class: port.ClassAccepted})

	c.dispatched(t, "m-1")

	if s := c.h.command(t, testCommand); s.Status != domain.StatusAcknowledged || s.ConfirmedAt != 0 {
		t.Fatalf("command = %+v, want acknowledged and no confirmation", s)
	}
	requireNames(t, c.events(), "gateway.external-command-submitted", "gateway.external-command-acknowledged")
}

func TestARetryableSubmissionLeavesTheCommandCreatedForTheRedelivery(t *testing.T) {
	c := newCycle(t, onSubmit, true).script(
		serviceskit.Reply{Class: port.ClassRetryableTransport},
		serviceskit.Reply{Class: port.ClassRetryableUpstream},
		serviceskit.Reply{Class: port.ClassRetryableUpstream},
	)

	for range 3 {
		disposition, err := c.dispatch(t, "m-1")
		var failure *usecase.Failure
		if disposition != usecase.R1D3 || !errors.As(err, &failure) || failure.Category() != usecase.TransientDependency {
			t.Fatalf("Dispatch() = %v, %v, want a redelivery of the same message", disposition, err)
		}
	}

	if s := c.h.command(t, testCommand); s.Status != domain.StatusCreated || s.Submissions != 0 {
		t.Fatalf("command = %+v, want created: no retryable reply reaches the domain", s)
	}
	if len(c.events()) != 0 {
		t.Fatalf("events = %v, want none", names(c.events()))
	}
	if submitted := c.partner.Submitted(); len(submitted) != 3 || submitted[0] != c.key || submitted[2] != c.key {
		t.Fatalf("submitted keys = %v, want the same outbound key each time", submitted)
	}
	must(c.h.service.CancelExternalCommand(withExecution(t), application.CancelExternalCommand{IdempotencyKey: testCommand})).accepted(t)
}

func TestAFinalBusinessRefusalFailsWithThePartnerWords(t *testing.T) {
	c := newCycle(t, onSubmit, true).script(serviceskit.Reply{Class: port.ClassFinalBusiness})

	c.dispatched(t, "m-1")

	if s := c.h.command(t, testCommand); s.Status != domain.StatusFinalFailure || s.FailureCategory != domain.CategoryFinalBusiness {
		t.Fatalf("command = %+v", s)
	}
	events := c.events()
	requireNames(t, events, "gateway.external-command-submitted", "gateway.external-command-failed")
	if failed := events[1].(domain.ExternalCommandFailed); failed.PartnerCode != partnerCode || failed.PartnerMessage != partnerMessage {
		t.Fatalf("ExternalCommandFailed = %+v, want the partner's original code and text", failed)
	}
}

func TestATimeoutAfterWritingIsAnUnknownOutcomeToQuery(t *testing.T) {
	c := newCycle(t, onSubmit, true).script(serviceskit.LostAfterEffect())

	c.dispatched(t, "m-1")

	s := c.h.command(t, testCommand)
	if s.Status != domain.StatusUnknownOutcome || s.WaitingSince != testOccurred || s.NextQueryAt != testOccurred+60*second || s.ReconciliationRequired {
		t.Fatalf("command = %+v, want unknown_outcome with the next query scheduled", s)
	}
	requireNames(t, c.events(), "gateway.external-command-submitted", "gateway.external-outcome-unknown")
}

func TestAnAmbiguityWithoutStateQueryGoesToHumanReconciliation(t *testing.T) {
	c := newCycle(t, onSubmit, false).script(serviceskit.Reply{Class: port.ClassUnknownAmbiguous})

	c.dispatched(t, "m-1")

	if s := c.h.command(t, testCommand); s.Status != domain.StatusUnknownOutcome || !s.ReconciliationRequired {
		t.Fatalf("command = %+v, want a reconciliation required", s)
	}
	if unknown := c.events()[1].(domain.ExternalOutcomeUnknown); !unknown.RequiresHumanResolution {
		t.Fatalf("ExternalOutcomeUnknown = %+v, want requiresHumanResolution", unknown)
	}
}

func TestASchemaMismatchSuspendsTheCommandUntilItIsQueried(t *testing.T) {
	c := newCycle(t, onSubmit, true).script(serviceskit.Reply{Class: port.ClassSchemaMismatch})

	c.dispatched(t, "m-1")

	if s := c.h.command(t, testCommand); s.Status != domain.StatusUnknownOutcome || s.FailureCategory != domain.CategorySchemaMismatch || s.ReconciliationRequired {
		t.Fatalf("command = %+v, want unknown_outcome by schema mismatch, waiting for a query", s)
	}
}

func TestTheSweepAbandonsABegunAttemptAsSentWithoutResending(t *testing.T) {
	c := newCycle(t, onSubmit, true)
	store := memory.NewAttemptStore(c.h.store)
	if _, err := store.Begin(withExecution(t), c.key, "m-1", c.h.clock.Now()); err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	c.h.clock.advance(staleAfter)
	stale, err := store.Stale(withExecution(t), c.h.clock.Now()-staleAfter, port.AttemptCursor{}, 10)
	if err != nil || len(stale) != 1 {
		t.Fatalf("Stale() = %v, %v", stale, err)
	}

	finished, err := c.h.service.AbandonExternalAttempt(withExecution(t), stale[0])

	if err != nil || finished.Result != port.FinishRecorded {
		t.Fatalf("AbandonExternalAttempt() = %+v, %v", finished, err)
	}
	if s := c.h.command(t, testCommand); s.Status != domain.StatusUnknownOutcome || s.FailureCategory != domain.CategoryUnknownSent || s.Submissions != 1 {
		t.Fatalf("command = %+v, want unknown_outcome presumed sent", s)
	}
	if len(c.partner.Submitted()) != 0 {
		t.Fatalf("the abandonment called the partner: %v", c.partner.Submitted())
	}
}

func TestAbandonRefusesAnAttemptOfNoCommand(t *testing.T) {
	c := newCycle(t, onSubmit, true)

	_, err := c.h.service.AbandonExternalAttempt(withExecution(t), port.Attempt{Tenant: testTenant, Key: "no-such-key", N: 1})

	if !errors.Is(err, application.ErrAttemptWithoutCommand) {
		t.Fatalf("AbandonExternalAttempt() = %v, want ErrAttemptWithoutCommand", err)
	}
}

func TestACancellationRacingTheSubmissionYieldsToTheExternalFact(t *testing.T) {
	c := newCycle(t, onSubmit, true).script(serviceskit.Reply{Class: port.ClassAccepted, Applied: true})
	must(c.h.service.CancelExternalCommand(withExecution(t), application.CancelExternalCommand{IdempotencyKey: testCommand})).accepted(t)

	c.dispatched(t, "m-1")

	if got := c.h.command(t, testCommand).Status; got != domain.StatusConfirmed {
		t.Fatalf("Status = %q, want confirmed", got)
	}
	requireNames(t, c.events(), "gateway.external-command-submitted", "gateway.external-command-confirmed")
}

func TestAConfirmedCommandTakesNoFurtherStep(t *testing.T) {
	c := newCycle(t, onSubmit, true).script(serviceskit.Reply{Class: port.ClassAccepted}, serviceskit.Reply{Class: port.ClassAccepted})
	c.dispatched(t, "m-1")

	_, err := c.dispatch(t, "m-2")

	if !errors.Is(err, application.ErrStepRefused) {
		t.Fatalf("Dispatch() over a confirmed command = %v, want the step refused", err)
	}
	c.resolve(t, "r-1", domain.ResolutionConfirmed).refused(t, string(domain.CodeExternalCommandNotWaiting))
}

func TestAHumanAbsenceRetriesWithTheSameKeyAfterTheQuiescence(t *testing.T) {
	c := newCycle(t, onSubmit, false).script(serviceskit.Reply{Class: port.ClassUnknownAmbiguous}, serviceskit.Reply{Class: port.ClassAccepted, Applied: true})
	c.dispatched(t, "m-1")
	before := len(c.events())

	got := c.resolve(t, "r-1", domain.ResolutionNotReceived).accepted(t)

	s := c.h.command(t, testCommand)
	if got.Status != domain.StatusRetryableFailure || s.ResendAfter != testOccurred+30*second || s.ReconciliationRequired || s.WaitingSince != 0 {
		t.Fatalf("command = %+v, want a retryable failure after the quiescence", s)
	}
	if len(c.events()) != before {
		t.Fatalf("events = %v, want none for an absence", names(c.events()[before:]))
	}
	must(c.h.service.RequestResend(withKey(t, "early"), application.RequestResend{IdempotencyKey: testCommand})).refused(t, string(domain.CodeExternalCommandQuiescenceNotElapsed))

	c.h.clock.advance(30 * second)
	if report := c.sweep(t); report.ResendRequested != 1 {
		t.Fatalf("SweepExternalCommands() = %+v, want one resend", report)
	}
	intents := c.intents()
	if len(intents) != 2 || intents[1].IdempotencyKey != testCommand {
		t.Fatalf("dispatch intents = %+v, want the resend under the same IntentID", intents)
	}
	c.dispatched(t, "m-2")
	if submitted := c.partner.Submitted(); len(submitted) != 2 || submitted[1] != c.key {
		t.Fatalf("submitted keys = %v, want the resend with the same outbound key", submitted)
	}
	if s := c.h.command(t, testCommand); s.Status != domain.StatusConfirmed || s.Submissions != 2 {
		t.Fatalf("command = %+v, want confirmed after the second submission", s)
	}
}

func TestAHumanConfirmationOfAnExpiredAcknowledgementConfirms(t *testing.T) {
	c := newCycle(t, byObservation, true).script(serviceskit.Reply{Class: port.ClassAccepted})
	c.dispatched(t, "m-1")
	c.h.clock.advance(3600 * second)
	if report := c.sweep(t); report.AmbiguityExpired != 1 {
		t.Fatalf("SweepExternalCommands() = %+v, want the wait escalated", report)
	}
	expired := c.events()[2].(domain.ExternalCommandAmbiguityExpired)
	if expired.Status != domain.StatusAcknowledged || expired.PartnerID != "prt-1" {
		t.Fatalf("ExternalCommandAmbiguityExpired = %+v", expired)
	}

	got := c.resolve(t, "r-1", domain.ResolutionConfirmed).accepted(t)

	if got.Status != domain.StatusConfirmed {
		t.Fatalf("Status = %q, want confirmed", got.Status)
	}
	confirmed := c.events()[3].(domain.ExternalCommandConfirmed)
	if confirmed.ConfirmationSource != domain.SourceHuman || confirmed.InteractionID != "int-9" {
		t.Fatalf("ExternalCommandConfirmed = %+v, want the human source and the evidence", confirmed)
	}
}

func TestAHumanResolutionNeedsEvidenceAndJustification(t *testing.T) {
	c := newCycle(t, onSubmit, false).script(serviceskit.Reply{Class: port.ClassUnknownAmbiguous})
	c.dispatched(t, "m-1")
	resolve := func(change func(*application.ResolveUnknownOutcome)) result[domain.CommandStatusResponse] {
		cmd := application.ResolveUnknownOutcome{IdempotencyKey: testCommand, Resolution: domain.ResolutionConfirmed, QueryInteractionID: "int-9", Justification: "j"}
		change(&cmd)
		return must(c.h.service.ResolveUnknownOutcome(withExecution(t), cmd))
	}

	resolve(func(c *application.ResolveUnknownOutcome) { c.QueryInteractionID = "" }).refused(t, string(domain.CodeExternalCommandQueryEvidenceMissing))
	resolve(func(c *application.ResolveUnknownOutcome) { c.Justification = "" }).refused(t, string(domain.CodeExternalCommandJustificationMissing))

	if s := c.h.command(t, testCommand); s.Status != domain.StatusUnknownOutcome {
		t.Fatalf("command = %+v, want it still waiting", s)
	}
}

func TestTheSweepExpiresEachEpisodeOnceFromItsOwnStart(t *testing.T) {
	c := newCycle(t, onSubmit, false).script(serviceskit.Reply{Class: port.ClassUnknownSent}, serviceskit.Reply{Class: port.ClassUnknownSent})
	c.dispatched(t, "m-1")

	must(c.h.service.ExpireAmbiguity(withKey(t, "early"), application.ExpireAmbiguity{IdempotencyKey: testCommand})).refused(t, string(domain.CodeExternalCommandAmbiguityDeadlineNotReached))

	c.h.clock.advance(3600 * second)
	first := c.sweep(t)
	again := c.sweep(t)
	if first.AmbiguityExpired != 1 || again.AmbiguityExpired != 0 {
		t.Fatalf("sweeps = %+v then %+v, want one expiry", first, again)
	}
	if s := c.h.command(t, testCommand); !s.ReconciliationRequired || s.AmbiguityExpiredAt != c.h.clock.Now() {
		t.Fatalf("command = %+v", s)
	}

	c.resolve(t, "r-1", domain.ResolutionNotReceived).accepted(t)
	c.h.clock.advance(30 * second)
	c.sweep(t)
	c.dispatched(t, "m-2")
	reopened := c.h.clock.Now()
	c.h.clock.advance(3600 * second)
	if report := c.sweep(t); report.AmbiguityExpired != 1 {
		t.Fatalf("SweepExternalCommands() = %+v, want the new episode escalated", report)
	}

	var expiries []domain.ExternalCommandAmbiguityExpired
	for _, e := range c.events() {
		if expired, ok := e.(domain.ExternalCommandAmbiguityExpired); ok {
			expiries = append(expiries, expired)
		}
	}
	if len(expiries) != 2 || expiries[1].WaitingSince != reopened {
		t.Fatalf("expiries = %+v, want a second one counted from %d", expiries, reopened)
	}
}
