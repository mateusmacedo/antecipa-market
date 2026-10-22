package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (e *Establishment) Add(cmd AddEstablishment) (kernel.Accepted[AddEstablishmentResponse], *kernel.Rejection) {
	return kernel.DecideOver(e, (*Establishment).clone, func(next *Establishment) (kernel.Accepted[AddEstablishmentResponse], *kernel.Rejection) {
		organization := kernel.Detail{Key: "organization", Value: string(cmd.OrganizationID)}
		switch {
		case cmd.OrganizationStatus != OrganizationActive:
			return kernel.Refuse[AddEstablishmentResponse](CodeEstablishmentOrganizationNotActive, "organization must exist and be active", organization)
		case NormalizeTaxID(cmd.TaxIDType, cmd.TaxID) == "":
			return kernel.Refuse[AddEstablishmentResponse](CodeEstablishmentTaxIDEmpty, "tax id must not be empty")
		}
		next.organizationID = cmd.OrganizationID
		next.taxIDType = cmd.TaxIDType
		next.taxID = cmd.TaxID
		next.taxIDHash = cmd.TaxIDHash
		next.taxIDHashKeyVersion = cmd.TaxIDHashKeyVersion
		next.taxIDMasked = MaskTaxID(cmd.TaxIDType, cmd.TaxID)
		next.tradeName = cmd.TradeName
		next.receiver = cmd.Receiver
		next.externalReferences = slices.Clone(cmd.ExternalReferences)
		next.createdAt = cmd.At
		return kernel.Accept(
			AddEstablishmentResponse{EstablishmentID: next.id},
			EstablishmentAdded{
				EstablishmentID: next.id,
				OrganizationID:  cmd.OrganizationID,
				TaxIDType:       cmd.TaxIDType,
				TaxIDHash:       cmd.TaxIDHash,
				Receiver:        cmd.Receiver,
				At:              cmd.At,
			},
		), nil
	})
}
