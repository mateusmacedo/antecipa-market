package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) RequestBankAccountChange(ctx context.Context, cmd RequestBankAccountChange) (usecase.Outcome[domain.RequestBankAccountChangeResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RequestBankAccountChangeResponse]{
		Operation: OperationRequestBankAccountChange,
		Object:    string(cmd.BankAccountChangeID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationRequestBankAccountChange).
			String(string(cmd.BankAccountChangeID)).
			String(string(cmd.OrganizationID)).
			String(cmd.DestinationEvidenceID).
			String(cmd.RequestedByUserID),
		Codec: requestBankAccountChangeCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RequestBankAccountChangeResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.RequestBankAccountChangeResponse], error) {
				return usecase.Outcome[domain.RequestBankAccountChangeResponse]{}, fmt.Errorf("application: request bank account change %s: %w", cmd.BankAccountChangeID, err)
			}
			status, err := organizationStatus(ctx, res.Organizations, cmd.OrganizationID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.BankAccountChanges, res.Outbox, origin(AggregateTypeBankAccountChange, string(cmd.BankAccountChangeID)), cmd.BankAccountChangeID, identity,
				loadAbsentBankAccountChange,
				func(c *domain.BankAccountChange) (kernel.Accepted[domain.RequestBankAccountChangeResponse], *kernel.Rejection) {
					return c.Request(domain.RequestBankAccountChange{
						OrganizationID:        cmd.OrganizationID,
						OrganizationStatus:    status,
						DestinationEvidenceID: cmd.DestinationEvidenceID,
						RequestedByUserID:     cmd.RequestedByUserID,
						At:                    identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}
