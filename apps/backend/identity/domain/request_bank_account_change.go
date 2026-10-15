package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (c *BankAccountChange) Request(cmd RequestBankAccountChange) (kernel.Accepted[RequestBankAccountChangeResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*BankAccountChange).clone, func(next *BankAccountChange) (kernel.Accepted[RequestBankAccountChangeResponse], *kernel.Rejection) {
		switch {
		case strings.TrimSpace(cmd.DestinationEvidenceID) == "":
			return kernel.Refuse[RequestBankAccountChangeResponse](CodeBankAccountChangeDestinationEvidenceMissing, "the evidence of the destination must be informed")
		case cmd.OrganizationStatus != OrganizationActive:
			return kernel.Refuse[RequestBankAccountChangeResponse](CodeBankAccountChangeOrganizationNotActive, "organization must exist and be active",
				kernel.Detail{Key: "organization", Value: string(cmd.OrganizationID)},
			)
		}
		next.organizationID = cmd.OrganizationID
		next.destinationEvidenceID = cmd.DestinationEvidenceID
		next.requestedByUserID = cmd.RequestedByUserID
		next.status = BankAccountChangeStatusRequested
		next.requestedAt = cmd.At
		return kernel.Accept(
			RequestBankAccountChangeResponse{BankAccountChangeID: next.id},
			BankAccountChangeRequested{
				BankAccountChangeID:   next.id,
				OrganizationID:        cmd.OrganizationID,
				DestinationEvidenceID: cmd.DestinationEvidenceID,
				RequestedByUserID:     cmd.RequestedByUserID,
				At:                    cmd.At,
			},
		), nil
	})
}
