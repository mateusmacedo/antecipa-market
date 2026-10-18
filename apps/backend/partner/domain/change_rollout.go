package domain

import (
	"strconv"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (c *Capability) ChangeRollout(cmd ChangeRollout) (kernel.Accepted[ChangeRolloutResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Capability).clone, func(next *Capability) (kernel.Accepted[ChangeRolloutResponse], *kernel.Rejection) {
		switch {
		case cmd.Mode == RolloutPercentage && (cmd.Percentage < 0 || cmd.Percentage > 100):
			return kernel.Refuse[ChangeRolloutResponse](CodeCapabilityPercentageOutOfRange, "the percentage is between 0 and 100",
				kernel.Detail{Key: "percentage", Value: strconv.Itoa(cmd.Percentage)},
			)
		case cmd.Mode == RolloutAllowlist && len(cmd.AllowlistTenantIDs) == 0:
			return kernel.Refuse[ChangeRolloutResponse](CodeCapabilityAllowlistEmpty, "an allowlist rollout names at least one tenant")
		}
		next.rollout = Rollout{
			Mode:               cmd.Mode,
			AllowlistTenantIDs: cmd.AllowlistTenantIDs,
			Percentage:         cmd.Percentage,
			ExcludedTenantIDs:  cmd.ExcludedTenantIDs,
		}.clone()
		return kernel.Accept(
			ChangeRolloutResponse{CapabilityID: next.id},
			CapabilityRolloutChanged{
				CapabilityID: next.id,
				PartnerID:    next.partnerID,
				Operation:    next.operation,
				AssetType:    next.assetType,
				Environment:  next.environment,
				Version:      next.version,
				Mode:         cmd.Mode,
				Percentage:   cmd.Percentage,
				At:           cmd.At,
			},
		), nil
	})
}
