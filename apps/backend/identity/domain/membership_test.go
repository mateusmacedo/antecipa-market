package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	membershipID   = domain.MembershipID("mem-100")
	inviteeEmail   = "convidada@sintetica.test"
	inviterID      = "user-sub-1"
	expiresAt      = at + 7*86_400_000_000_000
	grantedAt      = at + 86_400_000_000_000
	revokedAt      = grantedAt + 86_400_000_000_000
	revocationText = "saiu da empresa"
)

func inviteCommand() domain.InviteUser {
	return domain.InviteUser{
		OrganizationID:      organizationID,
		OrganizationStatus:  domain.OrganizationActive,
		OrganizationKinds:   domain.KindsOf(domain.KindClient),
		InviteeEmail:        inviteeEmail,
		Role:                domain.RoleOperator,
		InvitedByUserID:     inviterID,
		InvitationExpiresAt: expiresAt,
		At:                  at,
	}
}

func invitedMembership() domain.MembershipSnapshot {
	return domain.MembershipSnapshot{
		ID:                  membershipID,
		OrganizationID:      organizationID,
		InviteeEmail:        inviteeEmail,
		Role:                domain.RoleOperator,
		Status:              domain.MembershipStatusInvited,
		InvitedByUserID:     inviterID,
		InvitedAt:           at,
		InvitationExpiresAt: expiresAt,
	}
}

func grantCommand() domain.GrantMembership {
	return domain.GrantMembership{
		UserID:            subject,
		UserStatus:        domain.UserAccountActive,
		PersonID:          personID,
		ChannelConfirmed:  true,
		VerificationLevel: domain.VerificationPersonalDataChecked,
		At:                grantedAt,
	}
}

func activeMembership() domain.MembershipSnapshot {
	s := invitedMembership()
	s.Status = domain.MembershipStatusActive
	s.UserID = subject
	s.PersonID = personID
	s.VerificationLevel = domain.VerificationPersonalDataChecked
	s.GrantedAt = grantedAt
	return s
}

func revokedMembership() domain.MembershipSnapshot {
	s := activeMembership()
	s.Status = domain.MembershipStatusRevoked
	s.RevokedAt = revokedAt
	s.RevocationReason = revocationText
	return s
}

func TestInviteUserInvitesWithTheCommandFieldsAndNoEvent(t *testing.T) {
	m := domain.NewMembership(membershipID)

	acc, rej := m.Invite(inviteCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.InviteUserResponse{MembershipID: membershipID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if events := acc.Events(); len(events) != 0 {
		t.Fatalf("Events() = %v, want none", events)
	}
	if got := m.Snapshot(); !got.Equal(invitedMembership()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, invitedMembership())
	}
}

func TestInviteUserAcceptsEachClientRoleInAClientOrganization(t *testing.T) {
	for _, tc := range []struct {
		role  domain.Role
		kinds domain.OrganizationKinds
	}{
		{domain.RoleOperator, domain.KindsOf(domain.KindClient)},
		{domain.RoleAuditor, domain.KindsOf(domain.KindClient)},
		{domain.RoleAuditor, domain.KindsOf(domain.KindFunder, domain.KindClient)},
	} {
		cmd := inviteCommand()
		cmd.Role, cmd.OrganizationKinds = tc.role, tc.kinds

		_, rej := domain.NewMembership(membershipID).Invite(cmd)

		tb.RequireAccepted(t, rej)
	}
}

func TestInviteUserRejectsEachPrecondition(t *testing.T) {
	organization := kernel.Detail{Key: "organization", Value: string(organizationID)}
	for _, tc := range []struct {
		name    string
		mutate  func(*domain.InviteUser)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"invitee email empty", func(c *domain.InviteUser) { c.InviteeEmail = "" }, domain.CodeMembershipInviteeEmailEmpty, nil},
		{"invitee email of blanks", func(c *domain.InviteUser) { c.InviteeEmail = "  " }, domain.CodeMembershipInviteeEmailEmpty, nil},
		{"expiry equal to at", func(c *domain.InviteUser) { c.InvitationExpiresAt = c.At }, domain.CodeMembershipInvitationExpiryInvalid, nil},
		{"expiry before at", func(c *domain.InviteUser) { c.InvitationExpiresAt = c.At - 1 }, domain.CodeMembershipInvitationExpiryInvalid, nil},
		{"organization not found", func(c *domain.InviteUser) { c.OrganizationStatus = domain.OrganizationNew }, domain.CodeMembershipOrganizationNotActive, []kernel.Detail{organization}},
		{"organization suspended", func(c *domain.InviteUser) { c.OrganizationStatus = domain.OrganizationSuspended }, domain.CodeMembershipOrganizationNotActive, []kernel.Detail{organization}},
		{"organization offboarded", func(c *domain.InviteUser) { c.OrganizationStatus = domain.OrganizationOffboarded }, domain.CodeMembershipOrganizationNotActive, []kernel.Detail{organization}},
		{"operator in a funder", func(c *domain.InviteUser) { c.OrganizationKinds = domain.KindsOf(domain.KindFunder) }, domain.CodeMembershipRoleKindMismatch,
			[]kernel.Detail{{Key: "role", Value: "operator"}, organization}},
		{"auditor in a partner and platform", func(c *domain.InviteUser) {
			c.Role, c.OrganizationKinds = domain.RoleAuditor, domain.KindsOf(domain.KindPartner, domain.KindPlatform)
		}, domain.CodeMembershipRoleKindMismatch, []kernel.Detail{{Key: "role", Value: "auditor"}, organization}},
		{"a role outside the catalogue", func(c *domain.InviteUser) { c.Role = "approver" }, domain.CodeMembershipRoleKindMismatch,
			[]kernel.Detail{{Key: "role", Value: "approver"}, organization}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := inviteCommand()
			tc.mutate(&cmd)
			m := domain.NewMembership(membershipID)
			before := m.Snapshot()

			acc, rej := m.Invite(cmd)

			tb.RequireRejected(t, acc, rej, tc.code)
			if got := rej.Details(); !slices.Equal(got, tc.details) {
				t.Fatalf("Details() = %v, want %v", got, tc.details)
			}
			if after := m.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestInviteUserReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.InviteUser{OrganizationID: organizationID, Role: domain.RoleOperator, InvitationExpiresAt: at, At: at}
	want := []kernel.Code{
		domain.CodeMembershipInviteeEmailEmpty,
		domain.CodeMembershipInvitationExpiryInvalid,
		domain.CodeMembershipOrganizationNotActive,
		domain.CodeMembershipRoleKindMismatch,
	}
	fixes := []func(*domain.InviteUser){
		func(c *domain.InviteUser) { c.InviteeEmail = inviteeEmail },
		func(c *domain.InviteUser) { c.InvitationExpiresAt = expiresAt },
		func(c *domain.InviteUser) { c.OrganizationStatus = domain.OrganizationActive },
		func(c *domain.InviteUser) { c.OrganizationKinds = domain.KindsOf(domain.KindClient) },
	}
	for i, code := range want {
		acc, rej := domain.NewMembership(membershipID).Invite(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
	if _, rej := domain.NewMembership(membershipID).Invite(cmd); rej != nil {
		t.Fatalf("Invite() after every fix rejected with %s", rej.Code())
	}
}

func TestGrantMembershipActivatesTheInvitationAndEmitsTheGrant(t *testing.T) {
	m := domain.FromMembershipSnapshot(invitedMembership())

	acc, rej := m.Grant(grantCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.GrantMembershipResponse{MembershipID: membershipID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	tb.RequireSameEvents(t, acc.Events(), []kernel.DomainEvent{domain.MembershipGranted{
		MembershipID:      membershipID,
		OrganizationID:    organizationID,
		UserID:            subject,
		Role:              domain.RoleOperator,
		VerificationLevel: domain.VerificationPersonalDataChecked,
		At:                grantedAt,
	}})
	if got := m.Snapshot(); !got.Equal(activeMembership()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, activeMembership())
	}
}

func TestGrantMembershipAcceptsTheMinimumLevelOfEachRoleOrAbove(t *testing.T) {
	for _, tc := range []struct {
		role  domain.Role
		level domain.VerificationLevel
	}{
		{domain.RoleOperator, domain.VerificationPersonalDataChecked},
		{domain.RoleAuditor, domain.VerificationChannelOwnership},
		{domain.RoleAuditor, domain.VerificationPersonalDataChecked},
	} {
		invited := invitedMembership()
		invited.Role = tc.role
		cmd := grantCommand()
		cmd.VerificationLevel = tc.level

		_, rej := domain.FromMembershipSnapshot(invited).Grant(cmd)

		tb.RequireAccepted(t, rej)
	}
}

func TestGrantMembershipRejectsEachPrecondition(t *testing.T) {
	invitedAuditor := invitedMembership()
	invitedAuditor.Role = domain.RoleAuditor
	for _, tc := range []struct {
		name    string
		held    domain.MembershipSnapshot
		mutate  func(*domain.GrantMembership)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"membership already active", activeMembership(), func(*domain.GrantMembership) {}, domain.CodeMembershipNotInvited, nil},
		{"membership revoked", revokedMembership(), func(*domain.GrantMembership) {}, domain.CodeMembershipNotInvited, nil},
		{"invitation expiring at the instant", invitedMembership(), func(c *domain.GrantMembership) { c.At = expiresAt }, domain.CodeMembershipInvitationExpired, nil},
		{"invitation expired", invitedMembership(), func(c *domain.GrantMembership) { c.At = expiresAt + 1 }, domain.CodeMembershipInvitationExpired, nil},
		{"channel not confirmed", invitedMembership(), func(c *domain.GrantMembership) { c.ChannelConfirmed = false }, domain.CodeMembershipChannelNotConfirmed, nil},
		{"operator with channel ownership only", invitedMembership(), func(c *domain.GrantMembership) {
			c.VerificationLevel = domain.VerificationChannelOwnership
		}, domain.CodeMembershipVerificationInsufficient, []kernel.Detail{{Key: "role", Value: "operator"}}},
		{"operator with no level", invitedMembership(), func(c *domain.GrantMembership) { c.VerificationLevel = "" },
			domain.CodeMembershipVerificationInsufficient, []kernel.Detail{{Key: "role", Value: "operator"}}},
		{"auditor with a level outside the catalogue", invitedAuditor, func(c *domain.GrantMembership) { c.VerificationLevel = "document_and_liveness" },
			domain.CodeMembershipVerificationInsufficient, []kernel.Detail{{Key: "role", Value: "auditor"}}},
		{"user suspended", invitedMembership(), func(c *domain.GrantMembership) { c.UserStatus = domain.UserAccountSuspended },
			domain.CodeMembershipUserNotActive, []kernel.Detail{{Key: "user", Value: string(subject)}}},
		{"user not found", invitedMembership(), func(c *domain.GrantMembership) { c.UserStatus = domain.UserAccountNew },
			domain.CodeMembershipUserNotActive, []kernel.Detail{{Key: "user", Value: string(subject)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := grantCommand()
			tc.mutate(&cmd)
			m := domain.FromMembershipSnapshot(tc.held)
			before := m.Snapshot()

			acc, rej := m.Grant(cmd)

			tb.RequireRejected(t, acc, rej, tc.code)
			if got := rej.Details(); !slices.Equal(got, tc.details) {
				t.Fatalf("Details() = %v, want %v", got, tc.details)
			}
			if after := m.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestGrantMembershipReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.GrantMembership{UserID: subject, VerificationLevel: domain.VerificationChannelOwnership, At: expiresAt}
	want := []kernel.Code{
		domain.CodeMembershipInvitationExpired,
		domain.CodeMembershipChannelNotConfirmed,
		domain.CodeMembershipVerificationInsufficient,
		domain.CodeMembershipUserNotActive,
	}
	fixes := []func(*domain.GrantMembership){
		func(c *domain.GrantMembership) { c.At = grantedAt },
		func(c *domain.GrantMembership) { c.ChannelConfirmed = true },
		func(c *domain.GrantMembership) { c.VerificationLevel = domain.VerificationPersonalDataChecked },
		func(c *domain.GrantMembership) { c.UserStatus = domain.UserAccountActive },
	}
	acc, rej := domain.FromMembershipSnapshot(activeMembership()).Grant(cmd)
	tb.RequireRejected(t, acc, rej, domain.CodeMembershipNotInvited)
	for i, code := range want {
		acc, rej := domain.FromMembershipSnapshot(invitedMembership()).Grant(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
	if _, rej := domain.FromMembershipSnapshot(invitedMembership()).Grant(cmd); rej != nil {
		t.Fatalf("Grant() after every fix rejected with %s", rej.Code())
	}
}

func TestRevokeMembershipRevokesTheActiveMembershipAndEmitsTheRevocation(t *testing.T) {
	m := domain.FromMembershipSnapshot(activeMembership())

	acc, rej := m.Revoke(domain.RevokeMembership{Reason: revocationText, At: revokedAt})

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RevokeMembershipResponse{MembershipID: membershipID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	tb.RequireSameEvents(t, acc.Events(), []kernel.DomainEvent{domain.MembershipRevoked{
		MembershipID:   membershipID,
		OrganizationID: organizationID,
		UserID:         subject,
		Role:           domain.RoleOperator,
		Reason:         revocationText,
		At:             revokedAt,
	}})
	if got := m.Snapshot(); !got.Equal(revokedMembership()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, revokedMembership())
	}
}

func TestRevokeMembershipRejectsAMembershipThatIsNotActive(t *testing.T) {
	for name, held := range map[string]domain.MembershipSnapshot{
		"invited":   invitedMembership(),
		"revoked":   revokedMembership(),
		"never one": domain.NewMembership(membershipID).Snapshot(),
	} {
		t.Run(name, func(t *testing.T) {
			m := domain.FromMembershipSnapshot(held)
			before := m.Snapshot()

			acc, rej := m.Revoke(domain.RevokeMembership{Reason: revocationText, At: revokedAt})

			tb.RequireRejected(t, acc, rej, domain.CodeMembershipNotActive)
			if after := m.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestMembershipDeterminism(t *testing.T) {
	m1 := domain.FromMembershipSnapshot(invitedMembership())
	acc1, _ := m1.Grant(grantCommand())

	m2 := domain.FromMembershipSnapshot(invitedMembership())
	acc2, _ := m2.Grant(grantCommand())

	if !m1.Snapshot().Equal(m2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestFromMembershipSnapshotRoundTrips(t *testing.T) {
	for _, before := range []domain.MembershipSnapshot{invitedMembership(), activeMembership(), revokedMembership()} {
		if got := domain.FromMembershipSnapshot(before).Snapshot(); !got.Equal(before) {
			t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
		}
	}
}

func TestMembershipEqualTellsEveryFieldApart(t *testing.T) {
	base := revokedMembership()
	for name, mutate := range map[string]func(*domain.MembershipSnapshot){
		"id":                    func(s *domain.MembershipSnapshot) { s.ID = "mem-other" },
		"organization":          func(s *domain.MembershipSnapshot) { s.OrganizationID = "org-other" },
		"invitee email":         func(s *domain.MembershipSnapshot) { s.InviteeEmail = "other@sintetica.test" },
		"role":                  func(s *domain.MembershipSnapshot) { s.Role = domain.RoleAuditor },
		"status":                func(s *domain.MembershipSnapshot) { s.Status = domain.MembershipStatusActive },
		"inviter":               func(s *domain.MembershipSnapshot) { s.InvitedByUserID = "user-other" },
		"invited at":            func(s *domain.MembershipSnapshot) { s.InvitedAt++ },
		"invitation expires at": func(s *domain.MembershipSnapshot) { s.InvitationExpiresAt++ },
		"user":                  func(s *domain.MembershipSnapshot) { s.UserID = "user-sub-other" },
		"person":                func(s *domain.MembershipSnapshot) { s.PersonID = "per-other" },
		"verification level":    func(s *domain.MembershipSnapshot) { s.VerificationLevel = domain.VerificationChannelOwnership },
		"granted at":            func(s *domain.MembershipSnapshot) { s.GrantedAt++ },
		"revoked at":            func(s *domain.MembershipSnapshot) { s.RevokedAt++ },
		"revocation reason":     func(s *domain.MembershipSnapshot) { s.RevocationReason = "other" },
	} {
		changed := revokedMembership()
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func TestEachRoleAndLevelKeepsItsValue(t *testing.T) {
	values := map[string]string{
		string(domain.RoleOperator):                    "operator",
		string(domain.RoleAuditor):                     "auditor",
		string(domain.VerificationChannelOwnership):    "channel_ownership",
		string(domain.VerificationPersonalDataChecked): "personal_data_checked",
	}
	for got, want := range values {
		if got != want {
			t.Errorf("value = %q, want %q", got, want)
		}
	}
	statuses := []domain.MembershipStatus{domain.MembershipStatusNew, domain.MembershipStatusInvited, domain.MembershipStatusActive, domain.MembershipStatusRevoked}
	if want := []domain.MembershipStatus{0, 1, 2, 3}; !slices.Equal(statuses, want) {
		t.Fatalf("statuses = %v, want %v: the snapshot stores them as integers", statuses, want)
	}
}
