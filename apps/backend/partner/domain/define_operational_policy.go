package domain

import (
	"slices"
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (p *OperationalPolicy) Define(cmd DefineOperationalPolicy) (kernel.Accepted[DefineOperationalPolicyResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*OperationalPolicy).clone, func(next *OperationalPolicy) (kernel.Accepted[DefineOperationalPolicyResponse], *kernel.Rejection) {
		missing := missingContacts(cmd)
		switch {
		case cmd.RateLimitPerMinute <= 0:
			return kernel.Refuse[DefineOperationalPolicyResponse](CodeOperationalPolicyRateLimitInvalid, "the rate limit per minute must be positive")
		case len(missing) > 0:
			return kernel.Refuse[DefineOperationalPolicyResponse](CodeOperationalPolicyContactsMissing,
				"the support and the incident contacts are required",
				kernel.Detail{Key: "missing", Value: strings.Join(missing, ",")},
			)
		case cmd.SupersedesID != cmd.Head.ID:
			return kernel.Refuse[DefineOperationalPolicyResponse](CodeOperationalPolicySupersedesInvalid,
				"supersedes must name the last version of the series of the partner, and a series holds one first version",
				kernel.Detail{Key: "supersedes", Value: string(cmd.SupersedesID)},
			)
		}
		next.partnerID = cmd.PartnerID
		next.version = cmd.Head.Version + 1
		next.supersedesID = cmd.SupersedesID
		next.effectiveFrom = cmd.EffectiveFrom
		next.rateLimitPerMinute = cmd.RateLimitPerMinute
		next.businessDays = slices.Clone(cmd.BusinessDays)
		next.cutoffTime = cmd.CutoffTime
		next.processingWindow = cmd.ProcessingWindow
		next.slaRef = cmd.SlaRef
		next.fallback = cmd.Fallback
		next.supportContact = cmd.SupportContact
		next.incidentContact = cmd.IncidentContact
		return kernel.Accept(DefineOperationalPolicyResponse{PolicyID: next.id}), nil
	})
}

func missingContacts(cmd DefineOperationalPolicy) []string {
	var missing []string
	if strings.TrimSpace(cmd.SupportContact) == "" {
		missing = append(missing, "supportContact")
	}
	if strings.TrimSpace(cmd.IncidentContact) == "" {
		missing = append(missing, "incidentContact")
	}
	return missing
}
