package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) DefineOperationalPolicy(ctx context.Context, cmd DefineOperationalPolicy) (usecase.Outcome[domain.DefineOperationalPolicyResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.DefineOperationalPolicyResponse]{
		Operation:   OperationDefineOperationalPolicy,
		Object:      string(cmd.PolicyID),
		Input:       cmd,
		Fingerprint: defineOperationalPolicyFingerprint(cmd),
		Codec:       defineOperationalPolicyCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.DefineOperationalPolicyResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.DefineOperationalPolicyResponse], error) {
				return usecase.Outcome[domain.DefineOperationalPolicyResponse]{}, fmt.Errorf("application: define operational policy %s: %w", cmd.PolicyID, err)
			}
			head, err := s.policyHead(ctx, cmd.PartnerID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.OperationalPolicies, res.Outbox, origin(AggregateTypeOperationalPolicy, string(cmd.PolicyID)), cmd.PolicyID, identity,
				loadAbsentPolicy,
				func(p *domain.OperationalPolicy) (kernel.Accepted[domain.DefineOperationalPolicyResponse], *kernel.Rejection) {
					return p.Define(domain.DefineOperationalPolicy{
						PartnerID:          cmd.PartnerID,
						EffectiveFrom:      cmd.EffectiveFrom,
						SupersedesID:       cmd.SupersedesID,
						Head:               head,
						RateLimitPerMinute: cmd.RateLimitPerMinute,
						BusinessDays:       cmd.BusinessDays,
						CutoffTime:         cmd.CutoffTime,
						ProcessingWindow:   cmd.ProcessingWindow,
						SlaRef:             cmd.SlaRef,
						Fallback:           cmd.Fallback,
						SupportContact:     cmd.SupportContact,
						IncidentContact:    cmd.IncidentContact,
						At:                 identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}

func defineOperationalPolicyFingerprint(cmd DefineOperationalPolicy) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationDefineOperationalPolicy).
		String(string(cmd.PolicyID)).
		String(string(cmd.PartnerID)).
		Int(int64(cmd.EffectiveFrom)).
		String(string(cmd.SupersedesID)).
		Int(int64(cmd.RateLimitPerMinute))
	fingerprintList(f, cmd.BusinessDays)
	return f.
		String(cmd.CutoffTime).
		String(cmd.ProcessingWindow).
		String(cmd.SlaRef).
		String(cmd.Fallback).
		String(cmd.SupportContact).
		String(cmd.IncidentContact)
}
