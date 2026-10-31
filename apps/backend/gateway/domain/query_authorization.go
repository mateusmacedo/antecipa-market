package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type AuthorizationStatus string

const (
	AuthorizationGranted AuthorizationStatus = "granted"
	AuthorizationRevoked AuthorizationStatus = "revoked"
)

// QueryAuthorization is the local projection of a consent to query; its
// absence is a refusal, which keeps ReceivablesQueryPort fail-closed.
type QueryAuthorization struct {
	consentID       ConsentID
	organizationID  OrganizationID
	consentVersion  int
	status          AuthorizationStatus
	assetTypes      []string
	periodFromDate  string
	periodUntilDate string
	grantedAt       kernel.Instant
	revokedAt       kernel.Instant
}

func NewQueryAuthorization(id ConsentID) *QueryAuthorization {
	return &QueryAuthorization{consentID: id}
}

func FromQueryAuthorizationSnapshot(s QueryAuthorizationSnapshot) *QueryAuthorization {
	return &QueryAuthorization{
		consentID:       s.ConsentID,
		organizationID:  s.OrganizationID,
		consentVersion:  s.ConsentVersion,
		status:          s.Status,
		assetTypes:      slices.Clone(s.AssetTypes),
		periodFromDate:  s.PeriodFromDate,
		periodUntilDate: s.PeriodUntilDate,
		grantedAt:       s.GrantedAt,
		revokedAt:       s.RevokedAt,
	}
}

type QueryAuthorizationSnapshot struct {
	ConsentID       ConsentID
	OrganizationID  OrganizationID
	ConsentVersion  int
	Status          AuthorizationStatus
	AssetTypes      []string
	PeriodFromDate  string
	PeriodUntilDate string
	GrantedAt       kernel.Instant
	RevokedAt       kernel.Instant
}

func (a *QueryAuthorization) Snapshot() QueryAuthorizationSnapshot {
	return QueryAuthorizationSnapshot{
		ConsentID:       a.consentID,
		OrganizationID:  a.organizationID,
		ConsentVersion:  a.consentVersion,
		Status:          a.status,
		AssetTypes:      slices.Clone(a.assetTypes),
		PeriodFromDate:  a.periodFromDate,
		PeriodUntilDate: a.periodUntilDate,
		GrantedAt:       a.grantedAt,
		RevokedAt:       a.revokedAt,
	}
}

func (s QueryAuthorizationSnapshot) Equal(other QueryAuthorizationSnapshot) bool {
	return s.ConsentID == other.ConsentID &&
		s.OrganizationID == other.OrganizationID &&
		s.ConsentVersion == other.ConsentVersion &&
		s.Status == other.Status &&
		slices.Equal(s.AssetTypes, other.AssetTypes) &&
		s.PeriodFromDate == other.PeriodFromDate &&
		s.PeriodUntilDate == other.PeriodUntilDate &&
		s.GrantedAt == other.GrantedAt &&
		s.RevokedAt == other.RevokedAt
}

func (a *QueryAuthorization) clone() QueryAuthorization {
	c := *a
	c.assetTypes = slices.Clone(a.assetTypes)
	return c
}

func (a *QueryAuthorization) Grant(cmd GrantQueryAuthorization) (kernel.Accepted[AuthorizationResponse], *kernel.Rejection) {
	return kernel.DecideOver(a, (*QueryAuthorization).clone, func(next *QueryAuthorization) (kernel.Accepted[AuthorizationResponse], *kernel.Rejection) {
		response := AuthorizationResponse{ConsentID: next.consentID}
		switch {
		case next.status == "":
			next.organizationID = cmd.OrganizationID
			next.status = AuthorizationGranted
			next.grantedAt = cmd.At
			next.renew(cmd)
		case next.status == AuthorizationGranted && cmd.ConsentVersion > next.consentVersion:
			next.renew(cmd)
		}
		return kernel.Accept(response), nil
	})
}

func (a *QueryAuthorization) renew(cmd GrantQueryAuthorization) {
	a.consentVersion = cmd.ConsentVersion
	a.assetTypes = slices.Clone(cmd.AssetTypes)
	a.periodFromDate = cmd.PeriodFromDate
	a.periodUntilDate = cmd.PeriodUntilDate
}

func (a *QueryAuthorization) Revoke(cmd RevokeQueryAuthorization) (kernel.Accepted[AuthorizationResponse], *kernel.Rejection) {
	return kernel.DecideOver(a, (*QueryAuthorization).clone, func(next *QueryAuthorization) (kernel.Accepted[AuthorizationResponse], *kernel.Rejection) {
		if next.status != AuthorizationRevoked {
			next.status = AuthorizationRevoked
			next.revokedAt = cmd.EffectiveAt
		}
		return kernel.Accept(AuthorizationResponse{ConsentID: next.consentID}), nil
	})
}
