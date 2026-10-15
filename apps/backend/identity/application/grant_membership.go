package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) GrantMembership(ctx context.Context, cmd GrantMembership) (usecase.Outcome[domain.GrantMembershipResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.GrantMembershipResponse]{
		Operation: OperationGrantMembership,
		Object:    string(cmd.MembershipID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationGrantMembership).
			String(string(cmd.MembershipID)).
			String(string(cmd.UserID)).
			String(string(cmd.PersonID)).
			Int(flag(cmd.ChannelConfirmed)).
			String(string(cmd.VerificationLevel)),
		Codec: grantMembershipCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.GrantMembershipResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.GrantMembershipResponse], error) {
				return usecase.Outcome[domain.GrantMembershipResponse]{}, fmt.Errorf("application: grant membership %s: %w", cmd.MembershipID, err)
			}
			status, err := userAccountStatus(ctx, res.UserAccounts, cmd.UserID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Memberships, res.Outbox, origin(AggregateTypeMembership, string(cmd.MembershipID)), cmd.MembershipID, identity,
				loadExistingMembership,
				func(m *domain.Membership) (kernel.Accepted[domain.GrantMembershipResponse], *kernel.Rejection) {
					return m.Grant(domain.GrantMembership{
						UserID:            cmd.UserID,
						UserStatus:        status,
						PersonID:          cmd.PersonID,
						ChannelConfirmed:  cmd.ChannelConfirmed,
						VerificationLevel: cmd.VerificationLevel,
						At:                identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}
