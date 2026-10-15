package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func grant() application.GrantMembership {
	return application.GrantMembership{
		MembershipID:      testMembershipID,
		UserID:            testSubject,
		PersonID:          testPersonID,
		ChannelConfirmed:  true,
		VerificationLevel: domain.VerificationPersonalDataChecked,
	}
}

func withInvitationAndAccount(t *testing.T, h *harness, account domain.UserAccountSnapshot) {
	t.Helper()
	withMembership(t, h, invitedMembership(testMembershipID, domain.RoleOperator))
	withAccount(t, h, account)
}

func TestGrantMembershipWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withInvitationAndAccount(t, h, activeAccount(testSubject))

	if _, err := h.service.GrantMembership(withExecution(t, context.Background()), grant()); err != nil {
		t.Fatalf("GrantMembership() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"user_accounts.Load",
		"memberships.Load",
		"memberships.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2, spec note †)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestGrantMembershipActivatesItAndEnqueuesTheGrantInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	withInvitationAndAccount(t, h, activeAccount(testSubject))
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.GrantMembership(ctx, grant())

	if err != nil || outcome.Response() != (domain.GrantMembershipResponse{MembershipID: testMembershipID}) {
		t.Fatalf("GrantMembership() = %+v, %v; want accepted", outcome, err)
	}
	stored, version, err := membershipTable.Reader(h.store).Load(ctx, testMembershipID)
	if want := activeMembership(testMembershipID, domain.RoleOperator, testSubject); err != nil || version != 2 || !stored.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v at v2", stored, version, err, want)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeMembership || entry.AggregateID != string(testMembershipID) ||
		entry.AggregateVersion != 2 || entry.Intent.PartitionKey != string(testMembershipID) {
		t.Fatalf("outbox entry = %+v, want the grant of the membership at version 2", entry)
	}
	want := domain.MembershipGranted{
		MembershipID:      testMembershipID,
		OrganizationID:    testOrganizationID,
		UserID:            testSubject,
		Role:              domain.RoleOperator,
		VerificationLevel: domain.VerificationPersonalDataChecked,
		At:                testOccurred,
	}
	if got, ok := entry.Event.(domain.MembershipGranted); !ok || got != want {
		t.Fatalf("event = %+v, want %+v", entry.Event, want)
	}
}

func TestGrantMembershipRejectsEachPreconditionWithoutWriting(t *testing.T) {
	expired := invitedMembership(testMembershipID, domain.RoleOperator)
	expired.InvitationExpiresAt = testOccurred
	for _, tc := range []struct {
		name       string
		membership domain.MembershipSnapshot
		account    *domain.UserAccountSnapshot
		mutate     func(*application.GrantMembership)
		code       kernel.Code
	}{
		{"membership already active", activeMembership(testMembershipID, domain.RoleOperator, testSubject), ptr(activeAccount(testSubject)),
			func(*application.GrantMembership) {}, domain.CodeMembershipNotInvited},
		{"invitation expired", expired, ptr(activeAccount(testSubject)), func(*application.GrantMembership) {}, domain.CodeMembershipInvitationExpired},
		{"channel not confirmed", invitedMembership(testMembershipID, domain.RoleOperator), ptr(activeAccount(testSubject)),
			func(c *application.GrantMembership) { c.ChannelConfirmed = false }, domain.CodeMembershipChannelNotConfirmed},
		{"operator with channel ownership", invitedMembership(testMembershipID, domain.RoleOperator), ptr(activeAccount(testSubject)),
			func(c *application.GrantMembership) { c.VerificationLevel = domain.VerificationChannelOwnership }, domain.CodeMembershipVerificationInsufficient},
		{"user suspended", invitedMembership(testMembershipID, domain.RoleOperator), ptr(suspendedAccount(testSubject)),
			func(*application.GrantMembership) {}, domain.CodeMembershipUserNotActive},
		{"user not found", invitedMembership(testMembershipID, domain.RoleOperator), nil,
			func(*application.GrantMembership) {}, domain.CodeMembershipUserNotActive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			withMembership(t, h, tc.membership)
			if tc.account != nil {
				withAccount(t, h, *tc.account)
			}
			cmd := grant()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.GrantMembership(ctx, cmd)

			if err != nil {
				t.Fatalf("GrantMembership() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if stored, version, _ := membershipTable.Reader(h.store).Load(ctx, testMembershipID); version != 1 || !stored.Equal(tc.membership) {
				t.Fatalf("Load() = %+v v%d, want the membership untouched at v1", stored, version)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestGrantMembershipToAUserOfAnotherTenantIsNotActive(t *testing.T) {
	h := newHarness(t)
	withMembership(t, h, invitedMembership(testMembershipID, domain.RoleOperator))
	seed(t, h, withTenant(t, context.Background(), "globex"), userAccountTable, testSubject, activeAccount(testSubject))

	outcome, err := h.service.GrantMembership(withExecution(t, context.Background()), grant())

	if rej, refused := outcome.Rejection(); err != nil || !refused || rej.Code() != domain.CodeMembershipUserNotActive {
		t.Fatalf("GrantMembership() = %+v, %v; want %s (IDN-13)", outcome, err, domain.CodeMembershipUserNotActive)
	}
}

func TestGrantingAnUnknownMembershipIsNotFound(t *testing.T) {
	h := newHarness(t)
	withAccount(t, h, activeAccount(testSubject))

	_, err := h.service.GrantMembership(withExecution(t, context.Background()), grant())

	if want := "application: grant membership mem-100: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("GrantMembership() = %v, want %q", err, want)
	}
}

func TestEachFailureOfGrantMembershipCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		want    string
	}{
		{"user load", []option{withTableFaults("user_accounts", serviceskit.Faults{Load: errBoom})}, "application: grant membership mem-100: boom"},
		{"load", []option{withTableFaults("memberships", serviceskit.Faults{Load: errBoom})}, "application: grant membership mem-100: boom"},
		{"save", []option{withTableFaults("memberships", serviceskit.Faults{Save: errBoom})}, "application: grant membership mem-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, "application: grant membership mem-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withInvitationAndAccount(t, h, activeAccount(testSubject))

			_, err := h.service.GrantMembership(withExecution(t, context.Background()), grant())

			if !errors.Is(err, errBoom) || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestTheSameKeyWithAnotherGrantIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.GrantMembership){
		"membership":         func(c *application.GrantMembership) { c.MembershipID = "mem-other" },
		"user":               func(c *application.GrantMembership) { c.UserID = "user-sub-other" },
		"person":             func(c *application.GrantMembership) { c.PersonID = "per-other" },
		"channel confirmed":  func(c *application.GrantMembership) { c.ChannelConfirmed = false },
		"verification level": func(c *application.GrantMembership) { c.VerificationLevel = domain.VerificationChannelOwnership },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withInvitationAndAccount(t, h, activeAccount(testSubject))
			if _, err := h.service.GrantMembership(withKey(t, context.Background(), "k-reused"), grant()); err != nil {
				t.Fatalf("first GrantMembership() = %v, want nil", err)
			}
			second := grant()
			mutate(&second)

			_, err := h.service.GrantMembership(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second GrantMembership() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
