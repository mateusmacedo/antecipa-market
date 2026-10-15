package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) RevokeMembership(ctx context.Context, cmd RevokeMembership) (usecase.Outcome[domain.RevokeMembershipResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RevokeMembershipResponse]{
		Operation: OperationRevokeMembership,
		Object:    string(cmd.MembershipID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationRevokeMembership).
			String(string(cmd.MembershipID)).
			String(cmd.Reason),
		Codec: revokeMembershipCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RevokeMembershipResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Memberships, res.Outbox, origin(AggregateTypeMembership, string(cmd.MembershipID)), cmd.MembershipID, identity,
				loadExistingMembership,
				func(m *domain.Membership) (kernel.Accepted[domain.RevokeMembershipResponse], *kernel.Rejection) {
					return m.Revoke(domain.RevokeMembership{Reason: cmd.Reason, At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: revoke membership %s: %w", cmd.MembershipID, err)
			}
			return outcome, nil
		},
	})
}
