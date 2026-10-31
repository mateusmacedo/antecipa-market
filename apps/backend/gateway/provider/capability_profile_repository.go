package provider

import (
	"context"
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

type profileState struct {
	EffectiveFrom int64            `json:"effectiveFrom"`
	Operations    []operationState `json:"operations"`
}

type operationState struct {
	Operation              string   `json:"operation"`
	Supported              bool     `json:"supported"`
	Protocol               string   `json:"protocol"`
	StateQuerySupported    bool     `json:"stateQuerySupported"`
	IdempotencySupport     string   `json:"idempotencySupport"`
	PartialResultSupported bool     `json:"partialResultSupported"`
	AssetTypes             []string `json:"assetTypes"`
	MaxItemsPerRequest     int      `json:"maxItemsPerRequest"`
	MaxRequestsPerMinute   int      `json:"maxRequestsPerMinute"`
	MaxPayloadBytes        int      `json:"maxPayloadBytes"`
	BusinessCalendar       string   `json:"businessCalendar"`
	CutoffTime             string   `json:"cutoffTime"`
	ProcessingWindow       string   `json:"processingWindow"`
	EncumbranceCoverage    string   `json:"encumbranceCoverage"`
	PositionFreshnessSLA   int      `json:"positionFreshnessSla"`
	Efficacy               string   `json:"efficacy"`
	Quiescence             int64    `json:"quiescence"`
	AuthoritativeAbsence   bool     `json:"authoritativeAbsence"`
	MaxQueries             int      `json:"maxQueries"`
	KeyAlphabet            string   `json:"keyAlphabet"`
	KeyMaxLength           int      `json:"keyMaxLength"`
	QueryInterval          int      `json:"queryInterval"`
	AmbiguityDeadline      int      `json:"ambiguityDeadline"`
}

// profileTable guards connector_id: declaring a version takes the exclusive
// lock that CreateExternalCommand's read of the profile in force shares.
var profileTable = postgres.Table[domain.ProfileKey, domain.CapabilityProfileSnapshot]{
	Name:     "capability_profiles",
	IDColumn: "capability_profile_id",
	Columns:  []string{"connector_id", "profile_version", "snapshot"},
	Guard:    "connector_id",

	Encode: func(s domain.CapabilityProfileSnapshot) ([]any, error) {
		raw, err := json.Marshal(profileState{EffectiveFrom: int64(s.EffectiveFrom), Operations: operationStates(s.Operations)})
		if err != nil {
			return nil, err
		}
		return []any{string(s.ConnectorKey), s.Version, raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.CapabilityProfileSnapshot, error) {
		var (
			connector string
			version   int
			raw       []byte
		)
		if err := scan(&connector, &version, &raw); err != nil {
			return domain.CapabilityProfileSnapshot{}, err
		}
		var state profileState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.CapabilityProfileSnapshot{}, err
		}
		return domain.CapabilityProfileSnapshot{
			ConnectorKey: domain.ConnectorKey(connector), Version: version,
			EffectiveFrom: kernel.Instant(state.EffectiveFrom), Operations: operationsOf(state.Operations),
		}, nil
	},

	WithID: func(s domain.CapabilityProfileSnapshot, key domain.ProfileKey) domain.CapabilityProfileSnapshot {
		s.Key = key
		return s
	},
}

func NewCapabilityProfileRepository(tx *postgres.Tx) ports.CapabilityProfileRepository {
	return profileTable.Repository(tx)
}

func NewCapabilityProfileReader(pool postgres.ReadPool) ports.CapabilityProfileReader {
	return profileTable.Reader(pool)
}

var profilesByConnector = profileTable.Relation("connector_id")

type CapabilityProfilesByConnectorReader struct{ pool postgres.ReadPool }

func NewCapabilityProfilesByConnectorReader(pool postgres.ReadPool) *CapabilityProfilesByConnectorReader {
	return &CapabilityProfilesByConnectorReader{pool: pool}
}

func (r *CapabilityProfilesByConnectorReader) LoadByConnector(ctx context.Context, connector domain.ConnectorKey) ([]domain.CapabilityProfileSnapshot, error) {
	return profilesByConnector.Query(ctx, r.pool, string(connector))
}

// CapabilityProfilesByConnectorWithin reads the versions inside the command
// transaction, under the guard on connector_id (ADR-067).
type CapabilityProfilesByConnectorWithin struct{ tx *postgres.Tx }

func NewCapabilityProfilesByConnectorWithin(tx *postgres.Tx) *CapabilityProfilesByConnectorWithin {
	return &CapabilityProfilesByConnectorWithin{tx: tx}
}

func (r *CapabilityProfilesByConnectorWithin) LoadByConnector(ctx context.Context, connector domain.ConnectorKey) ([]domain.CapabilityProfileSnapshot, error) {
	return profilesByConnector.QueryWithin(ctx, r.tx, string(connector))
}

var (
	_ ports.CapabilityProfilesByConnectorReader = (*CapabilityProfilesByConnectorReader)(nil)
	_ ports.CapabilityProfilesByConnectorReader = (*CapabilityProfilesByConnectorWithin)(nil)
)

func operationStates(operations []domain.OperationCapability) []operationState {
	out := make([]operationState, 0, len(operations))
	for _, o := range operations {
		out = append(out, operationState{
			Operation: o.Operation, Supported: o.Supported, Protocol: string(o.Protocol), StateQuerySupported: o.StateQuerySupported,
			IdempotencySupport: string(o.IdempotencySupport), PartialResultSupported: o.PartialResultSupported,
			AssetTypes:         append(make([]string, 0, len(o.AssetTypes)), o.AssetTypes...),
			MaxItemsPerRequest: o.MaxItemsPerRequest, MaxRequestsPerMinute: o.MaxRequestsPerMinute, MaxPayloadBytes: o.MaxPayloadBytes,
			BusinessCalendar: o.BusinessCalendar, CutoffTime: o.CutoffTime, ProcessingWindow: o.ProcessingWindow,
			EncumbranceCoverage: string(o.EncumbranceCoverage), PositionFreshnessSLA: o.PositionFreshnessSLA, Efficacy: string(o.Efficacy),
			Quiescence: o.Quiescence, AuthoritativeAbsence: o.AuthoritativeAbsence, MaxQueries: o.MaxQueries,
			KeyAlphabet: o.KeyAlphabet, KeyMaxLength: o.KeyMaxLength, QueryInterval: o.QueryInterval, AmbiguityDeadline: o.AmbiguityDeadline,
		})
	}
	return out
}

func operationsOf(stored []operationState) []domain.OperationCapability {
	out := make([]domain.OperationCapability, 0, len(stored))
	for _, o := range stored {
		out = append(out, domain.OperationCapability{
			Operation: o.Operation, Supported: o.Supported, Protocol: domain.Protocol(o.Protocol), StateQuerySupported: o.StateQuerySupported,
			IdempotencySupport: domain.IdempotencySupport(o.IdempotencySupport), PartialResultSupported: o.PartialResultSupported,
			AssetTypes:         o.AssetTypes,
			MaxItemsPerRequest: o.MaxItemsPerRequest, MaxRequestsPerMinute: o.MaxRequestsPerMinute, MaxPayloadBytes: o.MaxPayloadBytes,
			BusinessCalendar: o.BusinessCalendar, CutoffTime: o.CutoffTime, ProcessingWindow: o.ProcessingWindow,
			EncumbranceCoverage: domain.EncumbranceCoverage(o.EncumbranceCoverage), PositionFreshnessSLA: o.PositionFreshnessSLA,
			Efficacy: domain.Efficacy(o.Efficacy), Quiescence: o.Quiescence, AuthoritativeAbsence: o.AuthoritativeAbsence,
			MaxQueries: o.MaxQueries, KeyAlphabet: o.KeyAlphabet, KeyMaxLength: o.KeyMaxLength,
			QueryInterval: o.QueryInterval, AmbiguityDeadline: o.AmbiguityDeadline,
		})
	}
	return out
}
