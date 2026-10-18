package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

type profileState struct {
	Version                  int      `json:"version"`
	ChangeKind               string   `json:"changeKind"`
	Protocols                []string `json:"protocols"`
	Endpoint                 string   `json:"endpoint"`
	SchemaVersion            string   `json:"schemaVersion"`
	SecretRef                string   `json:"secretRef"`
	CertificateRef           string   `json:"certificateRef"`
	EffectiveFrom            int64    `json:"effectiveFrom"`
	LastAlertedThresholdDays int      `json:"lastAlertedThresholdDays"`
}

var profileTable = postgres.Table[domain.ProfileID, domain.IntegrationProfileSnapshot]{
	Name:     "integration_profiles",
	IDColumn: "integration_profile_id",
	Columns:  []string{"partner_id", "environment", "certificate_expires_at", "supersedes_id", "snapshot"},

	Encode: func(s domain.IntegrationProfileSnapshot) ([]any, error) {
		raw, err := json.Marshal(profileState{
			Version:                  s.Version,
			ChangeKind:               string(s.ChangeKind),
			Protocols:                append(make([]string, 0, len(s.Protocols)), s.Protocols...),
			Endpoint:                 s.Endpoint,
			SchemaVersion:            s.SchemaVersion,
			SecretRef:                s.SecretRef,
			CertificateRef:           s.CertificateRef,
			EffectiveFrom:            int64(s.EffectiveFrom),
			LastAlertedThresholdDays: s.LastAlertedThresholdDays,
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.PartnerID), string(s.Environment), int64(s.CertificateExpiresAt), nullable(string(s.SupersedesID)), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.IntegrationProfileSnapshot, error) {
		var (
			partnerID, environment string
			certificateExpiresAt   int64
			supersedesID           *string
			raw                    []byte
		)
		if err := scan(&partnerID, &environment, &certificateExpiresAt, &supersedesID, &raw); err != nil {
			return domain.IntegrationProfileSnapshot{}, err
		}
		var state profileState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.IntegrationProfileSnapshot{}, err
		}
		return domain.IntegrationProfileSnapshot{
			PartnerID:                domain.PartnerID(partnerID),
			Environment:              domain.Environment(environment),
			Version:                  state.Version,
			SupersedesID:             domain.ProfileID(valueOf(supersedesID)),
			ChangeKind:               domain.IntegrationChangeKind(state.ChangeKind),
			Protocols:                state.Protocols,
			Endpoint:                 state.Endpoint,
			SchemaVersion:            state.SchemaVersion,
			SecretRef:                state.SecretRef,
			CertificateRef:           state.CertificateRef,
			CertificateExpiresAt:     kernel.Instant(certificateExpiresAt),
			EffectiveFrom:            kernel.Instant(state.EffectiveFrom),
			LastAlertedThresholdDays: state.LastAlertedThresholdDays,
		}, nil
	},

	WithID: func(s domain.IntegrationProfileSnapshot, id domain.ProfileID) domain.IntegrationProfileSnapshot {
		s.ID = id
		return s
	},
}

func NewIntegrationProfileRepository(tx *postgres.Tx) ports.IntegrationProfileRepository {
	return profileTable.Repository(tx)
}
