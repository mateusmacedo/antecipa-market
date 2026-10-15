package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (c *BankAccountChange) Approve(cmd ApproveBankAccountChange) (kernel.Accepted[ApproveBankAccountChangeResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*BankAccountChange).clone, func(next *BankAccountChange) (kernel.Accepted[ApproveBankAccountChangeResponse], *kernel.Rejection) {
		switch {
		case next.status != BankAccountChangeStatusRequested:
			return kernel.Refuse[ApproveBankAccountChangeResponse](CodeBankAccountChangeNotRequested, "the bank account change must be requested")
		case cmd.ApprovedByUserID == next.requestedByUserID:
			return kernel.Refuse[ApproveBankAccountChangeResponse](CodeBankAccountChangeSameActorApproval, "the approver must not be the requester")
		}
		next.status = BankAccountChangeStatusApproved
		next.approvedByUserID = cmd.ApprovedByUserID
		next.approvedAt = cmd.At
		return kernel.Accept(
			ApproveBankAccountChangeResponse{BankAccountChangeID: next.id},
			BankAccountChangeApproved{
				BankAccountChangeID: next.id,
				OrganizationID:      next.organizationID,
				ApprovedByUserID:    cmd.ApprovedByUserID,
				At:                  cmd.At,
			},
		), nil
	})
}
