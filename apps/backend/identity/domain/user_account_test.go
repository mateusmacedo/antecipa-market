package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	subject          = domain.UserID("user-sub-100")
	suspensionReason = "acesso indevido"
	suspendedAt      = at + 3_600_000_000_000
)

func provisionCommand() domain.ProvisionUserAccount {
	return domain.ProvisionUserAccount{PersonID: personID, At: at}
}

func activeAccount() domain.UserAccountSnapshot {
	return domain.UserAccountSnapshot{
		Subject:       subject,
		PersonID:      personID,
		Status:        domain.UserAccountActive,
		ProvisionedAt: at,
	}
}

func suspendedAccount() domain.UserAccountSnapshot {
	s := activeAccount()
	s.Status = domain.UserAccountSuspended
	s.SuspendedAt = suspendedAt
	s.SuspensionReason = suspensionReason
	return s
}

func TestProvisionUserAccountActivatesAnAbsentAccountWithoutAnEvent(t *testing.T) {
	u := domain.NewUserAccount(subject)

	acc, rej := u.Provision(provisionCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.ProvisionUserAccountResponse{UserID: subject}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if events := acc.Events(); len(events) != 0 {
		t.Fatalf("Events() = %v, want none", events)
	}
	if got := u.Snapshot(); !got.Equal(activeAccount()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, activeAccount())
	}
}

func TestProvisionUserAccountAcceptsAPresentAccountWithoutChangingIt(t *testing.T) {
	for name, held := range map[string]domain.UserAccountSnapshot{
		"active":    activeAccount(),
		"suspended": suspendedAccount(),
	} {
		t.Run(name, func(t *testing.T) {
			u := domain.FromUserAccountSnapshot(held)
			cmd := domain.ProvisionUserAccount{PersonID: "per-other", At: at + 1}

			acc, rej := u.Provision(cmd)

			tb.RequireAccepted(t, rej)
			if got, want := acc.Response(), (domain.ProvisionUserAccountResponse{UserID: subject}); got != want {
				t.Fatalf("Response() = %+v, want %+v", got, want)
			}
			if events := acc.Events(); len(events) != 0 {
				t.Fatalf("Events() = %v, want none: a present account has no business effect", events)
			}
			if got := u.Snapshot(); !got.Equal(held) {
				t.Fatalf("Snapshot() = %+v, want it unchanged %+v", got, held)
			}
		})
	}
}

func TestProvisionUserAccountRejectsAnEmptySubject(t *testing.T) {
	for name, empty := range map[string]domain.UserID{"empty": "", "blanks": "  "} {
		t.Run(name, func(t *testing.T) {
			u := domain.NewUserAccount(empty)
			before := u.Snapshot()

			acc, rej := u.Provision(provisionCommand())

			tb.RequireRejected(t, acc, rej, domain.CodeUserAccountSubjectEmpty)
			if after := u.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestSuspendUserSuspendsTheAccountAndEmitsTheFact(t *testing.T) {
	u := domain.FromUserAccountSnapshot(activeAccount())

	acc, rej := u.Suspend(domain.SuspendUser{Reason: suspensionReason, At: suspendedAt})

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.SuspendUserResponse{UserID: subject}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	tb.RequireSameEvents(t, acc.Events(), []kernel.DomainEvent{
		domain.UserSuspended{UserID: subject, Reason: suspensionReason, At: suspendedAt},
	})
	if got := u.Snapshot(); !got.Equal(suspendedAccount()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, suspendedAccount())
	}
}

func TestSuspendUserRejectsEachPrecondition(t *testing.T) {
	absent := domain.NewUserAccount(subject).Snapshot()
	for _, tc := range []struct {
		name   string
		held   domain.UserAccountSnapshot
		reason string
		code   kernel.Code
	}{
		{"account suspended", suspendedAccount(), suspensionReason, domain.CodeUserAccountNotActive},
		{"account never provisioned", absent, suspensionReason, domain.CodeUserAccountNotActive},
		{"reason empty", activeAccount(), "", domain.CodeUserAccountReasonEmpty},
		{"reason of blanks", activeAccount(), "  ", domain.CodeUserAccountReasonEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := domain.FromUserAccountSnapshot(tc.held)
			before := u.Snapshot()

			acc, rej := u.Suspend(domain.SuspendUser{Reason: tc.reason, At: suspendedAt})

			tb.RequireRejected(t, acc, rej, tc.code)
			if after := u.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestSuspendUserReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	acc, rej := domain.FromUserAccountSnapshot(suspendedAccount()).Suspend(domain.SuspendUser{At: suspendedAt})

	tb.RequireRejected(t, acc, rej, domain.CodeUserAccountNotActive)
}

func TestUserAccountDeterminism(t *testing.T) {
	u1 := domain.FromUserAccountSnapshot(activeAccount())
	acc1, _ := u1.Suspend(domain.SuspendUser{Reason: suspensionReason, At: suspendedAt})

	u2 := domain.FromUserAccountSnapshot(activeAccount())
	acc2, _ := u2.Suspend(domain.SuspendUser{Reason: suspensionReason, At: suspendedAt})

	if !u1.Snapshot().Equal(u2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestFromUserAccountSnapshotRoundTrips(t *testing.T) {
	for _, before := range []domain.UserAccountSnapshot{activeAccount(), suspendedAccount()} {
		if got := domain.FromUserAccountSnapshot(before).Snapshot(); !got.Equal(before) {
			t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
		}
	}
}

func TestUserAccountEqualTellsEveryFieldApart(t *testing.T) {
	base := suspendedAccount()
	for name, mutate := range map[string]func(*domain.UserAccountSnapshot){
		"subject":           func(s *domain.UserAccountSnapshot) { s.Subject = "user-sub-other" },
		"person":            func(s *domain.UserAccountSnapshot) { s.PersonID = "per-other" },
		"status":            func(s *domain.UserAccountSnapshot) { s.Status = domain.UserAccountActive },
		"provisioned at":    func(s *domain.UserAccountSnapshot) { s.ProvisionedAt++ },
		"suspended at":      func(s *domain.UserAccountSnapshot) { s.SuspendedAt++ },
		"suspension reason": func(s *domain.UserAccountSnapshot) { s.SuspensionReason = "other" },
	} {
		changed := suspendedAccount()
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func TestEachUserAccountStatusKeepsItsOrder(t *testing.T) {
	got := []domain.UserAccountStatus{domain.UserAccountNew, domain.UserAccountActive, domain.UserAccountSuspended}
	if want := []domain.UserAccountStatus{0, 1, 2}; !slices.Equal(got, want) {
		t.Fatalf("statuses = %v, want %v: the snapshot stores them as integers", got, want)
	}
}
