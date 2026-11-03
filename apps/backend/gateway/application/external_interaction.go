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

// ErrOutcomeNotConcluded is a conclusion asked with an outcome a normalization
// never ends in: stored and duplicate are where an interaction starts.
var ErrOutcomeNotConcluded = errors.New("application: the outcome is not one a normalization concludes with")

// RecordExternalInteraction is internal to the cycle: whoever wrote the raw
// payload to the store records it under a key derived from the interaction id,
// so a redelivery or a repeated sweep replays instead of recording twice.
func (s Service) RecordExternalInteraction(ctx context.Context, cmd RecordExternalInteraction) (usecase.Outcome[domain.InteractionResponse], error) {
	return s.record(ctx, s.executor(), cmd)
}

func (s Service) record(ctx context.Context, x executor, cmd RecordExternalInteraction) (usecase.Outcome[domain.InteractionResponse], error) {
	key, err := s.DerivedKeys.key("record", string(cmd.InteractionID))
	if err != nil {
		return usecase.Outcome[domain.InteractionResponse]{}, fmt.Errorf("application: record external interaction %s: %w", cmd.InteractionID, err)
	}
	return usecase.Execute(port.WithIdempotencyKey(ctx, key), x, command[domain.InteractionResponse]{
		Operation:   OperationRecordExternalInteraction,
		Object:      string(cmd.InteractionID),
		Input:       cmd,
		Fingerprint: recordFingerprint(cmd),
		Codec:       interactionCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.InteractionResponse], error) {
			outcome, err := usecase.Decide(ctx, res.ExternalInteractions, res.Outbox, normalizationOrigin(cmd.InteractionID), cmd.InteractionID, identity,
				loadAbsentInteraction,
				func(i *domain.ExternalInteraction) (kernel.Accepted[domain.InteractionResponse], *kernel.Rejection) {
					return i.Record(domain.RecordExternalInteraction{
						ConnectorKey: cmd.ConnectorKey, Operation: cmd.Operation, Direction: cmd.Direction, Source: cmd.Source,
						ContentType: cmd.ContentType, CommandID: cmd.CommandID, PartnerEventID: cmd.PartnerEventID, BusinessDate: cmd.BusinessDate,
						Request: cmd.Request, Response: cmd.Response, CapturedStep: cmd.CapturedStep, Mismatch: cmd.Mismatch,
						DuplicateOf: cmd.DuplicateOf, At: identity.OccurredAt,
					})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: record external interaction %s: %w", cmd.InteractionID, err)
			}
			return outcome, nil
		},
	})
}

func recordFingerprint(cmd RecordExternalInteraction) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationRecordExternalInteraction).
		String(string(cmd.InteractionID)).String(string(cmd.ConnectorKey)).String(cmd.Operation).
		String(string(cmd.Direction)).String(string(cmd.Source)).String(cmd.ContentType).String(string(cmd.CommandID)).
		String(cmd.PartnerEventID).String(cmd.BusinessDate)
	for _, raw := range []domain.RawPayload{cmd.Request, cmd.Response} {
		f.String(raw.Ref).String(raw.VersionID).String(raw.Hash)
	}
	return f.String(string(cmd.CapturedStep)).String(string(cmd.Mismatch.Kind)).String(cmd.Mismatch.Detail).
		String(cmd.Mismatch.PartnerDeclaredVersion).String(string(cmd.DuplicateOf))
}

// ConcludeInteraction is internal to the normalization, under a key derived
// from the interaction and the revision it concludes. The linked command and
// the connector are read in the unit of work: whether the command is terminal
// decides what a reprocessing announces, and the events are self-contained.
func (s Service) ConcludeInteraction(ctx context.Context, cmd ConcludeInteraction) (usecase.Outcome[domain.InteractionResponse], error) {
	return s.conclude(ctx, s.executor(), cmd)
}

func (s Service) conclude(ctx context.Context, x executor, cmd ConcludeInteraction) (usecase.Outcome[domain.InteractionResponse], error) {
	if !cmd.Outcome.Concluded() {
		return usecase.Outcome[domain.InteractionResponse]{}, usecase.NewFailure(usecase.Validation, false,
			fmt.Errorf("application: %s %s %s: %w", OperationConcludeInteraction, cmd.InteractionID, cmd.Outcome, ErrOutcomeNotConcluded))
	}
	key, err := s.DerivedKeys.key("conclude", string(cmd.InteractionID), strconv.Itoa(cmd.Revision))
	if err != nil {
		return usecase.Outcome[domain.InteractionResponse]{}, fmt.Errorf("application: conclude interaction %s: %w", cmd.InteractionID, err)
	}
	return usecase.Execute(port.WithIdempotencyKey(ctx, key), x, command[domain.InteractionResponse]{
		Operation: OperationConcludeInteraction,
		Object:    string(cmd.InteractionID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationConcludeInteraction).
			String(string(cmd.InteractionID)).Int(int64(cmd.Revision)).String(string(cmd.Outcome)).String(string(cmd.CommandID)).
			String(cmd.ReadingHash).String(string(cmd.Finding)).String(string(cmd.Mismatch.Kind)).String(cmd.Mismatch.Detail).
			String(cmd.Mismatch.PartnerDeclaredVersion),
		Codec: interactionCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.InteractionResponse], error) {
			facts, err := conclusionFactsOf(ctx, res, cmd)
			if err != nil {
				return usecase.Outcome[domain.InteractionResponse]{}, fmt.Errorf("application: conclude interaction %s: %w", cmd.InteractionID, err)
			}
			outcome, err := usecase.Decide(ctx, res.ExternalInteractions, res.Outbox, origin(AggregateTypeExternalInteraction, string(cmd.InteractionID)),
				cmd.InteractionID, identity, loadExistingInteraction,
				func(i *domain.ExternalInteraction) (kernel.Accepted[domain.InteractionResponse], *kernel.Rejection) {
					return i.Conclude(domain.ConcludeInteraction{
						Revision: cmd.Revision, Outcome: cmd.Outcome, CommandID: cmd.CommandID, ReadingHash: cmd.ReadingHash, Finding: cmd.Finding,
						Mismatch: cmd.Mismatch, CommandTerminal: facts.terminal, PartnerID: facts.partner, Environment: facts.environment,
						AssetType: facts.assetType, At: identity.OccurredAt,
					})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: conclude interaction %s: %w", cmd.InteractionID, err)
			}
			return outcome, nil
		},
	})
}

type conclusionFacts struct {
	partner     domain.PartnerID
	environment domain.Environment
	assetType   string
	terminal    bool
}

// conclusionFactsOf reads the command the reading names, or the one the
// interaction was recorded with. A command the partner names but the context
// never had is no command: the normalization concludes it as uncorrelated.
func conclusionFactsOf(ctx context.Context, res Resources, cmd ConcludeInteraction) (conclusionFacts, error) {
	var facts conclusionFacts
	interaction, _, err := res.ExternalInteractions.Load(ctx, cmd.InteractionID)
	if err != nil {
		return facts, err
	}
	connector, _, err := res.Connectors.Load(ctx, interaction.ConnectorKey)
	if err != nil {
		return facts, err
	}
	facts.partner, facts.environment = connector.PartnerID, connector.Environment
	key := cmd.CommandID
	if key == "" {
		key = interaction.CommandID
	}
	if key == "" {
		return facts, nil
	}
	linked, _, err := res.ExternalCommands.Load(ctx, key)
	switch {
	case errors.Is(err, port.ErrNotFound):
		return facts, nil
	case err != nil:
		return facts, err
	}
	facts.assetType, facts.terminal = linked.AssetType, !linked.Status.InFlight()
	return facts, nil
}

// ReprocessInteraction is a backoffice operation: it opens a new revision for
// the normalization and never resends an external command (04 §6).
func (s Service) ReprocessInteraction(ctx context.Context, cmd ReprocessInteraction) (usecase.Outcome[domain.ReprocessResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ReprocessResponse]{
		Operation:   OperationReprocessInteraction,
		Object:      string(cmd.InteractionID),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationReprocessInteraction).String(string(cmd.InteractionID)).String(cmd.Justification),
		Codec:       reprocessCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ReprocessResponse], error) {
			outcome, err := usecase.Decide(ctx, res.ExternalInteractions, res.Outbox, normalizationOrigin(cmd.InteractionID), cmd.InteractionID, identity,
				loadExistingInteraction,
				func(i *domain.ExternalInteraction) (kernel.Accepted[domain.ReprocessResponse], *kernel.Rejection) {
					return i.Reprocess(domain.ReprocessInteraction{Justification: cmd.Justification, At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: reprocess interaction %s: %w", cmd.InteractionID, err)
			}
			return outcome, nil
		},
	})
}
