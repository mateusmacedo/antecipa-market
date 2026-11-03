package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type Direction string

const (
	DirectionOutbound Direction = "outbound"
	DirectionInbound  Direction = "inbound"
)

// InteractionSource tells how the raw payload reached the context: the
// response of a submission, a state query, a callback, a file or an external read.
type InteractionSource string

const (
	InteractionSubmitResponse InteractionSource = "submit_response"
	InteractionStateQuery     InteractionSource = "state_query"
	InteractionCallback       InteractionSource = "callback"
	InteractionFile           InteractionSource = "file"
	InteractionExternalRead   InteractionSource = "external_read"
)

type InteractionStatus string

const (
	InteractionStored         InteractionStatus = "stored"
	InteractionDuplicate      InteractionStatus = "duplicate"
	InteractionNormalized     InteractionStatus = "normalized"
	InteractionUncorrelated   InteractionStatus = "uncorrelated"
	InteractionSchemaMismatch InteractionStatus = "schema_mismatch"
)

// Concluded tells the outcomes a normalization ends an interaction with, the
// ones a reprocessing starts from.
func (s InteractionStatus) Concluded() bool {
	return s == InteractionNormalized || s == InteractionUncorrelated || s == InteractionSchemaMismatch
}

// MismatchKind is what broke the contract of an operation; an unknown
// enumeration value is none of them (04 §5).
type MismatchKind string

const (
	MismatchStructure    MismatchKind = "structure"
	MismatchMissingField MismatchKind = "missing_field"
	MismatchType         MismatchKind = "type"
	MismatchVersion      MismatchKind = "version"
)

// Mismatch is how a response broke the contract, as the adapter found it.
type Mismatch struct {
	Kind                   MismatchKind
	Detail                 string
	PartnerDeclaredVersion string
}

// RawPayload names a raw payload in the kernel's object store: the key, the
// version the store returned for it and the SHA-256 of its content. The bytes
// never enter the state (ADR-018).
type RawPayload struct {
	Ref       string
	VersionID string
	Hash      string
}

func (p RawPayload) stored() bool { return p.Ref != "" }

// ExternalInteraction keeps a raw payload exchanged with a partner and the
// readings of it: the raw payload is never replaced by a normalized one, and a
// reprocessing reads it again under a new revision.
type ExternalInteraction struct {
	id                    InteractionID
	connectorKey          ConnectorKey
	operation             string
	direction             Direction
	source                InteractionSource
	commandID             IdempotencyKey
	contentType           string
	capturedStep          Step
	partnerEventID        string
	businessDate          string
	request               RawPayload
	response              RawPayload
	duplicateOf           InteractionID
	status                InteractionStatus
	normalizationRevision int
	readingHash           string
	finding               Finding
	previousOutcome       InteractionStatus
	previousReadingHash   string
	mismatch              Mismatch
	receivedAt            kernel.Instant
}

// NewExternalInteraction starts an interaction under the id its recorder
// minted, which also names the observation and the command keys it derives.
func NewExternalInteraction(id InteractionID) *ExternalInteraction {
	return &ExternalInteraction{id: id}
}

type ExternalInteractionSnapshot struct {
	ID                    InteractionID
	ConnectorKey          ConnectorKey
	Operation             string
	Direction             Direction
	Source                InteractionSource
	CommandID             IdempotencyKey
	ContentType           string
	CapturedStep          Step
	PartnerEventID        string
	BusinessDate          string
	Request               RawPayload
	Response              RawPayload
	DuplicateOf           InteractionID
	Status                InteractionStatus
	NormalizationRevision int
	ReadingHash           string
	Finding               Finding
	PreviousOutcome       InteractionStatus
	PreviousReadingHash   string
	Mismatch              Mismatch
	ReceivedAt            kernel.Instant
}

func FromExternalInteractionSnapshot(s ExternalInteractionSnapshot) *ExternalInteraction {
	return &ExternalInteraction{
		id:                    s.ID,
		connectorKey:          s.ConnectorKey,
		operation:             s.Operation,
		direction:             s.Direction,
		source:                s.Source,
		commandID:             s.CommandID,
		contentType:           s.ContentType,
		capturedStep:          s.CapturedStep,
		partnerEventID:        s.PartnerEventID,
		businessDate:          s.BusinessDate,
		request:               s.Request,
		response:              s.Response,
		duplicateOf:           s.DuplicateOf,
		status:                s.Status,
		normalizationRevision: s.NormalizationRevision,
		readingHash:           s.ReadingHash,
		finding:               s.Finding,
		previousOutcome:       s.PreviousOutcome,
		previousReadingHash:   s.PreviousReadingHash,
		mismatch:              s.Mismatch,
		receivedAt:            s.ReceivedAt,
	}
}

func (i *ExternalInteraction) Snapshot() ExternalInteractionSnapshot {
	return ExternalInteractionSnapshot{
		ID:                    i.id,
		ConnectorKey:          i.connectorKey,
		Operation:             i.operation,
		Direction:             i.direction,
		Source:                i.source,
		CommandID:             i.commandID,
		ContentType:           i.contentType,
		CapturedStep:          i.capturedStep,
		PartnerEventID:        i.partnerEventID,
		BusinessDate:          i.businessDate,
		Request:               i.request,
		Response:              i.response,
		DuplicateOf:           i.duplicateOf,
		Status:                i.status,
		NormalizationRevision: i.normalizationRevision,
		ReadingHash:           i.readingHash,
		Finding:               i.finding,
		PreviousOutcome:       i.previousOutcome,
		PreviousReadingHash:   i.previousReadingHash,
		Mismatch:              i.mismatch,
		ReceivedAt:            i.receivedAt,
	}
}

// Equal is plain comparison: the snapshot holds no reference type.
func (s ExternalInteractionSnapshot) Equal(other ExternalInteractionSnapshot) bool {
	return s == other
}

func (i *ExternalInteraction) clone() ExternalInteraction {
	return *i
}

func (i *ExternalInteraction) rawStored() bool {
	return i.request.stored() || i.response.stored()
}

func (i *ExternalInteraction) stored(at kernel.Instant) ExternalInteractionStored {
	return ExternalInteractionStored{
		InteractionID: i.id, ConnectorKey: i.connectorKey, Operation: i.operation, Direction: i.direction, Source: i.source,
		CommandID: i.commandID, NormalizationRevision: i.normalizationRevision, At: at,
	}
}
