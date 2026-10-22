package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) RegisterOrganization(ctx context.Context, cmd RegisterOrganization) (usecase.Outcome[domain.RegisterOrganizationResponse], error) {
	kinds := domain.KindsOf(cmd.Kinds...)
	return usecase.Execute(ctx, s.executor(), command[domain.RegisterOrganizationResponse]{
		Operation:   OperationRegisterOrganization,
		Object:      string(cmd.OrganizationID),
		Input:       cmd,
		Fingerprint: registerFingerprint(cmd, kinds),
		Codec:       registerOrganizationCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RegisterOrganizationResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.RegisterOrganizationResponse], error) {
				return usecase.Outcome[domain.RegisterOrganizationResponse]{}, fmt.Errorf("application: register organization %s: %w", cmd.OrganizationID, err)
			}
			protected, err := s.protectTaxID(ctx, cmd.TaxIDType, cmd.TaxID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Organizations, res.Outbox, origin(AggregateTypeOrganization, string(cmd.OrganizationID)), cmd.OrganizationID, identity,
				loadAbsentOrganization,
				func(o *domain.Organization) (kernel.Accepted[domain.RegisterOrganizationResponse], *kernel.Rejection) {
					return o.Register(domain.RegisterOrganization{
						Kinds:               kinds,
						Jurisdiction:        cmd.Jurisdiction,
						TaxIDType:           cmd.TaxIDType,
						TaxID:               cmd.TaxID,
						TaxIDHash:           protected.Hash,
						TaxIDHashKeyVersion: protected.KeyVersion,
						LegalName:           cmd.LegalName,
						TradeName:           cmd.TradeName,
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

// registerFingerprint takes the kinds as the set they decide, so the same kinds
// in another order are the same command under one idempotency key.
func registerFingerprint(cmd RegisterOrganization, kinds domain.OrganizationKinds) *usecase.Fingerprint {
	listed := kinds.List()
	f := usecase.NewFingerprint(OperationRegisterOrganization).
		String(string(cmd.OrganizationID)).
		Int(int64(len(listed)))
	for _, kind := range listed {
		f.String(string(kind))
	}
	return f.String(cmd.Jurisdiction).
		String(string(cmd.TaxIDType)).
		String(cmd.TaxID).
		String(cmd.LegalName).
		String(cmd.TradeName)
}
