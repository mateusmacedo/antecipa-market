package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) RetireProduct(ctx context.Context, cmd RetireProduct) (usecase.Outcome[domain.RetireProductResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RetireProductResponse]{
		Operation:   OperationRetireProduct,
		Object:      string(cmd.ProductID),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationRetireProduct).String(string(cmd.ProductID)).String(string(cmd.Reason)),
		Codec:       retireProductCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RetireProductResponse], error) {
			outcome, err := usecase.Decide(ctx, res.FinancialProducts, res.Outbox, origin(AggregateTypeFinancialProduct, string(cmd.ProductID)), cmd.ProductID, identity,
				loadExistingProduct,
				func(p *domain.FinancialProduct) (kernel.Accepted[domain.RetireProductResponse], *kernel.Rejection) {
					return p.Retire(domain.RetireProduct{Reason: cmd.Reason, At: identity.OccurredAt})
				})
			if err != nil {
				return usecase.Outcome[domain.RetireProductResponse]{}, fmt.Errorf("application: retire product %s: %w", cmd.ProductID, err)
			}
			return outcome, nil
		},
	})
}
