package api_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func representationBody() map[string]any {
	return map[string]any{
		"representationId": "rep-1", "personId": "p-1", "powerType": "attorney_in_fact", "sourceType": "power_of_attorney",
		"sourceEvidenceId": "ev-1", "scopeCurrency": "BRL", "scopeMaxAmountMinor": 100000, "scopeActTypes": []string{"sign", "assign"},
		"validFrom": asOf, "validUntil": asOf + 1,
	}
}

func invitationBody() map[string]any {
	return map[string]any{"membershipId": "m-1", "inviteeEmail": "ana@acme.example", "role": "operator", "invitationExpiresAt": asOf}
}

func membershipCommands() []identityCommand {
	return []identityCommand{
		{
			name:       "registerRepresentation",
			permission: "identity:register-representation",
			method:     methodRegisterRepresentation,
			path:       "/identity/organizations/org-1/representations",
			badPath:    "/identity/organizations/org%201/representations",
			body:       representationBody(),
			reply: answer(&servicev1.RegisterRepresentationResponse{Result: &servicev1.RegisterRepresentationResponse_Registered{
				Registered: &servicev1.RegisteredRepresentation{RepresentationId: "rep-1"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.RegisterRepresentationResponse{Result: &servicev1.RegisterRepresentationResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.RegisterRepresentationRequest{
				RepresentationId:    "rep-1",
				OrganizationId:      "org-1",
				PersonId:            "p-1",
				PowerType:           servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT,
				SourceType:          servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY,
				SourceEvidenceId:    "ev-1",
				ScopeCurrency:       "BRL",
				ScopeMaxAmountMinor: 100000,
				ScopeActTypes:       []string{"sign", "assign"},
				ValidFrom:           asOf,
				ValidUntil:          asOf + 1,
				ProposedByUserId:    requester,
			},
			status: http.StatusCreated,
			want:   `{"representationId":"rep-1"}`,
		},
		{
			name:       "inviteUser",
			permission: "identity:invite-user",
			method:     methodInviteUser,
			path:       "/identity/organizations/org-1/memberships",
			badPath:    "/identity/organizations/org%201/memberships",
			body:       invitationBody(),
			reply: answer(&servicev1.InviteUserResponse{Result: &servicev1.InviteUserResponse_Invited{
				Invited: &servicev1.InvitedUser{MembershipId: "m-1"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.InviteUserResponse{Result: &servicev1.InviteUserResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.InviteUserRequest{
				MembershipId:        "m-1",
				OrganizationId:      "org-1",
				InviteeEmail:        "ana@acme.example",
				Role:                servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR,
				InvitedByUserId:     requester,
				InvitationExpiresAt: asOf,
			},
			status: http.StatusCreated,
			want:   `{"membershipId":"m-1"}`,
		},
		{
			name:       "revokeMembership",
			permission: "identity:revoke-membership",
			method:     methodRevokeMembership,
			path:       "/identity/memberships/m-1/revoke",
			badPath:    "/identity/memberships/m%201/revoke",
			body:       map[string]any{"reason": "left the company"},
			reply: answer(&servicev1.RevokeMembershipResponse{Result: &servicev1.RevokeMembershipResponse_Revoked{
				Revoked: &servicev1.RevokedMembership{MembershipId: "m-1"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.RevokeMembershipResponse{Result: &servicev1.RevokeMembershipResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.RevokeMembershipRequest{MembershipId: "m-1", Reason: "left the company"},
			status:  http.StatusOK,
			want:    `{"membershipId":"m-1"}`,
		},
	}
}

func TestTheBodyOfARegisteredRepresentationIsBoundByTheSchema(t *testing.T) {
	body := func(edits ...edit) string { return bodyWith(t, representationBody(), edits...) }
	representation := func(holds func(*servicev1.RegisterRepresentationRequest) bool) func(proto.Message) bool {
		return routedAs(holds)
	}
	powerType := func(p servicev1.PowerType) func(proto.Message) bool {
		return representation(func(r *servicev1.RegisterRepresentationRequest) bool { return r.GetPowerType() == p })
	}
	sourceType := func(s servicev1.SourceType) func(proto.Message) bool {
		return representation(func(r *servicev1.RegisterRepresentationRequest) bool { return r.GetSourceType() == s })
	}
	requireBounds(t, "registerRepresentation", []bound{
		{"a legal representative", body(with("powerType", "legal_representative")), powerType(servicev1.PowerType_POWER_TYPE_LEGAL_REPRESENTATIVE)},
		{"an attorney in fact", body(with("powerType", "attorney_in_fact")), powerType(servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT)},
		{"an operator", body(with("powerType", "operator")), powerType(servicev1.PowerType_POWER_TYPE_OPERATOR)},
		{"articles of association", body(with("sourceType", "articles_of_association")), sourceType(servicev1.SourceType_SOURCE_TYPE_ARTICLES_OF_ASSOCIATION)},
		{"a power of attorney", body(with("sourceType", "power_of_attorney")), sourceType(servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY)},
		{"a board resolution", body(with("sourceType", "board_resolution")), sourceType(servicev1.SourceType_SOURCE_TYPE_BOARD_RESOLUTION)},
		{"an internal delegation", body(with("sourceType", "internal_delegation")), sourceType(servicev1.SourceType_SOURCE_TYPE_INTERNAL_DELEGATION)},
		{"no scope", body(without("scopeCurrency"), without("scopeMaxAmountMinor"), without("scopeActTypes"), without("validUntil")),
			representation(func(r *servicev1.RegisterRepresentationRequest) bool {
				return r.GetScopeCurrency() == "" && r.GetScopeMaxAmountMinor() == 0 && len(r.GetScopeActTypes()) == 0 && r.GetValidUntil() == 0
			})},
		{"a currency without amount goes to the domain", body(without("scopeMaxAmountMinor")),
			representation(func(r *servicev1.RegisterRepresentationRequest) bool {
				return r.GetScopeCurrency() == "BRL" && r.GetScopeMaxAmountMinor() == 0
			})},
		{"an empty evidence goes to the domain", body(with("sourceEvidenceId", "")),
			representation(func(r *servicev1.RegisterRepresentationRequest) bool { return r.GetSourceEvidenceId() == "" })},
		{"an evidence of 256 non-ASCII characters", body(with("sourceEvidenceId", nonASCII256)), reaches},
		{"a currency of 8 printable characters", body(with("scopeCurrency", "BRL ~ 12")), reaches},
		{"32 act types of 64 printable characters", body(with("scopeActTypes", repeated(32, strings.Repeat("a", 64)))),
			representation(func(r *servicev1.RegisterRepresentationRequest) bool {
				return slices.Equal(r.GetScopeActTypes(), repeated(32, strings.Repeat("a", 64)))
			})},
		{"an empty act type goes to the domain", body(with("scopeActTypes", []string{""})),
			representation(func(r *servicev1.RegisterRepresentationRequest) bool {
				return slices.Equal(r.GetScopeActTypes(), []string{""})
			})},
		{"a validity starting at zero", body(with("validFrom", 0)),
			representation(func(r *servicev1.RegisterRepresentationRequest) bool { return r.GetValidFrom() == 0 })},
		{"no representation id", body(without("representationId")), nil},
		{"no person id", body(without("personId")), nil},
		{"an empty person id", body(with("personId", "")), nil},
		{"no power type", body(without("powerType")), nil},
		{"an unknown power type", body(with("powerType", "proxy")), nil},
		{"a power type out of case", body(with("powerType", "OPERATOR")), nil},
		{"no source type", body(without("sourceType")), nil},
		{"an unknown source type", body(with("sourceType", "email")), nil},
		{"no evidence", body(without("sourceEvidenceId")), nil},
		{"an evidence of 257 characters", body(with("sourceEvidenceId", nonASCII256+"é")), nil},
		{"an evidence with a control character", body(with("sourceEvidenceId", "ev\t1")), nil},
		{"a currency of 9 characters", body(with("scopeCurrency", "BRL ~ 123")), nil},
		{"a non-ASCII currency", body(with("scopeCurrency", "R$é")), nil},
		{"a fractional amount", body(with("scopeMaxAmountMinor", 1.5)), nil},
		{"33 act types", body(with("scopeActTypes", repeated(33, "sign"))), nil},
		{"an act type of 65 characters", body(with("scopeActTypes", []string{strings.Repeat("a", 65)})), nil},
		{"a non-ASCII act type", body(with("scopeActTypes", []string{"assinaturaé"})), nil},
		{"a null act type", body(with("scopeActTypes", []any{nil})), nil},
		{"no validity start", body(without("validFrom")), nil},
		{"a null validity start", body(with("validFrom", nil)), nil},
		{"a validity start as text", body(with("validFrom", "1767225600")), nil},
		{"the proposer in the body", body(with("proposedByUserId", "u-forged")), nil},
	})
}

func TestTheBodyOfAnInvitedUserIsBoundByTheSchema(t *testing.T) {
	body := func(edits ...edit) string { return bodyWith(t, invitationBody(), edits...) }
	invitation := func(holds func(*servicev1.InviteUserRequest) bool) func(proto.Message) bool { return routedAs(holds) }
	requireBounds(t, "inviteUser", []bound{
		{"an auditor", body(with("role", "auditor")),
			invitation(func(r *servicev1.InviteUserRequest) bool {
				return r.GetRole() == servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR
			})},
		{"a platform ops", body(with("role", "platform_ops")),
			invitation(func(r *servicev1.InviteUserRequest) bool {
				return r.GetRole() == servicev1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS
			})},
		{"an empty email goes to the domain", body(with("inviteeEmail", "")),
			invitation(func(r *servicev1.InviteUserRequest) bool { return r.GetInviteeEmail() == "" })},
		{"an email of 254 non-ASCII characters", body(with("inviteeEmail", strings.Repeat("é", 254))), reaches},
		{"an expiry in the past goes to the domain", body(with("invitationExpiresAt", 1)),
			invitation(func(r *servicev1.InviteUserRequest) bool { return r.GetInvitationExpiresAt() == 1 })},
		{"no membership id", body(without("membershipId")), nil},
		{"a membership id outside the pattern", body(with("membershipId", "m/1")), nil},
		{"no email", body(without("inviteeEmail")), nil},
		{"an email of 255 characters", body(with("inviteeEmail", strings.Repeat("é", 255))), nil},
		{"an email with a control character", body(with("inviteeEmail", "ana@acme\r")), nil},
		{"no role", body(without("role")), nil},
		{"an unknown role", body(with("role", "admin")), nil},
		{"a reserved role", body(with("role", "platform_risk")), nil},
		{"no expiry", body(without("invitationExpiresAt")), nil},
		{"a null expiry", body(with("invitationExpiresAt", nil)), nil},
		{"an expiry as text", body(with("invitationExpiresAt", "tomorrow")), nil},
		{"the inviter in the body", body(with("invitedByUserId", "u-forged")), nil},
	})
}

func TestTheBodyOfARevokedMembershipIsBoundByTheSchema(t *testing.T) {
	requireBounds(t, "revokeMembership", reasonBounds(t, map[string]any{"reason": "left the company"}))
}
