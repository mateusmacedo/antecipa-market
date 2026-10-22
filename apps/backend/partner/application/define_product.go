package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) DefineProduct(ctx context.Context, cmd DefineProduct) (usecase.Outcome[domain.DefineProductResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.DefineProductResponse]{
		Operation:   OperationDefineProduct,
		Object:      string(cmd.ProductID),
		Input:       cmd,
		Fingerprint: defineProductFingerprint(cmd),
		Codec:       defineProductCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.DefineProductResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.DefineProductResponse], error) {
				return usecase.Outcome[domain.DefineProductResponse]{}, fmt.Errorf("application: define product %s: %w", cmd.ProductID, err)
			}
			head, err := s.productHead(ctx, cmd.PartnerID, cmd.Code)
			if err != nil {
				return fail(err)
			}
			sustained, err := agreementSustains(ctx, res, cmd.AgreementID, cmd.PartnerID, identity.OccurredAt)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.FinancialProducts, res.Outbox, origin(AggregateTypeFinancialProduct, string(cmd.ProductID)), cmd.ProductID, identity,
				loadAbsentProduct,
				func(p *domain.FinancialProduct) (kernel.Accepted[domain.DefineProductResponse], *kernel.Rejection) {
					return p.Define(domain.DefineProduct{
						PartnerID:             cmd.PartnerID,
						AgreementID:           cmd.AgreementID,
						Code:                  cmd.Code,
						Name:                  cmd.Name,
						AssetTypes:            cmd.AssetTypes,
						Regions:               cmd.Regions,
						Currency:              cmd.Currency,
						MinAmountMinor:        cmd.MinAmountMinor,
						MaxAmountMinor:        cmd.MaxAmountMinor,
						MinTermDays:           cmd.MinTermDays,
						MaxTermDays:           cmd.MaxTermDays,
						RequiredCheckIDs:      cmd.RequiredCheckIDs,
						ValidFrom:             cmd.ValidFrom,
						ValidUntil:            cmd.ValidUntil,
						SupersedesID:          cmd.SupersedesID,
						Head:                  head,
						HasEffectiveAgreement: sustained,
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

func defineProductFingerprint(cmd DefineProduct) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationDefineProduct).
		String(string(cmd.ProductID)).
		String(string(cmd.PartnerID)).
		String(string(cmd.AgreementID)).
		String(cmd.Code).
		String(cmd.Name)
	fingerprintList(f, cmd.AssetTypes)
	fingerprintList(f, cmd.Regions)
	f.String(cmd.Currency).
		Int(cmd.MinAmountMinor).
		Int(cmd.MaxAmountMinor).
		Int(int64(cmd.MinTermDays)).
		Int(int64(cmd.MaxTermDays))
	fingerprintList(f, cmd.RequiredCheckIDs)
	return f.
		Int(int64(cmd.ValidFrom)).
		Int(int64(cmd.ValidUntil)).
		String(string(cmd.SupersedesID))
}
