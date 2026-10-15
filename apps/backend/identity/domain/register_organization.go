package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (o *Organization) Register(cmd RegisterOrganization) (kernel.Accepted[RegisterOrganizationResponse], *kernel.Rejection) {
	return kernel.DecideOver(o, (*Organization).clone, func(next *Organization) (kernel.Accepted[RegisterOrganizationResponse], *kernel.Rejection) {
		switch {
		case cmd.Kinds.Empty():
			return kernel.Refuse[RegisterOrganizationResponse](CodeOrganizationKindsEmpty, "kinds must not be empty")
		case !validJurisdiction(cmd.Jurisdiction):
			return kernel.Refuse[RegisterOrganizationResponse](CodeOrganizationJurisdictionInvalid,
				"jurisdiction must be an ISO 3166-1 alpha-2 code",
				kernel.Detail{Key: "jurisdiction", Value: cmd.Jurisdiction},
			)
		case NormalizeTaxID(cmd.TaxIDType, cmd.TaxID) == "":
			return kernel.Refuse[RegisterOrganizationResponse](CodeOrganizationTaxIDEmpty, "tax id must not be empty")
		case strings.TrimSpace(cmd.LegalName) == "":
			return kernel.Refuse[RegisterOrganizationResponse](CodeOrganizationLegalNameEmpty, "legal name must not be empty")
		case cmd.TaxIDAlreadyRegistered:
			return kernel.Refuse[RegisterOrganizationResponse](CodeOrganizationDuplicateTaxID,
				"tax id already registered in the jurisdiction",
				kernel.Detail{Key: "jurisdiction", Value: cmd.Jurisdiction},
			)
		}
		masked := MaskTaxID(cmd.TaxIDType, cmd.TaxID)
		next.kinds = cmd.Kinds
		next.jurisdiction = cmd.Jurisdiction
		next.taxIDType = cmd.TaxIDType
		next.taxID = cmd.TaxID
		next.taxIDHash = cmd.TaxIDHash
		next.taxIDHashKeyVersion = cmd.TaxIDHashKeyVersion
		next.taxIDMasked = masked
		next.legalName = cmd.LegalName
		next.tradeName = cmd.TradeName
		next.status = OrganizationActive
		next.createdAt = cmd.At
		return kernel.Accept(
			RegisterOrganizationResponse{OrganizationID: next.id},
			OrganizationRegistered{
				OrganizationID: next.id,
				Kinds:          cmd.Kinds,
				Jurisdiction:   cmd.Jurisdiction,
				TaxIDType:      cmd.TaxIDType,
				TaxIDHash:      cmd.TaxIDHash,
				TaxIDMasked:    masked,
				At:             cmd.At,
			},
		), nil
	})
}

func validJurisdiction(code string) bool {
	return len(code) == 2 && isUpperASCII(code[0]) && isUpperASCII(code[1])
}

func isUpperASCII(c byte) bool { return c >= 'A' && c <= 'Z' }
