package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

var (
	operatorCatalogue = domain.RoleOperator.Permissions()
	auditorCatalogue  = domain.RoleAuditor.Permissions()
)

func TestFindEffectivePermissionsAnswersTheUnionOfTheActiveMemberships(t *testing.T) {
	union := domain.EffectivePermissions(domain.RoleOperator, domain.RoleAuditor)
	for _, tc := range []struct {
		name        string
		memberships []domain.MembershipSnapshot
		want        []domain.Permission
		count       int
	}{
		{"an operator", []domain.MembershipSnapshot{activeMembership("mem-a", domain.RoleOperator, testSubject)}, operatorCatalogue, 17},
		{"an auditor", []domain.MembershipSnapshot{activeMembership("mem-a", domain.RoleAuditor, testSubject)}, auditorCatalogue, 14},
		{"an operator and an auditor", []domain.MembershipSnapshot{
			activeMembership("mem-a", domain.RoleOperator, testSubject),
			activeMembership("mem-b", domain.RoleAuditor, testSubject),
		}, union, 20},
		{"an operator with a revoked auditor", []domain.MembershipSnapshot{
			activeMembership("mem-a", domain.RoleOperator, testSubject),
			revokedMembership("mem-b", domain.RoleAuditor, testSubject),
		}, operatorCatalogue, 17},
		{"a revoked membership alone", []domain.MembershipSnapshot{revokedMembership("mem-a", domain.RoleOperator, testSubject)}, []domain.Permission{}, 0},
		{"no membership", nil, []domain.Permission{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			withAccount(t, h, activeAccount(testSubject))
			for _, m := range tc.memberships {
				withMembership(t, h, m)
			}

			got, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject)

			if err != nil || got == nil || len(got) != tc.count || !slices.Equal(got, tc.want) {
				t.Fatalf("FindEffectivePermissions() = %v (%d), %v; want %v (%d)", got, len(got), err, tc.want, tc.count)
			}
			if !slices.IsSorted(got) {
				t.Fatalf("FindEffectivePermissions() = %v, want ascending lexicographic order", got)
			}
		})
	}
}

func TestFindEffectivePermissionsReadsTheMembershipsBeforeTheAccountOutsideAnyUnitOfWork(t *testing.T) {
	h := newHarness(t)
	withAccount(t, h, activeAccount(testSubject))
	withMembership(t, h, activeMembership("mem-a", domain.RoleOperator, testSubject))

	if _, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject); err != nil {
		t.Fatalf("FindEffectivePermissions() error = %v, want nil", err)
	}

	if want := []string{"authorize", "memberships.LoadByUser", "user_accounts.Read"}; !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("observed = %v, want %v: the suspension is the last check (fail-closed, UOW-11)", h.rec.Observed(), want)
	}
}

func TestFindEffectivePermissionsAnswersNothingWithoutAnActiveAccount(t *testing.T) {
	for name, prepare := range map[string]func(*testing.T, *harness){
		"a suspended account": func(t *testing.T, h *harness) { withAccount(t, h, suspendedAccount(testSubject)) },
		"no account":          func(*testing.T, *harness) {},
		"an account of another tenant": func(t *testing.T, h *harness) {
			seed(t, h, withTenant(t, context.Background(), "globex"), userAccountTable, testSubject, activeAccount(testSubject))
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			prepare(t, h)
			withMembership(t, h, activeMembership("mem-a", domain.RoleOperator, testSubject))

			got, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject)

			if err != nil || got == nil || len(got) != 0 {
				t.Fatalf("FindEffectivePermissions() = %v, %v; want the empty list", got, err)
			}
		})
	}
}

func TestRevocationAndSuspensionHoldOnTheNextQuery(t *testing.T) {
	h := newHarness(t)
	withAccount(t, h, activeAccount(testSubject))
	withMembership(t, h, invitedMembership(testMembershipID, domain.RoleOperator))
	withMembership(t, h, activeMembership("mem-auditor", domain.RoleAuditor, testSubject))
	permissions := func() []domain.Permission {
		t.Helper()
		got, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject)
		if err != nil {
			t.Fatalf("FindEffectivePermissions() error = %v", err)
		}
		return got
	}
	if got := permissions(); !slices.Equal(got, auditorCatalogue) {
		t.Fatalf("before the grant = %v, want the auditor's: an invitation grants nothing", got)
	}

	if _, err := h.service.GrantMembership(withExecution(t, context.Background()), grant()); err != nil {
		t.Fatalf("GrantMembership() = %v", err)
	}
	if got := permissions(); len(got) != 20 {
		t.Fatalf("after the grant = %v, want the 20 of the union", got)
	}
	if _, err := h.service.RevokeMembership(withExecution(t, context.Background()), revoke()); err != nil {
		t.Fatalf("RevokeMembership() = %v", err)
	}
	if got := permissions(); !slices.Equal(got, auditorCatalogue) {
		t.Fatalf("after the revocation = %v, want the operator's permissions gone", got)
	}
	if _, err := h.service.SuspendUser(withExecution(t, context.Background()), suspend()); err != nil {
		t.Fatalf("SuspendUser() = %v", err)
	}
	if got := permissions(); len(got) != 0 {
		t.Fatalf("after the suspension = %v, want the empty list", got)
	}
}

func TestFindEffectivePermissionsReportsACrossTenantAccessOfEitherRead(t *testing.T) {
	membershipsAccess := port.CrossTenantAccess{Object: "memberships?user_id=user-sub-100", ContextTenant: testTenant, DataTenant: "globex"}
	for name, prepare := range map[string]func(*testing.T, *harness){
		"memberships": func(t *testing.T, h *harness) {
			withAccount(t, h, activeAccount(testSubject))
			h.memberships.err = membershipsAccess
		},
		"account": func(t *testing.T, h *harness) {
			seed(t, h, withTenant(t, context.Background(), "globex"), userAccountTable, testSubject, activeAccount(testSubject))
			withMembership(t, h, activeMembership("mem-a", domain.RoleOperator, testSubject))
		},
	} {
		t.Run(name, func(t *testing.T) {
			h, instr := newInstrumentedHarness(t)
			prepare(t, h)

			got, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject)

			if err != nil || len(got) != 0 {
				t.Fatalf("FindEffectivePermissions() = %v, %v; want the empty list (IDN-13)", got, err)
			}
			var reported port.CrossTenantAccess
			if len(instr.results) != 1 || instr.results[0].Outcome != port.OutcomeFailed || !errors.As(instr.results[0].Err, &reported) {
				t.Fatalf("EndOperation(%+v), want the access handed over for the security record (IDN-12)", instr.results)
			}
		})
	}
}

func TestFindEffectivePermissionsRecordsAnAcceptedQuery(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	withAccount(t, h, activeAccount(testSubject))

	if _, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject); err != nil {
		t.Fatalf("FindEffectivePermissions() error = %v", err)
	}

	if len(instr.results) != 1 || instr.results[0] != (port.Result{Outcome: port.OutcomeAccepted}) {
		t.Fatalf("EndOperation(%+v), want one accepted result", instr.results)
	}
	if len(instr.audits) != 0 {
		t.Fatalf("audits = %+v, want none: a query is not a fact", instr.audits)
	}
}

func TestFindEffectivePermissionsDeniedReadsNothing(t *testing.T) {
	h := newHarness(t)
	h.service.Authorize = func(context.Context, application.Operation) error { return port.ErrDenied }

	_, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject)

	if !errors.Is(err, port.ErrDenied) || slices.Contains(h.rec.Observed(), "memberships.LoadByUser") {
		t.Fatalf("FindEffectivePermissions() = %v after %v; want a denial before any read", err, h.rec.Observed())
	}
}

func TestEachFailureOfFindEffectivePermissionsCarriesThePrefix(t *testing.T) {
	for name, fault := range map[string]func(*harness){
		"memberships": func(h *harness) { h.memberships.err = errBoom },
		"account":     func(h *harness) { h.accounts.err = errBoom },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			fault(h)

			_, err := h.service.FindEffectivePermissions(withExecution(t, context.Background()), testSubject)

			if want := "application: find effective permissions user-sub-100: boom"; !errors.Is(err, errBoom) || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}
