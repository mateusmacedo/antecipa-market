package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

// partnerState is the snapshot column, under JSON names fixed here so a rename
// in the domain does not rewrite stored rows.
type partnerState struct {
	RegisteredAt         int64  `json:"registeredAt"`
	StageChangedAt       int64  `json:"stageChangedAt"`
	StageReason          string `json:"stageReason"`
	OffboardingStartedAt int64  `json:"offboardingStartedAt"`
}

var partnerTable = postgres.Table[domain.PartnerID, domain.PartnerSnapshot]{
	Name:     "partners",
	IDColumn: "partner_id",
	Columns:  []string{"organization_id", "role", "status", "snapshot"},
	Unique: []postgres.Unique{{
		Name:      "partners_tenant_id_organization_id_role_key",
		Columns:   []string{"tenant_id", "organization_id", "role"},
		Rejection: domain.RejectPartnerAlreadyRegistered,
	}},

	Encode: func(s domain.PartnerSnapshot) ([]any, error) {
		raw, err := json.Marshal(partnerState{
			RegisteredAt:         int64(s.RegisteredAt),
			StageChangedAt:       int64(s.StageChangedAt),
			StageReason:          s.StageReason,
			OffboardingStartedAt: int64(s.OffboardingStartedAt),
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.OrganizationID), string(s.Role), string(s.Status), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.PartnerSnapshot, error) {
		var (
			organizationID, role, status string
			raw                          []byte
		)
		if err := scan(&organizationID, &role, &status, &raw); err != nil {
			return domain.PartnerSnapshot{}, err
		}
		var state partnerState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.PartnerSnapshot{}, err
		}
		return domain.PartnerSnapshot{
			OrganizationID:       domain.OrganizationID(organizationID),
			Role:                 domain.PartnerRole(role),
			Status:               domain.PartnerStatus(status),
			RegisteredAt:         kernel.Instant(state.RegisteredAt),
			StageChangedAt:       kernel.Instant(state.StageChangedAt),
			StageReason:          state.StageReason,
			OffboardingStartedAt: kernel.Instant(state.OffboardingStartedAt),
		}, nil
	},

	WithID: func(s domain.PartnerSnapshot, id domain.PartnerID) domain.PartnerSnapshot {
		s.ID = id
		return s
	},
}

func NewPartnerRepository(tx *postgres.Tx) ports.PartnerRepository {
	return partnerTable.Repository(tx)
}
