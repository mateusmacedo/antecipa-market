package provider

import (
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

type userAccountState struct {
	PersonID         string `json:"personId"`
	Status           int    `json:"status"`
	ProvisionedAt    int64  `json:"provisionedAt"`
	SuspendedAt      int64  `json:"suspendedAt"`
	SuspensionReason string `json:"suspensionReason"`
}

var userAccountTable = postgres.SnapshotTable("user_accounts", "user_account_id",
	func(s domain.UserAccountSnapshot) userAccountState {
		return userAccountState{
			PersonID:         string(s.PersonID),
			Status:           int(s.Status),
			ProvisionedAt:    int64(s.ProvisionedAt),
			SuspendedAt:      int64(s.SuspendedAt),
			SuspensionReason: s.SuspensionReason,
		}
	},
	func(state userAccountState) domain.UserAccountSnapshot {
		return domain.UserAccountSnapshot{
			PersonID:         domain.PersonID(state.PersonID),
			Status:           domain.UserAccountStatus(state.Status),
			ProvisionedAt:    kernel.Instant(state.ProvisionedAt),
			SuspendedAt:      kernel.Instant(state.SuspendedAt),
			SuspensionReason: state.SuspensionReason,
		}
	},
	func(s domain.UserAccountSnapshot, subject domain.UserID) domain.UserAccountSnapshot {
		s.Subject = subject
		return s
	},
)

func NewUserAccountRepository(tx *postgres.Tx) ports.UserAccountRepository {
	return userAccountTable.Repository(tx)
}

func NewUserAccountReader(pool postgres.ReadPool) ports.UserAccountReader {
	return userAccountTable.Reader(pool)
}
