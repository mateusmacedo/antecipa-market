package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (m *Membership) Revoke(cmd RevokeMembership) (kernel.Accepted[RevokeMembershipResponse], *kernel.Rejection) {
	return kernel.DecideOver(m, (*Membership).clone, func(next *Membership) (kernel.Accepted[RevokeMembershipResponse], *kernel.Rejection) {
		if next.status != MembershipStatusActive {
			return kernel.Refuse[RevokeMembershipResponse](CodeMembershipNotActive, "membership must be active")
		}
		next.status = MembershipStatusRevoked
		next.revokedAt = cmd.At
		next.revocationReason = cmd.Reason
		return kernel.Accept(
			RevokeMembershipResponse{MembershipID: next.id},
			MembershipRevoked{
				MembershipID:   next.id,
				OrganizationID: next.organizationID,
				UserID:         next.userID,
				Role:           next.role,
				Reason:         cmd.Reason,
				At:             cmd.At,
			},
		), nil
	})
}
