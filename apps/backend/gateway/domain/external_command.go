package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type CommandStatus string

const (
	StatusCreated          CommandStatus = "created"
	StatusAcknowledged     CommandStatus = "acknowledged"
	StatusConfirmed        CommandStatus = "confirmed"
	StatusRetryableFailure CommandStatus = "retryable_failure"
	StatusFinalFailure     CommandStatus = "final_failure"
	StatusUnknownOutcome   CommandStatus = "unknown_outcome"
	StatusCancelled        CommandStatus = "cancelled"
)

// waits is the episode of 00 §7.2.5: the stay in acknowledged or
// unknown_outcome, with one deadline however often it moves between the two.
func (s CommandStatus) waits() bool {
	return s == StatusAcknowledged || s == StatusUnknownOutcome
}

type FailureCategory string

const (
	CategoryNone               FailureCategory = ""
	CategoryFinalValidation    FailureCategory = "final_validation"
	CategoryFinalBusiness      FailureCategory = "final_business"
	CategoryFinalAuthorization FailureCategory = "final_authorization"
	CategoryUnknownSent        FailureCategory = "unknown_sent"
	CategoryUnknownAmbiguous   FailureCategory = "unknown_ambiguous"
	CategorySchemaMismatch     FailureCategory = "schema_mismatch"
)

func (c FailureCategory) final() bool {
	return c == CategoryFinalValidation || c == CategoryFinalBusiness || c == CategoryFinalAuthorization
}

// CommandProfile is the declaration of the operation copied at creation, so a
// new version of the profile never changes the key of a command in flight.
// Quiescence is in nanoseconds; QueryInterval and AmbiguityDeadline in seconds.
type CommandProfile struct {
	ProfileVersion         int
	StateQuerySupported    bool
	IdempotencySupport     IdempotencySupport
	PartialResultSupported bool
	Efficacy               Efficacy
	Quiescence             int64
	AuthoritativeAbsence   bool
	MaxQueries             int
	KeyAlphabet            string
	KeyMaxLength           int
	QueryInterval          int
	AmbiguityDeadline      int
}

// CommandProfileOf takes the most restrictive state query between the profile
// and the capability the Partner homologated (decision 6, fail-closed).
func CommandProfileOf(profileVersion int, o OperationCapability, capabilityStateQuery bool) CommandProfile {
	return CommandProfile{
		ProfileVersion:         profileVersion,
		StateQuerySupported:    o.StateQuerySupported && capabilityStateQuery,
		IdempotencySupport:     o.IdempotencySupport,
		PartialResultSupported: o.PartialResultSupported,
		Efficacy:               o.Efficacy,
		Quiescence:             o.Quiescence,
		AuthoritativeAbsence:   o.AuthoritativeAbsence,
		MaxQueries:             o.MaxQueries,
		KeyAlphabet:            o.KeyAlphabet,
		KeyMaxLength:           o.KeyMaxLength,
		QueryInterval:          o.QueryInterval,
		AmbiguityDeadline:      o.AmbiguityDeadline,
	}
}

// ExternalCommand keeps only the business outcome of a write sent to a
// partner: the kernel's ledger records each submission, by the outbound key
// derived at creation, which is kept to find the command of an attempt.
type ExternalCommand struct {
	key                    IdempotencyKey
	tenantID               TenantID
	connectorKey           ConnectorKey
	operation              string
	assetType              string
	businessKey            string
	payloadVersion         int
	seriesKey              string
	payload                string
	payloadHash            string
	profile                CommandProfile
	outboundKey            string
	status                 CommandStatus
	failureCategory        FailureCategory
	createdAt              kernel.Instant
	submissions            int
	lastSubmittedAt        kernel.Instant
	waitingSince           kernel.Instant
	acknowledgedNotified   bool
	unknownNotified        bool
	nextQueryAt            kernel.Instant
	resendAfter            kernel.Instant
	reconciliationRequired bool
	ambiguityExpiredAt     kernel.Instant
	confirmedAt            kernel.Instant
	externalReferenceID    ExternalReferenceID
}

func NewExternalCommand(key IdempotencyKey) *ExternalCommand {
	return &ExternalCommand{key: key}
}

type ExternalCommandSnapshot struct {
	Key                    IdempotencyKey
	TenantID               TenantID
	ConnectorKey           ConnectorKey
	Operation              string
	AssetType              string
	BusinessKey            string
	PayloadVersion         int
	SeriesKey              string
	Payload                string
	PayloadHash            string
	Profile                CommandProfile
	OutboundKey            string
	Status                 CommandStatus
	FailureCategory        FailureCategory
	CreatedAt              kernel.Instant
	Submissions            int
	LastSubmittedAt        kernel.Instant
	WaitingSince           kernel.Instant
	AcknowledgedNotified   bool
	UnknownNotified        bool
	NextQueryAt            kernel.Instant
	ResendAfter            kernel.Instant
	ReconciliationRequired bool
	AmbiguityExpiredAt     kernel.Instant
	ConfirmedAt            kernel.Instant
	ExternalReferenceID    ExternalReferenceID
}

func FromExternalCommandSnapshot(s ExternalCommandSnapshot) *ExternalCommand {
	return &ExternalCommand{
		key:                    s.Key,
		tenantID:               s.TenantID,
		connectorKey:           s.ConnectorKey,
		operation:              s.Operation,
		assetType:              s.AssetType,
		businessKey:            s.BusinessKey,
		payloadVersion:         s.PayloadVersion,
		seriesKey:              s.SeriesKey,
		payload:                s.Payload,
		payloadHash:            s.PayloadHash,
		profile:                s.Profile,
		outboundKey:            s.OutboundKey,
		status:                 s.Status,
		failureCategory:        s.FailureCategory,
		createdAt:              s.CreatedAt,
		submissions:            s.Submissions,
		lastSubmittedAt:        s.LastSubmittedAt,
		waitingSince:           s.WaitingSince,
		acknowledgedNotified:   s.AcknowledgedNotified,
		unknownNotified:        s.UnknownNotified,
		nextQueryAt:            s.NextQueryAt,
		resendAfter:            s.ResendAfter,
		reconciliationRequired: s.ReconciliationRequired,
		ambiguityExpiredAt:     s.AmbiguityExpiredAt,
		confirmedAt:            s.ConfirmedAt,
		externalReferenceID:    s.ExternalReferenceID,
	}
}

func (c *ExternalCommand) Snapshot() ExternalCommandSnapshot {
	return ExternalCommandSnapshot{
		Key:                    c.key,
		TenantID:               c.tenantID,
		ConnectorKey:           c.connectorKey,
		Operation:              c.operation,
		AssetType:              c.assetType,
		BusinessKey:            c.businessKey,
		PayloadVersion:         c.payloadVersion,
		SeriesKey:              c.seriesKey,
		Payload:                c.payload,
		PayloadHash:            c.payloadHash,
		Profile:                c.profile,
		OutboundKey:            c.outboundKey,
		Status:                 c.status,
		FailureCategory:        c.failureCategory,
		CreatedAt:              c.createdAt,
		Submissions:            c.submissions,
		LastSubmittedAt:        c.lastSubmittedAt,
		WaitingSince:           c.waitingSince,
		AcknowledgedNotified:   c.acknowledgedNotified,
		UnknownNotified:        c.unknownNotified,
		NextQueryAt:            c.nextQueryAt,
		ResendAfter:            c.resendAfter,
		ReconciliationRequired: c.reconciliationRequired,
		AmbiguityExpiredAt:     c.ambiguityExpiredAt,
		ConfirmedAt:            c.confirmedAt,
		ExternalReferenceID:    c.externalReferenceID,
	}
}

// Equal is plain comparison: the snapshot holds no reference type.
func (s ExternalCommandSnapshot) Equal(other ExternalCommandSnapshot) bool {
	return s == other
}

func (c *ExternalCommand) clone() ExternalCommand {
	return *c
}

func (c *ExternalCommand) header() CommandHeader {
	return CommandHeader{
		IdempotencyKey: c.key,
		TenantID:       c.tenantID,
		ConnectorKey:   c.connectorKey,
		Operation:      c.operation,
		BusinessKey:    c.businessKey,
	}
}

func (c *ExternalCommand) dispatchRequested(at kernel.Instant) ExternalCommandDispatchRequested {
	return ExternalCommandDispatchRequested{IdempotencyKey: c.key, ConnectorKey: c.connectorKey, Operation: c.operation, At: at}
}

// KeepsSeriesOpen tells whether this command, of another version of the same
// series, lets a new version be created: only a cancellation or a final
// validation failure releases the series (00 §7.3).
func (s ExternalCommandSnapshot) KeepsSeriesOpen(payloadVersion int) bool {
	if s.PayloadVersion == payloadVersion {
		return true
	}
	return s.Status == StatusCancelled || s.Status == StatusFinalFailure && s.FailureCategory == CategoryFinalValidation
}

func seconds(n int) kernel.Instant {
	return kernel.Instant(n) * nanosPerSecond
}
