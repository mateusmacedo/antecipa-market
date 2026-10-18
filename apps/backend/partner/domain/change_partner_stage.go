package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (p *Partner) ChangeStage(cmd ChangePartnerStage) (kernel.Accepted[ChangePartnerStageResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*Partner).clone, func(next *Partner) (kernel.Accepted[ChangePartnerStageResponse], *kernel.Rejection) {
		from := next.status
		informed := strings.TrimSpace(cmd.Reason) != ""
		switch {
		case !from.leadsTo(cmd.Target):
			return kernel.Refuse[ChangePartnerStageResponse](CodePartnerInvalidStageTransition, "the stage transition is not allowed",
				kernel.Detail{Key: "from", Value: string(from)},
				kernel.Detail{Key: "to", Value: string(cmd.Target)},
			)
		case cmd.Target.demandsReason() && !informed:
			return kernel.Refuse[ChangePartnerStageResponse](CodePartnerReasonEmpty, "reason must not be empty when suspending or offboarding")
		case cmd.Target == PartnerActive && !cmd.HasEffectiveAgreement:
			return kernel.Refuse[ChangePartnerStageResponse](CodePartnerAgreementNotActive, "an active agreement in effect is required")
		case cmd.Target == PartnerActive && from == PartnerHomologating && !cmd.HasEnabledCapability:
			return kernel.Refuse[ChangePartnerStageResponse](CodePartnerNoEnabledCapability, "at least one enabled capability is required")
		}
		next.status = cmd.Target
		next.stageChangedAt = cmd.At
		if informed {
			next.stageReason = cmd.Reason
		}
		response := ChangePartnerStageResponse{PartnerID: next.id, Status: next.status}
		if cmd.Target != PartnerOffboarding {
			return kernel.Accept(response), nil
		}
		next.offboardingStartedAt = cmd.At
		return kernel.Accept(response, PartnerOffboardingStarted{
			PartnerID:      next.id,
			OrganizationID: next.organizationID,
			Reason:         next.stageReason,
			At:             next.offboardingStartedAt,
		}), nil
	})
}
