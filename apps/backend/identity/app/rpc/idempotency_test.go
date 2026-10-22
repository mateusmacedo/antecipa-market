package rpc_test

import (
	"slices"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

var queries = []string{
	"FindOrganization", "FindOrganizationByTaxId", "FindEstablishmentsByOrganization",
	"FindPerson", "FindRepresentation", "FindRepresentationsByOrganization",
	"FindMembershipsByUser", "FindMembershipsByOrganization", "FindEffectivePermissions",
	"FindBankAccountChange", "FindBankAccountChangesByOrganization",
}

func TestEveryMethodThatIsNotAQueryIsACommand(t *testing.T) {
	commands := rpc.Commands()
	for _, command := range commands {
		if !slices.Contains(rpc.Methods(), command) {
			t.Errorf("command %s is not a method of %s", command, rpc.ServiceName)
		}
	}
	for _, method := range rpc.Methods() {
		query := slices.Contains(queries, method)
		if query == slices.Contains(commands, method) {
			t.Errorf("method %s: query %v, command %v; every method is exactly one of the two", method, query, !query)
		}
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeTheHandler(t *testing.T) {
	h := newHarness(t, nil)

	var resp servicev1.RegisterOrganizationResponse
	err := h.invoke(withoutKey(t), "RegisterOrganization", registerRequest("org-1"), &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
		t.Fatalf("RegisterOrganization() = %v (reason %q), want InvalidArgument with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonMissingIdempotencyKey)
	}
}

func TestACommandWithAMalformedKeyIsRefused(t *testing.T) {
	h := newHarness(t, nil)

	var resp servicev1.RegisterOrganizationResponse
	err := h.invoke(withKey(t, "not a key"), "RegisterOrganization", registerRequest("org-1"), &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonInvalidIdempotencyKey {
		t.Fatalf("RegisterOrganization() = %v (reason %q), want InvalidArgument with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonInvalidIdempotencyKey)
	}
}

func TestARepeatedCommandAnswersTheSameResponseMarkedAsReplay(t *testing.T) {
	h := newHarness(t, nil)
	req := registerRequest("org-1")

	var first, again servicev1.RegisterOrganizationResponse
	var firstHeader, againHeader metadata.MD
	if err := h.invoke(withKey(t, "k-replay"), "RegisterOrganization", req, &first, grpc.Header(&firstHeader)); err != nil {
		t.Fatalf("first RegisterOrganization() = %v", err)
	}
	if err := h.invoke(withKey(t, "k-replay"), "RegisterOrganization", req, &again, grpc.Header(&againHeader)); err != nil {
		t.Fatalf("repeated RegisterOrganization() = %v, want the stored response", err)
	}

	if again.GetRegistered().GetOrganizationId() != first.GetRegistered().GetOrganizationId() {
		t.Fatalf("replay = %v, want %v", &again, &first)
	}
	if got := firstHeader.Get(kernelgrpc.ReplayedHeader); len(got) != 0 {
		t.Fatalf("first response header %s = %v, want none", kernelgrpc.ReplayedHeader, got)
	}
	if got := againHeader.Get(kernelgrpc.ReplayedHeader); !slices.Equal(got, []string{"true"}) {
		t.Fatalf("replay header %s = %v, want [true]", kernelgrpc.ReplayedHeader, got)
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries, want 1: one effect and one event per key", n)
	}
}

func TestTheSameKeyWithAnotherRequestIsReused(t *testing.T) {
	h := newHarness(t, nil)
	var resp servicev1.RegisterOrganizationResponse
	if err := h.invoke(withKey(t, "k-reused"), "RegisterOrganization", registerRequest("org-1"), &resp); err != nil {
		t.Fatalf("first RegisterOrganization() = %v", err)
	}
	other := registerRequest("org-1")
	other.LegalName = "Outra Razao Sintetica Ltda"

	err := h.invoke(withKey(t, "k-reused"), "RegisterOrganization", other, &resp)

	if status.Code(err) != codes.FailedPrecondition || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonReusedIdempotencyKey {
		t.Fatalf("RegisterOrganization(other legal name) = %v (reason %q), want FailedPrecondition with %s", err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonReusedIdempotencyKey)
	}
}

type commandCall struct {
	method    string
	request   func() proto.Message
	differing func() proto.Message
	response  func() proto.Message
	id        func(proto.Message) string
}

var relationCommands = []commandCall{
	{
		method:    "AddEstablishment",
		request:   func() proto.Message { return addRequest("est-1", "org-1") },
		differing: func() proto.Message { r := addRequest("est-1", "org-1"); r.Receiver = false; return r },
		response:  func() proto.Message { return &servicev1.AddEstablishmentResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.AddEstablishmentResponse).GetAdded().GetEstablishmentId()
		},
	},
	{
		method:    "RegisterPerson",
		request:   func() proto.Message { r := personRequest("per-2"); r.Document = "529.982.247-25"; return r },
		differing: func() proto.Message { r := personRequest("per-2"); r.Document = "987.654.321-00"; return r },
		response:  func() proto.Message { return &servicev1.RegisterPersonResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.RegisterPersonResponse).GetRegistered().GetPersonId()
		},
	},
	{
		method:  "RegisterRepresentation",
		request: func() proto.Message { return representationRequest("rep-1", "org-1", "per-1") },
		differing: func() proto.Message {
			r := representationRequest("rep-1", "org-1", "per-1")
			r.ScopeActTypes = nil
			return r
		},
		response: func() proto.Message { return &servicev1.RegisterRepresentationResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.RegisterRepresentationResponse).GetRegistered().GetRepresentationId()
		},
	},
}

func withOrganizationAndPerson(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	registerPerson(t, h, personRequest("per-1"))
	return h
}

func TestEachCommandOfTheRelationsWithoutAKeyIsRefusedBeforeTheHandler(t *testing.T) {
	h := withOrganizationAndPerson(t)
	for _, c := range relationCommands {
		t.Run(c.method, func(t *testing.T) {
			err := h.invoke(withoutKey(t), c.method, c.request(), c.response())

			if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
				t.Fatalf("%s() = %v (reason %q), want InvalidArgument with %s", c.method, err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonMissingIdempotencyKey)
			}
		})
	}
}

func TestEachRepeatedCommandOfTheRelationsAnswersTheStoredResponse(t *testing.T) {
	for _, c := range relationCommands {
		t.Run(c.method, func(t *testing.T) {
			h := withOrganizationAndPerson(t)
			first, again := c.response(), c.response()
			var header metadata.MD
			if err := h.invoke(withKey(t, "k-replay-"+c.method), c.method, c.request(), first); err != nil {
				t.Fatalf("first %s() = %v", c.method, err)
			}
			entries := len(h.store.Entries())

			err := h.invoke(withKey(t, "k-replay-"+c.method), c.method, c.request(), again, grpc.Header(&header))

			if err != nil || c.id(again) == "" || c.id(again) != c.id(first) {
				t.Fatalf("repeated %s() = %v (%v), want the stored response %v", c.method, err, again, first)
			}
			if !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
				t.Fatalf("replay header %s = %v, want [true]", kernelgrpc.ReplayedHeader, header.Get(kernelgrpc.ReplayedHeader))
			}
			if n := len(h.store.Entries()); n != entries {
				t.Fatalf("outbox grew from %d to %d on a replay, want no second effect", entries, n)
			}
		})
	}
}

func TestEachCommandOfTheRelationsRefusesItsKeyReusedForAnotherRequest(t *testing.T) {
	for _, c := range relationCommands {
		t.Run(c.method, func(t *testing.T) {
			h := withOrganizationAndPerson(t)
			if err := h.invoke(withKey(t, "k-reused-"+c.method), c.method, c.request(), c.response()); err != nil {
				t.Fatalf("first %s() = %v", c.method, err)
			}

			err := h.invoke(withKey(t, "k-reused-"+c.method), c.method, c.differing(), c.response())

			if status.Code(err) != codes.FailedPrecondition || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonReusedIdempotencyKey {
				t.Fatalf("%s(differing) = %v (reason %q), want FailedPrecondition with %s", c.method, err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonReusedIdempotencyKey)
			}
		})
	}
}

var accessCommands = []commandCall{
	{
		method:    "ProvisionUserAccount",
		request:   func() proto.Message { return provisionRequest("sub-2") },
		differing: func() proto.Message { r := provisionRequest("sub-2"); r.PersonId = "per-2"; return r },
		response:  func() proto.Message { return &servicev1.ProvisionUserAccountResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.ProvisionUserAccountResponse).GetProvisioned().GetUserId()
		},
	},
	{
		method:    "SuspendUser",
		request:   func() proto.Message { return suspendRequest("sub-1") },
		differing: func() proto.Message { r := suspendRequest("sub-1"); r.Reason = "another review"; return r },
		response:  func() proto.Message { return &servicev1.SuspendUserResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.SuspendUserResponse).GetSuspended().GetUserId()
		},
	},
	{
		method:  "InviteUser",
		request: func() proto.Message { return inviteRequest("mbr-2") },
		differing: func() proto.Message {
			r := inviteRequest("mbr-2")
			r.Role = servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR
			return r
		},
		response: func() proto.Message { return &servicev1.InviteUserResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.InviteUserResponse).GetInvited().GetMembershipId()
		},
	},
	{
		method:  "GrantMembership",
		request: func() proto.Message { return grantRequest("mbr-1") },
		differing: func() proto.Message {
			r := grantRequest("mbr-1")
			r.VerificationLevel = servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP
			return r
		},
		response: func() proto.Message { return &servicev1.GrantMembershipResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.GrantMembershipResponse).GetGranted().GetMembershipId()
		},
	},
	{
		method:    "RevokeMembership",
		request:   func() proto.Message { return revokeRequest("mbr-1") },
		differing: func() proto.Message { r := revokeRequest("mbr-1"); r.Reason = "role changed"; return r },
		response:  func() proto.Message { return &servicev1.RevokeMembershipResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.RevokeMembershipResponse).GetRevoked().GetMembershipId()
		},
	},
}

func withAccessParties(t *testing.T, method string) *harness {
	t.Helper()
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	invokeOK(t, h, "ProvisionUserAccount", provisionRequest("sub-1"), &servicev1.ProvisionUserAccountResponse{})
	switch method {
	case "GrantMembership":
		invokeOK(t, h, "InviteUser", inviteRequest("mbr-1"), &servicev1.InviteUserResponse{})
	case "RevokeMembership":
		invokeOK(t, h, "InviteUser", inviteRequest("mbr-1"), &servicev1.InviteUserResponse{})
		invokeOK(t, h, "GrantMembership", grantRequest("mbr-1"), &servicev1.GrantMembershipResponse{})
	}
	return h
}

func TestEachCommandOfTheAccessWithoutAKeyIsRefusedBeforeTheHandler(t *testing.T) {
	for _, c := range accessCommands {
		t.Run(c.method, func(t *testing.T) {
			h := withAccessParties(t, c.method)

			err := h.invoke(withoutKey(t), c.method, c.request(), c.response())

			if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
				t.Fatalf("%s() = %v (reason %q), want InvalidArgument with %s", c.method, err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonMissingIdempotencyKey)
			}
		})
	}
}

func TestEachRepeatedCommandOfTheAccessAnswersTheStoredResponse(t *testing.T) {
	for _, c := range accessCommands {
		t.Run(c.method, func(t *testing.T) {
			h := withAccessParties(t, c.method)
			first, again := c.response(), c.response()
			var header metadata.MD
			if err := h.invoke(withKey(t, "k-replay-"+c.method), c.method, c.request(), first); err != nil {
				t.Fatalf("first %s() = %v", c.method, err)
			}
			entries := len(h.store.Entries())

			err := h.invoke(withKey(t, "k-replay-"+c.method), c.method, c.request(), again, grpc.Header(&header))

			if err != nil || c.id(again) == "" || c.id(again) != c.id(first) {
				t.Fatalf("repeated %s() = %v (%v), want the stored response %v", c.method, err, again, first)
			}
			if !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
				t.Fatalf("replay header %s = %v, want [true]", kernelgrpc.ReplayedHeader, header.Get(kernelgrpc.ReplayedHeader))
			}
			if n := len(h.store.Entries()); n != entries {
				t.Fatalf("outbox grew from %d to %d on a replay, want no second effect", entries, n)
			}
		})
	}
}

func TestEachCommandOfTheAccessRefusesItsKeyReusedForAnotherRequest(t *testing.T) {
	for _, c := range accessCommands {
		t.Run(c.method, func(t *testing.T) {
			h := withAccessParties(t, c.method)
			if err := h.invoke(withKey(t, "k-reused-"+c.method), c.method, c.request(), c.response()); err != nil {
				t.Fatalf("first %s() = %v", c.method, err)
			}

			err := h.invoke(withKey(t, "k-reused-"+c.method), c.method, c.differing(), c.response())

			if status.Code(err) != codes.FailedPrecondition || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonReusedIdempotencyKey {
				t.Fatalf("%s(differing) = %v (reason %q), want FailedPrecondition with %s", c.method, err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonReusedIdempotencyKey)
			}
		})
	}
}

var bankAccountChangeCommands = []commandCall{
	{
		method:    "RequestBankAccountChange",
		request:   func() proto.Message { return requestChangeRequest("bac-2") },
		differing: func() proto.Message { r := requestChangeRequest("bac-2"); r.DestinationEvidenceId = "evd-2"; return r },
		response:  func() proto.Message { return &servicev1.RequestBankAccountChangeResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.RequestBankAccountChangeResponse).GetRequested().GetBankAccountChangeId()
		},
	},
	{
		method:    "ApproveBankAccountChange",
		request:   func() proto.Message { return approveChangeRequest("bac-1", "sub-2") },
		differing: func() proto.Message { return approveChangeRequest("bac-1", "sub-3") },
		response:  func() proto.Message { return &servicev1.ApproveBankAccountChangeResponse{} },
		id: func(m proto.Message) string {
			return m.(*servicev1.ApproveBankAccountChangeResponse).GetApproved().GetBankAccountChangeId()
		},
	},
}

func TestEachCommandOfTheBankAccountChangeWithoutAKeyIsRefusedBeforeTheHandler(t *testing.T) {
	h := withRequestedChange(t)
	for _, c := range bankAccountChangeCommands {
		t.Run(c.method, func(t *testing.T) {
			err := h.invoke(withoutKey(t), c.method, c.request(), c.response())

			if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
				t.Fatalf("%s() = %v (reason %q), want InvalidArgument with %s", c.method, err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonMissingIdempotencyKey)
			}
		})
	}
}

func TestEachRepeatedCommandOfTheBankAccountChangeAnswersTheStoredResponse(t *testing.T) {
	for _, c := range bankAccountChangeCommands {
		t.Run(c.method, func(t *testing.T) {
			h := withRequestedChange(t)
			first, again := c.response(), c.response()
			var header metadata.MD
			if err := h.invoke(withKey(t, "k-replay-"+c.method), c.method, c.request(), first); err != nil {
				t.Fatalf("first %s() = %v", c.method, err)
			}
			entries := len(h.store.Entries())

			err := h.invoke(withKey(t, "k-replay-"+c.method), c.method, c.request(), again, grpc.Header(&header))

			if err != nil || c.id(again) == "" || c.id(again) != c.id(first) {
				t.Fatalf("repeated %s() = %v (%v), want the stored response %v", c.method, err, again, first)
			}
			if !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
				t.Fatalf("replay header %s = %v, want [true]", kernelgrpc.ReplayedHeader, header.Get(kernelgrpc.ReplayedHeader))
			}
			if n := len(h.store.Entries()); n != entries {
				t.Fatalf("outbox grew from %d to %d on a replay, want no second effect", entries, n)
			}
		})
	}
}

func TestEachCommandOfTheBankAccountChangeRefusesItsKeyReusedForAnotherRequest(t *testing.T) {
	for _, c := range bankAccountChangeCommands {
		t.Run(c.method, func(t *testing.T) {
			h := withRequestedChange(t)
			if err := h.invoke(withKey(t, "k-reused-"+c.method), c.method, c.request(), c.response()); err != nil {
				t.Fatalf("first %s() = %v", c.method, err)
			}

			err := h.invoke(withKey(t, "k-reused-"+c.method), c.method, c.differing(), c.response())

			if status.Code(err) != codes.FailedPrecondition || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonReusedIdempotencyKey {
				t.Fatalf("%s(differing) = %v (reason %q), want FailedPrecondition with %s", c.method, err, kernelgrpc.ReasonOf(err), kernelgrpc.ReasonReusedIdempotencyKey)
			}
		})
	}
}
