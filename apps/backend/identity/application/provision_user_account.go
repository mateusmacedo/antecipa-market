package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) ProvisionUserAccount(ctx context.Context, cmd ProvisionUserAccount) (usecase.Outcome[domain.ProvisionUserAccountResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ProvisionUserAccountResponse]{
		Operation: OperationProvisionUserAccount,
		Object:    string(cmd.Subject),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationProvisionUserAccount).
			String(string(cmd.Subject)).
			String(string(cmd.PersonID)),
		Codec: provisionUserAccountCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ProvisionUserAccountResponse], error) {
			outcome, err := usecase.Decide(ctx, res.UserAccounts, res.Outbox, origin(AggregateTypeUserAccount, string(cmd.Subject)), cmd.Subject, identity,
				loadUserAccount,
				func(u *domain.UserAccount) (kernel.Accepted[domain.ProvisionUserAccountResponse], *kernel.Rejection) {
					return u.Provision(domain.ProvisionUserAccount{PersonID: cmd.PersonID, At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: provision user account %s: %w", cmd.Subject, err)
			}
			return outcome, nil
		},
	})
}
