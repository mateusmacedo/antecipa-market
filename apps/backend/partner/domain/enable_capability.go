package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (c *Capability) Enable(cmd EnableCapability) (kernel.Accepted[EnableCapabilityResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Capability).clone, func(next *Capability) (kernel.Accepted[EnableCapabilityResponse], *kernel.Rejection) {
		switch {
		case next.status != CapabilityStatusHomologated && next.status != CapabilityStatusSuspended:
			return kernel.Refuse[EnableCapabilityResponse](CodeCapabilityNotHomologated, "only a homologated or suspended capability is enabled")
		case next.suspendedBy(TriggerSchemaMismatch):
			return kernel.Refuse[EnableCapabilityResponse](CodeCapabilityRehomologationRequired, "a capability suspended by a schema mismatch must be homologated again")
		case !cmd.HasEffectiveAgreement:
			return kernel.Refuse[EnableCapabilityResponse](CodeCapabilityAgreementNotActive, "an active agreement in effect is required")
		case !cmd.HasEffectiveProfile:
			return kernel.Refuse[EnableCapabilityResponse](CodeCapabilityIntegrationProfileMissing, "an integration profile in effect for the environment is required")
		}
		next.status = CapabilityStatusEnabled
		next.enabledAt = cmd.At
		next.suspension = Suspension{}
		return kernel.Accept(
			EnableCapabilityResponse{CapabilityID: next.id},
			CapabilityEnabled{
				CapabilityID:        next.id,
				PartnerID:           next.partnerID,
				Operation:           next.operation,
				AssetType:           next.assetType,
				Environment:         next.environment,
				Version:             next.version,
				RolloutMode:         next.rollout.Mode,
				StateQuerySupported: next.stateQuerySupported,
				RestrictionNote:     next.restrictionNote,
				At:                  cmd.At,
			},
		), nil
	})
}
