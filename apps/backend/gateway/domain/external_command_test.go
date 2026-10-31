package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const commandKey = domain.IdempotencyKey("idem-1")

func commandProfile() domain.CommandProfile {
	return domain.CommandProfileOf(2, submitEffectCapability(), true)
}

func create() domain.CreateExternalCommand {
	return domain.CreateExternalCommand{
		TenantID: "tnt-1", ConnectorKey: connectorKey, Operation: "submitEffect", AssetType: "duplicata",
		BusinessKey: "eff-1", PayloadVersion: 1, Payload: `{"amount":100}`, PayloadHash: "h1",
		Profile: commandProfile(), OutboundKey: "0a1b2c", CapabilityGate: domain.GateAllowed, SeriesOpen: true, At: at,
	}
}

func newCreated(t *testing.T) *domain.ExternalCommand {
	t.Helper()
	c := domain.NewExternalCommand(commandKey)
	if _, rej := c.Create(create()); rej != nil {
		t.Fatalf("setup: Create rejected: %v", rej)
	}
	return c
}

func header() domain.CommandHeader {
	return domain.CommandHeader{IdempotencyKey: commandKey, TenantID: "tnt-1", ConnectorKey: connectorKey, Operation: "submitEffect", BusinessKey: "eff-1"}
}

func requireCommandUnchanged(t *testing.T, before domain.ExternalCommandSnapshot, c *domain.ExternalCommand) {
	t.Helper()
	if after := c.Snapshot(); !before.Equal(after) {
		t.Fatalf("command changed\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestCommandProfileOfTakesTheMostRestrictiveStateQuery(t *testing.T) {
	declared := submitEffectCapability()
	if domain.CommandProfileOf(2, declared, false).StateQuerySupported {
		t.Fatalf("a capability homologated without state query must win over the profile")
	}
	declared.StateQuerySupported = false
	if domain.CommandProfileOf(2, declared, true).StateQuerySupported {
		t.Fatalf("a profile without state query must win over the capability")
	}
	got := domain.CommandProfileOf(2, submitEffectCapability(), true)
	want := domain.CommandProfile{
		ProfileVersion: 2, StateQuerySupported: true, IdempotencySupport: domain.IdempotencyKeyHeader, Efficacy: domain.EfficacyByObservation,
		Quiescence: int64(30 * second), MaxQueries: 5, KeyAlphabet: "0123456789abcdef", KeyMaxLength: 64, QueryInterval: 60, AmbiguityDeadline: 3600,
	}
	if got != want {
		t.Fatalf("CommandProfileOf() = %+v, want %+v", got, want)
	}
}

func TestCreateExternalCommandBornsCreatedAndRequestsTheDispatch(t *testing.T) {
	c := domain.NewExternalCommand(commandKey)

	acc, rej := c.Create(create())

	tb.RequireAccepted(t, rej)
	if got := acc.Response(); got != (domain.CommandStatusResponse{IdempotencyKey: commandKey, Status: domain.StatusCreated}) {
		t.Fatalf("Response() = %+v", got)
	}
	events := acc.Events()
	want := domain.ExternalCommandDispatchRequested{IdempotencyKey: commandKey, ConnectorKey: connectorKey, Operation: "submitEffect", At: at}
	if len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want only %+v", events, want)
	}
	s := c.Snapshot()
	if s.Status != domain.StatusCreated || s.CreatedAt != at || s.Submissions != 0 || s.ReconciliationRequired ||
		s.SeriesKey != "prt-1:production:submitEffect:eff-1" || s.Profile != commandProfile() || s.OutboundKey != "0a1b2c" ||
		s.Payload != `{"amount":100}` || s.PayloadHash != "h1" || s.TenantID != "tnt-1" || s.AssetType != "duplicata" {
		t.Fatalf("Snapshot() = %+v", s)
	}
}

func TestCreateExternalCommandReturnsTheExistingCommandForTheSamePayload(t *testing.T) {
	c := newCreated(t)
	before := c.Snapshot()
	again := create()
	again.At = at + second

	acc, rej := c.Create(again)

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	if got := acc.Response(); got.Status != domain.StatusCreated {
		t.Fatalf("Response() = %+v", got)
	}
	requireCommandUnchanged(t, before, c)
}

// The key already names a command, so a gate that closed since does not turn
// the repeat into a refusal that would read as "never created" (spec L595).
func TestCreateExternalCommandResolvesATakenKeyBeforeTheGate(t *testing.T) {
	for _, gate := range []domain.CapabilityGate{domain.GateSuspended, domain.GateNotEnabled} {
		t.Run(string(gate), func(t *testing.T) {
			c := newCreated(t)
			before := c.Snapshot()
			again := create()
			again.CapabilityGate, again.At = gate, at+second

			acc, rej := c.Create(again)

			tb.RequireAccepted(t, rej)
			requireNoEvents(t, acc)
			if got := acc.Response(); got != (domain.CommandStatusResponse{IdempotencyKey: commandKey, Status: domain.StatusCreated}) {
				t.Fatalf("Response() = %+v, want the existing command", got)
			}
			requireCommandUnchanged(t, before, c)

			again.PayloadHash = "h2"
			acc, rej = c.Create(again)

			tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandIdempotencyKeyConflict)
			requireCommandUnchanged(t, before, c)
		})
	}
}

func TestCreateExternalCommandRejects(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
		change func(*domain.CreateExternalCommand)
		code   kernel.Code
	}{
		{"an operation not enabled", false, func(c *domain.CreateExternalCommand) { c.CapabilityGate = domain.GateNotEnabled }, domain.CodeExternalCommandCapabilityNotEnabled},
		{"a suspended operation", false, func(c *domain.CreateExternalCommand) { c.CapabilityGate = domain.GateSuspended }, domain.CodeExternalCommandOperationSuspended},
		{"an empty business key", false, func(c *domain.CreateExternalCommand) { c.BusinessKey = "" }, domain.CodeExternalCommandBusinessKeyEmpty},
		{"an empty payload", false, func(c *domain.CreateExternalCommand) { c.Payload = "" }, domain.CodeExternalCommandPayloadEmpty},
		{"another payload under a taken key", true, func(c *domain.CreateExternalCommand) { c.PayloadHash = "h2" }, domain.CodeExternalCommandIdempotencyKeyConflict},
		{"a series with another version alive", false, func(c *domain.CreateExternalCommand) { c.SeriesOpen = false }, domain.CodeExternalCommandContentChangedAfterSend},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := domain.NewExternalCommand(commandKey)
			if tc.exists {
				c = newCreated(t)
			}
			before := c.Snapshot()
			cmd := create()
			tc.change(&cmd)

			acc, rej := c.Create(cmd)

			tb.RequireRejected(t, acc, rej, tc.code)
			requireCommandUnchanged(t, before, c)
		})
	}
}

func TestCancelExternalCommandCancelsWhatWasNeverSent(t *testing.T) {
	c := newCreated(t)

	acc, rej := c.Cancel(domain.CancelExternalCommand{At: at + second})

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	if got := acc.Response(); got != (domain.CommandResponse{IdempotencyKey: commandKey}) {
		t.Fatalf("Response() = %+v", got)
	}
	if got := c.Snapshot().Status; got != domain.StatusCancelled {
		t.Fatalf("Status = %q, want cancelled", got)
	}
}

func TestCancelExternalCommandRejectsACommandAlreadySent(t *testing.T) {
	c := newCreated(t)
	apply(t, c, submitted(domain.StepAwait, domain.CategoryNone))
	before := c.Snapshot()

	acc, rej := c.Cancel(domain.CancelExternalCommand{At: at + second})

	tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandAlreadySent)
	requireCommandUnchanged(t, before, c)
}

func retryable(t *testing.T) *domain.ExternalCommand {
	t.Helper()
	c := newCreated(t)
	apply(t, c, submitted(domain.StepQuery, domain.CategoryUnknownSent))
	apply(t, c, observed(domain.StepRetry, domain.FindingAbsent, at+second))
	return c
}

func TestRequestResendRequestsTheSameDispatchAfterTheQuiescence(t *testing.T) {
	c := retryable(t)
	resendAfter := c.Snapshot().ResendAfter

	acc, rej := c.RequestResend(domain.RequestResend{At: resendAfter})

	tb.RequireAccepted(t, rej)
	want := domain.ExternalCommandDispatchRequested{IdempotencyKey: commandKey, ConnectorKey: connectorKey, Operation: "submitEffect", At: resendAfter}
	if events := acc.Events(); len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want only %+v", events, want)
	}
	if s := c.Snapshot(); s.ResendAfter != 0 || s.Status != domain.StatusRetryableFailure || s.OutboundKey != "0a1b2c" {
		t.Fatalf("Snapshot() = %+v, want resendAfter cleared on the same outbound key", s)
	}
}

func TestRequestResendRejects(t *testing.T) {
	t.Run("within the quiescence", func(t *testing.T) {
		c := retryable(t)
		before := c.Snapshot()

		acc, rej := c.RequestResend(domain.RequestResend{At: before.ResendAfter - 1})

		tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandQuiescenceNotElapsed)
		requireCommandUnchanged(t, before, c)
	})
	t.Run("a command that is not a retryable failure", func(t *testing.T) {
		c := newCreated(t)
		before := c.Snapshot()

		acc, rej := c.RequestResend(domain.RequestResend{At: at + second})

		tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandNotResendable)
		requireCommandUnchanged(t, before, c)
	})
}

func waiting(t *testing.T, step domain.Step, category domain.FailureCategory) *domain.ExternalCommand {
	t.Helper()
	c := newCreated(t)
	apply(t, c, submitted(step, category))
	return c
}

const deadline = kernel.Instant(3600) * second

func TestExpireAmbiguityEscalatesOncePerEpisode(t *testing.T) {
	c := waiting(t, domain.StepQuery, domain.CategoryUnknownSent)

	acc, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: at + deadline})

	tb.RequireAccepted(t, rej)
	want := domain.ExternalCommandAmbiguityExpired{
		CommandHeader: header(), PartnerID: partnerID, Status: domain.StatusUnknownOutcome, WaitingSince: at,
		AmbiguityDeadline: 3600, StateQuerySupported: true, At: at + deadline,
	}
	if events := acc.Events(); len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want only %+v", events, want)
	}
	if s := c.Snapshot(); s.AmbiguityExpiredAt != at+deadline || !s.ReconciliationRequired {
		t.Fatalf("Snapshot() = %+v, want the expiry recorded and the reconciliation required", s)
	}
	before := c.Snapshot()

	again, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: at + 2*deadline})

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, again)
	requireCommandUnchanged(t, before, c)
}

func TestExpireAmbiguityReportsTheAcknowledgedWait(t *testing.T) {
	c := waiting(t, domain.StepAwait, domain.CategoryNone)

	acc, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: at + deadline})

	tb.RequireAccepted(t, rej)
	events := acc.Events()
	if len(events) != 1 || events[0].(domain.ExternalCommandAmbiguityExpired).Status != domain.StatusAcknowledged {
		t.Fatalf("Events() = %+v, want one expiry with status acknowledged", events)
	}
}

func TestExpireAmbiguityCountsANewEpisodeFromItsOwnStart(t *testing.T) {
	c := waiting(t, domain.StepQuery, domain.CategoryUnknownSent)
	if _, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: at + deadline}); rej != nil {
		t.Fatalf("first expiry rejected: %v", rej)
	}
	resolvedAt := at + deadline + second
	apply(t, c, observedBy(domain.StepRetry, domain.FindingAbsent, domain.SourceHuman, resolvedAt))
	resentAt := c.Snapshot().ResendAfter
	if _, rej := c.RequestResend(domain.RequestResend{At: resentAt}); rej != nil {
		t.Fatalf("RequestResend rejected: %v", rej)
	}
	resend := submitted(domain.StepQuery, domain.CategoryUnknownSent)
	resend.At = resentAt + 1
	apply(t, c, resend)
	if s := c.Snapshot(); s.WaitingSince != resentAt+1 || s.AmbiguityExpiredAt != 0 || s.ReconciliationRequired {
		t.Fatalf("Snapshot() = %+v, want a fresh episode", s)
	}

	early, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: resentAt + deadline})
	tb.RequireRejected(t, early, rej, domain.CodeExternalCommandAmbiguityDeadlineNotReached)

	acc, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: resentAt + 1 + deadline})
	tb.RequireAccepted(t, rej)
	if events := acc.Events(); len(events) != 1 || events[0].(domain.ExternalCommandAmbiguityExpired).WaitingSince != resentAt+1 {
		t.Fatalf("Events() = %+v, want the expiry counted from the new waitingSince", events)
	}
}

func TestExpireAmbiguityRejects(t *testing.T) {
	t.Run("a wait within its deadline", func(t *testing.T) {
		c := waiting(t, domain.StepQuery, domain.CategoryUnknownSent)
		before := c.Snapshot()

		acc, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: at + deadline - 1})

		tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandAmbiguityDeadlineNotReached)
		requireCommandUnchanged(t, before, c)
	})
	t.Run("a command that is not waiting", func(t *testing.T) {
		c := newCreated(t)
		before := c.Snapshot()

		acc, rej := c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partnerID, At: at + deadline})

		tb.RequireRejected(t, acc, rej, domain.CodeExternalCommandNotWaiting)
		requireCommandUnchanged(t, before, c)
	})
}

func resolution() domain.ResolveUnknownOutcome {
	return domain.ResolveUnknownOutcome{
		Resolution: domain.ResolutionConfirmed, QueryInteractionID: "int-9", Justification: "confirmed by phone with the registry",
	}
}

func TestResolveUnknownOutcomeTurnsTheResolutionIntoAFinding(t *testing.T) {
	tests := map[domain.Resolution]domain.Finding{
		domain.ResolutionConfirmed:    domain.FindingApplied,
		domain.ResolutionFinalFailure: domain.FindingRejected,
		domain.ResolutionNotReceived:  domain.FindingAbsent,
	}
	for resolved, finding := range tests {
		c := waiting(t, domain.StepReconcile, domain.CategoryUnknownAmbiguous)
		before := c.Snapshot()
		cmd := resolution()
		cmd.Resolution = resolved

		acc, rej := c.ResolveUnknownOutcome(cmd)

		tb.RequireAccepted(t, rej)
		requireNoEvents(t, acc)
		if got := acc.Response().Finding; got != finding {
			t.Errorf("%s: Finding = %q, want %q", resolved, got, finding)
		}
		requireCommandUnchanged(t, before, c)
	}
}

func TestResolveUnknownOutcomeRejects(t *testing.T) {
	confirmed := func(t *testing.T) *domain.ExternalCommand {
		t.Helper()
		c := newCreated(t)
		apply(t, c, submitted(domain.StepConfirm, domain.CategoryNone))
		return c
	}
	unknown := func(t *testing.T) *domain.ExternalCommand {
		t.Helper()
		return waiting(t, domain.StepQuery, domain.CategoryUnknownSent)
	}
	tests := []struct {
		name    string
		command func(*testing.T) *domain.ExternalCommand
		change  func(*domain.ResolveUnknownOutcome)
		code    kernel.Code
	}{
		{"a command already confirmed", confirmed, func(*domain.ResolveUnknownOutcome) {}, domain.CodeExternalCommandNotWaiting},
		{"an empty justification", unknown, func(c *domain.ResolveUnknownOutcome) { c.Justification = "" }, domain.CodeExternalCommandJustificationMissing},
		{"no query evidence", unknown, func(c *domain.ResolveUnknownOutcome) { c.QueryInteractionID = "" }, domain.CodeExternalCommandQueryEvidenceMissing},
		{"a partial result the partner never declared", unknown, func(c *domain.ResolveUnknownOutcome) { c.Partial = true }, domain.CodeExternalCommandPartialResultNotSupported},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.command(t)
			before := c.Snapshot()
			cmd := resolution()
			tc.change(&cmd)

			acc, rej := c.ResolveUnknownOutcome(cmd)

			tb.RequireRejected(t, acc, rej, tc.code)
			requireCommandUnchanged(t, before, c)
		})
	}
}

func TestExternalCommandSnapshotRoundTrips(t *testing.T) {
	c := waiting(t, domain.StepQuery, domain.CategoryUnknownSent)
	s := c.Snapshot()
	if !domain.FromExternalCommandSnapshot(s).Snapshot().Equal(s) {
		t.Fatalf("round trip lost state: %+v", s)
	}
}
