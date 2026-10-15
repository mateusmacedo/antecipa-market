package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) InviteUser(ctx context.Context, cmd InviteUser) (usecase.Outcome[domain.InviteUserResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.InviteUserResponse]{
		Operation: OperationInviteUser,
		Object:    string(cmd.MembershipID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationInviteUser).
			String(string(cmd.MembershipID)).
			String(string(cmd.OrganizationID)).
			String(cmd.InviteeEmail).
			String(string(cmd.Role)).
			String(cmd.InvitedByUserID).
			Int(int64(cmd.InvitationExpiresAt)),
		Codec: inviteUserCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.InviteUserResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.InviteUserResponse], error) {
				return usecase.Outcome[domain.InviteUserResponse]{}, fmt.Errorf("application: invite user %s: %w", cmd.MembershipID, err)
			}
			organization, err := organizationOf(ctx, res.Organizations, cmd.OrganizationID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Memberships, res.Outbox, origin(AggregateTypeMembership, string(cmd.MembershipID)), cmd.MembershipID, identity,
				loadAbsentMembership,
				func(m *domain.Membership) (kernel.Accepted[domain.InviteUserResponse], *kernel.Rejection) {
					return m.Invite(domain.InviteUser{
						OrganizationID:      cmd.OrganizationID,
						OrganizationStatus:  organization.Status,
						OrganizationKinds:   organization.Kinds,
						InviteeEmail:        cmd.InviteeEmail,
						Role:                cmd.Role,
						InvitedByUserID:     cmd.InvitedByUserID,
						InvitationExpiresAt: cmd.InvitationExpiresAt,
						At:                  identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}
