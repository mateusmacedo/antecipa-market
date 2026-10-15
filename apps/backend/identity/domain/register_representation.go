package domain

import (
	"slices"
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (r *Representation) Register(cmd RegisterRepresentation) (kernel.Accepted[RegisterRepresentationResponse], *kernel.Rejection) {
	return kernel.DecideOver(r, (*Representation).clone, func(next *Representation) (kernel.Accepted[RegisterRepresentationResponse], *kernel.Rejection) {
		currency := strings.TrimSpace(cmd.ScopeCurrency)
		switch {
		case strings.TrimSpace(cmd.SourceEvidenceID) == "":
			return kernel.Refuse[RegisterRepresentationResponse](CodeRepresentationSourceEvidenceMissing, "the evidence of the powers must be informed")
		case cmd.ValidUntil != 0 && cmd.ValidUntil <= cmd.ValidFrom:
			return kernel.Refuse[RegisterRepresentationResponse](CodeRepresentationValidityInvalid, "valid until must come after valid from")
		case !validScope(currency, cmd.ScopeMaxAmountMinor):
			return kernel.Refuse[RegisterRepresentationResponse](CodeRepresentationScopeInvalid, "the scope needs a positive amount with its currency, or neither")
		case cmd.OrganizationStatus != OrganizationActive:
			return kernel.Refuse[RegisterRepresentationResponse](CodeRepresentationOrganizationNotActive, "organization must exist and be active",
				kernel.Detail{Key: "organization", Value: string(cmd.OrganizationID)},
			)
		case !cmd.PersonFound:
			return kernel.Refuse[RegisterRepresentationResponse](CodeRepresentationPersonNotFound, "person must exist",
				kernel.Detail{Key: "person", Value: string(cmd.PersonID)},
			)
		}
		next.organizationID = cmd.OrganizationID
		next.personID = cmd.PersonID
		next.powerType = cmd.PowerType
		next.sourceType = cmd.SourceType
		next.sourceEvidenceID = cmd.SourceEvidenceID
		next.scopeCurrency = currency
		next.scopeMaxAmountMinor = cmd.ScopeMaxAmountMinor
		next.scopeActTypes = slices.Clone(cmd.ScopeActTypes)
		next.validFrom = cmd.ValidFrom
		next.validUntil = cmd.ValidUntil
		next.verificationStatus = RepresentationDeclared
		next.proposedByUserID = cmd.ProposedByUserID
		return kernel.Accept(RegisterRepresentationResponse{RepresentationID: next.id}), nil
	})
}

func validScope(currency string, maxAmountMinor int64) bool {
	informed := currency != ""
	return (informed && maxAmountMinor > 0) || (!informed && maxAmountMinor == 0)
}
