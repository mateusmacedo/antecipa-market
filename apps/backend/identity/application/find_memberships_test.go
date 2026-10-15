package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func membershipIDs(snapshots []domain.MembershipSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}

func withMembershipsOfTwoUsers(t *testing.T, h *harness) {
	t.Helper()
	elsewhere := activeMembership("mem-c", domain.RoleAuditor, testSubject)
	elsewhere.OrganizationID = "org-other"
	for _, m := range []domain.MembershipSnapshot{
		activeMembership("mem-a", domain.RoleOperator, testSubject),
		revokedMembership("mem-b", domain.RoleAuditor, testSubject),
		elsewhere,
		activeMembership("mem-d", domain.RoleOperator, "user-sub-other"),
		invitedMembership("mem-e", domain.RoleAuditor),
	} {
		withMembership(t, h, m)
	}
}

func TestFindMembershipsByUserAnswersEveryMembershipOfTheSubject(t *testing.T) {
	h := newHarness(t)
	withMembershipsOfTwoUsers(t, h)

	got, err := h.service.FindMembershipsByUser(withExecution(t, context.Background()), testSubject)

	if want := []string{"mem-a", "mem-b", "mem-c"}; err != nil || !slices.Equal(membershipIDs(got), want) {
		t.Fatalf("FindMembershipsByUser() = %v, %v; want %v", membershipIDs(got), err, want)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindMembershipsByOrganizationAnswersEveryMembershipOfIt(t *testing.T) {
	h := newHarness(t)
	withMembershipsOfTwoUsers(t, h)

	got, err := h.service.FindMembershipsByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if want := []string{"mem-a", "mem-b", "mem-d", "mem-e"}; err != nil || !slices.Equal(membershipIDs(got), want) {
		t.Fatalf("FindMembershipsByOrganization() = %v, %v; want %v", membershipIDs(got), err, want)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v: a query runs outside any unit of work (UOW-11)", h.rec.Observed())
	}
}

func TestFindMembershipsOfAnotherTenantAnswersEmptyAndReportsTheAccess(t *testing.T) {
	access := port.CrossTenantAccess{Object: "memberships?user_id=user-sub-100", ContextTenant: "globex", DataTenant: testTenant}
	for name, find := range map[string]func(*harness, context.Context) ([]domain.MembershipSnapshot, error){
		"by user": func(h *harness, ctx context.Context) ([]domain.MembershipSnapshot, error) {
			return h.service.FindMembershipsByUser(ctx, testSubject)
		},
		"by organization": func(h *harness, ctx context.Context) ([]domain.MembershipSnapshot, error) {
			return h.service.FindMembershipsByOrganization(ctx, testOrganizationID)
		},
	} {
		t.Run(name, func(t *testing.T) {
			h, instr := newInstrumentedHarness(t)
			h.memberships.err = access

			got, err := find(h, withExecution(t, context.Background()))

			if err != nil || got == nil || len(got) != 0 {
				t.Fatalf("find() = %v, %v; want the empty answer (IDN-13)", got, err)
			}
			var reported port.CrossTenantAccess
			if len(instr.results) != 1 || !errors.As(instr.results[0].Err, &reported) || reported != access {
				t.Fatalf("EndOperation(%+v), want the access handed over for the security record (IDN-12)", instr.results)
			}
		})
	}
}

func TestFindMembershipsCarriesThePrefixOnFailure(t *testing.T) {
	h := newHarness(t)
	h.memberships.err = errBoom
	ctx := withExecution(t, context.Background())

	_, byUser := h.service.FindMembershipsByUser(ctx, testSubject)
	_, byOrganization := h.service.FindMembershipsByOrganization(ctx, testOrganizationID)

	if want := "application: find memberships by user user-sub-100: boom"; !errors.Is(byUser, errBoom) || byUser.Error() != want {
		t.Fatalf("FindMembershipsByUser() = %v, want %q", byUser, want)
	}
	if want := "application: find memberships by organization org-100: boom"; !errors.Is(byOrganization, errBoom) || byOrganization.Error() != want {
		t.Fatalf("FindMembershipsByOrganization() = %v, want %q", byOrganization, want)
	}
}
