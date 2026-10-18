package domain

import (
	"slices"
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

var (
	contractualCriteria          = []string{"H14", "H15", "H17", "H18"}
	regulatedContractualCriteria = []string{"H14", "H15", "H16", "H17", "H18"}
)

func (a *PartnerAgreement) Activate(cmd ActivateAgreement) (kernel.Accepted[ActivateAgreementResponse], *kernel.Rejection) {
	return kernel.DecideOver(a, (*PartnerAgreement).clone, func(next *PartnerAgreement) (kernel.Accepted[ActivateAgreementResponse], *kernel.Rejection) {
		missing := missingCriteria(cmd.SatisfiedCriteria, cmd.RegulatoryCheckApplicable)
		switch {
		case next.status != AgreementDraft:
			return kernel.Refuse[ActivateAgreementResponse](CodePartnerAgreementNotDraft, "only a draft agreement is activated")
		case strings.TrimSpace(cmd.ContractEvidenceID) == "":
			return kernel.Refuse[ActivateAgreementResponse](CodePartnerAgreementContractEvidenceMissing, "the evidence of the signed contract is required")
		case len(missing) > 0:
			return kernel.Refuse[ActivateAgreementResponse](CodePartnerAgreementContractualCriteriaIncomplete,
				"the contractual criteria must all be satisfied",
				kernel.Detail{Key: "missing", Value: strings.Join(missing, ",")},
			)
		case !next.validThrough(cmd.At):
			return kernel.Refuse[ActivateAgreementResponse](CodePartnerAgreementValidityInvalid, "valid until must be after the activation")
		}
		next.status = AgreementActive
		next.contractEvidenceID = cmd.ContractEvidenceID
		next.activatedAt = cmd.At
		return kernel.Accept(
			ActivateAgreementResponse{AgreementID: next.id},
			PartnerAgreementActivated{
				AgreementID: next.id,
				PartnerID:   next.partnerID,
				Version:     next.version,
				ValidFrom:   next.validFrom,
				ValidUntil:  next.validUntil,
				At:          cmd.At,
			},
		), nil
	})
}

func missingCriteria(satisfied []string, regulatory bool) []string {
	required := contractualCriteria
	if regulatory {
		required = regulatedContractualCriteria
	}
	var missing []string
	for _, criterion := range required {
		if !slices.Contains(satisfied, criterion) {
			missing = append(missing, criterion)
		}
	}
	return missing
}
