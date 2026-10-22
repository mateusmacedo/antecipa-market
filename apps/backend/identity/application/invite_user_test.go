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

const (
	testMembershipID = domain.MembershipID("mem-100")
	testInviteeEmail = "convidada@sintetica.test"
	testInviterID    = "user-sub-1"
)

var testExpiresAt = testOccurred + 7*86_400_000_000_000

func invite() application.InviteUser {
	return application.InviteUser{
		MembershipID:        testMembershipID,
		OrganizationID:      testOrganizationID,
		InviteeEmail:        testInviteeEmail,
		Role:                domain.RoleOperator,
		InvitedByUserID:     testInviterID,
		InvitationExpiresAt: testExpiresAt,
	}
}

func invitedMembership(id domain.MembershipID, role domain.Role) domain.MembershipSnapshot {
	return domain.MembershipSnapshot{
		ID:                  id,
		OrganizationID:      testOrganizationID,
		InviteeEmail:        testInviteeEmail,
		Role:                role,
		Status:              domain.MembershipStatusInvited,
		InvitedByUserID:     testInviterID,
		InvitedAt:           testOccurred,
		InvitationExpiresAt: testExpiresAt,
	}
}

func activeMembership(id domain.MembershipID, role domain.Role, user domain.UserID) domain.MembershipSnapshot {
	s := invitedMembership(id, role)
	s.Status = domain.MembershipStatusActive
	s.UserID = user
	s.PersonID = testPersonID
	s.VerificationLevel = domain.VerificationPersonalDataChecked
	s.GrantedAt = testOccurred
	return s
}

func withMembership(t *testing.T, h *harness, membership domain.MembershipSnapshot) {
	t.Helper()
	seed(t, h, withExecution(t, context.Background()), membershipTable, membership.ID, membership)
	h.memberships.track(membership.ID)
	h.rec.Reset()
}

func withOrganizationOfKinds(t *testing.T, h *harness, status domain.OrganizationStatus, kinds ...domain.OrganizationKind) {
	t.Helper()
	organization := registeredSnapshot(testOrganizationID, testTenant, testJurisdiction)
	organization.Status = status
	organization.Kinds = domain.KindsOf(kinds...)
	h.seedOrganization(t, withExecution(t, context.Background()), organization)
	h.rec.Reset()
}

func TestInviteUserWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)

	if _, err := h.service.InviteUser(withExecution(t, context.Background()), invite()); err != nil {
		t.Fatalf("InviteUser() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"organizations.Load",
		"memberships.Load",
		"memberships.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2, spec note †)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestInviteUserInvitesWithoutAnEvent(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.InviteUser(ctx, invite())

	if err != nil || outcome.Response() != (domain.InviteUserResponse{MembershipID: testMembershipID}) {
		t.Fatalf("InviteUser() = %+v, %v; want accepted", outcome, err)
	}
	stored, version, err := membershipTable.Reader(h.store).Load(ctx, testMembershipID)
	if err != nil || version != 1 || !stored.Equal(invitedMembership(testMembershipID, domain.RoleOperator)) {
		t.Fatalf("Load() = %+v v%d, %v; want the invited membership at v1", stored, version, err)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none", got)
	}
}

func TestInviteUserInvitesPlatformOpsIntoAPlatformOrganization(t *testing.T) {
	h := newHarness(t)
	withOrganizationOfKinds(t, h, domain.OrganizationActive, domain.KindPlatform)
	ctx := withExecution(t, context.Background())
	cmd := invite()
	cmd.Role = domain.RolePlatformOps

	outcome, err := h.service.InviteUser(ctx, cmd)

	if err != nil || outcome.Response() != (domain.InviteUserResponse{MembershipID: testMembershipID}) {
		t.Fatalf("InviteUser() = %+v, %v; want accepted", outcome, err)
	}
	stored, version, err := membershipTable.Reader(h.store).Load(ctx, testMembershipID)
	if err != nil || version != 1 || !stored.Equal(invitedMembership(testMembershipID, domain.RolePlatformOps)) {
		t.Fatalf("Load() = %+v v%d, %v; want the invited platform ops at v1", stored, version, err)
	}
}

func TestInviteUserRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *harness)
		mutate  func(*application.InviteUser)
		code    kernel.Code
	}{
		{"invitee email empty", activeClient, func(c *application.InviteUser) { c.InviteeEmail = "" }, domain.CodeMembershipInviteeEmailEmpty},
		{"expiry before the request", activeClient, func(c *application.InviteUser) { c.InvitationExpiresAt = testOccurred - 1 }, domain.CodeMembershipInvitationExpiryInvalid},
		{"organization not found", func(*testing.T, *harness) {}, func(*application.InviteUser) {}, domain.CodeMembershipOrganizationNotActive},
		{"organization suspended", func(t *testing.T, h *harness) { withOrganization(t, h, domain.OrganizationSuspended) },
			func(*application.InviteUser) {}, domain.CodeMembershipOrganizationNotActive},
		{"operator in a funder", func(t *testing.T, h *harness) {
			withOrganizationOfKinds(t, h, domain.OrganizationActive, domain.KindFunder)
		},
			func(*application.InviteUser) {}, domain.CodeMembershipRoleKindMismatch},
		{"platform ops in a client", activeClient, func(c *application.InviteUser) { c.Role = domain.RolePlatformOps },
			domain.CodeMembershipRoleKindMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.prepare(t, h)
			cmd := invite()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.InviteUser(ctx, cmd)

			if err != nil {
				t.Fatalf("InviteUser() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := membershipTable.Reader(h.store).Load(ctx, testMembershipID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func activeClient(t *testing.T, h *harness) {
	t.Helper()
	withOrganization(t, h, domain.OrganizationActive)
}

func TestInviteUserIntoAnOrganizationOfAnotherTenantIsNotActive(t *testing.T) {
	h := newHarness(t)
	h.seedOrganization(t, withTenant(t, context.Background(), "globex"), registeredSnapshot(testOrganizationID, "globex", testJurisdiction))

	outcome, err := h.service.InviteUser(withExecution(t, context.Background()), invite())

	if rej, refused := outcome.Rejection(); err != nil || !refused || rej.Code() != domain.CodeMembershipOrganizationNotActive {
		t.Fatalf("InviteUser() = %+v, %v; want %s (IDN-13)", outcome, err, domain.CodeMembershipOrganizationNotActive)
	}
}

func TestEachFailureOfInviteUserCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
	}{
		{"organization load", []option{withLoadError(errBoom)}},
		{"load", []option{withTableFaults("memberships", serviceskit.Faults{Load: errBoom})}},
		{"save", []option{withTableFaults("memberships", serviceskit.Faults{Save: errBoom})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withOrganization(t, h, domain.OrganizationActive)

			_, err := h.service.InviteUser(withExecution(t, context.Background()), invite())

			if want := "application: invite user mem-100: boom"; !errors.Is(err, errBoom) || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

func TestTheSameKeyWithAnotherInvitationIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.InviteUser){
		"membership":    func(c *application.InviteUser) { c.MembershipID = "mem-other" },
		"organization":  func(c *application.InviteUser) { c.OrganizationID = "org-other" },
		"invitee email": func(c *application.InviteUser) { c.InviteeEmail = "outra@sintetica.test" },
		"role":          func(c *application.InviteUser) { c.Role = domain.RoleAuditor },
		"inviter":       func(c *application.InviteUser) { c.InvitedByUserID = "user-sub-2" },
		"expiry":        func(c *application.InviteUser) { c.InvitationExpiresAt++ },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withOrganization(t, h, domain.OrganizationActive)
			if _, err := h.service.InviteUser(withKey(t, context.Background(), "k-reused"), invite()); err != nil {
				t.Fatalf("first InviteUser() = %v, want nil", err)
			}
			second := invite()
			mutate(&second)

			_, err := h.service.InviteUser(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second InviteUser() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
