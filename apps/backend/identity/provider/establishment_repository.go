package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// establishmentState is the snapshot column, under JSON names fixed here like
// organizationState. The tax id stays in clear until the kernel provides
// envelope encryption for personal data.
type establishmentState struct {
	TaxIDType           string                   `json:"taxIdType"`
	TaxID               string                   `json:"taxId"`
	TaxIDHashKeyVersion int                      `json:"taxIdHashKeyVersion"`
	TaxIDMasked         string                   `json:"taxIdMasked"`
	TradeName           string                   `json:"tradeName"`
	Receiver            bool                     `json:"receiver"`
	ExternalReferences  []externalReferenceState `json:"externalReferences"`
	CreatedAt           int64                    `json:"createdAt"`
}

type externalReferenceState struct {
	ProviderID      string `json:"providerId"`
	System          string `json:"system"`
	ReferenceType   string `json:"referenceType"`
	Reference       string `json:"reference"`
	AsOf            int64  `json:"asOf"`
	ObservedVersion string `json:"observedVersion"`
}

var establishmentTable = postgres.Table[domain.EstablishmentID, domain.EstablishmentSnapshot]{
	Name:     "establishments",
	IDColumn: "establishment_id",
	Columns:  []string{"organization_id", "tax_id_hash", "snapshot"},
	Unique: []postgres.Unique{{
		Name:      "establishments_tenant_id_organization_id_tax_id_hash_key",
		Columns:   []string{"tenant_id", "organization_id", "tax_id_hash"},
		Rejection: domain.RejectEstablishmentDuplicateTaxID,
	}},

	Encode: func(s domain.EstablishmentSnapshot) ([]any, error) {
		// A nil slice would be stored as null rather than [] (armadilha 11).
		references := make([]externalReferenceState, 0, len(s.ExternalReferences))
		for _, ref := range s.ExternalReferences {
			references = append(references, externalReferenceState{
				ProviderID:      ref.ProviderID,
				System:          string(ref.System),
				ReferenceType:   string(ref.ReferenceType),
				Reference:       ref.Reference,
				AsOf:            int64(ref.AsOf),
				ObservedVersion: ref.ObservedVersion,
			})
		}
		raw, err := json.Marshal(establishmentState{
			TaxIDType:           string(s.TaxIDType),
			TaxID:               s.TaxID,
			TaxIDHashKeyVersion: s.TaxIDHashKeyVersion,
			TaxIDMasked:         s.TaxIDMasked,
			TradeName:           s.TradeName,
			Receiver:            s.Receiver,
			ExternalReferences:  references,
			CreatedAt:           int64(s.CreatedAt),
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.OrganizationID), s.TaxIDHash, raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.EstablishmentSnapshot, error) {
		var (
			organizationID string
			taxIDHash      string
			raw            []byte
		)
		if err := scan(&organizationID, &taxIDHash, &raw); err != nil {
			return domain.EstablishmentSnapshot{}, err
		}
		var state establishmentState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.EstablishmentSnapshot{}, err
		}
		references := make([]domain.ExternalReference, 0, len(state.ExternalReferences))
		for _, ref := range state.ExternalReferences {
			references = append(references, domain.ExternalReference{
				ProviderID:      ref.ProviderID,
				System:          domain.ExternalSystem(ref.System),
				ReferenceType:   domain.ExternalReferenceType(ref.ReferenceType),
				Reference:       ref.Reference,
				AsOf:            kernel.Instant(ref.AsOf),
				ObservedVersion: ref.ObservedVersion,
			})
		}
		return domain.EstablishmentSnapshot{
			OrganizationID:      domain.OrganizationID(organizationID),
			TaxIDType:           domain.TaxIDType(state.TaxIDType),
			TaxID:               state.TaxID,
			TaxIDHash:           taxIDHash,
			TaxIDHashKeyVersion: state.TaxIDHashKeyVersion,
			TaxIDMasked:         state.TaxIDMasked,
			TradeName:           state.TradeName,
			Receiver:            state.Receiver,
			ExternalReferences:  references,
			CreatedAt:           kernel.Instant(state.CreatedAt),
		}, nil
	},

	WithID: func(s domain.EstablishmentSnapshot, id domain.EstablishmentID) domain.EstablishmentSnapshot {
		s.ID = id
		return s
	},
}

func NewEstablishmentRepository(tx *postgres.Tx) ports.EstablishmentRepository {
	return establishmentTable.Repository(tx)
}
