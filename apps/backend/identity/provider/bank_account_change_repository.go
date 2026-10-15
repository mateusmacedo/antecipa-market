package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

type bankAccountChangeState struct {
	DestinationEvidenceID string `json:"destinationEvidenceId"`
	RequestedByUserID     string `json:"requestedByUserId"`
	ApprovedByUserID      string `json:"approvedByUserId"`
	Status                int    `json:"status"`
	RequestedAt           int64  `json:"requestedAt"`
	ApprovedAt            int64  `json:"approvedAt"`
}

var bankAccountChangeTable = postgres.Table[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]{
	Name:     "bank_account_changes",
	IDColumn: "bank_account_change_id",
	Columns:  []string{"organization_id", "snapshot"},

	Encode: func(s domain.BankAccountChangeSnapshot) ([]any, error) {
		raw, err := json.Marshal(bankAccountChangeState{
			DestinationEvidenceID: s.DestinationEvidenceID,
			RequestedByUserID:     s.RequestedByUserID,
			ApprovedByUserID:      s.ApprovedByUserID,
			Status:                int(s.Status),
			RequestedAt:           int64(s.RequestedAt),
			ApprovedAt:            int64(s.ApprovedAt),
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.OrganizationID), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.BankAccountChangeSnapshot, error) {
		var (
			organizationID string
			raw            []byte
		)
		if err := scan(&organizationID, &raw); err != nil {
			return domain.BankAccountChangeSnapshot{}, err
		}
		var state bankAccountChangeState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.BankAccountChangeSnapshot{}, err
		}
		return domain.BankAccountChangeSnapshot{
			OrganizationID:        domain.OrganizationID(organizationID),
			DestinationEvidenceID: state.DestinationEvidenceID,
			RequestedByUserID:     state.RequestedByUserID,
			ApprovedByUserID:      state.ApprovedByUserID,
			Status:                domain.BankAccountChangeStatus(state.Status),
			RequestedAt:           kernel.Instant(state.RequestedAt),
			ApprovedAt:            kernel.Instant(state.ApprovedAt),
		}, nil
	},

	WithID: func(s domain.BankAccountChangeSnapshot, id domain.BankAccountChangeID) domain.BankAccountChangeSnapshot {
		s.ID = id
		return s
	},
}

func NewBankAccountChangeRepository(tx *postgres.Tx) ports.BankAccountChangeRepository {
	return bankAccountChangeTable.Repository(tx)
}
