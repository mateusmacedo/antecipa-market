package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

type productState struct {
	Version          int      `json:"version"`
	AgreementID      string   `json:"agreementId"`
	Name             string   `json:"name"`
	ValidFrom        int64    `json:"validFrom"`
	AssetTypes       []string `json:"assetTypes"`
	Regions          []string `json:"regions"`
	Currency         string   `json:"currency"`
	MinAmountMinor   int64    `json:"minAmountMinor"`
	MaxAmountMinor   int64    `json:"maxAmountMinor"`
	MinTermDays      int      `json:"minTermDays"`
	MaxTermDays      int      `json:"maxTermDays"`
	RequiredCheckIDs []string `json:"requiredCheckIds"`
}

var productTable = postgres.Table[domain.ProductID, domain.FinancialProductSnapshot]{
	Name:     "financial_products",
	IDColumn: "financial_product_id",
	Columns:  []string{"partner_id", "code", "status", "valid_until", "supersedes_id", "snapshot"},
	Unique: []postgres.Unique{
		{Name: "financial_products_tenant_id_supersedes_id_key", Columns: []string{"tenant_id", "supersedes_id"}, Rejection: domain.RejectFinancialProductSupersedesInvalid},
		{Name: "financial_products_tenant_id_series_origin_key", Columns: []string{"tenant_id", "series_origin"}, Rejection: domain.RejectFinancialProductSupersedesInvalid},
	},

	Encode: func(s domain.FinancialProductSnapshot) ([]any, error) {
		raw, err := json.Marshal(productState{
			Version:          s.Version,
			AgreementID:      string(s.AgreementID),
			Name:             s.Name,
			ValidFrom:        int64(s.ValidFrom),
			AssetTypes:       append(make([]string, 0, len(s.AssetTypes)), s.AssetTypes...),
			Regions:          append(make([]string, 0, len(s.Regions)), s.Regions...),
			Currency:         s.Currency,
			MinAmountMinor:   s.MinAmountMinor,
			MaxAmountMinor:   s.MaxAmountMinor,
			MinTermDays:      s.MinTermDays,
			MaxTermDays:      s.MaxTermDays,
			RequiredCheckIDs: append(make([]string, 0, len(s.RequiredCheckIDs)), s.RequiredCheckIDs...),
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.PartnerID), s.Code, string(s.Status), int64(s.ValidUntil), nullable(string(s.SupersedesID)), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.FinancialProductSnapshot, error) {
		var (
			partnerID, code, status string
			validUntil              int64
			supersedesID            *string
			raw                     []byte
		)
		if err := scan(&partnerID, &code, &status, &validUntil, &supersedesID, &raw); err != nil {
			return domain.FinancialProductSnapshot{}, err
		}
		var state productState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.FinancialProductSnapshot{}, err
		}
		return domain.FinancialProductSnapshot{
			PartnerID:        domain.PartnerID(partnerID),
			AgreementID:      domain.AgreementID(state.AgreementID),
			Code:             code,
			Name:             state.Name,
			Version:          state.Version,
			SupersedesID:     domain.ProductID(valueOf(supersedesID)),
			Status:           domain.ProductStatus(status),
			ValidFrom:        kernel.Instant(state.ValidFrom),
			ValidUntil:       kernel.Instant(validUntil),
			AssetTypes:       state.AssetTypes,
			Regions:          state.Regions,
			Currency:         state.Currency,
			MinAmountMinor:   state.MinAmountMinor,
			MaxAmountMinor:   state.MaxAmountMinor,
			MinTermDays:      state.MinTermDays,
			MaxTermDays:      state.MaxTermDays,
			RequiredCheckIDs: state.RequiredCheckIDs,
		}, nil
	},

	WithID: func(s domain.FinancialProductSnapshot, id domain.ProductID) domain.FinancialProductSnapshot {
		s.ID = id
		return s
	},
}

func NewFinancialProductRepository(tx *postgres.Tx) ports.FinancialProductRepository {
	return productTable.Repository(tx)
}
