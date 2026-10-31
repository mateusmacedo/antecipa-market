package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// SweepReport counts what one sweep did: a refusal is a command a concurrent
// step already moved on (Skipped), a failure one the next sweep retries.
type SweepReport struct {
	ResendRequested  int
	AmbiguityExpired int
	Skipped          int
	Failed           int
}

// SweepExternalCommands is the time of the cycle, the caller's and not the
// kernel's (ADR-070 item 6); the state query needs the raw interaction, so
// the kernel's object storage port comes before that branch.
func (s Service) SweepExternalCommands(ctx context.Context) (SweepReport, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationSweepExternalCommands)
	if err := s.Authorize(ctx, SweepExternalCommands{}); err != nil {
		end(port.AuthorizationResult(err))
		return SweepReport{}, err
	}
	report, err := s.sweep(ctx, s.Clock.Now())
	if err != nil {
		failed := fmt.Errorf("application: sweep external commands: %w", err)
		end(port.Result{Outcome: port.OutcomeFailed, Err: failed})
		return report, failed
	}
	end(port.Result{Outcome: port.OutcomeAccepted})
	return report, nil
}

func (s Service) sweep(ctx context.Context, now kernel.Instant) (SweepReport, error) {
	var report SweepReport
	retryable, err := s.commandsIn(ctx, domain.StatusRetryableFailure)
	if err != nil {
		return report, err
	}
	for _, c := range retryable {
		if c.ResendAfter == 0 || c.ResendAfter > now {
			continue
		}
		outcome, err := s.RequestResend(sweepKey(ctx, "resend", c.Key, c.ResendAfter), RequestResend{IdempotencyKey: c.Key})
		report.count(outcome, err, &report.ResendRequested)
	}
	for _, status := range []domain.CommandStatus{domain.StatusAcknowledged, domain.StatusUnknownOutcome} {
		waiting, err := s.commandsIn(ctx, status)
		if err != nil {
			return report, err
		}
		for _, c := range waiting {
			if !expired(c, now) {
				continue
			}
			outcome, err := s.ExpireAmbiguity(sweepKey(ctx, "expire", c.Key, c.WaitingSince), ExpireAmbiguity{IdempotencyKey: c.Key})
			report.count(outcome, err, &report.AmbiguityExpired)
		}
	}
	return report, nil
}

func (s Service) commandsIn(ctx context.Context, status domain.CommandStatus) ([]domain.ExternalCommandSnapshot, error) {
	found, err := s.CommandsByStatus.LoadByStatus(ctx, status)
	if err != nil && !isNotFound(err) {
		return nil, fmt.Errorf("status %s: %w", status, err)
	}
	return found, nil
}

func expired(c domain.ExternalCommandSnapshot, now kernel.Instant) bool {
	deadline := kernel.Instant(c.Profile.AmbiguityDeadline) * 1_000_000_000
	return c.WaitingSince != 0 && c.AmbiguityExpiredAt == 0 && c.WaitingSince+deadline <= now
}

// sweepKey is the command key of one step the sweep takes, fixed by the
// instant that enabled it: the same step on the same episode replays.
func sweepKey(ctx context.Context, step string, key domain.IdempotencyKey, at kernel.Instant) context.Context {
	return port.WithIdempotencyKey(ctx, step+"."+string(key)+"."+strconv.FormatInt(int64(at), 10))
}

func (r *SweepReport) count(outcome usecase.Outcome[domain.CommandResponse], err error, done *int) {
	switch _, refused := outcome.Rejection(); {
	case err != nil:
		r.Failed++
	case refused:
		r.Skipped++
	default:
		*done++
	}
}

func isNotFound(err error) bool { return errors.Is(err, port.ErrNotFound) }
