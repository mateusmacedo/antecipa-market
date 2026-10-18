package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

type policyState struct {
	Version            int      `json:"version"`
	EffectiveFrom      int64    `json:"effectiveFrom"`
	RateLimitPerMinute int      `json:"rateLimitPerMinute"`
	BusinessDays       []string `json:"businessDays"`
	CutoffTime         string   `json:"cutoffTime"`
	ProcessingWindow   string   `json:"processingWindow"`
	SlaRef             string   `json:"slaRef"`
	Fallback           string   `json:"fallback"`
	SupportContact     string   `json:"supportContact"`
	IncidentContact    string   `json:"incidentContact"`
}

var policyTable = postgres.Table[domain.PolicyID, domain.OperationalPolicySnapshot]{
	Name:     "operational_policies",
	IDColumn: "operational_policy_id",
	Columns:  []string{"partner_id", "supersedes_id", "snapshot"},

	Encode: func(s domain.OperationalPolicySnapshot) ([]any, error) {
		raw, err := json.Marshal(policyState{
			Version:            s.Version,
			EffectiveFrom:      int64(s.EffectiveFrom),
			RateLimitPerMinute: s.RateLimitPerMinute,
			BusinessDays:       append(make([]string, 0, len(s.BusinessDays)), s.BusinessDays...),
			CutoffTime:         s.CutoffTime,
			ProcessingWindow:   s.ProcessingWindow,
			SlaRef:             s.SlaRef,
			Fallback:           s.Fallback,
			SupportContact:     s.SupportContact,
			IncidentContact:    s.IncidentContact,
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.PartnerID), nullable(string(s.SupersedesID)), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.OperationalPolicySnapshot, error) {
		var (
			partnerID    string
			supersedesID *string
			raw          []byte
		)
		if err := scan(&partnerID, &supersedesID, &raw); err != nil {
			return domain.OperationalPolicySnapshot{}, err
		}
		var state policyState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.OperationalPolicySnapshot{}, err
		}
		return domain.OperationalPolicySnapshot{
			PartnerID:          domain.PartnerID(partnerID),
			Version:            state.Version,
			SupersedesID:       domain.PolicyID(valueOf(supersedesID)),
			EffectiveFrom:      kernel.Instant(state.EffectiveFrom),
			RateLimitPerMinute: state.RateLimitPerMinute,
			BusinessDays:       state.BusinessDays,
			CutoffTime:         state.CutoffTime,
			ProcessingWindow:   state.ProcessingWindow,
			SlaRef:             state.SlaRef,
			Fallback:           state.Fallback,
			SupportContact:     state.SupportContact,
			IncidentContact:    state.IncidentContact,
		}, nil
	},

	WithID: func(s domain.OperationalPolicySnapshot, id domain.PolicyID) domain.OperationalPolicySnapshot {
		s.ID = id
		return s
	},
}

func NewOperationalPolicyRepository(tx *postgres.Tx) ports.OperationalPolicyRepository {
	return policyTable.Repository(tx)
}
