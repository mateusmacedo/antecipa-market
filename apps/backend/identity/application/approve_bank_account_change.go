package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) ApproveBankAccountChange(ctx context.Context, cmd ApproveBankAccountChange) (usecase.Outcome[domain.ApproveBankAccountChangeResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ApproveBankAccountChangeResponse]{
		Operation: OperationApproveBankAccountChange,
		Object:    string(cmd.BankAccountChangeID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationApproveBankAccountChange).
			String(string(cmd.BankAccountChangeID)).
			String(cmd.ApprovedByUserID),
		Codec: approveBankAccountChangeCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ApproveBankAccountChangeResponse], error) {
			outcome, err := usecase.Decide(ctx, res.BankAccountChanges, res.Outbox, origin(AggregateTypeBankAccountChange, string(cmd.BankAccountChangeID)), cmd.BankAccountChangeID, identity,
				loadExistingBankAccountChange,
				func(c *domain.BankAccountChange) (kernel.Accepted[domain.ApproveBankAccountChangeResponse], *kernel.Rejection) {
					return c.Approve(domain.ApproveBankAccountChange{ApprovedByUserID: cmd.ApprovedByUserID, At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: approve bank account change %s: %w", cmd.BankAccountChangeID, err)
			}
			return outcome, nil
		},
	})
}
