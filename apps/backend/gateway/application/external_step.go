package application

import (
	"context"
	"errors"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	humanProvider    = "human"
	resolutionPrefix = "resolution."
)

var (
	// ErrStepRefused is a step the command did not take: the kernel's ledger
	// and the command disagree on whether it is in flight, and the unit of work
	// rolls back the outcome it recorded.
	ErrStepRefused = errors.New("application: the command refused the external step")

	// ErrAttemptWithoutCommand is an attempt of the ledger whose outbound key
	// names no command, or more than one, of this context.
	ErrAttemptWithoutCommand = errors.New("application: the attempt names no single external command")
)

// stepDetail is what the caller of the cycle knows that the kernel's step does
// not carry: which command, from where, and the partner's own words.
type stepDetail struct {
	key                 domain.IdempotencyKey
	origin              domain.StepOrigin
	source              domain.ConfirmationSource
	interactionID       domain.InteractionID
	externalReferenceID domain.ExternalReferenceID
	partnerCode         string
	partnerMessage      string
	partial             bool
}

// applier is the Apply of application.ExternalDispatch, ExternalAbandon and
// ExternalResolution: identity is resolved before the cycle opens its unit of
// work, so a retry of the step does not mint new facts.
func (s Service) applier(identity usecase.Identity, d stepDetail) func(context.Context, Resources, usecase.ExternalStep) error {
	return func(ctx context.Context, res Resources, step usecase.ExternalStep) error {
		next, known := stepOf(step.Step)
		if !known {
			return fmt.Errorf("application: apply external step %s: %w: step %s", d.key, ErrStepRefused, step.Step)
		}
		reference := d.externalReferenceID
		if reference == "" {
			reference = domain.ExternalReferenceID(step.Observation.ExternalRef)
		}
		outcome, err := usecase.Decide(ctx, res.ExternalCommands, res.Outbox, origin(AggregateTypeExternalCommand, string(d.key)), d.key, identity,
			loadExistingCommand,
			func(c *domain.ExternalCommand) (kernel.Accepted[domain.StepApplied], *kernel.Rejection) {
				return c.ApplyExternalStep(domain.ApplyExternalStep{
					Step: next, Origin: d.origin, ClassCategory: categoryOf(step.Class), Finding: findingOf(step.Observation.Finding),
					Source: d.source, InteractionID: d.interactionID, ExternalReferenceID: reference,
					PartnerCode: d.partnerCode, PartnerMessage: d.partnerMessage, Partial: d.partial, At: identity.OccurredAt,
				})
			})
		if err != nil {
			return fmt.Errorf("application: apply external step %s: %w", d.key, err)
		}
		if rejection, refused := outcome.Rejection(); refused {
			return fmt.Errorf("application: apply external step %s: %w",
				d.key, usecase.NewFailure(usecase.DomainRejection, false, fmt.Errorf("%w: %s", ErrStepRefused, rejection.Code())))
		}
		return nil
	}
}

// ResolveUnknownOutcome is the human reconciliation of RES-43: the decision
// reaches the kernel as an observation, and the kernel picks the step.
func (s Service) ResolveUnknownOutcome(ctx context.Context, cmd ResolveUnknownOutcome) (usecase.Outcome[domain.CommandStatusResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.CommandStatusResponse]{
		Operation: OperationResolveUnknownOutcome,
		Object:    string(cmd.IdempotencyKey),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationResolveUnknownOutcome).
			String(string(cmd.IdempotencyKey)).String(string(cmd.Resolution)).String(string(cmd.QueryInteractionID)).
			String(string(cmd.ExternalReferenceID)).String(cmd.PartnerCode).String(cmd.PartnerMessage).String(cmd.Justification).
			Int(flag(cmd.Partial)),
		Codec: commandStatusCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.CommandStatusResponse], error) {
			outcome, err := s.resolve(ctx, res, identity, cmd)
			if err != nil {
				return outcome, fmt.Errorf("application: resolve unknown outcome %s: %w", cmd.IdempotencyKey, err)
			}
			return outcome, nil
		},
	})
}

func (s Service) resolve(ctx context.Context, res Resources, identity usecase.Identity, cmd ResolveUnknownOutcome) (usecase.Outcome[domain.CommandStatusResponse], error) {
	var none usecase.Outcome[domain.CommandStatusResponse]
	snapshot, _, err := res.ExternalCommands.Load(ctx, cmd.IdempotencyKey)
	if err != nil {
		return none, err
	}
	accepted, rejection := domain.FromExternalCommandSnapshot(snapshot).ResolveUnknownOutcome(domain.ResolveUnknownOutcome{
		Resolution: cmd.Resolution, QueryInteractionID: cmd.QueryInteractionID, ExternalReferenceID: cmd.ExternalReferenceID,
		PartnerCode: cmd.PartnerCode, PartnerMessage: cmd.PartnerMessage, Justification: cmd.Justification, Partial: cmd.Partial,
	})
	if rejection != nil {
		return usecase.Rejected[domain.CommandStatusResponse](rejection), nil
	}
	resolution, _ := port.IdempotencyKeyFrom(ctx)
	cycle := cycleOf(snapshot)
	_, err = usecase.Resolve(ctx, usecase.ExternalResolution[Resources]{
		UoW:    joined{res: res},
		Ledger: ledgerOf,
		Observation: port.Observation{
			Key: cycle.Key, Finding: externalFinding(accepted.Response().Finding), SourceAt: identity.OccurredAt,
			Provider: humanProvider, ExternalRef: string(cmd.ExternalReferenceID), ID: resolutionPrefix + resolution,
		},
		Profile:         cycle.Profile,
		QueryAvailable:  cycle.QueryAvailable,
		Clock:           instant(identity.OccurredAt),
		Instrumentation: s.instrumentation(),
		Apply: s.applier(identity, stepDetail{
			key: cmd.IdempotencyKey, origin: domain.OriginObservation, source: domain.SourceHuman,
			interactionID: cmd.QueryInteractionID, externalReferenceID: cmd.ExternalReferenceID,
			partnerCode: cmd.PartnerCode, partnerMessage: cmd.PartnerMessage, partial: cmd.Partial,
		}),
	})
	if err != nil {
		return none, err
	}
	after, _, err := res.ExternalCommands.Load(ctx, cmd.IdempotencyKey)
	if err != nil {
		return none, err
	}
	return usecase.Accepted(domain.CommandStatusResponse{IdempotencyKey: cmd.IdempotencyKey, Status: after.Status}), nil
}

// AbandonExternalAttempt is the Abandon the kernel's sweep of attempts calls
// for one it found begun past the cut: ClassUnknownSent is presumed, nothing
// is resent (RES-43), and the step reaches the command as a submission.
func (s Service) AbandonExternalAttempt(ctx context.Context, attempt port.Attempt) (port.Finished, error) {
	commands, err := s.CommandsByOutboundKey.LoadByOutboundKey(ctx, attempt.Key)
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		return port.Finished{}, fmt.Errorf("application: abandon external attempt %s: %w: attempt %d", attempt.Key, err, attempt.N)
	}
	if len(commands) != 1 {
		return port.Finished{}, fmt.Errorf("application: abandon external attempt %s: %w: %d commands, attempt %d", attempt.Key, ErrAttemptWithoutCommand, len(commands), attempt.N)
	}
	command := commands[0]
	cycle := cycleOf(command)
	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	return usecase.Abandon(ctx, usecase.ExternalAbandon[Resources]{
		UoW:             s.UoW,
		Ledger:          ledgerOf,
		Attempt:         attempt,
		Profile:         cycle.Profile,
		QueryAvailable:  cycle.QueryAvailable,
		Clock:           instant(identity.OccurredAt),
		Instrumentation: s.instrumentation(),
		Apply:           s.applier(identity, stepDetail{key: command.Key, origin: domain.OriginSubmit, source: domain.SourceSubmitResponse}),
	})
}

// joined runs the kernel's Resolve inside the unit of work Execute opened:
// the inbox entry of the command, the observation the ledger records and the
// step must commit together, or none of them.
type joined struct{ res Resources }

func (j joined) Within(ctx context.Context, fn func(context.Context, Resources) error) error {
	return fn(ctx, j.res)
}

// instant pins the ledger to the instant of the command, so the observation
// and the step it applies carry one time.
type instant port.Instant

func (i instant) Now() port.Instant { return port.Instant(i) }
