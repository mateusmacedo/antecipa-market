package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (m *Membership) Grant(cmd GrantMembership) (kernel.Accepted[GrantMembershipResponse], *kernel.Rejection) {
	return kernel.DecideOver(m, (*Membership).clone, func(next *Membership) (kernel.Accepted[GrantMembershipResponse], *kernel.Rejection) {
		switch {
		case next.status != MembershipStatusInvited:
			return kernel.Refuse[GrantMembershipResponse](CodeMembershipNotInvited, "membership must be invited")
		case next.invitationExpiresAt <= cmd.At:
			return kernel.Refuse[GrantMembershipResponse](CodeMembershipInvitationExpired, "the invitation has expired")
		case !cmd.ChannelConfirmed:
			return kernel.Refuse[GrantMembershipResponse](CodeMembershipChannelNotConfirmed, "the ownership of the channel must be confirmed")
		case !next.role.verifiedBy(cmd.VerificationLevel):
			return kernel.Refuse[GrantMembershipResponse](CodeMembershipVerificationInsufficient, "the verification level is below the minimum of the role",
				kernel.Detail{Key: "role", Value: string(next.role)},
			)
		case cmd.UserStatus != UserAccountActive:
			return kernel.Refuse[GrantMembershipResponse](CodeMembershipUserNotActive, "user account must exist and be active",
				kernel.Detail{Key: "user", Value: string(cmd.UserID)},
			)
		}
		next.status = MembershipStatusActive
		next.grantedAt = cmd.At
		next.userID = cmd.UserID
		next.personID = cmd.PersonID
		next.verificationLevel = cmd.VerificationLevel
		return kernel.Accept(
			GrantMembershipResponse{MembershipID: next.id},
			MembershipGranted{
				MembershipID:      next.id,
				OrganizationID:    next.organizationID,
				UserID:            cmd.UserID,
				Role:              next.role,
				VerificationLevel: cmd.VerificationLevel,
				At:                cmd.At,
			},
		), nil
	})
}
