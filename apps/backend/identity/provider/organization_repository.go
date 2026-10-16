package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// organizationState is the snapshot column: the fields no query filters by,
// under JSON names fixed here so a rename in the domain does not rewrite stored
// rows. The personal data fields stay in clear until the kernel provides
// envelope encryption.
type organizationState struct {
	Kinds               []string `json:"kinds"`
	TaxIDType           string   `json:"taxIdType"`
	TaxID               string   `json:"taxId"`
	TaxIDHashKeyVersion int      `json:"taxIdHashKeyVersion"`
	TaxIDMasked         string   `json:"taxIdMasked"`
	LegalName           string   `json:"legalName"`
	TradeName           string   `json:"tradeName"`
	Status              int      `json:"status"`
	CreatedAt           int64    `json:"createdAt"`
}

// Only the columns are declared here: the statements, and with them the tenant
// predicate, are the kernel's, so tenant isolation never depends on each author
// remembering the condition.
var organizationTable = postgres.Table[domain.OrganizationID, domain.OrganizationSnapshot]{
	Name:     "organizations",
	IDColumn: "organization_id",
	Columns:  []string{"tax_id_hash", "jurisdiction", "snapshot"},

	Encode: func(s domain.OrganizationSnapshot) ([]any, error) {
		listed := s.Kinds.List()
		kinds := make([]string, 0, len(listed))
		for _, kind := range listed {
			kinds = append(kinds, string(kind))
		}
		raw, err := json.Marshal(organizationState{
			Kinds:               kinds,
			TaxIDType:           string(s.TaxIDType),
			TaxID:               s.TaxID,
			TaxIDHashKeyVersion: s.TaxIDHashKeyVersion,
			TaxIDMasked:         s.TaxIDMasked,
			LegalName:           s.LegalName,
			TradeName:           s.TradeName,
			Status:              int(s.Status),
			CreatedAt:           int64(s.CreatedAt),
		})
		if err != nil {
			return nil, err
		}
		return []any{s.TaxIDHash, s.Jurisdiction, raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.OrganizationSnapshot, error) {
		var (
			taxIDHash    string
			jurisdiction string
			raw          []byte
		)
		if err := scan(&taxIDHash, &jurisdiction, &raw); err != nil {
			return domain.OrganizationSnapshot{}, err
		}
		var state organizationState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.OrganizationSnapshot{}, err
		}
		kinds := make([]domain.OrganizationKind, 0, len(state.Kinds))
		for _, kind := range state.Kinds {
			kinds = append(kinds, domain.OrganizationKind(kind))
		}
		return domain.OrganizationSnapshot{
			Kinds:               domain.KindsOf(kinds...),
			Jurisdiction:        jurisdiction,
			TaxIDType:           domain.TaxIDType(state.TaxIDType),
			TaxID:               state.TaxID,
			TaxIDHash:           taxIDHash,
			TaxIDHashKeyVersion: state.TaxIDHashKeyVersion,
			TaxIDMasked:         state.TaxIDMasked,
			LegalName:           state.LegalName,
			TradeName:           state.TradeName,
			Status:              domain.OrganizationStatus(state.Status),
			CreatedAt:           kernel.Instant(state.CreatedAt),
		}, nil
	},

	WithID: func(s domain.OrganizationSnapshot, id domain.OrganizationID) domain.OrganizationSnapshot {
		s.ID = id
		return s
	},
}

func NewOrganizationRepository(tx *postgres.Tx) ports.OrganizationRepository {
	return organizationTable.Repository(tx)
}
