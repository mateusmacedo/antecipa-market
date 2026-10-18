package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) ChangePartnerStage(ctx context.Context, cmd ChangePartnerStage) (usecase.Outcome[domain.ChangePartnerStageResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ChangePartnerStageResponse]{
		Operation: OperationChangePartnerStage,
		Object:    string(cmd.PartnerID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationChangePartnerStage).
			String(string(cmd.PartnerID)).
			String(string(cmd.Target)).
			String(cmd.Reason),
		Codec: changePartnerStageCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ChangePartnerStageResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.ChangePartnerStageResponse], error) {
				return usecase.Outcome[domain.ChangePartnerStageResponse]{}, fmt.Errorf("application: change partner stage %s: %w", cmd.PartnerID, err)
			}
			facts, err := s.activationFacts(ctx, res, cmd, identity.OccurredAt)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Partners, res.Outbox, origin(AggregateTypePartner, string(cmd.PartnerID)), cmd.PartnerID, identity,
				loadExistingPartner,
				func(p *domain.Partner) (kernel.Accepted[domain.ChangePartnerStageResponse], *kernel.Rejection) {
					return p.ChangeStage(domain.ChangePartnerStage{
						Target:                cmd.Target,
						Reason:                cmd.Reason,
						HasEffectiveAgreement: facts.effectiveAgreement,
						HasEnabledCapability:  facts.enabledCapability,
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

type activationFacts struct {
	effectiveAgreement bool
	enabledCapability  bool
}

func (s Service) activationFacts(ctx context.Context, res Resources, cmd ChangePartnerStage, at kernel.Instant) (activationFacts, error) {
	var facts activationFacts
	if cmd.Target != domain.PartnerActive {
		return facts, nil
	}
	partner, found, err := loaded(ctx, res.Partners, cmd.PartnerID)
	if err != nil || !found {
		return facts, err
	}
	if facts.effectiveAgreement, err = s.agreementInEffect(ctx, cmd.PartnerID, at); err != nil {
		return facts, err
	}
	if partner.Status == domain.PartnerHomologating {
		facts.enabledCapability, err = s.capabilityEnabled(ctx, cmd.PartnerID)
	}
	return facts, err
}
