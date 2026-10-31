package application_test

import (
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func TestACommandReplayedUnderItsKeyRunsOnce(t *testing.T) {
	h := ready(t)
	ctx := withKey(t, "create-1")

	first := must(h.service.CreateExternalCommand(ctx, createCommand(testCommand, 1))).accepted(t)
	again := must(h.service.CreateExternalCommand(ctx, createCommand(testCommand, 1))).accepted(t)

	if first != again || len(h.entries()) != 1 {
		t.Fatalf("replay = %+v then %+v with %d intents, want one effect", first, again, len(h.entries()))
	}
}

func TestAKeyReusedForAnotherCommandIsRefused(t *testing.T) {
	h := ready(t)
	ctx := withKey(t, "create-1")
	must(h.service.CreateExternalCommand(ctx, createCommand(testCommand, 1))).accepted(t)

	_, err := h.service.CreateExternalCommand(ctx, createCommand("idem-other", 1))

	if !errors.Is(err, port.ErrIdempotencyMismatch) {
		t.Fatalf("CreateExternalCommand() = %v, want ErrIdempotencyMismatch", err)
	}
}

func TestAFailedWriteLeavesNoIntentBehind(t *testing.T) {
	failure := errors.New("disk full")
	for name, fault := range map[string]option{"save": withSaveError(failure), "enqueue": withEnqueueError(failure)} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, fault)
			h.registered(t)
			h.enabled(t, "submitEffect", true, 1)
			h.declared(t, simulatorOperation("submitEffect"))

			_, err := h.service.CreateExternalCommand(withExecution(t), createCommand(testCommand, 1))

			if !errors.Is(err, failure) {
				t.Fatalf("CreateExternalCommand() = %v, want the %s failure", err, name)
			}
			if n := len(h.entries()); n != 0 {
				t.Fatalf("outbox has %d entries after a failed %s", n, name)
			}
			if _, err := h.service.FindExternalCommand(withExecution(t), testCommand); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("FindExternalCommand() = %v, want the command never stored", err)
			}
		})
	}
}

func TestTheSweepKeyIsFixedByTheEpisode(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.force(t, testCommand, func(s *domain.ExternalCommandSnapshot) {
		s.Status, s.Submissions, s.WaitingSince = domain.StatusUnknownOutcome, 1, testOccurred
	})
	h.clock.advance(3600 * second)

	report, err := h.service.SweepExternalCommands(withExecution(t))
	if err != nil || report.AmbiguityExpired != 1 {
		t.Fatalf("SweepExternalCommands() = %+v, %v", report, err)
	}
	replayed := must(h.service.ExpireAmbiguity(withKey(t, "expire.idem-1.1755432000000000000"), application.ExpireAmbiguity{IdempotencyKey: testCommand})).accepted(t)
	if replayed.IdempotencyKey != testCommand || len(h.entries()) != 2 {
		t.Fatalf("the key of the sweep did not replay: %+v with %d entries", replayed, len(h.entries()))
	}
}
