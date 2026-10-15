package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) SuspendUser(ctx context.Context, cmd SuspendUser) (usecase.Outcome[domain.SuspendUserResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.SuspendUserResponse]{
		Operation: OperationSuspendUser,
		Object:    string(cmd.UserID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationSuspendUser).
			String(string(cmd.UserID)).
			String(cmd.Reason),
		Codec: suspendUserCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.SuspendUserResponse], error) {
			outcome, err := usecase.Decide(ctx, res.UserAccounts, res.Outbox, origin(AggregateTypeUserAccount, string(cmd.UserID)), cmd.UserID, identity,
				loadExistingUserAccount,
				func(u *domain.UserAccount) (kernel.Accepted[domain.SuspendUserResponse], *kernel.Rejection) {
					return u.Suspend(domain.SuspendUser{Reason: cmd.Reason, At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: suspend user %s: %w", cmd.UserID, err)
			}
			return outcome, nil
		},
	})
}
