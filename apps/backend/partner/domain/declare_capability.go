package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (c *Capability) Declare(cmd DeclareCapability) (kernel.Accepted[DeclareCapabilityResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Capability).clone, func(next *Capability) (kernel.Accepted[DeclareCapabilityResponse], *kernel.Rejection) {
		switch {
		case strings.TrimSpace(cmd.Operation) == "":
			return kernel.Refuse[DeclareCapabilityResponse](CodeCapabilityOperationEmpty, "the operation is required")
		case strings.TrimSpace(cmd.AssetType) == "":
			return kernel.Refuse[DeclareCapabilityResponse](CodeCapabilityAssetTypeEmpty, "the asset type is required")
		case cmd.IsWrite && !cmd.StateQuerySupported && strings.TrimSpace(cmd.RestrictionNote) == "":
			return kernel.Refuse[DeclareCapabilityResponse](CodeCapabilityStateQueryRestrictionUndeclared,
				"a write operation without a state query must declare its restriction")
		case cmd.SupersedesID != cmd.Head.ID:
			return kernel.Refuse[DeclareCapabilityResponse](CodeCapabilitySupersedesInvalid,
				"supersedes must name the last version of the series of the partner, operation, asset type and environment, and a series holds one first version",
				kernel.Detail{Key: "supersedes", Value: string(cmd.SupersedesID)},
			)
		}
		next.partnerID = cmd.PartnerID
		next.operation = cmd.Operation
		next.assetType = cmd.AssetType
		next.environment = cmd.Environment
		next.version = cmd.Head.Version + 1
		next.supersedesID = cmd.SupersedesID
		next.isWrite = cmd.IsWrite
		next.stateQuerySupported = cmd.StateQuerySupported
		next.restrictionNote = cmd.RestrictionNote
		next.callbackApplicable = cmd.CallbackApplicable
		next.status = CapabilityStatusDeclared
		next.rollout = Rollout{Mode: RolloutDisabled}
		return kernel.Accept(DeclareCapabilityResponse{CapabilityID: next.id}), nil
	})
}
