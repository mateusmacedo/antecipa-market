package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) RegisterPartner(ctx context.Context, cmd RegisterPartner) (usecase.Outcome[domain.RegisterPartnerResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RegisterPartnerResponse]{
		Operation:   OperationRegisterPartner,
		Object:      string(cmd.PartnerID),
		Input:       cmd,
		Fingerprint: registerPartnerFingerprint(cmd),
		Codec:       registerPartnerCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RegisterPartnerResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.RegisterPartnerResponse], error) {
				return usecase.Outcome[domain.RegisterPartnerResponse]{}, fmt.Errorf("application: register partner %s: %w", cmd.PartnerID, err)
			}
			kinds, found, err := s.organizationKinds(ctx, cmd.OrganizationID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Partners, res.Outbox, origin(AggregateTypePartner, string(cmd.PartnerID)), cmd.PartnerID, identity,
				loadAbsentPartner,
				func(p *domain.Partner) (kernel.Accepted[domain.RegisterPartnerResponse], *kernel.Rejection) {
					return p.Register(domain.RegisterPartner{
						OrganizationID:    cmd.OrganizationID,
						Role:              cmd.Role,
						OrganizationFound: found,
						OrganizationKinds: kinds,
						At:                identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}

func registerPartnerFingerprint(cmd RegisterPartner) *usecase.Fingerprint {
	return usecase.NewFingerprint(OperationRegisterPartner).
		String(string(cmd.PartnerID)).
		String(string(cmd.OrganizationID)).
		String(string(cmd.Role))
}
