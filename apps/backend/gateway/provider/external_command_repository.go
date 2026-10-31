package provider

import (
	"context"
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// commandState holds the payload too: it is CONFIDENTIAL and opaque to the
// domain, and the dispatch handler reads it from here, never from the intent.
type commandState struct {
	TenantID               string       `json:"tenantId"`
	Operation              string       `json:"operation"`
	AssetType              string       `json:"assetType"`
	PayloadVersion         int          `json:"payloadVersion"`
	Payload                string       `json:"payload"`
	PayloadHash            string       `json:"payloadHash"`
	Profile                commandShape `json:"profile"`
	FailureCategory        string       `json:"failureCategory"`
	CreatedAt              int64        `json:"createdAt"`
	Submissions            int          `json:"submissions"`
	LastSubmittedAt        int64        `json:"lastSubmittedAt"`
	WaitingSince           int64        `json:"waitingSince"`
	AcknowledgedNotified   bool         `json:"acknowledgedNotified"`
	UnknownNotified        bool         `json:"unknownNotified"`
	NextQueryAt            int64        `json:"nextQueryAt"`
	ResendAfter            int64        `json:"resendAfter"`
	ReconciliationRequired bool         `json:"reconciliationRequired"`
	AmbiguityExpiredAt     int64        `json:"ambiguityExpiredAt"`
	ConfirmedAt            int64        `json:"confirmedAt"`
	ExternalReferenceID    string       `json:"externalReferenceId"`
}

type commandShape struct {
	ProfileVersion         int    `json:"profileVersion"`
	StateQuerySupported    bool   `json:"stateQuerySupported"`
	IdempotencySupport     string `json:"idempotencySupport"`
	PartialResultSupported bool   `json:"partialResultSupported"`
	Efficacy               string `json:"efficacy"`
	Quiescence             int64  `json:"quiescence"`
	AuthoritativeAbsence   bool   `json:"authoritativeAbsence"`
	MaxQueries             int    `json:"maxQueries"`
	KeyAlphabet            string `json:"keyAlphabet"`
	KeyMaxLength           int    `json:"keyMaxLength"`
	QueryInterval          int    `json:"queryInterval"`
	AmbiguityDeadline      int    `json:"ambiguityDeadline"`
}

// commandTable guards series_key: CreateExternalCommand reads the series under
// the shared lock, and a save of a command of the series waits for it.
var commandTable = postgres.Table[domain.IdempotencyKey, domain.ExternalCommandSnapshot]{
	Name:     "external_commands",
	IDColumn: "external_command_id",
	Columns:  []string{"connector_id", "business_key", "series_key", "status", "outbound_key", "snapshot"},
	Guard:    "series_key",

	Encode: func(s domain.ExternalCommandSnapshot) ([]any, error) {
		raw, err := json.Marshal(stateOfCommand(s))
		if err != nil {
			return nil, err
		}
		return []any{string(s.ConnectorKey), s.BusinessKey, s.SeriesKey, string(s.Status), s.OutboundKey, raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.ExternalCommandSnapshot, error) {
		var (
			connector, businessKey, seriesKey, status, outboundKey string
			raw                                                    []byte
		)
		if err := scan(&connector, &businessKey, &seriesKey, &status, &outboundKey, &raw); err != nil {
			return domain.ExternalCommandSnapshot{}, err
		}
		var state commandState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.ExternalCommandSnapshot{}, err
		}
		s := commandOf(state)
		s.ConnectorKey, s.BusinessKey, s.SeriesKey = domain.ConnectorKey(connector), businessKey, seriesKey
		s.Status, s.OutboundKey = domain.CommandStatus(status), outboundKey
		return s, nil
	},

	WithID: func(s domain.ExternalCommandSnapshot, key domain.IdempotencyKey) domain.ExternalCommandSnapshot {
		s.Key = key
		return s
	},
}

func NewExternalCommandRepository(tx *postgres.Tx) ports.ExternalCommandRepository {
	return commandTable.Repository(tx)
}

func NewExternalCommandReader(pool postgres.ReadPool) ports.ExternalCommandReader {
	return commandTable.Reader(pool)
}

var (
	commandsBySeries      = commandTable.Relation("series_key")
	commandsByBusinessKey = commandTable.Relation("business_key")
	commandsByStatus      = commandTable.Relation("status")
	commandsByOutboundKey = commandTable.Relation("outbound_key")
)

// ExternalCommandsReader serves the reads outside the unit of work: by
// business key, by status for the sweep and by outbound key for the kernel's.
type ExternalCommandsReader struct{ pool postgres.ReadPool }

func NewExternalCommandsReader(pool postgres.ReadPool) *ExternalCommandsReader {
	return &ExternalCommandsReader{pool: pool}
}

func (r *ExternalCommandsReader) LoadByBusinessKey(ctx context.Context, businessKey string) ([]domain.ExternalCommandSnapshot, error) {
	return commandsByBusinessKey.Query(ctx, r.pool, businessKey)
}

func (r *ExternalCommandsReader) LoadByStatus(ctx context.Context, status domain.CommandStatus) ([]domain.ExternalCommandSnapshot, error) {
	return commandsByStatus.Query(ctx, r.pool, string(status))
}

func (r *ExternalCommandsReader) LoadByOutboundKey(ctx context.Context, key port.OutboundKey) ([]domain.ExternalCommandSnapshot, error) {
	return commandsByOutboundKey.Query(ctx, r.pool, string(key))
}

// ExternalCommandsBySeriesWithin reads a series inside the command
// transaction, under the guard on series_key (ADR-067).
type ExternalCommandsBySeriesWithin struct{ tx *postgres.Tx }

func NewExternalCommandsBySeriesWithin(tx *postgres.Tx) *ExternalCommandsBySeriesWithin {
	return &ExternalCommandsBySeriesWithin{tx: tx}
}

func (r *ExternalCommandsBySeriesWithin) LoadBySeries(ctx context.Context, seriesKey string) ([]domain.ExternalCommandSnapshot, error) {
	return commandsBySeries.QueryWithin(ctx, r.tx, seriesKey)
}

var (
	_ ports.ExternalCommandsByBusinessKeyReader = (*ExternalCommandsReader)(nil)
	_ ports.ExternalCommandsByStatusReader      = (*ExternalCommandsReader)(nil)
	_ ports.ExternalCommandsByOutboundKeyReader = (*ExternalCommandsReader)(nil)
	_ ports.ExternalCommandsBySeriesReader      = (*ExternalCommandsBySeriesWithin)(nil)
)

func stateOfCommand(s domain.ExternalCommandSnapshot) commandState {
	p := s.Profile
	return commandState{
		TenantID: string(s.TenantID), Operation: s.Operation, AssetType: s.AssetType, PayloadVersion: s.PayloadVersion,
		Payload: s.Payload, PayloadHash: s.PayloadHash,
		Profile: commandShape{
			ProfileVersion: p.ProfileVersion, StateQuerySupported: p.StateQuerySupported, IdempotencySupport: string(p.IdempotencySupport),
			PartialResultSupported: p.PartialResultSupported, Efficacy: string(p.Efficacy), Quiescence: p.Quiescence,
			AuthoritativeAbsence: p.AuthoritativeAbsence, MaxQueries: p.MaxQueries, KeyAlphabet: p.KeyAlphabet, KeyMaxLength: p.KeyMaxLength,
			QueryInterval: p.QueryInterval, AmbiguityDeadline: p.AmbiguityDeadline,
		},
		FailureCategory: string(s.FailureCategory), CreatedAt: int64(s.CreatedAt), Submissions: s.Submissions,
		LastSubmittedAt: int64(s.LastSubmittedAt), WaitingSince: int64(s.WaitingSince),
		AcknowledgedNotified: s.AcknowledgedNotified, UnknownNotified: s.UnknownNotified,
		NextQueryAt: int64(s.NextQueryAt), ResendAfter: int64(s.ResendAfter), ReconciliationRequired: s.ReconciliationRequired,
		AmbiguityExpiredAt: int64(s.AmbiguityExpiredAt), ConfirmedAt: int64(s.ConfirmedAt), ExternalReferenceID: string(s.ExternalReferenceID),
	}
}

func commandOf(state commandState) domain.ExternalCommandSnapshot {
	p := state.Profile
	return domain.ExternalCommandSnapshot{
		TenantID: domain.TenantID(state.TenantID), Operation: state.Operation, AssetType: state.AssetType,
		PayloadVersion: state.PayloadVersion, Payload: state.Payload, PayloadHash: state.PayloadHash,
		Profile: domain.CommandProfile{
			ProfileVersion: p.ProfileVersion, StateQuerySupported: p.StateQuerySupported, IdempotencySupport: domain.IdempotencySupport(p.IdempotencySupport),
			PartialResultSupported: p.PartialResultSupported, Efficacy: domain.Efficacy(p.Efficacy), Quiescence: p.Quiescence,
			AuthoritativeAbsence: p.AuthoritativeAbsence, MaxQueries: p.MaxQueries, KeyAlphabet: p.KeyAlphabet, KeyMaxLength: p.KeyMaxLength,
			QueryInterval: p.QueryInterval, AmbiguityDeadline: p.AmbiguityDeadline,
		},
		FailureCategory: domain.FailureCategory(state.FailureCategory), CreatedAt: kernel.Instant(state.CreatedAt), Submissions: state.Submissions,
		LastSubmittedAt: kernel.Instant(state.LastSubmittedAt), WaitingSince: kernel.Instant(state.WaitingSince),
		AcknowledgedNotified: state.AcknowledgedNotified, UnknownNotified: state.UnknownNotified,
		NextQueryAt: kernel.Instant(state.NextQueryAt), ResendAfter: kernel.Instant(state.ResendAfter),
		ReconciliationRequired: state.ReconciliationRequired, AmbiguityExpiredAt: kernel.Instant(state.AmbiguityExpiredAt),
		ConfirmedAt: kernel.Instant(state.ConfirmedAt), ExternalReferenceID: domain.ExternalReferenceID(state.ExternalReferenceID),
	}
}
