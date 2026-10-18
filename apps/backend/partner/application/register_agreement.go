package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) RegisterAgreement(ctx context.Context, cmd RegisterAgreement) (usecase.Outcome[domain.RegisterAgreementResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RegisterAgreementResponse]{
		Operation:   OperationRegisterAgreement,
		Object:      string(cmd.AgreementID),
		Input:       cmd,
		Fingerprint: registerAgreementFingerprint(cmd),
		Codec:       registerAgreementCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RegisterAgreementResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.RegisterAgreementResponse], error) {
				return usecase.Outcome[domain.RegisterAgreementResponse]{}, fmt.Errorf("application: register agreement %s: %w", cmd.AgreementID, err)
			}
			head, err := s.agreementHead(ctx, cmd.PartnerID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Agreements, res.Outbox, origin(AggregateTypePartnerAgreement, string(cmd.AgreementID)), cmd.AgreementID, identity,
				loadAbsentAgreement,
				func(a *domain.PartnerAgreement) (kernel.Accepted[domain.RegisterAgreementResponse], *kernel.Rejection) {
					return a.Register(domain.RegisterAgreement{
						PartnerID:            cmd.PartnerID,
						ValidFrom:            cmd.ValidFrom,
						ValidUntil:           cmd.ValidUntil,
						InstrumentTemplateID: cmd.InstrumentTemplateID,
						SupersedesID:         cmd.SupersedesID,
						Head:                 head,
						At:                   identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}

func registerAgreementFingerprint(cmd RegisterAgreement) *usecase.Fingerprint {
	return usecase.NewFingerprint(OperationRegisterAgreement).
		String(string(cmd.AgreementID)).
		String(string(cmd.PartnerID)).
		Int(int64(cmd.ValidFrom)).
		Int(int64(cmd.ValidUntil)).
		String(cmd.InstrumentTemplateID).
		String(string(cmd.SupersedesID))
}
