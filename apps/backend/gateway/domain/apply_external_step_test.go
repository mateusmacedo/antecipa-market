package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func submitted(step domain.Step, category domain.FailureCategory) domain.ApplyExternalStep {
	return domain.ApplyExternalStep{
		Step: step, Origin: domain.OriginSubmit, ClassCategory: category, Source: domain.SourceSubmitResponse,
		InteractionID: "int-1", ExternalReferenceID: "ext-1", PartnerCode: "E42", PartnerMessage: "registro recusado", At: at,
	}
}

func observed(step domain.Step, finding domain.Finding, when kernel.Instant) domain.ApplyExternalStep {
	return observedBy(step, finding, domain.SourceStateQuery, when)
}

func observedBy(step domain.Step, finding domain.Finding, source domain.ConfirmationSource, when kernel.Instant) domain.ApplyExternalStep {
	return domain.ApplyExternalStep{
		Step: step, Origin: domain.OriginObservation, ClassCategory: domain.CategoryUnknownSent, Finding: finding, Source: source,
		InteractionID: "int-2", ExternalReferenceID: "ext-2", At: when,
	}
}

func apply(t *testing.T, c *domain.ExternalCommand, cmd domain.ApplyExternalStep) []kernel.DomainEvent {
	t.Helper()
	acc, rej := c.ApplyExternalStep(cmd)
	if rej != nil {
		t.Fatalf("ApplyExternalStep(%s) rejected: %v", cmd.Step, rej)
	}
	return acc.Events()
}

func TestApplyExternalStepConfirmsOnSubmitWithTwoFacts(t *testing.T) {
	c := newCreated(t)

	events := apply(t, c, submitted(domain.StepConfirm, domain.CategoryNone))

	s := c.Snapshot()
	if s.Status != domain.StatusConfirmed || s.Submissions != 1 || s.LastSubmittedAt != at || s.ConfirmedAt != at || s.ExternalReferenceID != "ext-1" {
		t.Fatalf("Snapshot() = %+v", s)
	}
	want := []kernel.DomainEvent{
		domain.ExternalCommandSubmitted{CommandHeader: header(), Submission: 1, At: at},
		domain.ExternalCommandConfirmed{CommandHeader: header(), ExternalReferenceID: "ext-1", ConfirmationSource: domain.SourceSubmitResponse, InteractionID: "int-1", ConfirmedAt: at},
	}
	tb.RequireSameEvents(t, events, want)
}

func TestApplyExternalStepAwaitsAnAcceptanceThatIsNotTheEffect(t *testing.T) {
	c := newCreated(t)

	events := apply(t, c, submitted(domain.StepAwait, domain.CategoryNone))

	s := c.Snapshot()
	if s.Status != domain.StatusAcknowledged || s.WaitingSince != at || s.NextQueryAt != at+60*second || !s.AcknowledgedNotified || s.ConfirmedAt != 0 {
		t.Fatalf("Snapshot() = %+v", s)
	}
	tb.RequireSameEvents(t, events, []kernel.DomainEvent{
		domain.ExternalCommandSubmitted{CommandHeader: header(), Submission: 1, At: at},
		domain.ExternalCommandAcknowledged{CommandHeader: header(), ExternalReferenceID: "ext-1", At: at},
	})
}

// externalReferenceId is optional in ResolveUnknownOutcome: a confirmation
// that brings none keeps the one the acceptance recorded, as await does.
func TestApplyExternalStepConfirmationKeepsTheReferenceItDoesNotBring(t *testing.T) {
	c := newCreated(t)
	apply(t, c, submitted(domain.StepAwait, domain.CategoryNone))
	confirmation := observedBy(domain.StepConfirm, domain.FindingApplied, domain.SourceHuman, at+second)
	confirmation.ExternalReferenceID = ""

	events := apply(t, c, confirmation)

	if s := c.Snapshot(); s.Status != domain.StatusConfirmed || s.ExternalReferenceID != "ext-1" {
		t.Fatalf("Snapshot() = %+v, want confirmed keeping ext-1", s)
	}
	for _, e := range events {
		if confirmed, ok := e.(domain.ExternalCommandConfirmed); ok && confirmed.ExternalReferenceID != "ext-1" {
			t.Fatalf("ExternalCommandConfirmed.ExternalReferenceID = %q, want ext-1", confirmed.ExternalReferenceID)
		}
	}
}

func TestApplyExternalStepFailsWithThePartnerCodeAndText(t *testing.T) {
	c := newCreated(t)

	events := apply(t, c, submitted(domain.StepFail, domain.CategoryFinalBusiness))

	if s := c.Snapshot(); s.Status != domain.StatusFinalFailure || s.FailureCategory != domain.CategoryFinalBusiness {
		t.Fatalf("Snapshot() = %+v", s)
	}
	tb.RequireSameEvents(t, events, []kernel.DomainEvent{
		domain.ExternalCommandSubmitted{CommandHeader: header(), Submission: 1, At: at},
		domain.ExternalCommandFailed{CommandHeader: header(), Category: domain.CategoryFinalBusiness, PartnerCode: "E42", PartnerMessage: "registro recusado", InteractionID: "int-1", At: at},
	})
}

func TestApplyExternalStepFailsAsBusinessOnARejectedFinding(t *testing.T) {
	c := waiting(t, domain.StepQuery, domain.CategoryUnknownSent)

	events := apply(t, c, observed(domain.StepFail, domain.FindingRejected, at+second))

	if s := c.Snapshot(); s.Status != domain.StatusFinalFailure || s.FailureCategory != domain.CategoryFinalBusiness || s.WaitingSince != 0 {
		t.Fatalf("Snapshot() = %+v", s)
	}
	if len(events) != 1 || events[0].(domain.ExternalCommandFailed).Category != domain.CategoryFinalBusiness {
		t.Fatalf("Events() = %+v", events)
	}
}

func TestApplyExternalStepQueriesAnUnknownOutcomeNeverFailingIt(t *testing.T) {
	c := newCreated(t)

	events := apply(t, c, submitted(domain.StepQuery, domain.CategoryUnknownSent))

	s := c.Snapshot()
	if s.Status != domain.StatusUnknownOutcome || s.FailureCategory != domain.CategoryUnknownSent || s.WaitingSince != at ||
		s.NextQueryAt != at+60*second || s.ReconciliationRequired || !s.UnknownNotified {
		t.Fatalf("Snapshot() = %+v", s)
	}
	tb.RequireSameEvents(t, events, []kernel.DomainEvent{
		domain.ExternalCommandSubmitted{CommandHeader: header(), Submission: 1, At: at},
		domain.ExternalOutcomeUnknown{
			CommandHeader: header(), FailureCategory: domain.CategoryUnknownSent, Submissions: 1, LastSubmittedAt: at,
			StateQuerySupported: true, UnknownSince: at,
		},
	})
}

func TestApplyExternalStepSendsAnAmbiguityWithoutStateQueryToReconciliation(t *testing.T) {
	c := domain.NewExternalCommand(commandKey)
	cmd := create()
	cmd.Profile = domain.CommandProfileOf(2, submitEffectCapability(), false)
	if _, rej := c.Create(cmd); rej != nil {
		t.Fatalf("setup: %v", rej)
	}

	events := apply(t, c, submitted(domain.StepReconcile, domain.CategoryUnknownAmbiguous))

	if s := c.Snapshot(); s.Status != domain.StatusUnknownOutcome || !s.ReconciliationRequired || s.NextQueryAt != 0 {
		t.Fatalf("Snapshot() = %+v", s)
	}
	unknown := events[1].(domain.ExternalOutcomeUnknown)
	if !unknown.RequiresHumanResolution || unknown.StateQuerySupported || unknown.FailureCategory != domain.CategoryUnknownAmbiguous {
		t.Fatalf("ExternalOutcomeUnknown = %+v, want a human resolution required", unknown)
	}
}

func TestApplyExternalStepSuspendsOnASchemaMismatch(t *testing.T) {
	c := newCreated(t)

	apply(t, c, submitted(domain.StepSuspend, domain.CategorySchemaMismatch))

	if s := c.Snapshot(); s.Status != domain.StatusUnknownOutcome || s.FailureCategory != domain.CategorySchemaMismatch || s.ReconciliationRequired || s.NextQueryAt != at {
		t.Fatalf("Snapshot() = %+v, want a query due now and no reconciliation", s)
	}
}

func TestApplyExternalStepRetriesAnAbsenceSilently(t *testing.T) {
	c := waiting(t, domain.StepQuery, domain.CategoryUnknownSent)

	events := apply(t, c, observed(domain.StepRetry, domain.FindingAbsent, at+second))

	if len(events) != 0 {
		t.Fatalf("Events() = %+v, want none: an absence is intermediate", events)
	}
	s := c.Snapshot()
	if s.Status != domain.StatusRetryableFailure || s.ResendAfter != at+second+30*second || s.FailureCategory != domain.CategoryNone ||
		s.WaitingSince != 0 || s.NextQueryAt != 0 || s.UnknownNotified || s.AcknowledgedNotified {
		t.Fatalf("Snapshot() = %+v, want the episode closed and the resend after the quiescence", s)
	}
}

func TestApplyExternalStepKeepsOneEpisodeAcrossTheTwoWaits(t *testing.T) {
	c := waiting(t, domain.StepQuery, domain.CategoryUnknownSent)

	pending := apply(t, c, observed(domain.StepAwait, domain.FindingPending, at+second))
	inconclusive := apply(t, c, observed(domain.StepQuery, domain.FindingInconclusive, at+2*second))
	again := apply(t, c, observed(domain.StepAwait, domain.FindingPending, at+3*second))

	if len(pending) != 1 || pending[0].EventName() != "gateway.external-command-acknowledged" {
		t.Fatalf("first acknowledgement = %+v", pending)
	}
	if len(inconclusive) != 0 || len(again) != 0 {
		t.Fatalf("the episode emitted again: %+v %+v", inconclusive, again)
	}
	if s := c.Snapshot(); s.WaitingSince != at || s.Status != domain.StatusAcknowledged {
		t.Fatalf("Snapshot() = %+v, want waitingSince kept", s)
	}
}

func TestApplyExternalStepConfirmsAHumanResolutionOfAnExpiredWait(t *testing.T) {
	c := waiting(t, domain.StepAwait, domain.CategoryNone)
	if _, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: at + deadline}); rej != nil {
		t.Fatalf("setup: %v", rej)
	}

	events := apply(t, c, observedBy(domain.StepConfirm, domain.FindingApplied, domain.SourceHuman, at+deadline+second))

	if s := c.Snapshot(); s.Status != domain.StatusConfirmed || s.ReconciliationRequired || s.AmbiguityExpiredAt != 0 || s.WaitingSince != 0 {
		t.Fatalf("Snapshot() = %+v", s)
	}
	if len(events) != 1 || events[0].(domain.ExternalCommandConfirmed).ConfirmationSource != domain.SourceHuman {
		t.Fatalf("Events() = %+v, want a confirmation by a human", events)
	}
}

func TestApplyExternalStepClearsTheReconciliationOfAHumanFindingThatKeepsWaiting(t *testing.T) {
	c := waiting(t, domain.StepReconcile, domain.CategoryUnknownAmbiguous)

	apply(t, c, observedBy(domain.StepQuery, domain.FindingInconclusive, domain.SourceHuman, at+second))

	if s := c.Snapshot(); s.ReconciliationRequired || s.Status != domain.StatusUnknownOutcome {
		t.Fatalf("Snapshot() = %+v, want reconciliationRequired cleared", s)
	}
}

func TestApplyExternalStepLetsTheExternalFactWinOverACancellation(t *testing.T) {
	c := newCreated(t)
	if _, rej := c.Cancel(domain.CancelExternalCommand{At: at}); rej != nil {
		t.Fatalf("setup: %v", rej)
	}

	events := apply(t, c, submitted(domain.StepConfirm, domain.CategoryNone))

	if got := c.Snapshot().Status; got != domain.StatusConfirmed {
		t.Fatalf("Status = %q, want confirmed", got)
	}
	if len(events) != 2 {
		t.Fatalf("Events() = %+v, want submitted and confirmed", events)
	}
}

func TestApplyExternalStepRejectsACommandNotInFlight(t *testing.T) {
	tests := []struct {
		name string
		cmd  domain.ApplyExternalStep
	}{
		{"a submission over a confirmed command", submitted(domain.StepConfirm, domain.CategoryNone)},
		{"an observation over a confirmed command", observed(domain.StepConfirm, domain.FindingApplied, at+second)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCreated(t)
			apply(t, c, submitted(domain.StepConfirm, domain.CategoryNone))
			before := c.Snapshot()

			acc, rej := c.ApplyExternalStep(tc.cmd)

			tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandNotInFlight)
			requireCommandUnchanged(t, before, c)
		})
	}
	t.Run("an observation over a command never sent", func(t *testing.T) {
		c := newCreated(t)
		acc, rej := c.ApplyExternalStep(observed(domain.StepConfirm, domain.FindingApplied, at))
		tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandNotInFlight)
	})
}
