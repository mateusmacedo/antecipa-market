package application_test

import (
	"errors"
	"strings"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const testInteraction = domain.InteractionID("int-1")

var (
	rawRequest  = domain.RawPayload{Ref: "interactions/prt-1:production/int-1/request", VersionID: "v-req", Hash: "h-req"}
	rawResponse = domain.RawPayload{Ref: "interactions/prt-1:production/int-1/response", VersionID: "v-res", Hash: "h-res"}
	brokenField = domain.Mismatch{Kind: domain.MismatchMissingField, Detail: "titular ausente", PartnerDeclaredVersion: "2.1"}
)

func inboundCallback() application.RecordExternalInteraction {
	return application.RecordExternalInteraction{
		InteractionID: testInteraction, ConnectorKey: testConnector, Operation: "submitEffect", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, ContentType: "application/json", PartnerEventID: "evt-p-1", BusinessDate: "2026-09-02",
		Request: rawRequest,
	}
}

func (h *harness) interaction(t *testing.T, id domain.InteractionID) domain.ExternalInteractionSnapshot {
	t.Helper()
	s, _, err := interactionsTable.Reader(h.store).Load(withExecution(t), id)
	if err != nil {
		t.Fatalf("load interaction %s: %v", id, err)
	}
	return s
}

func (h *harness) recorded(t *testing.T, cmd application.RecordExternalInteraction) {
	t.Helper()
	must(h.service.RecordExternalInteraction(withExecution(t), cmd)).accepted(t)
}

func (h *harness) concluded(t *testing.T, cmd application.ConcludeInteraction) {
	t.Helper()
	must(h.service.ConcludeInteraction(withExecution(t), cmd)).accepted(t)
}

func (h *harness) reprocessed(t *testing.T, id domain.InteractionID) {
	t.Helper()
	must(h.service.ReprocessInteraction(withExecution(t), application.ReprocessInteraction{InteractionID: id, Justification: "normalizador corrigido"})).accepted(t)
}

// eventsTo returns the events the outbox holds for a destination, in order.
func (h *harness) eventsTo(destination string) []kernel.DomainEvent {
	var events []kernel.DomainEvent
	for _, entry := range h.entries() {
		if entry.Intent.Destination == destination {
			events = append(events, entry.Event)
		}
	}
	return events
}

func normalizedAs(command domain.IdempotencyKey, readingHash string, finding domain.Finding, revision int) application.ConcludeInteraction {
	return application.ConcludeInteraction{
		InteractionID: testInteraction, Revision: revision, Outcome: domain.InteractionNormalized, CommandID: command,
		ReadingHash: readingHash, Finding: finding,
	}
}

func TestTheDerivedKeyKeepsAKeyThatFitsAndShortensOneThatDoesNot(t *testing.T) {
	policy := application.DerivedKeyPolicy{Digest: serviceskit.FoldDigest}

	plain, err := policy.Key("conclude", "int-1", "2")
	if err != nil || plain != "conclude.int-1.2" {
		t.Fatalf("Key(int-1, 2) = %q, %v; want the readable key", plain, err)
	}
	for name, parts := range map[string][]string{
		"too long": {strings.Repeat("x", 128)},
		"colons":   {"prt-1:registry:asset:dup-1", "int-1"},
	} {
		t.Run(name, func(t *testing.T) {
			first, err := policy.Key("link", parts...)
			if err != nil || !port.ValidIdempotencyKey(first) || !strings.HasPrefix(first, "link.") {
				t.Fatalf("Key() = %q, %v; want a valid key under the step", first, err)
			}
			again, _ := policy.Key("link", parts...)
			if again != first {
				t.Fatalf("Key() = %q then %q, want the same key for the same parts", first, again)
			}
		})
	}
	if _, err := (application.DerivedKeyPolicy{}).Key("link", "a:b"); !errors.Is(err, application.ErrNoKeyDigest) {
		t.Fatalf("Key() without digest = %v, want ErrNoKeyDigest", err)
	}
}

func TestRecordStoresTheInteractionAndHandsItToTheNormalization(t *testing.T) {
	h := newHarness(t)

	got := must(h.service.RecordExternalInteraction(withExecution(t), inboundCallback())).accepted(t)

	if got != (domain.InteractionResponse{InteractionID: testInteraction, Status: domain.InteractionStored}) {
		t.Fatalf("Response() = %+v", got)
	}
	if s := h.interaction(t, testInteraction); s.Status != domain.InteractionStored || s.NormalizationRevision != 1 || s.Request != rawRequest {
		t.Fatalf("interaction = %+v, want stored at revision 1 with its raw payload", s)
	}
	stored := h.eventsTo(application.NormalizationDestination)
	if len(stored) != 1 || len(h.entries()) != 1 {
		t.Fatalf("outbox = %+v, want one ExternalInteractionStored on gateway.normalizations", h.entries())
	}
	if e, ok := stored[0].(domain.ExternalInteractionStored); !ok || e.InteractionID != testInteraction || e.NormalizationRevision != 1 {
		t.Fatalf("event = %+v, want the stored interaction at revision 1", stored[0])
	}
}

// Each call carries a key of its own from the edge, so only the key derived
// from the interaction makes the second one a replay.
func TestARepeatedRecordReplaysUnderTheDerivedKey(t *testing.T) {
	h := newHarness(t)
	h.recorded(t, inboundCallback())

	again := must(h.service.RecordExternalInteraction(withExecution(t), inboundCallback())).accepted(t)

	if again.Status != domain.InteractionStored || len(h.entries()) != 1 {
		t.Fatalf("Response() = %+v with %d outbox entries, want the replay of the first record", again, len(h.entries()))
	}
}

func TestARecordOfASeenPartnerEventIsADuplicateThatNoNormalizationReads(t *testing.T) {
	h := newHarness(t)
	h.recorded(t, inboundCallback())
	repeated := inboundCallback()
	repeated.InteractionID = "int-2"
	repeated.DuplicateOf = testInteraction

	got := must(h.service.RecordExternalInteraction(withExecution(t), repeated)).accepted(t)

	if got.Status != domain.InteractionDuplicate || h.interaction(t, "int-2").Request != rawRequest {
		t.Fatalf("Response() = %+v, want a duplicate that keeps its raw payload", got)
	}
	if n := len(h.eventsTo(application.NormalizationDestination)); n != 1 {
		t.Fatalf("%d ExternalInteractionStored, want only the first reception's", n)
	}
}

func TestARecordWithoutRawPayloadIsRefusedAndWritesNothing(t *testing.T) {
	h := newHarness(t)
	cmd := inboundCallback()
	cmd.Request = domain.RawPayload{}

	must(h.service.RecordExternalInteraction(withExecution(t), cmd)).refused(t, string(domain.CodeExternalInteractionRawPayloadMissing))

	if n := len(h.entries()); n != 0 {
		t.Fatalf("%d outbox entries, want none", n)
	}
}

func TestConcludeReadsTheConnectorAndTheCommandForASelfContainedMismatch(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.recorded(t, application.RecordExternalInteraction{
		InteractionID: testInteraction, ConnectorKey: testConnector, Operation: "submitEffect", Direction: domain.DirectionOutbound,
		Source: domain.InteractionSubmitResponse, CommandID: testCommand, Request: rawRequest, Response: rawResponse,
		CapturedStep: domain.StepSuspend, Mismatch: brokenField,
	})

	got := must(h.service.ConcludeInteraction(withExecution(t), application.ConcludeInteraction{
		InteractionID: testInteraction, Revision: 1, Outcome: domain.InteractionSchemaMismatch, Mismatch: brokenField,
	})).accepted(t)

	if got.Status != domain.InteractionSchemaMismatch {
		t.Fatalf("Response() = %+v", got)
	}
	events := h.eventsTo(application.Destination)
	want := domain.ExternalSchemaMismatchDetected{
		InteractionID: testInteraction, ConnectorKey: testConnector, PartnerID: "prt-1", Environment: domain.EnvironmentProduction,
		Operation: "submitEffect", AssetType: "duplicata", CommandID: testCommand, Mismatch: brokenField, At: testOccurred,
	}
	if len(events) != 1 || events[0] != want {
		t.Fatalf("gateway.events = %+v, want %+v", events, want)
	}
}

func TestConcludeUncorrelatedAnnouncesTheInteraction(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.recorded(t, inboundCallback())

	must(h.service.ConcludeInteraction(withExecution(t), application.ConcludeInteraction{
		InteractionID: testInteraction, Revision: 1, Outcome: domain.InteractionUncorrelated,
	})).accepted(t)

	events := h.eventsTo(application.Destination)
	if e, ok := events[0].(domain.ExternalInteractionUncorrelated); len(events) != 1 || !ok || e.PartnerEventID != "evt-p-1" || e.PartnerID != "prt-1" {
		t.Fatalf("gateway.events = %+v, want ExternalInteractionUncorrelated with the partner and its event", events)
	}
}

func TestConcludeNormalizedRecordsTheFindingOnTheCommand(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.recorded(t, inboundCallback())

	h.concluded(t, normalizedAs(testCommand, "rh-1", domain.FindingApplied, 1))

	s := h.interaction(t, testInteraction)
	if s.Status != domain.InteractionNormalized || s.CommandID != testCommand || s.Finding != domain.FindingApplied || s.Request != rawRequest {
		t.Fatalf("interaction = %+v, want normalized, linked to the command, with the finding and the raw payload", s)
	}
	if n := len(h.eventsTo(application.Destination)); n != 0 {
		t.Fatalf("%d events on gateway.events, want none for a first normalized reading", n)
	}
}

// The revision derives the key, so a redelivered conclusion replays and a
// different reading of the same revision never overwrites the first.
func TestConcludeOfAConcludedRevisionReplaysAndAnotherReadingIsAMismatch(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.recorded(t, inboundCallback())
	first := application.ConcludeInteraction{InteractionID: testInteraction, Revision: 1, Outcome: domain.InteractionUncorrelated}
	h.concluded(t, first)

	again := must(h.service.ConcludeInteraction(withExecution(t), first)).accepted(t)
	if again.Status != domain.InteractionUncorrelated || len(h.eventsTo(application.Destination)) != 1 {
		t.Fatalf("Response() = %+v, want the replay of the first conclusion and one event", again)
	}

	other := first
	other.Outcome = domain.InteractionNormalized
	if _, err := h.service.ConcludeInteraction(withExecution(t), other); !errors.Is(err, port.ErrIdempotencyMismatch) {
		t.Fatalf("ConcludeInteraction(another reading) = %v, want ErrIdempotencyMismatch", err)
	}
	if s := h.interaction(t, testInteraction); s.Status != domain.InteractionUncorrelated {
		t.Fatalf("interaction = %+v, want the first reading kept", s)
	}
}

func TestConcludeOfAnOutcomeANormalizationNeverEndsInIsInvalid(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.ConcludeInteraction(withExecution(t), application.ConcludeInteraction{
		InteractionID: testInteraction, Revision: 1, Outcome: domain.InteractionStored,
	})

	if !errors.Is(err, application.ErrOutcomeNotConcluded) || usecase.CategoryOf(err) != string(usecase.Validation) {
		t.Fatalf("ConcludeInteraction(stored) = %v, want a validation failure", err)
	}
}

func TestConcludeSchemaMismatchWithoutDetailIsRefused(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.recorded(t, inboundCallback())

	must(h.service.ConcludeInteraction(withExecution(t), application.ConcludeInteraction{
		InteractionID: testInteraction, Revision: 1, Outcome: domain.InteractionSchemaMismatch, Mismatch: domain.Mismatch{Kind: domain.MismatchType},
	})).refused(t, string(domain.CodeExternalInteractionMismatchDetailMissing))
}

func TestReprocessOpensANewRevisionForTheNormalizationWithoutResending(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.recorded(t, inboundCallback())
	h.concluded(t, normalizedAs(testCommand, "rh-pending", domain.FindingPending, 1))
	before := h.command(t, testCommand)
	intents := len(h.eventsTo(application.DispatchDestination))

	got := must(h.service.ReprocessInteraction(withKey(t, "reprocess-1"), application.ReprocessInteraction{
		InteractionID: testInteraction, Justification: "normalizador corrigido",
	})).accepted(t)

	if got != (domain.ReprocessResponse{InteractionID: testInteraction, NormalizationRevision: 2}) {
		t.Fatalf("Response() = %+v", got)
	}
	s := h.interaction(t, testInteraction)
	if s.Status != domain.InteractionStored || s.PreviousOutcome != domain.InteractionNormalized || s.PreviousReadingHash != "rh-pending" {
		t.Fatalf("interaction = %+v, want stored again with the previous reading kept", s)
	}
	stored := h.eventsTo(application.NormalizationDestination)
	if e, ok := stored[len(stored)-1].(domain.ExternalInteractionStored); len(stored) != 2 || !ok || e.NormalizationRevision != 2 {
		t.Fatalf("gateway.normalizations = %+v, want the revision 2 handed to the normalization", stored)
	}
	if after := h.command(t, testCommand); !before.Equal(after) || len(h.eventsTo(application.DispatchDestination)) != intents {
		t.Fatalf("command = %+v, want it untouched and no dispatch intent", after)
	}
}

func TestReprocessRefusesWhatTheSpecRefuses(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.recorded(t, inboundCallback())

	must(h.service.ReprocessInteraction(withKey(t, "reprocess-stored"), application.ReprocessInteraction{
		InteractionID: testInteraction, Justification: "conferência",
	})).refused(t, string(domain.CodeExternalInteractionNotConcluded))

	h.concluded(t, application.ConcludeInteraction{InteractionID: testInteraction, Revision: 1, Outcome: domain.InteractionUncorrelated})
	must(h.service.ReprocessInteraction(withKey(t, "reprocess-blank"), application.ReprocessInteraction{
		InteractionID: testInteraction,
	})).refused(t, string(domain.CodeExternalInteractionJustificationMissing))
}

func TestARevisionThatConfirmsATerminalCommandLeavesItAndAnnouncesTheCase(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.recorded(t, inboundCallback())
	h.concluded(t, normalizedAs(testCommand, "rh-rejected", domain.FindingRejected, 1))
	h.force(t, testCommand, func(s *domain.ExternalCommandSnapshot) {
		s.Status, s.FailureCategory = domain.StatusFinalFailure, domain.CategoryFinalBusiness
	})
	before := h.command(t, testCommand)
	h.reprocessed(t, testInteraction)

	h.concluded(t, normalizedAs(testCommand, "rh-applied", domain.FindingApplied, 2))

	if after := h.command(t, testCommand); !before.Equal(after) {
		t.Fatalf("command = %+v, want the terminal command untouched", after)
	}
	events := h.eventsTo(application.Destination)
	e, ok := events[len(events)-1].(domain.ExternalInteractionReprocessed)
	if !ok || !e.OutcomeChanged || !e.TerminalCommandAffected || e.PreviousRevision != 1 || e.NormalizationRevision != 2 || e.PartnerID != "prt-1" {
		t.Fatalf("gateway.events = %+v, want ExternalInteractionReprocessed affecting the terminal command", events)
	}
}

func TestARevisionThatReadsTheSameAnnouncesOnlyAnUnchangedReprocessing(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.recorded(t, inboundCallback())
	h.concluded(t, normalizedAs(testCommand, "rh-1", domain.FindingPending, 1))
	h.reprocessed(t, testInteraction)

	h.concluded(t, normalizedAs(testCommand, "rh-1", domain.FindingPending, 2))

	events := h.eventsTo(application.Destination)
	if e, ok := events[0].(domain.ExternalInteractionReprocessed); len(events) != 1 || !ok || e.OutcomeChanged || e.TerminalCommandAffected {
		t.Fatalf("gateway.events = %+v, want only an unchanged ExternalInteractionReprocessed", events)
	}
}
