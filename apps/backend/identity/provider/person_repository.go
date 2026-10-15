package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// personState holds no document field on purpose: the document is
// PII_SENSITIVE and is not stored in this version (spec note ‡‡).
type personState struct {
	FullName               string `json:"fullName"`
	DocumentHashKeyVersion int    `json:"documentHashKeyVersion"`
	DocumentMasked         string `json:"documentMasked"`
	CreatedAt              int64  `json:"createdAt"`
}

var personTable = postgres.Table[domain.PersonID, domain.PersonSnapshot]{
	Name:     "persons",
	IDColumn: "person_id",
	Columns:  []string{"document_hash", "snapshot"},

	Encode: func(s domain.PersonSnapshot) ([]any, error) {
		raw, err := json.Marshal(personState{
			FullName:               s.FullName,
			DocumentHashKeyVersion: s.DocumentHashKeyVersion,
			DocumentMasked:         s.DocumentMasked,
			CreatedAt:              int64(s.CreatedAt),
		})
		if err != nil {
			return nil, err
		}
		return []any{s.DocumentHash, raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.PersonSnapshot, error) {
		var (
			documentHash string
			raw          []byte
		)
		if err := scan(&documentHash, &raw); err != nil {
			return domain.PersonSnapshot{}, err
		}
		var state personState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.PersonSnapshot{}, err
		}
		return domain.PersonSnapshot{
			FullName:               state.FullName,
			DocumentHash:           documentHash,
			DocumentHashKeyVersion: state.DocumentHashKeyVersion,
			DocumentMasked:         state.DocumentMasked,
			CreatedAt:              kernel.Instant(state.CreatedAt),
		}, nil
	},

	WithID: func(s domain.PersonSnapshot, id domain.PersonID) domain.PersonSnapshot {
		s.ID = id
		return s
	},
}

func NewPersonRepository(tx *postgres.Tx) ports.PersonRepository {
	return personTable.Repository(tx)
}
