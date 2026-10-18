package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (c *Capability) Suspend(cmd SuspendCapability) (kernel.Accepted[SuspendCapabilityResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Capability).clone, func(next *Capability) (kernel.Accepted[SuspendCapabilityResponse], *kernel.Rejection) {
		switch {
		case next.status != CapabilityStatusEnabled:
			return kernel.Refuse[SuspendCapabilityResponse](CodeCapabilityNotEnabled, "only an enabled capability is suspended")
		case strings.TrimSpace(cmd.Reason) == "":
			return kernel.Refuse[SuspendCapabilityResponse](CodeCapabilityReasonEmpty, "reason must not be empty when suspending")
		}
		next.status = CapabilityStatusSuspended
		next.suspension = Suspension{Reason: cmd.Reason, Trigger: cmd.Trigger, SuspendedAt: cmd.At}
		return kernel.Accept(
			SuspendCapabilityResponse{CapabilityID: next.id},
			CapabilitySuspended{
				CapabilityID: next.id,
				PartnerID:    next.partnerID,
				Operation:    next.operation,
				AssetType:    next.assetType,
				Environment:  next.environment,
				Version:      next.version,
				Reason:       cmd.Reason,
				Trigger:      cmd.Trigger,
				At:           cmd.At,
			},
		), nil
	})
}
