package domain

import (
	"strconv"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// Conclude records the reading of the revision under normalization. The first
// revision announces a mismatch or a missing correlation; a later one always
// announces the reprocessing, and the rest only when the reading changed, so a
// reprocessing that reads the same never repeats a business event (decisions 7
// and 8).
func (i *ExternalInteraction) Conclude(cmd ConcludeInteraction) (kernel.Accepted[InteractionResponse], *kernel.Rejection) {
	return kernel.DecideOver(i, (*ExternalInteraction).clone, func(next *ExternalInteraction) (kernel.Accepted[InteractionResponse], *kernel.Rejection) {
		// WHY: a late message of an earlier revision must not conclude the
		// revision a reprocessing opened since.
		if next.status != InteractionStored || cmd.Revision != next.normalizationRevision {
			return kernel.Refuse[InteractionResponse](CodeExternalInteractionAlreadyConcluded, "this revision of the interaction is already concluded",
				kernel.Detail{Key: "status", Value: string(next.status)},
				kernel.Detail{Key: "normalizationRevision", Value: strconv.Itoa(next.normalizationRevision)})
		}
		if cmd.Outcome == InteractionSchemaMismatch && (cmd.Mismatch.Kind == "" || cmd.Mismatch.Detail == "") {
			return kernel.Refuse[InteractionResponse](CodeExternalInteractionMismatchDetailMissing,
				"a schema mismatch states its kind and its detail")
		}
		next.status = cmd.Outcome
		next.readingHash = cmd.ReadingHash
		if cmd.CommandID != "" {
			next.commandID = cmd.CommandID
		}
		next.finding = FindingNone
		if cmd.Outcome == InteractionNormalized && next.commandID != "" {
			next.finding = cmd.Finding
		}
		next.mismatch = Mismatch{}
		if cmd.Outcome == InteractionSchemaMismatch {
			next.mismatch = cmd.Mismatch
		}
		return kernel.Accept(InteractionResponse{InteractionID: next.id, Status: next.status}, next.concluded(cmd)...), nil
	})
}

func (i *ExternalInteraction) concluded(cmd ConcludeInteraction) []kernel.DomainEvent {
	var events []kernel.DomainEvent
	changed := true
	if i.normalizationRevision > 1 {
		changed = i.status != i.previousOutcome || i.readingHash != i.previousReadingHash
		events = append(events, ExternalInteractionReprocessed{
			InteractionID: i.id, ConnectorKey: i.connectorKey, PartnerID: cmd.PartnerID, Operation: i.operation, CommandID: i.commandID,
			PreviousRevision: i.normalizationRevision - 1, NormalizationRevision: i.normalizationRevision,
			PreviousOutcome: i.previousOutcome, Outcome: i.status,
			OutcomeChanged: changed, TerminalCommandAffected: changed && cmd.CommandTerminal, At: cmd.At,
		})
	}
	if !changed {
		return events
	}
	switch i.status {
	case InteractionSchemaMismatch:
		events = append(events, ExternalSchemaMismatchDetected{
			InteractionID: i.id, ConnectorKey: i.connectorKey, PartnerID: cmd.PartnerID, Environment: cmd.Environment,
			Operation: i.operation, AssetType: cmd.AssetType, CommandID: i.commandID, Mismatch: i.mismatch, At: cmd.At,
		})
	case InteractionUncorrelated:
		events = append(events, ExternalInteractionUncorrelated{
			InteractionID: i.id, ConnectorKey: i.connectorKey, PartnerID: cmd.PartnerID, Environment: cmd.Environment,
			Operation: i.operation, PartnerEventID: i.partnerEventID, BusinessDate: i.businessDate, ReceivedAt: i.receivedAt, At: cmd.At,
		})
	}
	return events
}

// Reprocess reads the raw payload again under a new revision; it never resends
// an external command, and a terminal command is never changed by the new
// reading (04 §6; decision 7).
func (i *ExternalInteraction) Reprocess(cmd ReprocessInteraction) (kernel.Accepted[ReprocessResponse], *kernel.Rejection) {
	return kernel.DecideOver(i, (*ExternalInteraction).clone, func(next *ExternalInteraction) (kernel.Accepted[ReprocessResponse], *kernel.Rejection) {
		if !next.status.Concluded() {
			return kernel.Refuse[ReprocessResponse](CodeExternalInteractionNotConcluded, "only a concluded interaction is reprocessed",
				kernel.Detail{Key: "status", Value: string(next.status)})
		}
		if cmd.Justification == "" {
			return kernel.Refuse[ReprocessResponse](CodeExternalInteractionJustificationMissing, "a reprocessing states its justification")
		}
		if !next.rawStored() {
			return kernel.Refuse[ReprocessResponse](CodeExternalInteractionRawPayloadMissing,
				"an interaction without the reference of its raw payload cannot be read again")
		}
		next.previousOutcome = next.status
		next.previousReadingHash = next.readingHash
		next.status = InteractionStored
		next.normalizationRevision++
		return kernel.Accept(ReprocessResponse{InteractionID: next.id, NormalizationRevision: next.normalizationRevision}, next.stored(cmd.At)), nil
	})
}
