package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

func (u *UserAccount) Provision(cmd ProvisionUserAccount) (kernel.Accepted[ProvisionUserAccountResponse], *kernel.Rejection) {
	return kernel.DecideOver(u, (*UserAccount).clone, func(next *UserAccount) (kernel.Accepted[ProvisionUserAccountResponse], *kernel.Rejection) {
		if strings.TrimSpace(string(next.subject)) == "" {
			return kernel.Refuse[ProvisionUserAccountResponse](CodeUserAccountSubjectEmpty, "subject must not be empty")
		}
		response := ProvisionUserAccountResponse{UserID: next.subject}
		// WHY: the spec initializes only when absent; a present account, even a
		// suspended one, is accepted with no business effect.
		if next.status != UserAccountNew {
			return kernel.Accept(response), nil
		}
		next.personID = cmd.PersonID
		next.status = UserAccountActive
		next.provisionedAt = cmd.At
		return kernel.Accept(response), nil
	})
}
