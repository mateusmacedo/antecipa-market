package provider_test

import (
	"strconv"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

var membershipStatusNames = map[domain.MembershipStatus]string{
	domain.MembershipStatusNew:     "new",
	domain.MembershipStatusInvited: "invited",
	domain.MembershipStatusActive:  "active",
	domain.MembershipStatusRevoked: "revoked",
}

func TestMembershipsMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "membership.golden")
	if f.Identity.Context != "identity" || f.Identity.Aggregate != "membership" || len(f.Cases) != 13 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runMembership)
}

// The invitee email is personal data: it lives in the state, never in an
// event of the membership.
func TestNoProjectedMembershipEventHoldsTheInviteeEmail(t *testing.T) {
	for _, c := range projectionFixture(t, "membership.golden").Cases {
		email := c.StateBefore["invitee_email"]
		if email == "" {
			email = c.Command["invitee_email"]
		}
		for _, event := range c.Expected.Events {
			for field, value := range event.Fields {
				if email != "" && value == email {
					t.Fatalf("%s: event %s field %s holds the invitee email", c.Name, event.Name, field)
				}
			}
		}
	}
}

func runMembership(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	at := kernel.Instant(integer(t, c.Command["at"]))
	switch c.Command["upr"] {
	case "invite":
		s := membershipSubject(func(m *domain.Membership) (kernel.Accepted[domain.InviteUserResponse], *kernel.Rejection) {
			return m.Invite(domain.InviteUser{
				OrganizationID:      domain.OrganizationID(c.Command["organization"]),
				OrganizationStatus:  statusFrom(c.Command["organization_status"]),
				OrganizationKinds:   kindsFrom(c.Command["organization_kinds"]),
				InviteeEmail:        c.Command["invitee_email"],
				Role:                domain.Role(c.Command["role"]),
				InvitedByUserID:     c.Command["invited_by"],
				InvitationExpiresAt: kernel.Instant(integer(t, c.Command["invitation_expires_at"])),
				At:                  at,
			})
		}, func(r domain.InviteUserResponse) domainkit.Fields {
			return domainkit.Fields{"membership": string(r.MembershipID)}
		})
		return domainkit.Run(membershipFromState(t, c.StateBefore), s), domainkit.ReadTwice(membershipFromState(t, c.StateBefore), s)
	case "grant":
		s := membershipSubject(func(m *domain.Membership) (kernel.Accepted[domain.GrantMembershipResponse], *kernel.Rejection) {
			return m.Grant(domain.GrantMembership{
				UserID:            domain.UserID(c.Command["user"]),
				UserStatus:        userAccountStatusFrom(c.Command["user_status"]),
				PersonID:          domain.PersonID(c.Command["person"]),
				ChannelConfirmed:  boolean(t, c.Command["channel_confirmed"]),
				VerificationLevel: domain.VerificationLevel(c.Command["verification_level"]),
				At:                at,
			})
		}, func(r domain.GrantMembershipResponse) domainkit.Fields {
			return domainkit.Fields{"membership": string(r.MembershipID)}
		})
		return domainkit.Run(membershipFromState(t, c.StateBefore), s), domainkit.ReadTwice(membershipFromState(t, c.StateBefore), s)
	case "revoke":
		s := membershipSubject(func(m *domain.Membership) (kernel.Accepted[domain.RevokeMembershipResponse], *kernel.Rejection) {
			return m.Revoke(domain.RevokeMembership{Reason: c.Command["reason"], At: at})
		}, func(r domain.RevokeMembershipResponse) domainkit.Fields {
			return domainkit.Fields{"membership": string(r.MembershipID)}
		})
		return domainkit.Run(membershipFromState(t, c.StateBefore), s), domainkit.ReadTwice(membershipFromState(t, c.StateBefore), s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func membershipState(m *domain.Membership) domainkit.Fields {
	s := m.Snapshot()
	return domainkit.Fields{
		"membership":            string(s.ID),
		"organization":          string(s.OrganizationID),
		"invitee_email":         s.InviteeEmail,
		"role":                  string(s.Role),
		"status":                membershipStatusNames[s.Status],
		"invited_by":            s.InvitedByUserID,
		"invited_at":            strconv.FormatInt(int64(s.InvitedAt), 10),
		"invitation_expires_at": strconv.FormatInt(int64(s.InvitationExpiresAt), 10),
		"user":                  string(s.UserID),
		"person":                string(s.PersonID),
		"verification_level":    string(s.VerificationLevel),
		"granted_at":            strconv.FormatInt(int64(s.GrantedAt), 10),
		"revoked_at":            strconv.FormatInt(int64(s.RevokedAt), 10),
		"revocation_reason":     s.RevocationReason,
	}
}

func membershipFromState(t *testing.T, f domainkit.Fields) *domain.Membership {
	t.Helper()
	s := domain.MembershipSnapshot{
		ID:                  domain.MembershipID(f["membership"]),
		OrganizationID:      domain.OrganizationID(f["organization"]),
		InviteeEmail:        f["invitee_email"],
		Role:                domain.Role(f["role"]),
		InvitedByUserID:     f["invited_by"],
		InvitedAt:           kernel.Instant(integer(t, f["invited_at"])),
		InvitationExpiresAt: kernel.Instant(integer(t, f["invitation_expires_at"])),
		UserID:              domain.UserID(f["user"]),
		PersonID:            domain.PersonID(f["person"]),
		VerificationLevel:   domain.VerificationLevel(f["verification_level"]),
		GrantedAt:           kernel.Instant(integer(t, f["granted_at"])),
		RevokedAt:           kernel.Instant(integer(t, f["revoked_at"])),
		RevocationReason:    f["revocation_reason"],
	}
	for status, name := range membershipStatusNames {
		if name == f["status"] {
			s.Status = status
		}
	}
	return domain.FromMembershipSnapshot(s)
}

func membershipEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch event := e.(type) {
	case domain.MembershipGranted:
		return event.EventName(), domainkit.Fields{
			"membership":         string(event.MembershipID),
			"organization":       string(event.OrganizationID),
			"user":               string(event.UserID),
			"role":               string(event.Role),
			"verification_level": string(event.VerificationLevel),
			"at":                 strconv.FormatInt(int64(event.At), 10),
		}
	case domain.MembershipRevoked:
		return event.EventName(), domainkit.Fields{
			"membership":   string(event.MembershipID),
			"organization": string(event.OrganizationID),
			"user":         string(event.UserID),
			"role":         string(event.Role),
			"reason":       event.Reason,
			"at":           strconv.FormatInt(int64(event.At), 10),
		}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func membershipSubject[R any](decide func(*domain.Membership) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.Membership, R] {
	return domainkit.Subject[*domain.Membership, R]{
		Decide:   decide,
		Response: response,
		Event:    membershipEvent,
		Snapshot: membershipState,
		Clone: func(m *domain.Membership) *domain.Membership {
			return domain.FromMembershipSnapshot(m.Snapshot())
		},
	}
}
