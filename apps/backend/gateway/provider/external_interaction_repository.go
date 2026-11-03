package provider

import (
	"context"
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// interactionState holds the references of the raw payloads, never the bytes,
// which live in the kernel's object store.
type interactionState struct {
	Operation             string        `json:"operation"`
	Direction             string        `json:"direction"`
	Source                string        `json:"source"`
	ContentType           string        `json:"contentType"`
	CapturedStep          string        `json:"capturedStep"`
	BusinessDate          string        `json:"businessDate"`
	Request               rawShape      `json:"request"`
	Response              rawShape      `json:"response"`
	DuplicateOf           string        `json:"duplicateOf"`
	Status                string        `json:"status"`
	NormalizationRevision int           `json:"normalizationRevision"`
	ReadingHash           string        `json:"readingHash"`
	Finding               string        `json:"finding"`
	PreviousOutcome       string        `json:"previousOutcome"`
	PreviousReadingHash   string        `json:"previousReadingHash"`
	Mismatch              mismatchShape `json:"mismatch"`
	ReceivedAt            int64         `json:"receivedAt"`
}

type rawShape struct {
	Ref       string `json:"ref"`
	VersionID string `json:"versionId"`
	Hash      string `json:"hash"`
}

type mismatchShape struct {
	Kind                   string `json:"kind"`
	Detail                 string `json:"detail"`
	PartnerDeclaredVersion string `json:"partnerDeclaredVersion"`
}

// interactionTable guards command_id (ADR-067). partner_event_id carries no
// UNIQUE: a repeated reception is a row of its own, in status duplicate, and the
// derived id of the first reception is what the primary key serializes.
var interactionTable = postgres.Table[domain.InteractionID, domain.ExternalInteractionSnapshot]{
	Name:     "external_interactions",
	IDColumn: "external_interaction_id",
	Columns:  []string{"connector_id", "partner_event_id", "command_id", "snapshot"},
	Guard:    "command_id",

	Encode: func(s domain.ExternalInteractionSnapshot) ([]any, error) {
		raw, err := json.Marshal(stateOfInteraction(s))
		if err != nil {
			return nil, err
		}
		return []any{string(s.ConnectorKey), s.PartnerEventID, string(s.CommandID), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.ExternalInteractionSnapshot, error) {
		var (
			connector, partnerEvent, command string
			raw                              []byte
		)
		if err := scan(&connector, &partnerEvent, &command, &raw); err != nil {
			return domain.ExternalInteractionSnapshot{}, err
		}
		var state interactionState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.ExternalInteractionSnapshot{}, err
		}
		s := interactionOf(state)
		s.ConnectorKey, s.PartnerEventID, s.CommandID = domain.ConnectorKey(connector), partnerEvent, domain.IdempotencyKey(command)
		return s, nil
	},

	WithID: func(s domain.ExternalInteractionSnapshot, id domain.InteractionID) domain.ExternalInteractionSnapshot {
		s.ID = id
		return s
	},
}

func NewExternalInteractionRepository(tx *postgres.Tx) ports.ExternalInteractionRepository {
	return interactionTable.Repository(tx)
}

func NewExternalInteractionReader(pool postgres.ReadPool) ports.ExternalInteractionReader {
	return interactionTable.Reader(pool)
}

var (
	interactionsByPartnerEvent = interactionTable.Relation("partner_event_id")
	interactionsByCommand      = interactionTable.Relation("command_id")
)

// ExternalInteractionsReader serves the reads outside the unit of work: by
// partner event for the deduplication and by command for the follow-up.
type ExternalInteractionsReader struct{ pool postgres.ReadPool }

func NewExternalInteractionsReader(pool postgres.ReadPool) *ExternalInteractionsReader {
	return &ExternalInteractionsReader{pool: pool}
}

func (r *ExternalInteractionsReader) LoadByPartnerEvent(ctx context.Context, partnerEventID string) ([]domain.ExternalInteractionSnapshot, error) {
	return interactionsByPartnerEvent.Query(ctx, r.pool, partnerEventID)
}

func (r *ExternalInteractionsReader) LoadByCommand(ctx context.Context, command domain.IdempotencyKey) ([]domain.ExternalInteractionSnapshot, error) {
	return interactionsByCommand.Query(ctx, r.pool, string(command))
}

var (
	_ ports.ExternalInteractionsByPartnerEventReader = (*ExternalInteractionsReader)(nil)
	_ ports.ExternalInteractionsByCommandReader      = (*ExternalInteractionsReader)(nil)
)

func stateOfInteraction(s domain.ExternalInteractionSnapshot) interactionState {
	return interactionState{
		Operation: s.Operation, Direction: string(s.Direction), Source: string(s.Source), ContentType: s.ContentType,
		CapturedStep: string(s.CapturedStep), BusinessDate: s.BusinessDate,
		Request: rawShapeOf(s.Request), Response: rawShapeOf(s.Response), DuplicateOf: string(s.DuplicateOf),
		Status: string(s.Status), NormalizationRevision: s.NormalizationRevision, ReadingHash: s.ReadingHash, Finding: string(s.Finding),
		PreviousOutcome: string(s.PreviousOutcome), PreviousReadingHash: s.PreviousReadingHash,
		Mismatch: mismatchShape{
			Kind: string(s.Mismatch.Kind), Detail: s.Mismatch.Detail, PartnerDeclaredVersion: s.Mismatch.PartnerDeclaredVersion,
		},
		ReceivedAt: int64(s.ReceivedAt),
	}
}

func interactionOf(state interactionState) domain.ExternalInteractionSnapshot {
	return domain.ExternalInteractionSnapshot{
		Operation: state.Operation, Direction: domain.Direction(state.Direction), Source: domain.InteractionSource(state.Source),
		ContentType: state.ContentType, CapturedStep: domain.Step(state.CapturedStep), BusinessDate: state.BusinessDate,
		Request: rawPayloadOf(state.Request), Response: rawPayloadOf(state.Response), DuplicateOf: domain.InteractionID(state.DuplicateOf),
		Status: domain.InteractionStatus(state.Status), NormalizationRevision: state.NormalizationRevision, ReadingHash: state.ReadingHash,
		Finding: domain.Finding(state.Finding), PreviousOutcome: domain.InteractionStatus(state.PreviousOutcome),
		PreviousReadingHash: state.PreviousReadingHash,
		Mismatch: domain.Mismatch{
			Kind: domain.MismatchKind(state.Mismatch.Kind), Detail: state.Mismatch.Detail, PartnerDeclaredVersion: state.Mismatch.PartnerDeclaredVersion,
		},
		ReceivedAt: kernel.Instant(state.ReceivedAt),
	}
}

func rawShapeOf(p domain.RawPayload) rawShape {
	return rawShape{Ref: p.Ref, VersionID: p.VersionID, Hash: p.Hash}
}

func rawPayloadOf(s rawShape) domain.RawPayload {
	return domain.RawPayload{Ref: s.Ref, VersionID: s.VersionID, Hash: s.Hash}
}
