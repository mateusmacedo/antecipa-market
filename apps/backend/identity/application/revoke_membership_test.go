package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func revoke() application.RevokeMembership {
	return application.RevokeMembership{MembershipID: testMembershipID, Reason: "saiu da empresa"}
}

func revokedMembership(id domain.MembershipID, role domain.Role, user domain.UserID) domain.MembershipSnapshot {
	s := activeMembership(id, role, user)
	s.Status = domain.MembershipStatusRevoked
	s.RevokedAt = testOccurred
	s.RevocationReason = "saiu da empresa"
	return s
}

func TestRevokeMembershipWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withMembership(t, h, activeMembership(testMembershipID, domain.RoleOperator, testSubject))

	if _, err := h.service.RevokeMembership(withExecution(t, context.Background()), revoke()); err != nil {
		t.Fatalf("RevokeMembership() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"memberships.Load",
		"memberships.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRevokeMembershipRevokesItAndEnqueuesTheRevocationInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	withMembership(t, h, activeMembership(testMembershipID, domain.RoleOperator, testSubject))
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RevokeMembership(ctx, revoke())

	if err != nil || outcome.Response() != (domain.RevokeMembershipResponse{MembershipID: testMembershipID}) {
		t.Fatalf("RevokeMembership() = %+v, %v; want accepted", outcome, err)
	}
	stored, version, err := membershipTable.Reader(h.store).Load(ctx, testMembershipID)
	if want := revokedMembership(testMembershipID, domain.RoleOperator, testSubject); err != nil || version != 2 || !stored.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v at v2", stored, version, err, want)
	}
	entries := h.store.Entries()
	if len(entries) != 1 || entries[0].AggregateType != application.AggregateTypeMembership || entries[0].AggregateVersion != 2 {
		t.Fatalf("outbox entries = %+v, want the revocation of the membership at version 2", entries)
	}
	want := domain.MembershipRevoked{
		MembershipID:   testMembershipID,
		OrganizationID: testOrganizationID,
		UserID:         testSubject,
		Role:           domain.RoleOperator,
		Reason:         "saiu da empresa",
		At:             testOccurred,
	}
	if got, ok := entries[0].Event.(domain.MembershipRevoked); !ok || got != want {
		t.Fatalf("event = %+v, want %+v", entries[0].Event, want)
	}
}

func TestRevokeMembershipRejectsAnInvitationWithoutWriting(t *testing.T) {
	h := newHarness(t)
	invited := invitedMembership(testMembershipID, domain.RoleOperator)
	withMembership(t, h, invited)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RevokeMembership(ctx, revoke())

	if rej, refused := outcome.Rejection(); err != nil || !refused || rej.Code() != domain.CodeMembershipNotActive {
		t.Fatalf("RevokeMembership() = %+v, %v; want %s", outcome, err, domain.CodeMembershipNotActive)
	}
	if stored, version, _ := membershipTable.Reader(h.store).Load(ctx, testMembershipID); version != 1 || !stored.Equal(invited) {
		t.Fatalf("Load() = %+v v%d, want the invitation untouched at v1", stored, version)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none", got)
	}
}

func TestEachFailureOfRevokeMembershipCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		want    string
	}{
		{"load", []option{withTableFaults("memberships", serviceskit.Faults{Load: errBoom})}, "application: revoke membership mem-100: boom"},
		{"save", []option{withTableFaults("memberships", serviceskit.Faults{Save: errBoom})}, "application: revoke membership mem-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, "application: revoke membership mem-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withMembership(t, h, activeMembership(testMembershipID, domain.RoleOperator, testSubject))

			_, err := h.service.RevokeMembership(withExecution(t, context.Background()), revoke())

			if !errors.Is(err, errBoom) || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestTheSameKeyWithAnotherRevocationIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.RevokeMembership){
		"membership": func(c *application.RevokeMembership) { c.MembershipID = "mem-other" },
		"reason":     func(c *application.RevokeMembership) { c.Reason = "outro motivo" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withMembership(t, h, activeMembership(testMembershipID, domain.RoleOperator, testSubject))
			if _, err := h.service.RevokeMembership(withKey(t, context.Background(), "k-reused"), revoke()); err != nil {
				t.Fatalf("first RevokeMembership() = %v, want nil", err)
			}
			second := revoke()
			mutate(&second)

			_, err := h.service.RevokeMembership(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second RevokeMembership() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
