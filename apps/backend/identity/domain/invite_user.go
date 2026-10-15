package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (m *Membership) Invite(cmd InviteUser) (kernel.Accepted[InviteUserResponse], *kernel.Rejection) {
	return kernel.DecideOver(m, (*Membership).clone, func(next *Membership) (kernel.Accepted[InviteUserResponse], *kernel.Rejection) {
		organization := kernel.Detail{Key: "organization", Value: string(cmd.OrganizationID)}
		switch {
		case strings.TrimSpace(cmd.InviteeEmail) == "":
			return kernel.Refuse[InviteUserResponse](CodeMembershipInviteeEmailEmpty, "invitee email must not be empty")
		case cmd.InvitationExpiresAt <= cmd.At:
			return kernel.Refuse[InviteUserResponse](CodeMembershipInvitationExpiryInvalid, "the invitation must expire after it is made")
		case cmd.OrganizationStatus != OrganizationActive:
			return kernel.Refuse[InviteUserResponse](CodeMembershipOrganizationNotActive, "organization must exist and be active", organization)
		case !cmd.Role.admits(cmd.OrganizationKinds):
			return kernel.Refuse[InviteUserResponse](CodeMembershipRoleKindMismatch, "the role does not fit the kinds of the organization",
				kernel.Detail{Key: "role", Value: string(cmd.Role)}, organization,
			)
		}
		next.organizationID = cmd.OrganizationID
		next.inviteeEmail = cmd.InviteeEmail
		next.role = cmd.Role
		next.status = MembershipStatusInvited
		next.invitedByUserID = cmd.InvitedByUserID
		next.invitedAt = cmd.At
		next.invitationExpiresAt = cmd.InvitationExpiresAt
		return kernel.Accept(InviteUserResponse{MembershipID: next.id}), nil
	})
}
