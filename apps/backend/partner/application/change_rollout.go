package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) ChangeRollout(ctx context.Context, cmd ChangeRollout) (usecase.Outcome[domain.ChangeRolloutResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ChangeRolloutResponse]{
		Operation:   OperationChangeRollout,
		Object:      string(cmd.CapabilityID),
		Input:       cmd,
		Fingerprint: changeRolloutFingerprint(cmd),
		Codec:       changeRolloutCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ChangeRolloutResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Capabilities, res.Outbox, origin(AggregateTypeCapability, string(cmd.CapabilityID)), cmd.CapabilityID, identity,
				loadExistingCapability,
				func(c *domain.Capability) (kernel.Accepted[domain.ChangeRolloutResponse], *kernel.Rejection) {
					return c.ChangeRollout(domain.ChangeRollout{
						Mode:               cmd.Mode,
						AllowlistTenantIDs: cmd.AllowlistTenantIDs,
						Percentage:         cmd.Percentage,
						ExcludedTenantIDs:  cmd.ExcludedTenantIDs,
						At:                 identity.OccurredAt,
					})
				})
			if err != nil {
				return usecase.Outcome[domain.ChangeRolloutResponse]{}, fmt.Errorf("application: change rollout %s: %w", cmd.CapabilityID, err)
			}
			return outcome, nil
		},
	})
}

func changeRolloutFingerprint(cmd ChangeRollout) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationChangeRollout).
		String(string(cmd.CapabilityID)).
		String(string(cmd.Mode)).
		Int(int64(cmd.Percentage))
	fingerprintList(f, cmd.AllowlistTenantIDs)
	fingerprintList(f, cmd.ExcludedTenantIDs)
	return f
}
