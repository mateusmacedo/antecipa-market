package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

// agreementState is the snapshot column, under JSON names fixed here like
// partnerState; the version of the domain lives here, apart from the version
// column, which is the optimistic lock of the kernel Table.
type agreementState struct {
	Version              int    `json:"version"`
	ValidFrom            int64  `json:"validFrom"`
	ValidUntil           int64  `json:"validUntil"`
	ContractEvidenceID   string `json:"contractEvidenceId"`
	InstrumentTemplateID string `json:"instrumentTemplateId"`
	ActivatedAt          int64  `json:"activatedAt"`
}

var agreementTable = postgres.Table[domain.AgreementID, domain.PartnerAgreementSnapshot]{
	Name:     "partner_agreements",
	IDColumn: "partner_agreement_id",
	Columns:  []string{"partner_id", "status", "supersedes_id", "snapshot"},
	Guard:    "partner_id",
	Unique: []postgres.Unique{
		{Name: "partner_agreements_tenant_id_supersedes_id_key", Columns: []string{"tenant_id", "supersedes_id"}, Rejection: domain.RejectPartnerAgreementSupersedesInvalid},
		{Name: "partner_agreements_tenant_id_series_origin_key", Columns: []string{"tenant_id", "series_origin"}, Rejection: domain.RejectPartnerAgreementSupersedesInvalid},
	},

	Encode: func(s domain.PartnerAgreementSnapshot) ([]any, error) {
		raw, err := json.Marshal(agreementState{
			Version:              s.Version,
			ValidFrom:            int64(s.ValidFrom),
			ValidUntil:           int64(s.ValidUntil),
			ContractEvidenceID:   s.ContractEvidenceID,
			InstrumentTemplateID: s.InstrumentTemplateID,
			ActivatedAt:          int64(s.ActivatedAt),
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.PartnerID), string(s.Status), nullable(string(s.SupersedesID)), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.PartnerAgreementSnapshot, error) {
		var (
			partnerID, status string
			supersedesID      *string
			raw               []byte
		)
		if err := scan(&partnerID, &status, &supersedesID, &raw); err != nil {
			return domain.PartnerAgreementSnapshot{}, err
		}
		var state agreementState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.PartnerAgreementSnapshot{}, err
		}
		return domain.PartnerAgreementSnapshot{
			PartnerID:            domain.PartnerID(partnerID),
			Version:              state.Version,
			SupersedesID:         domain.AgreementID(valueOf(supersedesID)),
			Status:               domain.AgreementStatus(status),
			ValidFrom:            kernel.Instant(state.ValidFrom),
			ValidUntil:           kernel.Instant(state.ValidUntil),
			ContractEvidenceID:   state.ContractEvidenceID,
			InstrumentTemplateID: state.InstrumentTemplateID,
			ActivatedAt:          kernel.Instant(state.ActivatedAt),
		}, nil
	},

	WithID: func(s domain.PartnerAgreementSnapshot, id domain.AgreementID) domain.PartnerAgreementSnapshot {
		s.ID = id
		return s
	},
}

func NewAgreementRepository(tx *postgres.Tx) ports.AgreementRepository {
	return agreementTable.Repository(tx)
}

// nullable stores the first version of a series as NULL: series_origin keys
// the series only where supersedes_id is NULL, and the unique supersedes_id
// skips NULLs, which an empty string would not.
func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
