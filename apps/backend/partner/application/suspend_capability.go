package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) SuspendCapability(ctx context.Context, cmd SuspendCapability) (usecase.Outcome[domain.SuspendCapabilityResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.SuspendCapabilityResponse]{
		Operation: OperationSuspendCapability,
		Object:    string(cmd.CapabilityID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationSuspendCapability).
			String(string(cmd.CapabilityID)).
			String(cmd.Reason).
			String(string(cmd.Trigger)),
		Codec: suspendCapabilityCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.SuspendCapabilityResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Capabilities, res.Outbox, origin(AggregateTypeCapability, string(cmd.CapabilityID)), cmd.CapabilityID, identity,
				loadExistingCapability,
				func(c *domain.Capability) (kernel.Accepted[domain.SuspendCapabilityResponse], *kernel.Rejection) {
					return c.Suspend(domain.SuspendCapability{Reason: cmd.Reason, Trigger: cmd.Trigger, At: identity.OccurredAt})
				})
			if err != nil {
				return usecase.Outcome[domain.SuspendCapabilityResponse]{}, fmt.Errorf("application: suspend capability %s: %w", cmd.CapabilityID, err)
			}
			return outcome, nil
		},
	})
}
