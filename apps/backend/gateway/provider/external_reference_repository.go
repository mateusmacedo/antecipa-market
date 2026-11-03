package provider

import (
	"context"
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

type referenceState struct {
	ProviderID          string `json:"providerId"`
	System              string `json:"system"`
	ReferenceType       string `json:"referenceType"`
	Reference           string `json:"reference"`
	NormalizedReference string `json:"normalizedReference"`
	AsOf                int64  `json:"asOf"`
	ObservedVersion     string `json:"observedVersion"`
}

// referenceTable keys the link by the reference key, so the primary key is what
// keeps one reference on one internal object.
var referenceTable = postgres.Table[domain.ReferenceKey, domain.ExternalReferenceSnapshot]{
	Name:     "external_references",
	IDColumn: "external_reference_id",
	Columns:  []string{"subject_type", "subject_id", "snapshot"},

	Encode: func(s domain.ExternalReferenceSnapshot) ([]any, error) {
		raw, err := json.Marshal(referenceState{
			ProviderID: string(s.ProviderID), System: string(s.System), ReferenceType: string(s.ReferenceType), Reference: s.Reference,
			NormalizedReference: s.NormalizedReference, AsOf: int64(s.AsOf), ObservedVersion: s.ObservedVersion,
		})
		if err != nil {
			return nil, err
		}
		return []any{s.SubjectType, string(s.SubjectID), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.ExternalReferenceSnapshot, error) {
		var (
			subjectType, subjectID string
			raw                    []byte
		)
		if err := scan(&subjectType, &subjectID, &raw); err != nil {
			return domain.ExternalReferenceSnapshot{}, err
		}
		var state referenceState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.ExternalReferenceSnapshot{}, err
		}
		return domain.ExternalReferenceSnapshot{
			ProviderID: domain.PartnerID(state.ProviderID), System: domain.ExternalSystem(state.System),
			ReferenceType: domain.ReferenceType(state.ReferenceType), Reference: state.Reference, NormalizedReference: state.NormalizedReference,
			SubjectType: subjectType, SubjectID: domain.SubjectID(subjectID), AsOf: kernel.Instant(state.AsOf), ObservedVersion: state.ObservedVersion,
		}, nil
	},

	WithID: func(s domain.ExternalReferenceSnapshot, key domain.ReferenceKey) domain.ExternalReferenceSnapshot {
		s.Key = key
		return s
	},
}

func NewExternalReferenceRepository(tx *postgres.Tx) ports.ExternalReferenceRepository {
	return referenceTable.Repository(tx)
}

func NewExternalReferenceReader(pool postgres.ReadPool) ports.ExternalReferenceReader {
	return referenceTable.Reader(pool)
}

var referencesBySubject = referenceTable.Relation("subject_id")

// ExternalReferencesReader serves FindExternalReferencesBySubject outside the
// unit of work.
type ExternalReferencesReader struct{ pool postgres.ReadPool }

func NewExternalReferencesReader(pool postgres.ReadPool) *ExternalReferencesReader {
	return &ExternalReferencesReader{pool: pool}
}

func (r *ExternalReferencesReader) LoadBySubject(ctx context.Context, subject domain.SubjectID) ([]domain.ExternalReferenceSnapshot, error) {
	return referencesBySubject.Query(ctx, r.pool, string(subject))
}

var _ ports.ExternalReferencesBySubjectReader = (*ExternalReferencesReader)(nil)
