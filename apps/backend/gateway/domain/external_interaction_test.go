package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const interactionID = domain.InteractionID("int-1")

var (
	request = domain.RawPayload{Ref: "interactions/prt-1:production/int-1/request", VersionID: "v-req", Hash: "h-req"}
	answer  = domain.RawPayload{Ref: "interactions/prt-1:production/int-1/response", VersionID: "v-res", Hash: "h-res"}
	broken  = domain.Mismatch{Kind: domain.MismatchMissingField, Detail: "titular ausente", PartnerDeclaredVersion: "2.1"}
)

func callback() domain.RecordExternalInteraction {
	return domain.RecordExternalInteraction{
		ConnectorKey: connectorKey, Operation: "submitEffect", Direction: domain.DirectionInbound, Source: domain.InteractionCallback,
		ContentType: "application/json", PartnerEventID: "evt-p-1", BusinessDate: "2026-09-02", Request: request, At: at,
	}
}

func recorded(t *testing.T, cmd domain.RecordExternalInteraction) *domain.ExternalInteraction {
	t.Helper()
	i := domain.NewExternalInteraction(interactionID)
	if _, rej := i.Record(cmd); rej != nil {
		t.Fatalf("setup: Record rejected: %v", rej)
	}
	return i
}

func conclusion(outcome domain.InteractionStatus) domain.ConcludeInteraction {
	return domain.ConcludeInteraction{
		Revision: 1, Outcome: outcome, PartnerID: partnerID, Environment: domain.EnvironmentProduction, At: at + second,
	}
}

func concluded(t *testing.T, i *domain.ExternalInteraction, cmd domain.ConcludeInteraction) {
	t.Helper()
	if _, rej := i.Conclude(cmd); rej != nil {
		t.Fatalf("setup: Conclude rejected: %v", rej)
	}
}

func reprocessed(t *testing.T, i *domain.ExternalInteraction) {
	t.Helper()
	if _, rej := i.Reprocess(domain.ReprocessInteraction{Justification: "normalizador corrigido", At: at + 2*second}); rej != nil {
		t.Fatalf("setup: Reprocess rejected: %v", rej)
	}
}

func requireInteractionUnchanged(t *testing.T, before domain.ExternalInteractionSnapshot, i *domain.ExternalInteraction) {
	t.Helper()
	if after := i.Snapshot(); !before.Equal(after) {
		t.Fatalf("interaction changed\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func requireEvents[R any](t *testing.T, acc kernel.Accepted[R], want ...kernel.DomainEvent) {
	t.Helper()
	tb.RequireSameEvents(t, acc.Events(), want)
}

func TestRecordKeepsTheRawPayloadAsStoredBeforeAnyReading(t *testing.T) {
	i := domain.NewExternalInteraction(interactionID)

	acc, rej := i.Record(callback())

	if rej != nil {
		t.Fatalf("Record rejected: %v", rej)
	}
	if got := acc.Response(); got != (domain.InteractionResponse{InteractionID: interactionID, Status: domain.InteractionStored}) {
		t.Fatalf("Response() = %+v", got)
	}
	s := i.Snapshot()
	if s.Status != domain.InteractionStored || s.NormalizationRevision != 1 || s.ReceivedAt != at || s.Request != request || s.ReadingHash != "" {
		t.Fatalf("snapshot = %+v, want stored at revision 1 with the raw payload and no reading", s)
	}
	requireEvents(t, acc, domain.ExternalInteractionStored{
		InteractionID: interactionID, ConnectorKey: connectorKey, Operation: "submitEffect", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, NormalizationRevision: 1, At: at,
	})
}

func TestRecordOfASeenPartnerEventKeepsTheDuplicateWithoutNormalizingIt(t *testing.T) {
	cmd := callback()
	cmd.DuplicateOf = "int-0"
	i := domain.NewExternalInteraction(interactionID)

	acc, rej := i.Record(cmd)

	if rej != nil {
		t.Fatalf("Record rejected: %v", rej)
	}
	s := i.Snapshot()
	if s.Status != domain.InteractionDuplicate || s.DuplicateOf != "int-0" || s.Request != request || acc.Response().Status != domain.InteractionDuplicate {
		t.Fatalf("snapshot = %+v, want a duplicate that keeps its raw payload", s)
	}
	requireNoEvents(t, acc)
}

func TestRecordWithoutAReferenceToTheRawPayloadIsRefused(t *testing.T) {
	cmd := callback()
	cmd.Request = domain.RawPayload{}
	i := domain.NewExternalInteraction(interactionID)

	acc, rej := i.Record(cmd)

	tb.RequireRejected(t, acc, rej, domain.CodeExternalInteractionRawPayloadMissing)
	if s := i.Snapshot(); s.Status != "" || s.ReceivedAt != 0 {
		t.Fatalf("snapshot = %+v, want nothing recorded", s)
	}
}

func TestRecordOfACaptureKeepsTheStepAndTheMismatchTheAdapterFound(t *testing.T) {
	i := recorded(t, domain.RecordExternalInteraction{
		ConnectorKey: connectorKey, Operation: "submitEffect", Direction: domain.DirectionOutbound, Source: domain.InteractionSubmitResponse,
		CommandID: commandKey, Request: request, Response: answer, CapturedStep: domain.StepSuspend, Mismatch: broken, At: at,
	})

	s := i.Snapshot()
	if s.CapturedStep != domain.StepSuspend || s.Mismatch != broken || s.CommandID != commandKey || s.Response != answer {
		t.Fatalf("snapshot = %+v, want the captured step and mismatch kept", s)
	}
}

func TestConcludeNormalizedLinksTheCommandAndKeepsTheRawPayload(t *testing.T) {
	i := recorded(t, callback())
	cmd := conclusion(domain.InteractionNormalized)
	cmd.CommandID, cmd.ReadingHash, cmd.Finding = commandKey, "rh-1", domain.FindingApplied

	acc, rej := i.Conclude(cmd)

	if rej != nil {
		t.Fatalf("Conclude rejected: %v", rej)
	}
	s := i.Snapshot()
	if s.Status != domain.InteractionNormalized || s.CommandID != commandKey || s.ReadingHash != "rh-1" || s.Finding != domain.FindingApplied {
		t.Fatalf("snapshot = %+v, want normalized, linked, with its reading and finding", s)
	}
	if s.Request != request || s.NormalizationRevision != 1 {
		t.Fatalf("snapshot = %+v, want the raw payload intact at revision 1", s)
	}
	requireNoEvents(t, acc)
}

func TestConcludeKeepsAFindingOnlyOnANormalizedReadingOfACommand(t *testing.T) {
	cases := map[string]struct {
		outcome domain.InteractionStatus
		command domain.IdempotencyKey
	}{
		"normalized without command": {domain.InteractionNormalized, ""},
		"uncorrelated":               {domain.InteractionUncorrelated, commandKey},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			i := recorded(t, callback())
			cmd := conclusion(c.outcome)
			cmd.CommandID, cmd.Finding = c.command, domain.FindingApplied

			concluded(t, i, cmd)

			if got := i.Snapshot().Finding; got != domain.FindingNone {
				t.Fatalf("Finding = %q, want none: there is no command reading to record", got)
			}
		})
	}
}

func TestConcludeSchemaMismatchAnnouncesItSelfContained(t *testing.T) {
	i := recorded(t, callback())
	cmd := conclusion(domain.InteractionSchemaMismatch)
	cmd.CommandID, cmd.AssetType, cmd.Mismatch = commandKey, "duplicata", broken

	acc, rej := i.Conclude(cmd)

	if rej != nil {
		t.Fatalf("Conclude rejected: %v", rej)
	}
	if s := i.Snapshot(); s.Status != domain.InteractionSchemaMismatch || s.Mismatch != broken {
		t.Fatalf("snapshot = %+v, want schema_mismatch with its detail", s)
	}
	requireEvents(t, acc, domain.ExternalSchemaMismatchDetected{
		InteractionID: interactionID, ConnectorKey: connectorKey, PartnerID: partnerID, Environment: domain.EnvironmentProduction,
		Operation: "submitEffect", AssetType: "duplicata", CommandID: commandKey, Mismatch: broken, At: at + second,
	})
}

func TestConcludeUncorrelatedAnnouncesTheInteractionToTheBackoffice(t *testing.T) {
	i := recorded(t, callback())

	acc, rej := i.Conclude(conclusion(domain.InteractionUncorrelated))

	if rej != nil {
		t.Fatalf("Conclude rejected: %v", rej)
	}
	requireEvents(t, acc, domain.ExternalInteractionUncorrelated{
		InteractionID: interactionID, ConnectorKey: connectorKey, PartnerID: partnerID, Environment: domain.EnvironmentProduction,
		Operation: "submitEffect", PartnerEventID: "evt-p-1", BusinessDate: "2026-09-02", ReceivedAt: at, At: at + second,
	})
}

func TestConcludeOfAConcludedRevisionIsRefused(t *testing.T) {
	i := recorded(t, callback())
	concluded(t, i, conclusion(domain.InteractionNormalized))
	before := i.Snapshot()

	acc, rej := i.Conclude(conclusion(domain.InteractionUncorrelated))

	tb.RequireRejected(t, acc, rej, domain.CodeExternalInteractionAlreadyConcluded)
	requireInteractionUnchanged(t, before, i)
}

func TestConcludeOfAnEarlierRevisionLeavesTheReprocessingOpen(t *testing.T) {
	i := recorded(t, callback())
	concluded(t, i, conclusion(domain.InteractionNormalized))
	reprocessed(t, i)
	before := i.Snapshot()

	acc, rej := i.Conclude(conclusion(domain.InteractionNormalized))

	tb.RequireRejected(t, acc, rej, domain.CodeExternalInteractionAlreadyConcluded)
	requireInteractionUnchanged(t, before, i)
}

func TestConcludeSchemaMismatchWithoutItsDetailIsRefused(t *testing.T) {
	for name, mismatch := range map[string]domain.Mismatch{
		"no detail": {Kind: domain.MismatchType},
		"no kind":   {Detail: "amount is a string"},
	} {
		t.Run(name, func(t *testing.T) {
			i := recorded(t, callback())
			before := i.Snapshot()
			cmd := conclusion(domain.InteractionSchemaMismatch)
			cmd.Mismatch = mismatch

			acc, rej := i.Conclude(cmd)

			tb.RequireRejected(t, acc, rej, domain.CodeExternalInteractionMismatchDetailMissing)
			requireInteractionUnchanged(t, before, i)
		})
	}
}

func TestReprocessOpensANewRevisionWithoutResendingAnything(t *testing.T) {
	i := recorded(t, callback())
	first := conclusion(domain.InteractionNormalized)
	first.CommandID, first.ReadingHash = commandKey, "rh-1"
	concluded(t, i, first)

	acc, rej := i.Reprocess(domain.ReprocessInteraction{Justification: "normalizador corrigido", At: at + 2*second})

	if rej != nil {
		t.Fatalf("Reprocess rejected: %v", rej)
	}
	if got := acc.Response(); got != (domain.ReprocessResponse{InteractionID: interactionID, NormalizationRevision: 2}) {
		t.Fatalf("Response() = %+v", got)
	}
	s := i.Snapshot()
	if s.Status != domain.InteractionStored || s.NormalizationRevision != 2 || s.PreviousOutcome != domain.InteractionNormalized ||
		s.PreviousReadingHash != "rh-1" || s.Request != request {
		t.Fatalf("snapshot = %+v, want stored at revision 2 with the previous reading kept", s)
	}
	requireEvents(t, acc, domain.ExternalInteractionStored{
		InteractionID: interactionID, ConnectorKey: connectorKey, Operation: "submitEffect", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, CommandID: commandKey, NormalizationRevision: 2, At: at + 2*second,
	})
}

func TestReprocessOfAnInteractionNotConcludedIsRefused(t *testing.T) {
	duplicate := callback()
	duplicate.DuplicateOf = "int-0"
	for name, cmd := range map[string]domain.RecordExternalInteraction{"stored": callback(), "duplicate": duplicate} {
		t.Run(name, func(t *testing.T) {
			i := recorded(t, cmd)
			before := i.Snapshot()

			acc, rej := i.Reprocess(domain.ReprocessInteraction{Justification: "conferência", At: at})

			tb.RequireRejected(t, acc, rej, domain.CodeExternalInteractionNotConcluded)
			requireInteractionUnchanged(t, before, i)
		})
	}
}

func TestReprocessWithoutJustificationIsRefused(t *testing.T) {
	i := recorded(t, callback())
	concluded(t, i, conclusion(domain.InteractionNormalized))
	before := i.Snapshot()

	acc, rej := i.Reprocess(domain.ReprocessInteraction{At: at})

	tb.RequireRejected(t, acc, rej, domain.CodeExternalInteractionJustificationMissing)
	requireInteractionUnchanged(t, before, i)
}

func TestReprocessWithoutTheRawPayloadIsRefused(t *testing.T) {
	i := domain.FromExternalInteractionSnapshot(domain.ExternalInteractionSnapshot{
		ID: interactionID, ConnectorKey: connectorKey, Operation: "submitEffect", Status: domain.InteractionNormalized, NormalizationRevision: 1,
	})
	before := i.Snapshot()

	acc, rej := i.Reprocess(domain.ReprocessInteraction{Justification: "conferência", At: at})

	tb.RequireRejected(t, acc, rej, domain.CodeExternalInteractionRawPayloadMissing)
	requireInteractionUnchanged(t, before, i)
}

func reprocessedOver(t *testing.T, first domain.ConcludeInteraction) *domain.ExternalInteraction {
	t.Helper()
	i := recorded(t, callback())
	concluded(t, i, first)
	reprocessed(t, i)
	return i
}

func TestARevisionThatConfirmsATerminalCommandAnnouncesTheAffectedCommand(t *testing.T) {
	first := conclusion(domain.InteractionNormalized)
	first.CommandID, first.ReadingHash, first.Finding = commandKey, "rh-rejected", domain.FindingRejected
	i := reprocessedOver(t, first)
	revised := conclusion(domain.InteractionNormalized)
	revised.Revision, revised.ReadingHash, revised.Finding, revised.CommandTerminal = 2, "rh-applied", domain.FindingApplied, true

	acc, rej := i.Conclude(revised)

	if rej != nil {
		t.Fatalf("Conclude rejected: %v", rej)
	}
	if s := i.Snapshot(); s.Finding != domain.FindingApplied || s.ReadingHash != "rh-applied" || s.CommandID != commandKey {
		t.Fatalf("snapshot = %+v, want the new reading recorded over the same command", s)
	}
	requireEvents(t, acc, domain.ExternalInteractionReprocessed{
		InteractionID: interactionID, ConnectorKey: connectorKey, PartnerID: partnerID, Operation: "submitEffect", CommandID: commandKey,
		PreviousRevision: 1, NormalizationRevision: 2, PreviousOutcome: domain.InteractionNormalized, Outcome: domain.InteractionNormalized,
		OutcomeChanged: true, TerminalCommandAffected: true, At: at + second,
	})
}

func TestARevisionThatConfirmsACommandStillWaitingAffectsNoTerminalCommand(t *testing.T) {
	first := conclusion(domain.InteractionNormalized)
	first.CommandID, first.ReadingHash, first.Finding = commandKey, "rh-pending", domain.FindingPending
	i := reprocessedOver(t, first)
	revised := conclusion(domain.InteractionNormalized)
	revised.Revision, revised.ReadingHash, revised.Finding = 2, "rh-applied", domain.FindingApplied

	acc, rej := i.Conclude(revised)

	if rej != nil {
		t.Fatalf("Conclude rejected: %v", rej)
	}
	events := acc.Events()
	reprocessedEvent, ok := events[0].(domain.ExternalInteractionReprocessed)
	if len(events) != 1 || !ok || !reprocessedEvent.OutcomeChanged || reprocessedEvent.TerminalCommandAffected {
		t.Fatalf("Events() = %+v, want only a changed reprocessing that affects no terminal command", events)
	}
}

func TestARevisionThatReadsTheSameAnnouncesOnlyTheReprocessing(t *testing.T) {
	first := conclusion(domain.InteractionUncorrelated)
	i := reprocessedOver(t, first)
	revised := conclusion(domain.InteractionUncorrelated)
	revised.Revision, revised.CommandTerminal = 2, true

	acc, rej := i.Conclude(revised)

	if rej != nil {
		t.Fatalf("Conclude rejected: %v", rej)
	}
	requireEvents(t, acc, domain.ExternalInteractionReprocessed{
		InteractionID: interactionID, ConnectorKey: connectorKey, PartnerID: partnerID, Operation: "submitEffect",
		PreviousRevision: 1, NormalizationRevision: 2, PreviousOutcome: domain.InteractionUncorrelated, Outcome: domain.InteractionUncorrelated,
		At: at + second,
	})
}

func TestARevisionThatFindsTheMismatchAnnouncesItAfterTheReprocessing(t *testing.T) {
	first := conclusion(domain.InteractionNormalized)
	first.ReadingHash = "rh-1"
	i := reprocessedOver(t, first)
	revised := conclusion(domain.InteractionSchemaMismatch)
	revised.Revision, revised.Mismatch = 2, broken

	acc, rej := i.Conclude(revised)

	if rej != nil {
		t.Fatalf("Conclude rejected: %v", rej)
	}
	events := acc.Events()
	if len(events) != 2 {
		t.Fatalf("Events() = %+v, want the reprocessing and the mismatch", events)
	}
	if e, ok := events[0].(domain.ExternalInteractionReprocessed); !ok || !e.OutcomeChanged || e.Outcome != domain.InteractionSchemaMismatch {
		t.Fatalf("Events()[0] = %+v, want a changed reprocessing", events[0])
	}
	if e, ok := events[1].(domain.ExternalSchemaMismatchDetected); !ok || e.Mismatch != broken {
		t.Fatalf("Events()[1] = %+v, want the mismatch", events[1])
	}
}
