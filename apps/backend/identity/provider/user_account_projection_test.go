package provider_test

import (
	"strconv"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

var userAccountStatusNames = map[domain.UserAccountStatus]string{
	domain.UserAccountNew:       "new",
	domain.UserAccountActive:    "active",
	domain.UserAccountSuspended: "suspended",
}

func TestUserAccountsMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "user-account.golden")
	if f.Identity.Context != "identity" || f.Identity.Aggregate != "user-account" || len(f.Cases) != 7 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runUserAccount)
}

func runUserAccount(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	at := kernel.Instant(integer(t, c.Command["at"]))
	switch c.Command["upr"] {
	case "provision":
		s := userAccountSubject(func(u *domain.UserAccount) (kernel.Accepted[domain.ProvisionUserAccountResponse], *kernel.Rejection) {
			return u.Provision(domain.ProvisionUserAccount{PersonID: domain.PersonID(c.Command["person"]), At: at})
		}, func(r domain.ProvisionUserAccountResponse) domainkit.Fields {
			return domainkit.Fields{"user": string(r.UserID)}
		})
		return domainkit.Run(userAccountFromState(t, c.StateBefore), s), domainkit.ReadTwice(userAccountFromState(t, c.StateBefore), s)
	case "suspend":
		s := userAccountSubject(func(u *domain.UserAccount) (kernel.Accepted[domain.SuspendUserResponse], *kernel.Rejection) {
			return u.Suspend(domain.SuspendUser{Reason: c.Command["reason"], At: at})
		}, func(r domain.SuspendUserResponse) domainkit.Fields { return domainkit.Fields{"user": string(r.UserID)} })
		return domainkit.Run(userAccountFromState(t, c.StateBefore), s), domainkit.ReadTwice(userAccountFromState(t, c.StateBefore), s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func userAccountStatusFrom(name string) domain.UserAccountStatus {
	for status, known := range userAccountStatusNames {
		if known == name {
			return status
		}
	}
	return domain.UserAccountNew
}

func userAccountState(u *domain.UserAccount) domainkit.Fields {
	s := u.Snapshot()
	return domainkit.Fields{
		"user":              string(s.Subject),
		"person":            string(s.PersonID),
		"status":            userAccountStatusNames[s.Status],
		"provisioned_at":    strconv.FormatInt(int64(s.ProvisionedAt), 10),
		"suspended_at":      strconv.FormatInt(int64(s.SuspendedAt), 10),
		"suspension_reason": s.SuspensionReason,
	}
}

func userAccountFromState(t *testing.T, f domainkit.Fields) *domain.UserAccount {
	t.Helper()
	return domain.FromUserAccountSnapshot(domain.UserAccountSnapshot{
		Subject:          domain.UserID(f["user"]),
		PersonID:         domain.PersonID(f["person"]),
		Status:           userAccountStatusFrom(f["status"]),
		ProvisionedAt:    kernel.Instant(integer(t, f["provisioned_at"])),
		SuspendedAt:      kernel.Instant(integer(t, f["suspended_at"])),
		SuspensionReason: f["suspension_reason"],
	})
}

func userAccountEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	suspended, ok := e.(domain.UserSuspended)
	if !ok {
		return e.EventName(), domainkit.Fields{}
	}
	return suspended.EventName(), domainkit.Fields{
		"user":   string(suspended.UserID),
		"reason": suspended.Reason,
		"at":     strconv.FormatInt(int64(suspended.At), 10),
	}
}

func userAccountSubject[R any](decide func(*domain.UserAccount) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.UserAccount, R] {
	return domainkit.Subject[*domain.UserAccount, R]{
		Decide:   decide,
		Response: response,
		Event:    userAccountEvent,
		Snapshot: userAccountState,
		Clone: func(u *domain.UserAccount) *domain.UserAccount {
			return domain.FromUserAccountSnapshot(u.Snapshot())
		},
	}
}
