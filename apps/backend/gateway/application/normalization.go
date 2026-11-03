package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// ErrReadingRefused is a reading of the normalizer its conclusion refuses.
var ErrReadingRefused = errors.New("application: the conclusion refused the reading of the normalizer")

// ConsumeResponseCaptured records the outbound interaction of a capture the
// dispatch kept (decision 17), under the key of its interaction: a redelivery
// replays, in the message inbox and in the command inbox alike.
func (s Service) ConsumeResponseCaptured(ctx context.Context, cmd ConsumeResponseCaptured) (usecase.Disposition, error) {
	if err := s.Authorize(ctx, cmd); err != nil {
		return usecase.Classify(err), err
	}
	c := cmd.Capture
	record := RecordExternalInteraction{
		InteractionID: c.InteractionID, ConnectorKey: c.ConnectorKey, Operation: c.Operation,
		Direction: domain.DirectionOutbound, Source: domain.InteractionSubmitResponse, ContentType: c.ContentType,
		CommandID: c.CommandID, Request: c.Request, Response: c.Response, CapturedStep: c.Step, Mismatch: c.Mismatch,
	}
	disposition, err := s.receive(ctx, interactionInboxOf, cmd.Receipt, InteractionConsumer, func(ctx context.Context, x executor) (string, error) {
		outcome, err := s.record(ctx, x, record)
		return refusalOf(outcome), err
	})
	if err != nil {
		return disposition, fmt.Errorf("application: consume response captured %s: %w", c.InteractionID, err)
	}
	return disposition, nil
}

// ConsumeInteractionStored normalizes the revision a message hands over
// (decision 18). A message of a revision a reprocessing has passed is late and
// acknowledged. A revision already concluded is not read again: the kernel
// gets the finding the conclusion recorded, so a new normalizer never
// contradicts what an earlier delivery observed.
func (s Service) ConsumeInteractionStored(ctx context.Context, cmd ConsumeInteractionStored) (usecase.Disposition, error) {
	if err := s.Authorize(ctx, cmd); err != nil {
		return usecase.Classify(err), err
	}
	disposition, err := s.normalization(ctx, cmd)
	if err != nil {
		return disposition, fmt.Errorf("application: consume interaction stored %s: %w", cmd.InteractionID, err)
	}
	return disposition, nil
}

func (s Service) normalization(ctx context.Context, cmd ConsumeInteractionStored) (usecase.Disposition, error) {
	interaction, err := s.handedOver(ctx, cmd.InteractionID)
	if err != nil {
		return usecase.Classify(err), err
	}
	if interaction.NormalizationRevision != cmd.Revision {
		return usecase.R2, nil
	}
	disposition, seen := usecase.R2, sighting{sourceAt: interaction.ReceivedAt}
	if interaction.Status == domain.InteractionStored {
		if disposition, seen, err = s.normalize(ctx, cmd, interaction); err != nil || disposition == usecase.R4 {
			return disposition, err
		}
		if interaction, err = s.handedOver(ctx, cmd.InteractionID); err != nil {
			return usecase.Classify(err), err
		}
	}
	if !interaction.Status.Concluded() || interaction.NormalizationRevision != cmd.Revision {
		return disposition, nil
	}
	if err := s.observe(ctx, interaction, seen); err != nil {
		return usecase.Classify(err), err
	}
	return disposition, nil
}

// handedOver reads the interaction a message names. Its record wrote the
// message in the same unit of work, so one not found yet is a read that has
// not caught up.
func (s Service) handedOver(ctx context.Context, id domain.InteractionID) (domain.ExternalInteractionSnapshot, error) {
	interaction, _, err := s.InteractionReader.Load(ctx, id)
	if errors.Is(err, port.ErrNotFound) {
		return interaction, usecase.NewFailure(usecase.TransientDependency, true, err)
	}
	return interaction, err
}

// sighting is what the observation of a reading carries besides the finding
// the conclusion recorded; a revision already concluded has only the instant
// the raw payload came in.
type sighting struct {
	externalRef string
	sourceAt    port.Instant
}

// normalize reads the raw payload by the version recorded and concludes the
// revision. The references are linked before the conclusion: each link is
// idempotent on its own key, and once the revision is concluded no
// redelivery reads the payload again to find them.
func (s Service) normalize(ctx context.Context, cmd ConsumeInteractionStored, interaction domain.ExternalInteractionSnapshot) (usecase.Disposition, sighting, error) {
	seen := sighting{sourceAt: interaction.ReceivedAt}
	reading, err := s.read(ctx, interaction)
	if err != nil {
		return usecase.Classify(err), seen, err
	}
	if reading.AsOf != 0 {
		seen.sourceAt = reading.AsOf
	}
	seen.externalRef = reading.ExternalRef
	conclusion, err := s.conclusionOf(ctx, interaction, reading)
	if err != nil {
		return usecase.Classify(err), seen, err
	}
	if err := s.linkReferences(ctx, interaction.ID, reading.References); err != nil {
		return usecase.Classify(err), seen, err
	}
	disposition, err := s.receive(ctx, normalizationInboxOf, cmd.Receipt, NormalizationConsumer, func(ctx context.Context, x executor) (string, error) {
		outcome, err := s.conclude(ctx, x, conclusion)
		if err != nil {
			return "", err
		}
		return concluded(outcome)
	})
	return disposition, seen, err
}

// concluded keeps the refusal of a revision a concurrent delivery concluded
// first. Any other refusal is a reading the normalizer got wrong: it rolls
// back into the quarantine, from which it can be driven again once the
// normalizer is fixed, where a refusal kept under the conclusion key would
// replay forever.
func concluded(outcome usecase.Outcome[domain.InteractionResponse]) (string, error) {
	rejection, refused := outcome.Rejection()
	switch {
	case !refused:
		return "", nil
	case rejection.Code() == domain.CodeExternalInteractionAlreadyConcluded:
		return string(rejection.Code()), nil
	default:
		return "", usecase.NewFailure(usecase.Unexpected, false, fmt.Errorf("%w: %s", ErrReadingRefused, rejection.Code()))
	}
}

// read normalizes both sides of the raw payload the interaction recorded, by
// the adapter of its connector and operation.
func (s Service) read(ctx context.Context, interaction domain.ExternalInteractionSnapshot) (ports.Reading, error) {
	normalizer, err := s.partners().Normalizer(interaction.ConnectorKey, interaction.Operation)
	if err != nil {
		return ports.Reading{}, dependency(err)
	}
	var exchange ports.RawExchange
	if exchange.Request, err = s.rawOf(ctx, interaction.Request); err != nil {
		return ports.Reading{}, err
	}
	if exchange.Response, err = s.rawOf(ctx, interaction.Response); err != nil {
		return ports.Reading{}, err
	}
	reading, err := normalizer.Normalize(interaction.Source, exchange)
	if err != nil {
		return ports.Reading{}, usecase.NewFailure(usecase.Unexpected, false, err)
	}
	return reading, nil
}

// rawOf reads one recorded side back by its version; a store that is not
// there heals by redelivery, a payload that does not match its record never.
func (s Service) rawOf(ctx context.Context, payload domain.RawPayload) (ports.RawBody, error) {
	if payload.Ref == "" {
		return ports.RawBody{}, nil
	}
	if s.Raw == nil {
		return ports.RawBody{}, dependency(ports.ErrRawStoreUnavailable)
	}
	body, err := s.Raw.Get(ctx, payload)
	switch {
	case errors.Is(err, ports.ErrRawPayloadDefect):
		return body, usecase.NewFailure(usecase.Unexpected, false, err)
	case errors.Is(err, ports.ErrRawStoreUnavailable):
		return body, dependency(err)
	}
	return body, err
}

// conclusionOf turns the reading into the conclusion of the revision. The
// first revision of a captured submission follows the step the dispatch
// applied, which the reading cannot overrule; anywhere else the reading names
// its command by the outbound key the partner echoes.
func (s Service) conclusionOf(ctx context.Context, interaction domain.ExternalInteractionSnapshot, reading ports.Reading) (ConcludeInteraction, error) {
	conclusion := ConcludeInteraction{
		InteractionID: interaction.ID, Revision: interaction.NormalizationRevision, Outcome: reading.Outcome, Mismatch: reading.Mismatch,
	}
	if capturedSubmission(interaction) {
		conclusion.CommandID = interaction.CommandID
		if interaction.CapturedStep == domain.StepSuspend {
			conclusion.Outcome, conclusion.Mismatch = domain.InteractionSchemaMismatch, interaction.Mismatch
			if interaction.Mismatch.Kind == "" || interaction.Mismatch.Detail == "" {
				conclusion.Mismatch = reading.Mismatch
			}
			return conclusion, nil
		}
		conclusion.Outcome, conclusion.Mismatch = domain.InteractionNormalized, domain.Mismatch{}
		conclusion.ReadingHash, conclusion.Finding = reading.ReadingHash, findingOf(reading.Finding)
		return conclusion, nil
	}
	if reading.Outcome != domain.InteractionNormalized {
		return conclusion, nil
	}
	conclusion.ReadingHash, conclusion.Finding = reading.ReadingHash, findingOf(reading.Finding)
	if reading.CommandKey == "" {
		return conclusion, nil
	}
	commands, err := s.CommandsByOutboundKey.LoadByOutboundKey(ctx, reading.CommandKey)
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		return conclusion, err
	}
	switch len(commands) {
	case 0:
		// WHY: a key the partner echoes that no command of this context holds
		// correlates the reading with nothing (decision 5).
		return ConcludeInteraction{InteractionID: interaction.ID, Revision: interaction.NormalizationRevision, Outcome: domain.InteractionUncorrelated}, nil
	case 1:
		conclusion.CommandID = commands[0].Key
		return conclusion, nil
	default:
		return conclusion, usecase.NewFailure(usecase.Unexpected, false,
			fmt.Errorf("%w: %d commands under the outbound key of the reading", ErrAttemptWithoutCommand, len(commands)))
	}
}

// capturedSubmission is the first revision of the response the dispatch
// captured: the step is already applied, so no observation follows it.
func capturedSubmission(interaction domain.ExternalInteractionSnapshot) bool {
	return interaction.Direction == domain.DirectionOutbound && interaction.Source == domain.InteractionSubmitResponse &&
		interaction.NormalizationRevision == 1
}

// linkReferences records each reference the reading carries. A reference already
// linked to another subject is the partner's word against an earlier one:
// telemetry, not a reason to stop the normalization.
func (s Service) linkReferences(ctx context.Context, id domain.InteractionID, references []domain.LinkExternalReference) error {
	for _, r := range references {
		outcome, err := s.LinkInteractionReference(ctx, LinkInteractionReference{InteractionID: id, Link: LinkExternalReference{
			ProviderID: r.ProviderID, System: r.System, ReferenceType: r.ReferenceType, Reference: r.Reference,
			NormalizedReference: r.NormalizedReference, SubjectType: r.SubjectType, SubjectID: r.SubjectID,
			AsOf: r.AsOf, ObservedVersion: r.ObservedVersion,
		}})
		if err != nil {
			return err
		}
		if refusal := refusalOf(outcome); refusal != "" {
			s.logger().LogAttrs(ctx, slog.LevelWarn, "a reference of the reading was not linked",
				slog.String("interaction_id", string(id)), slog.String("rejection", refusal))
		}
	}
	return nil
}

// observe hands the finding the conclusion recorded to the kernel, after its
// commit (decision 18). The observation id carries the revision, so a
// reprocessing is a new observation and a redelivery a duplicate. A conflict
// or a saturation leaves the command as it is: the kernel's ledger and metric
// keep it, and here it is a warning, never a failure of the delivery.
func (s Service) observe(ctx context.Context, interaction domain.ExternalInteractionSnapshot, seen sighting) error {
	source, observable := observationSourceOf(interaction.Source)
	if !observable || interaction.Status != domain.InteractionNormalized || interaction.Finding == domain.FindingNone ||
		interaction.CommandID == "" || capturedSubmission(interaction) {
		return nil
	}
	command, _, err := s.CommandReader.Load(ctx, interaction.CommandID)
	if err != nil {
		return err
	}
	if !command.Status.InFlight() {
		return nil
	}
	cycle := cycleOf(command)
	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	observed, err := usecase.Resolve(ctx, usecase.ExternalResolution[Resources]{
		UoW:    s.UoW,
		Ledger: ledgerOf,
		Observation: port.Observation{
			Key: cycle.Key, Finding: externalFinding(interaction.Finding), SourceAt: seen.sourceAt,
			Provider: string(interaction.ConnectorKey), ExternalRef: seen.externalRef, ID: ObservationIDOf(interaction),
		},
		Profile:         cycle.Profile,
		QueryAvailable:  cycle.QueryAvailable,
		Clock:           instant(identity.OccurredAt),
		Instrumentation: s.instrumentation(),
		Apply:           s.applier(identity, stepDetail{key: command.Key, origin: domain.OriginObservation, source: source, interactionID: interaction.ID}),
	})
	if err != nil {
		if errors.Is(err, port.ErrAttemptInFlight) {
			// WHY: a partner may answer before the submission settles; the
			// observation waits for the attempt instead of being lost.
			return usecase.NewFailure(usecase.TransientDependency, true, err)
		}
		return err
	}
	if observed.Result == port.ObserveConflict || observed.Result == port.ObserveSaturated {
		s.logger().LogAttrs(ctx, slog.LevelWarn, "the observation of an interaction was not applied",
			slog.String("interaction_id", string(interaction.ID)), slog.String("idempotency_key", string(command.Key)),
			slog.String("result", observed.Result.String()))
	}
	return nil
}

// ObservationIDOf is the id the kernel's ledger keeps for the observation of
// one revision of an interaction.
func ObservationIDOf(interaction domain.ExternalInteractionSnapshot) string {
	return string(interaction.ID) + ".r" + strconv.Itoa(interaction.NormalizationRevision)
}

// observationSourceOf is where an observation of a command comes from; an
// external read observes no command of this context.
func observationSourceOf(source domain.InteractionSource) (domain.ConfirmationSource, bool) {
	switch source {
	case domain.InteractionSubmitResponse:
		return domain.SourceSubmitResponse, true
	case domain.InteractionStateQuery:
		return domain.SourceStateQuery, true
	case domain.InteractionCallback:
		return domain.SourceCallback, true
	case domain.InteractionFile:
		return domain.SourceFile, true
	default:
		return "", false
	}
}
