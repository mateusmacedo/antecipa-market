package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (u *UserAccount) Suspend(cmd SuspendUser) (kernel.Accepted[SuspendUserResponse], *kernel.Rejection) {
	return kernel.DecideOver(u, (*UserAccount).clone, func(next *UserAccount) (kernel.Accepted[SuspendUserResponse], *kernel.Rejection) {
		switch {
		case next.status != UserAccountActive:
			return kernel.Refuse[SuspendUserResponse](CodeUserAccountNotActive, "user account must be active")
		case strings.TrimSpace(cmd.Reason) == "":
			return kernel.Refuse[SuspendUserResponse](CodeUserAccountReasonEmpty, "reason must not be empty")
		}
		next.status = UserAccountSuspended
		next.suspendedAt = cmd.At
		next.suspensionReason = cmd.Reason
		return kernel.Accept(
			SuspendUserResponse{UserID: next.subject},
			UserSuspended{UserID: next.subject, Reason: cmd.Reason, At: cmd.At},
		), nil
	})
}
