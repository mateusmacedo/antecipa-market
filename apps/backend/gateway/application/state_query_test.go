package application_test

import (
	"fmt"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// unknown is a command whose response the partner lost after applying it,
// waiting in unknown_outcome for the state query due at nextQueryAt.
func unknown(t *testing.T, queries ...partnersim.Answer) (*dispatching, domain.ExternalCommandSnapshot) {
	t.Helper()
	d := newDispatching(t, nil)
	d.scripted(t, partnersim.Directive{Replies: []partnersim.Reply{{Fault: partnersim.FaultLost, Applied: true}}, Queries: queries})
	d.dispatched(t, "m-1", usecase.R1D1)
	s := d.command(t, testCommand)
	if s.Status != domain.StatusUnknownOutcome || s.NextQueryAt == 0 || s.ReconciliationRequired {
		t.Fatalf("command = %+v, want unknown_outcome waiting for its state query", s)
	}
	return d, s
}

func (d *dispatching) queryID(t *testing.T, s domain.ExternalCommandSnapshot) domain.InteractionID {
	t.Helper()
	id, err := d.service.DerivedKeys.QueryInteractionID(s.Key, s.NextQueryAt)
	if err != nil {
		t.Fatalf("QueryInteractionID() = %v", err)
	}
	return id
}

// L898-900 and L694-696: the sweep keeps both raw sides of the query, records
// the state_query interaction and the finding reaches the command through the
// normalization, without a resend.
func TestTheSweepQueriesTheStateOfACommandDueAndTheNormalizationConfirmsIt(t *testing.T) {
	d, s := unknown(t, partnersim.Answer{Finding: "applied", ExternalRef: "ext-q"})
	d.clock.advance(61 * second)
	id := d.queryID(t, s)

	if report := d.sweep(t); report.StateQueried != 1 || report.Failed != 0 {
		t.Fatalf("SweepExternalCommands() = %+v, want one state query", report)
	}

	i := d.interaction(t, id)
	if i.Direction != domain.DirectionOutbound || i.Source != domain.InteractionStateQuery || i.CommandID != testCommand ||
		i.Status != domain.InteractionStored || i.ContentType != partnersim.ContentType || i.Request.VersionID == "" || i.Response.VersionID == "" {
		t.Fatalf("interaction = %+v, want the stored state query of the command", i)
	}
	for _, part := range []ports.RawPart{ports.RawRequest, ports.RawResponse} {
		if n := d.raw.versions(ports.RawKeyOf(testConnector, id, part)); n != 1 {
			t.Fatalf("%s versions = %d, want the raw %s kept once", part, n, part)
		}
	}
	handovers := d.handovers()
	stored := handovers[len(handovers)-1]
	if stored.InteractionID != id || stored.NormalizationRevision != 1 {
		t.Fatalf("last handover = %+v, want the query handed to the normalization", stored)
	}
	if got := d.command(t, testCommand); got.Status != domain.StatusUnknownOutcome {
		t.Fatalf("command = %+v, want it untouched until the normalization", got)
	}

	d.normalized(t, "n-q", stored, usecase.R1D1)

	got := d.command(t, testCommand)
	if got.Status != domain.StatusConfirmed || got.Submissions != 1 || got.ExternalReferenceID != "ext-q" {
		t.Fatalf("command = %+v, want confirmed by the query, never resent", got)
	}
	events := d.eventsTo(application.Destination)
	confirmed, ok := events[len(events)-1].(domain.ExternalCommandConfirmed)
	if !ok || confirmed.ConfirmationSource != domain.SourceStateQuery || confirmed.InteractionID != id {
		t.Fatalf("last event = %+v, want ExternalCommandConfirmed by the state query", events[len(events)-1])
	}
	if intents := d.eventsTo(application.DispatchDestination); len(intents) != 1 {
		t.Fatalf("intents = %+v, want only the first dispatch", intents)
	}
}

// Repeating the sweep before the normalization asks the partner nothing: the
// query of one nextQueryAt is made once.
func TestARepeatedSweepDoesNotQueryTheSameNextQueryAtAgain(t *testing.T) {
	d, s := unknown(t, partnersim.Answer{Finding: "applied"}, partnersim.Answer{Finding: "applied"})
	d.clock.advance(61 * second)
	id := d.queryID(t, s)

	first, again := d.sweep(t), d.sweep(t)

	if first.StateQueried != 1 || again.StateQueried != 0 || again.Failed != 0 {
		t.Fatalf("sweeps = %+v then %+v, want one query", first, again)
	}
	if n := d.raw.versions(ports.RawKeyOf(testConnector, id, ports.RawResponse)); n != 1 {
		t.Fatalf("response versions = %d, want the partner asked once", n)
	}
}

// L698-700: a command of a suspended operation is not queried until
// EnableOperation lifts the suspension.
func TestTheSweepDoesNotQueryACommandOfASuspendedOperation(t *testing.T) {
	d, s := unknown(t, partnersim.Answer{Finding: "applied"})
	d.clock.advance(61 * second)
	disposition, err := d.service.ConsumeSchemaMismatchDetected(withExecution(t), mismatchDelivery("evt-m", "hm"))
	consumed(t, disposition, err, usecase.R1D1)

	if report := d.sweep(t); report.StateQueried != 0 || report.Failed != 0 {
		t.Fatalf("sweep while suspended = %+v, want no query", report)
	}
	if n := d.raw.versions(ports.RawKeyOf(testConnector, d.queryID(t, s), ports.RawRequest)); n != 0 {
		t.Fatalf("request versions = %d, want the partner never asked", n)
	}

	d.enabled(t, "submitEffect", true, 2)

	if report := d.sweep(t); report.StateQueried != 1 {
		t.Fatalf("sweep after the rehomologation = %+v, want the query", report)
	}
}

// Without a reader or the raw store the query fails closed, as a failure of
// the sweep, before any call to the partner.
func TestTheStateQueryFailsClosedWithoutAdapterOrStore(t *testing.T) {
	tests := map[string]func(*dispatching){
		"no adapter":   func(d *dispatching) { d.service.Partners = ports.NoPartnerAdapters{} },
		"no raw store": func(d *dispatching) { d.service.Raw = nil },
	}
	for name, unavailable := range tests {
		t.Run(name, func(t *testing.T) {
			d, s := unknown(t, partnersim.Answer{Finding: "applied"})
			d.clock.advance(61 * second)
			unavailable(d)

			if report := d.sweep(t); report.StateQueried != 0 || report.Failed != 1 {
				t.Fatalf("SweepExternalCommands() = %+v, want the query counted as failed", report)
			}
			id := d.queryID(t, s)
			if n := d.raw.versions(ports.RawKeyOf(testConnector, id, ports.RawRequest)); n != 0 {
				t.Fatalf("request versions = %d, want no call kept", n)
			}
			if _, _, err := interactionsTable.Reader(d.store).Load(withExecution(t), id); err == nil {
				t.Fatal("the query was recorded, want nothing recorded")
			}
		})
	}
}

// A command not yet due, or with no state query, is left to the next sweep or
// to the reconciliation.
func TestTheSweepQueriesOnlyACommandDueWithAStateQuery(t *testing.T) {
	d, _ := unknown(t, partnersim.Answer{Finding: "applied"})

	if report := d.sweep(t); report.StateQueried != 0 {
		t.Fatalf("sweep before nextQueryAt = %+v, want no query", report)
	}
	d.clock.advance(61 * second)
	d.force(t, testCommand, func(s *domain.ExternalCommandSnapshot) { s.ReconciliationRequired = true })

	if report := d.sweep(t); report.StateQueried != 0 || report.Failed != 0 {
		t.Fatalf("sweep of a command sent to the reconciliation = %+v, want no query", report)
	}
}

// The gateway's side of L691-693: every state query that answers inconclusive
// keeps the command in unknown_outcome with the next query scheduled, until
// the ceiling of the profile sends it to the reconciliation. Reading a partial
// confirmation as inconclusive is the adapter's, which comes with the
// providers.
func TestInconclusiveQueriesKeepTheCommandWaitingUntilTheCeilingSendsItToTheReconciliation(t *testing.T) {
	const maxQueries = 3
	answers := make([]partnersim.Answer, maxQueries)
	for i := range answers {
		answers[i] = partnersim.Answer{Finding: "inconclusive"}
	}
	d, _ := unknown(t, answers...)

	for round := 1; round <= maxQueries; round++ {
		d.clock.advance(61 * second)
		if report := d.sweep(t); report.StateQueried != 1 || report.Failed != 0 {
			t.Fatalf("sweep %d = %+v, want one state query", round, report)
		}
		handovers := d.handovers()
		d.normalized(t, port.MessageID(fmt.Sprintf("n-q-%d", round)), handovers[len(handovers)-1], usecase.R1D1)

		got := d.command(t, testCommand)
		if got.Status != domain.StatusUnknownOutcome || got.Submissions != 1 {
			t.Fatalf("command after query %d = %+v, want unknown_outcome, never resent", round, got)
		}
		if reconcile := round == maxQueries; got.ReconciliationRequired != reconcile || (got.NextQueryAt == 0) == !reconcile {
			t.Fatalf("command after query %d of %d = %+v, want the reconciliation only at the ceiling", round, maxQueries, got)
		}
	}
	if report := d.sweep(t); report.StateQueried != 0 {
		t.Fatalf("sweep past the ceiling = %+v, want no query", report)
	}
}
