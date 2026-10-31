package provider

import (
	"context"
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

type authorizationState struct {
	ConsentVersion  int      `json:"consentVersion"`
	Status          string   `json:"status"`
	AssetTypes      []string `json:"assetTypes"`
	PeriodFromDate  string   `json:"periodFromDate"`
	PeriodUntilDate string   `json:"periodUntilDate"`
	GrantedAt       int64    `json:"grantedAt"`
	RevokedAt       int64    `json:"revokedAt"`
}

var authorizationTable = postgres.Table[domain.ConsentID, domain.QueryAuthorizationSnapshot]{
	Name:     "query_authorizations",
	IDColumn: "query_authorization_id",
	Columns:  []string{"organization_id", "snapshot"},

	Encode: func(s domain.QueryAuthorizationSnapshot) ([]any, error) {
		raw, err := json.Marshal(authorizationState{
			ConsentVersion: s.ConsentVersion, Status: string(s.Status), AssetTypes: append(make([]string, 0, len(s.AssetTypes)), s.AssetTypes...),
			PeriodFromDate: s.PeriodFromDate, PeriodUntilDate: s.PeriodUntilDate, GrantedAt: int64(s.GrantedAt), RevokedAt: int64(s.RevokedAt),
		})
		if err != nil {
			return nil, err
		}
		return []any{string(s.OrganizationID), raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.QueryAuthorizationSnapshot, error) {
		var (
			organization string
			raw          []byte
		)
		if err := scan(&organization, &raw); err != nil {
			return domain.QueryAuthorizationSnapshot{}, err
		}
		var state authorizationState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.QueryAuthorizationSnapshot{}, err
		}
		return domain.QueryAuthorizationSnapshot{
			OrganizationID: domain.OrganizationID(organization), ConsentVersion: state.ConsentVersion,
			Status: domain.AuthorizationStatus(state.Status), AssetTypes: state.AssetTypes,
			PeriodFromDate: state.PeriodFromDate, PeriodUntilDate: state.PeriodUntilDate,
			GrantedAt: kernel.Instant(state.GrantedAt), RevokedAt: kernel.Instant(state.RevokedAt),
		}, nil
	},

	WithID: func(s domain.QueryAuthorizationSnapshot, id domain.ConsentID) domain.QueryAuthorizationSnapshot {
		s.ConsentID = id
		return s
	},
}

func NewQueryAuthorizationRepository(tx *postgres.Tx) ports.QueryAuthorizationRepository {
	return authorizationTable.Repository(tx)
}

var authorizationsByOrganization = authorizationTable.Relation("organization_id")

type QueryAuthorizationsByOrganizationReader struct{ pool postgres.ReadPool }

func NewQueryAuthorizationsByOrganizationReader(pool postgres.ReadPool) *QueryAuthorizationsByOrganizationReader {
	return &QueryAuthorizationsByOrganizationReader{pool: pool}
}

func (r *QueryAuthorizationsByOrganizationReader) LoadByOrganization(ctx context.Context, organization domain.OrganizationID) ([]domain.QueryAuthorizationSnapshot, error) {
	return authorizationsByOrganization.Query(ctx, r.pool, string(organization))
}

var _ ports.QueryAuthorizationsByOrganizationReader = (*QueryAuthorizationsByOrganizationReader)(nil)
