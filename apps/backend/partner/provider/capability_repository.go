package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

type capabilityState struct {
	Version             int              `json:"version"`
	IsWrite             bool             `json:"isWrite"`
	StateQuerySupported bool             `json:"stateQuerySupported"`
	RestrictionNote     string           `json:"restrictionNote"`
	CallbackApplicable  bool             `json:"callbackApplicable"`
	Homologation        []criterionState `json:"homologation"`
	Rollout             rolloutState     `json:"rollout"`
	Suspension          suspensionState  `json:"suspension"`
	EnabledAt           int64            `json:"enabledAt"`
}

type criterionState struct {
	CriterionID string `json:"criterionId"`
	Passed      bool   `json:"passed"`
	EvidenceID  string `json:"evidenceId"`
	RecordedAt  int64  `json:"recordedAt"`
}

type rolloutState struct {
	Mode               string   `json:"mode"`
	AllowlistTenantIDs []string `json:"allowlistTenantIds"`
	Percentage         int      `json:"percentage"`
	ExcludedTenantIDs  []string `json:"excludedTenantIds"`
}

type suspensionState struct {
	Reason      string `json:"reason"`
	Trigger     string `json:"trigger"`
	SuspendedAt int64  `json:"suspendedAt"`
}

var capabilityTable = postgres.Table[domain.CapabilityID, domain.CapabilitySnapshot]{
	Name:     "capabilities",
	IDColumn: "capability_id",
	Columns:  []string{"partner_id", "operation", "asset_type", "environment", "status", "supersedes_id", "snapshot"},

	Encode: func(s domain.CapabilitySnapshot) ([]any, error) {
		raw, err := json.Marshal(stateOfCapability(s))
		if err != nil {
			return nil, err
		}
		return []any{string(s.PartnerID), s.Operation, s.AssetType, string(s.Environment), string(s.Status), nullable(string(s.SupersedesID)), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.CapabilitySnapshot, error) {
		var (
			partnerID, operation, assetType, environment, status string
			supersedesID                                         *string
			raw                                                  []byte
		)
		if err := scan(&partnerID, &operation, &assetType, &environment, &status, &supersedesID, &raw); err != nil {
			return domain.CapabilitySnapshot{}, err
		}
		var state capabilityState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.CapabilitySnapshot{}, err
		}
		return domain.CapabilitySnapshot{
			PartnerID:           domain.PartnerID(partnerID),
			Operation:           operation,
			AssetType:           assetType,
			Environment:         domain.Environment(environment),
			Version:             state.Version,
			SupersedesID:        domain.CapabilityID(valueOf(supersedesID)),
			IsWrite:             state.IsWrite,
			StateQuerySupported: state.StateQuerySupported,
			RestrictionNote:     state.RestrictionNote,
			CallbackApplicable:  state.CallbackApplicable,
			Status:              domain.CapabilityStatus(status),
			Homologation:        homologationOf(state.Homologation),
			Rollout: domain.Rollout{
				Mode:               domain.RolloutMode(state.Rollout.Mode),
				AllowlistTenantIDs: state.Rollout.AllowlistTenantIDs,
				Percentage:         state.Rollout.Percentage,
				ExcludedTenantIDs:  state.Rollout.ExcludedTenantIDs,
			},
			Suspension: domain.Suspension{
				Reason:      state.Suspension.Reason,
				Trigger:     domain.SuspensionTrigger(state.Suspension.Trigger),
				SuspendedAt: kernel.Instant(state.Suspension.SuspendedAt),
			},
			EnabledAt: kernel.Instant(state.EnabledAt),
		}, nil
	},

	WithID: func(s domain.CapabilitySnapshot, id domain.CapabilityID) domain.CapabilitySnapshot {
		s.ID = id
		return s
	},
}

func NewCapabilityRepository(tx *postgres.Tx) ports.CapabilityRepository {
	return capabilityTable.Repository(tx)
}

func stateOfCapability(s domain.CapabilitySnapshot) capabilityState {
	homologation := make([]criterionState, 0, len(s.Homologation))
	for _, r := range s.Homologation {
		homologation = append(homologation, criterionState{CriterionID: r.CriterionID, Passed: r.Passed, EvidenceID: r.EvidenceID, RecordedAt: int64(r.RecordedAt)})
	}
	return capabilityState{
		Version:             s.Version,
		IsWrite:             s.IsWrite,
		StateQuerySupported: s.StateQuerySupported,
		RestrictionNote:     s.RestrictionNote,
		CallbackApplicable:  s.CallbackApplicable,
		Homologation:        homologation,
		Rollout: rolloutState{
			Mode:               string(s.Rollout.Mode),
			AllowlistTenantIDs: append(make([]string, 0, len(s.Rollout.AllowlistTenantIDs)), s.Rollout.AllowlistTenantIDs...),
			Percentage:         s.Rollout.Percentage,
			ExcludedTenantIDs:  append(make([]string, 0, len(s.Rollout.ExcludedTenantIDs)), s.Rollout.ExcludedTenantIDs...),
		},
		Suspension: suspensionState{
			Reason:      s.Suspension.Reason,
			Trigger:     string(s.Suspension.Trigger),
			SuspendedAt: int64(s.Suspension.SuspendedAt),
		},
		EnabledAt: int64(s.EnabledAt),
	}
}

func homologationOf(stored []criterionState) []domain.CriterionResult {
	out := make([]domain.CriterionResult, 0, len(stored))
	for _, r := range stored {
		out = append(out, domain.CriterionResult{CriterionID: r.CriterionID, Passed: r.Passed, EvidenceID: r.EvidenceID, RecordedAt: kernel.Instant(r.RecordedAt)})
	}
	return out
}
