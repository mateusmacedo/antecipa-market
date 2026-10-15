package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) AddEstablishment(ctx context.Context, cmd AddEstablishment) (usecase.Outcome[domain.AddEstablishmentResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.AddEstablishmentResponse]{
		Operation:   OperationAddEstablishment,
		Object:      string(cmd.EstablishmentID),
		Input:       cmd,
		Fingerprint: addEstablishmentFingerprint(cmd),
		Codec:       addEstablishmentCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.AddEstablishmentResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.AddEstablishmentResponse], error) {
				return usecase.Outcome[domain.AddEstablishmentResponse]{}, fmt.Errorf("application: add establishment %s: %w", cmd.EstablishmentID, err)
			}
			protected, err := s.protectTaxID(ctx, cmd.TaxIDType, cmd.TaxID)
			if err != nil {
				return fail(err)
			}
			registered, err := s.establishmentTaxIDRegistered(ctx, protected.Hash, cmd.OrganizationID)
			if err != nil {
				return fail(err)
			}
			status, err := organizationStatus(ctx, res.Organizations, cmd.OrganizationID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Establishments, res.Outbox, origin(AggregateTypeEstablishment, string(cmd.EstablishmentID)), cmd.EstablishmentID, identity,
				loadAbsentEstablishment,
				func(e *domain.Establishment) (kernel.Accepted[domain.AddEstablishmentResponse], *kernel.Rejection) {
					return e.Add(domain.AddEstablishment{
						OrganizationID:         cmd.OrganizationID,
						OrganizationStatus:     status,
						TaxIDType:              cmd.TaxIDType,
						TaxID:                  cmd.TaxID,
						TaxIDHash:              protected.Hash,
						TaxIDHashKeyVersion:    protected.KeyVersion,
						TaxIDAlreadyRegistered: registered,
						TradeName:              cmd.TradeName,
						Receiver:               cmd.Receiver,
						ExternalReferences:     cmd.ExternalReferences,
						At:                     identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}

func addEstablishmentFingerprint(cmd AddEstablishment) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationAddEstablishment).
		String(string(cmd.EstablishmentID)).
		String(string(cmd.OrganizationID)).
		String(string(cmd.TaxIDType)).
		String(cmd.TaxID).
		String(cmd.TradeName).
		Int(flag(cmd.Receiver)).
		Int(int64(len(cmd.ExternalReferences)))
	for _, ref := range cmd.ExternalReferences {
		f.String(ref.ProviderID).
			String(string(ref.System)).
			String(string(ref.ReferenceType)).
			String(ref.Reference).
			Int(int64(ref.AsOf)).
			String(ref.ObservedVersion)
	}
	return f
}

func flag(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
