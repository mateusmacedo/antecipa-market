package provider

import (
	"encoding/json"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

type membershipState struct {
	InviteeEmail        string `json:"inviteeEmail"`
	Role                string `json:"role"`
	Status              int    `json:"status"`
	InvitedByUserID     string `json:"invitedByUserId"`
	InvitedAt           int64  `json:"invitedAt"`
	InvitationExpiresAt int64  `json:"invitationExpiresAt"`
	PersonID            string `json:"personId"`
	VerificationLevel   string `json:"verificationLevel"`
	GrantedAt           int64  `json:"grantedAt"`
	RevokedAt           int64  `json:"revokedAt"`
	RevocationReason    string `json:"revocationReason"`
}

var membershipTable = postgres.Table[domain.MembershipID, domain.MembershipSnapshot]{
	Name:     "memberships",
	IDColumn: "membership_id",
	Columns:  []string{"organization_id", "user_id", "snapshot"},

	Encode: func(s domain.MembershipSnapshot) ([]any, error) {
		raw, err := json.Marshal(membershipState{
			InviteeEmail:        s.InviteeEmail,
			Role:                string(s.Role),
			Status:              int(s.Status),
			InvitedByUserID:     s.InvitedByUserID,
			InvitedAt:           int64(s.InvitedAt),
			InvitationExpiresAt: int64(s.InvitationExpiresAt),
			PersonID:            string(s.PersonID),
			VerificationLevel:   string(s.VerificationLevel),
			GrantedAt:           int64(s.GrantedAt),
			RevokedAt:           int64(s.RevokedAt),
			RevocationReason:    s.RevocationReason,
		})
		if err != nil {
			return nil, err
		}
		var userID *string
		if s.UserID != "" {
			granted := string(s.UserID)
			userID = &granted
		}
		return []any{string(s.OrganizationID), userID, raw}, nil
	},

	Decode: func(scan func(dest ...any) error) (domain.MembershipSnapshot, error) {
		var (
			organizationID string
			userID         *string
			raw            []byte
		)
		if err := scan(&organizationID, &userID, &raw); err != nil {
			return domain.MembershipSnapshot{}, err
		}
		var state membershipState
		if err := json.Unmarshal(raw, &state); err != nil {
			return domain.MembershipSnapshot{}, err
		}
		s := domain.MembershipSnapshot{
			OrganizationID:      domain.OrganizationID(organizationID),
			InviteeEmail:        state.InviteeEmail,
			Role:                domain.Role(state.Role),
			Status:              domain.MembershipStatus(state.Status),
			InvitedByUserID:     state.InvitedByUserID,
			InvitedAt:           kernel.Instant(state.InvitedAt),
			InvitationExpiresAt: kernel.Instant(state.InvitationExpiresAt),
			PersonID:            domain.PersonID(state.PersonID),
			VerificationLevel:   domain.VerificationLevel(state.VerificationLevel),
			GrantedAt:           kernel.Instant(state.GrantedAt),
			RevokedAt:           kernel.Instant(state.RevokedAt),
			RevocationReason:    state.RevocationReason,
		}
		if userID != nil {
			s.UserID = domain.UserID(*userID)
		}
		return s, nil
	},

	WithID: func(s domain.MembershipSnapshot, id domain.MembershipID) domain.MembershipSnapshot {
		s.ID = id
		return s
	},
}

func NewMembershipRepository(tx *postgres.Tx) ports.MembershipRepository {
	return membershipTable.Repository(tx)
}
