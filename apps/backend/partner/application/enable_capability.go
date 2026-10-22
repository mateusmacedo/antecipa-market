package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) EnableCapability(ctx context.Context, cmd EnableCapability) (usecase.Outcome[domain.EnableCapabilityResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.EnableCapabilityResponse]{
		Operation:   OperationEnableCapability,
		Object:      string(cmd.CapabilityID),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationEnableCapability).String(string(cmd.CapabilityID)),
		Codec:       enableCapabilityCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.EnableCapabilityResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.EnableCapabilityResponse], error) {
				return usecase.Outcome[domain.EnableCapabilityResponse]{}, fmt.Errorf("application: enable capability %s: %w", cmd.CapabilityID, err)
			}
			facts, err := s.enablementFacts(ctx, res, cmd.CapabilityID, identity.OccurredAt)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Capabilities, res.Outbox, origin(AggregateTypeCapability, string(cmd.CapabilityID)), cmd.CapabilityID, identity,
				loadExistingCapability,
				func(c *domain.Capability) (kernel.Accepted[domain.EnableCapabilityResponse], *kernel.Rejection) {
					return c.Enable(domain.EnableCapability{
						HasEffectiveAgreement: facts.effectiveAgreement,
						HasEffectiveProfile:   facts.effectiveProfile,
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

type enablementFacts struct {
	effectiveAgreement bool
	effectiveProfile   bool
}

func (s Service) enablementFacts(ctx context.Context, res Resources, id domain.CapabilityID, at kernel.Instant) (enablementFacts, error) {
	var facts enablementFacts
	capability, found, err := loaded(ctx, res.Capabilities, id)
	if err != nil || !found {
		return facts, err
	}
	if facts.effectiveAgreement, err = agreementInEffect(ctx, res, capability.PartnerID, at); err != nil {
		return facts, err
	}
	facts.effectiveProfile, err = profileInEffect(ctx, res, capability.PartnerID, capability.Environment, at)
	return facts, err
}
