package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

type representationState struct {
	PowerType           string   `json:"powerType"`
	SourceType          string   `json:"sourceType"`
	SourceEvidenceID    string   `json:"sourceEvidenceId"`
	ScopeCurrency       string   `json:"scopeCurrency"`
	ScopeMaxAmountMinor int64    `json:"scopeMaxAmountMinor"`
	ScopeActTypes       []string `json:"scopeActTypes"`
	ValidFrom           int64    `json:"validFrom"`
	ValidUntil          int64    `json:"validUntil"`
	VerificationStatus  int      `json:"verificationStatus"`
	ProposedByUserID    string   `json:"proposedByUserId"`
}

var representationTable = postgres.Table[domain.RepresentationID, domain.RepresentationSnapshot]{
	Name:     "representations",
	IDColumn: "representation_id",
	Columns:  []string{"organization_id", "person_id", "snapshot"},

	Encode: func(s domain.RepresentationSnapshot) ([]any, error) {
		raw, err := json.Marshal(representationState{
			PowerType:           string(s.PowerType),
			SourceType:          string(s.SourceType),
			SourceEvidenceID:    s.SourceEvidenceID,
			ScopeCurrency:       s.ScopeCurrency,
			ScopeMaxAmountMinor: s.ScopeMaxAmountMinor,
			ScopeActTypes:       append(make([]string, 0, len(s.ScopeActTypes)), s.ScopeActTypes...),
			ValidFrom:           int64(s.ValidFrom),
			ValidUntil:          int64(s.ValidUntil),
			VerificationStatus:  int(s.VerificationStatus),
			ProposedByUserID:    s.ProposedByUserID,
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.OrganizationID), string(s.PersonID), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.RepresentationSnapshot, error) {
		var (
			organizationID string
			personID       string
			raw            []byte
		)
		if err := scan(&organizationID, &personID, &raw); err != nil {
			return domain.RepresentationSnapshot{}, err
		}
		var state representationState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.RepresentationSnapshot{}, err
		}
		return domain.RepresentationSnapshot{
			OrganizationID:      domain.OrganizationID(organizationID),
			PersonID:            domain.PersonID(personID),
			PowerType:           domain.PowerType(state.PowerType),
			SourceType:          domain.SourceType(state.SourceType),
			SourceEvidenceID:    state.SourceEvidenceID,
			ScopeCurrency:       state.ScopeCurrency,
			ScopeMaxAmountMinor: state.ScopeMaxAmountMinor,
			ScopeActTypes:       append(make([]string, 0, len(state.ScopeActTypes)), state.ScopeActTypes...),
			ValidFrom:           kernel.Instant(state.ValidFrom),
			ValidUntil:          kernel.Instant(state.ValidUntil),
			VerificationStatus:  domain.RepresentationStatus(state.VerificationStatus),
			ProposedByUserID:    state.ProposedByUserID,
		}, nil
	},

	WithID: func(s domain.RepresentationSnapshot, id domain.RepresentationID) domain.RepresentationSnapshot {
		s.ID = id
		return s
	},
}

func NewRepresentationRepository(tx *postgres.Tx) ports.RepresentationRepository {
	return representationTable.Repository(tx)
}
