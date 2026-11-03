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
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// SweepReport counts what one sweep did: a refusal is a command a concurrent
// step already moved on (Skipped), a failure one the next sweep retries.
type SweepReport struct {
	ResendRequested  int
	StateQueried     int
	AmbiguityExpired int
	Skipped          int
	Failed           int
}

// SweepExternalCommands is the time of the cycle, the caller's and not the
// kernel's (ADR-070 item 6): it resends, queries the state of the commands due
// and escalates the expired waits.
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
	gates := s.gates()
	retryable, err := s.commandsIn(ctx, domain.StatusRetryableFailure)
	if err != nil {
		return report, err
	}
	for _, c := range retryable {
		if c.ResendAfter == 0 || c.ResendAfter > now {
			continue
		}
		outcome, err := s.RequestResend(sweepKey(ctx, "resend", c.Key, c.ResendAfter), RequestResend{IdempotencyKey: c.Key})
		tally(&report, outcome, err, &report.ResendRequested)
	}
	created, err := s.commandsIn(ctx, domain.StatusCreated)
	if err != nil {
		return report, err
	}
	s.release(ctx, gates, append(created, retryable...), &report)
	for _, status := range []domain.CommandStatus{domain.StatusAcknowledged, domain.StatusUnknownOutcome} {
		waiting, err := s.commandsIn(ctx, status)
		if err != nil {
			return report, err
		}
		for _, c := range waiting {
			if queryDue(c, now) {
				s.queryState(ctx, gates, c, &report)
			}
			if !expired(c, now) {
				continue
			}
			outcome, err := s.ExpireAmbiguity(sweepKey(ctx, "expire", c.Key, c.WaitingSince), ExpireAmbiguity{IdempotencyKey: c.Key})
			tally(&report, outcome, err, &report.AmbiguityExpired)
		}
	}
	return report, nil
}

// release resends each dispatch a suspension held once its operation is
// allowed again (decision 14); a connector it cannot read is retried by the
// next sweep, never released blind.
func (s Service) release(ctx context.Context, gates gates, commands []domain.ExternalCommandSnapshot, report *SweepReport) {
	for _, c := range commands {
		if c.DispatchHeldSince == 0 {
			continue
		}
		allowed, err := gates.allowed(ctx, c)
		if err != nil {
			report.Failed++
			continue
		}
		if !allowed {
			continue
		}
		outcome, err := s.RequestResend(sweepKey(ctx, "release", c.Key, c.DispatchHeldSince), RequestResend{IdempotencyKey: c.Key})
		tally(report, outcome, err, &report.ResendRequested)
	}
}

// queryState asks the partner for the state of a command whose query is due,
// keeps both raw sides and records the interaction; the finding reaches the
// command through the normalization (decision 18). The query of one
// nextQueryAt is made once: a sweep that finds it recorded leaves it to the
// normalization, and one over a suspended operation leaves it until
// EnableOperation lifts the suspension.
func (s Service) queryState(ctx context.Context, gates gates, c domain.ExternalCommandSnapshot, report *SweepReport) {
	allowed, err := gates.allowed(ctx, c)
	if err != nil {
		report.Failed++
		return
	}
	if !allowed {
		return
	}
	id, err := s.DerivedKeys.QueryInteractionID(c.Key, c.NextQueryAt)
	if err != nil {
		report.Failed++
		return
	}
	switch _, _, err := s.InteractionReader.Load(ctx, id); {
	case err == nil:
		return
	case !isNotFound(err):
		report.Failed++
		return
	}
	outcome, err := s.query(ctx, c, id)
	tally(report, outcome, err, &report.StateQueried)
}

// query fails closed before any call when the connector has no reader or the
// raw store is not there: an answer that could not be kept is not asked for.
func (s Service) query(ctx context.Context, c domain.ExternalCommandSnapshot, id domain.InteractionID) (usecase.Outcome[domain.InteractionResponse], error) {
	var none usecase.Outcome[domain.InteractionResponse]
	reader, err := s.partners().Reader(c.ConnectorKey, c.Operation)
	if err != nil {
		return none, dependency(err)
	}
	if s.Raw == nil {
		return none, dependency(ports.ErrRawStoreUnavailable)
	}
	exchange, err := reader.QueryState(ctx, port.OutboundKey(c.OutboundKey))
	if err != nil {
		return none, err
	}
	request, err := s.Raw.Put(ctx, ports.RawKeyOf(c.ConnectorKey, id, ports.RawRequest), exchange.Request)
	if err != nil {
		return none, err
	}
	response, err := s.Raw.Put(ctx, ports.RawKeyOf(c.ConnectorKey, id, ports.RawResponse), exchange.Response)
	if err != nil {
		return none, err
	}
	return s.recordAs(ctx, s.executor(), QueryExternalState{IdempotencyKey: c.Key, NextQueryAt: c.NextQueryAt}, RecordExternalInteraction{
		InteractionID: id, ConnectorKey: c.ConnectorKey, Operation: c.Operation,
		Direction: domain.DirectionOutbound, Source: domain.InteractionStateQuery, ContentType: exchange.Response.ContentType,
		CommandID: c.Key, Request: request, Response: response,
	})
}

// gates reads each connector one sweep needs once.
type gates struct {
	reader ports.ConnectorReader
	loaded map[domain.ConnectorKey]domain.ConnectorSnapshot
}

func (s Service) gates() gates {
	return gates{reader: s.ConnectorReader, loaded: map[domain.ConnectorKey]domain.ConnectorSnapshot{}}
}

func (g gates) allowed(ctx context.Context, c domain.ExternalCommandSnapshot) (bool, error) {
	connector, read := g.loaded[c.ConnectorKey]
	if !read {
		loaded, _, err := g.reader.Load(ctx, c.ConnectorKey)
		if err != nil {
			return false, err
		}
		connector, g.loaded[c.ConnectorKey] = loaded, loaded
	}
	gate, _ := connector.Gate(c.Operation, c.AssetType)
	return gate == domain.GateAllowed, nil
}

func (s Service) commandsIn(ctx context.Context, status domain.CommandStatus) ([]domain.ExternalCommandSnapshot, error) {
	found, err := s.CommandsByStatus.LoadByStatus(ctx, status)
	if err != nil && !isNotFound(err) {
		return nil, fmt.Errorf("status %s: %w", status, err)
	}
	return found, nil
}

// queryDue is a command with a state query whose time came; one sent to the
// reconciliation is no longer queried.
func queryDue(c domain.ExternalCommandSnapshot, now kernel.Instant) bool {
	return c.Profile.StateQuerySupported && !c.ReconciliationRequired && c.NextQueryAt != 0 && c.NextQueryAt <= now
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

func tally[R any](r *SweepReport, outcome usecase.Outcome[R], err error, done *int) {
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
