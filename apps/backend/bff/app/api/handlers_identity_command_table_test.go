package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

type identityCommand struct {
	name       string
	permission string
	method     string
	path       string
	badPath    string
	body       map[string]any
	reply      reply
	rejection  func(*servicev1.Rejection) proto.Message
	request    proto.Message
	status     int
	want       string
}

var commandRoutesUnderTest = func() []string {
	names := []string{"findOrganizationByTaxId"}
	for _, command := range identityCommands() {
		names = append(names, command.name)
	}
	return names
}()

func identityCommands() []identityCommand {
	return slices.Concat(bankAccountCommands(), organizationCommands(), peopleCommands(), membershipCommands())
}

func bankAccountCommands() []identityCommand {
	return []identityCommand{{
		name:       "requestBankAccountChange",
		permission: requestBankAccountChangePermission,
		method:     methodRequestBankAccountChange,
		path:       bankAccountChangesPath,
		badPath:    "/identity/organizations/org%201/bank-account-changes",
		body:       map[string]any{"bankAccountChangeId": "bac-1", "destinationEvidenceId": "ev-9"},
		reply:      requested("bac-1"),
		rejection: func(r *servicev1.Rejection) proto.Message {
			return &servicev1.RequestBankAccountChangeResponse{Result: &servicev1.RequestBankAccountChangeResponse_Rejection{Rejection: r}}
		},
		request: &servicev1.RequestBankAccountChangeRequest{
			BankAccountChangeId: "bac-1", OrganizationId: "org-1", DestinationEvidenceId: "ev-9", RequestedByUserId: requester,
		},
		status: http.StatusCreated,
		want:   `{"bankAccountChangeId":"bac-1"}`,
	}, {
		name:       "approveBankAccountChange",
		permission: "identity:approve-bank-account-change",
		method:     methodApproveBankAccountChange,
		path:       "/identity/bank-account-changes/bac-1/approve",
		badPath:    "/identity/bank-account-changes/bac%201/approve",
		reply: answer(&servicev1.ApproveBankAccountChangeResponse{Result: &servicev1.ApproveBankAccountChangeResponse_Approved{
			Approved: &servicev1.ApprovedBankAccountChange{BankAccountChangeId: "bac-1"},
		}}),
		rejection: func(r *servicev1.Rejection) proto.Message {
			return &servicev1.ApproveBankAccountChangeResponse{Result: &servicev1.ApproveBankAccountChangeResponse_Rejection{Rejection: r}}
		},
		request: &servicev1.ApproveBankAccountChangeRequest{BankAccountChangeId: "bac-1", ApprovedByUserId: requester},
		status:  http.StatusOK,
		want:    `{"bankAccountChangeId":"bac-1"}`,
	}}
}

func reasonBounds(t *testing.T, base map[string]any) []bound {
	t.Helper()
	body := func(edits ...edit) string { return bodyWith(t, base, edits...) }
	reason := func(want string) func(proto.Message) bool {
		return func(m proto.Message) bool {
			r, ok := m.(interface{ GetReason() string })
			return ok && r.GetReason() == want
		}
	}
	return []bound{
		{"an empty reason goes to the domain", body(with("reason", "")), reason("")},
		{"a reason of 512 non-ASCII characters", body(with("reason", strings.Repeat("é", 512))), reason(strings.Repeat("é", 512))},
		{"no reason", body(without("reason")), nil},
		{"a null reason", body(with("reason", nil)), nil},
		{"a reason of 513 characters", body(with("reason", strings.Repeat("é", 513))), nil},
		{"a reason with a control character", body(with("reason", "fraud\u009b")), nil},
		{"an unknown field", body(with("actor", "u-forged")), nil},
	}
}

func commandNamed(t *testing.T, name string) identityCommand {
	t.Helper()
	for _, command := range identityCommands() {
		if command.name == name {
			return command
		}
	}
	t.Fatalf("no identity command %s", name)
	return identityCommand{}
}

func (c identityCommand) edge(t *testing.T, respond reply, header metadata.MD) (fixture, *fakeIdentity) {
	t.Helper()
	fake := (&fakeIdentity{}).grant(c.permission).replying(c.method, respond)
	if header != nil {
		fake.sendingHeader(c.method, header)
	}
	return identityEdge(t, fake), fake
}

func (c identityCommand) send(t *testing.T, f fixture, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	headers := []string{"Idempotency-Key", "k-1", "Content-Type", "application/json", "Authorization", requesterCredential}
	return f.do(t, http.MethodPost, path, strings.NewReader(body), headers...)
}

func (c identityCommand) baseBody(t *testing.T) string {
	t.Helper()
	if c.body == nil {
		return ""
	}
	return bodyWith(t, c.body)
}

type edit func(map[string]any)

func with(field string, value any) edit { return func(m map[string]any) { m[field] = value } }

func without(field string) edit { return func(m map[string]any) { delete(m, field) } }

func cloned(t *testing.T, base map[string]any, edits ...edit) map[string]any {
	t.Helper()
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("Marshal(%v) = %v", base, err)
	}
	stream := json.NewDecoder(bytes.NewReader(raw))
	stream.UseNumber()
	var copied map[string]any
	if err := stream.Decode(&copied); err != nil {
		t.Fatalf("Decode(%s) = %v", raw, err)
	}
	for _, e := range edits {
		e(copied)
	}
	return copied
}

func bodyWith(t *testing.T, base map[string]any, edits ...edit) string {
	t.Helper()
	raw, err := json.Marshal(cloned(t, base, edits...))
	if err != nil {
		t.Fatalf("Marshal = %v", err)
	}
	return string(raw)
}

func repeated[T any](n int, item T) []T {
	items := make([]T, n)
	for i := range items {
		items[i] = item
	}
	return items
}

type bound struct {
	name   string
	body   string
	routed func(proto.Message) bool
}

func reaches(proto.Message) bool { return true }

func routedAs[Req proto.Message](holds func(Req) bool) func(proto.Message) bool {
	return func(m proto.Message) bool {
		req, ok := m.(Req)
		return ok && holds(req)
	}
}

func requireBounds(t *testing.T, name string, bounds []bound) {
	t.Helper()
	requireBoundsOf(t, commandNamed(t, name), bounds)
}

func requireBoundsOf(t *testing.T, command identityCommand, bounds []bound) {
	t.Helper()
	for _, b := range bounds {
		t.Run(b.name, func(t *testing.T) {
			f, fake := command.edge(t, command.reply, nil)

			rec := command.send(t, f, command.path, b.body)

			if b.routed == nil {
				requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
				requireHops(t, fake, command.method, 0)
				return
			}
			if rec.Code != command.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, command.status, rec.Body.String())
			}
			if routed := requireHops(t, fake, command.method, 1)[0].req.(proto.Message); !b.routed(routed) {
				t.Fatalf("request = %v, not the one the body describes", routed)
			}
		})
	}
}

func TestEachIdentityCommandForwardsTheRequestOfTheContractAndAnswersItsView(t *testing.T) {
	for _, command := range identityCommands() {
		t.Run(command.name, func(t *testing.T) {
			f, fake := command.edge(t, command.reply, nil)

			rec := command.send(t, f, command.path, command.baseBody(t))

			requireAnswer(t, rec, command.status, command.want)
			if got := rec.Header().Get(api.ReplayedHeader); got != "" {
				t.Fatalf("%s = %q on a first answer, want none", api.ReplayedHeader, got)
			}
			routed := requireHops(t, fake, command.method, 1)[0]
			if !proto.Equal(routed.req.(proto.Message), command.request) {
				t.Fatalf("request = %v, want %v", routed.req, command.request)
			}
			if key := routed.md.Get(kernelgrpc.IdempotencyKey); !slices.Equal(key, []string{derived(requester, "k-1")}) {
				t.Fatalf("%s = %v, want the key derived from the subject of the token", kernelgrpc.IdempotencyKey, key)
			}
			if tenant := routed.md.Get(kernelgrpc.TenantKey); !slices.Equal(tenant, []string{testTenant}) {
				t.Fatalf("tenant = %v, want the tenant of the token", tenant)
			}
		})
	}
}

func TestEachIdentityCommandAnswersARejectionOfTheContextWith422(t *testing.T) {
	const code = "identity-organization/probe/refused"
	for _, command := range identityCommands() {
		t.Run(command.name, func(t *testing.T) {
			f, fake := command.edge(t, answer(command.rejection(&servicev1.Rejection{Code: code, Message: "refused by the domain"})), nil)

			rec := command.send(t, f, command.path, command.baseBody(t))

			requireAnswer(t, rec, http.StatusUnprocessableEntity, `{"code":"`+code+`","message":"refused by the domain"}`)
			requireHops(t, fake, command.method, 1)
		})
	}
}

func TestEachIdentityCommandMarksTheAnswerTheContextReplayed(t *testing.T) {
	for _, command := range identityCommands() {
		t.Run(command.name, func(t *testing.T) {
			f, _ := command.edge(t, command.reply, metadata.Pairs(kernelgrpc.ReplayedHeader, "true"))

			rec := command.send(t, f, command.path, command.baseBody(t))

			if rec.Code != command.status || rec.Header().Get(api.ReplayedHeader) != "true" {
				t.Fatalf("status = %d and %s = %q, want %d replayed", rec.Code, api.ReplayedHeader, rec.Header().Get(api.ReplayedHeader), command.status)
			}
		})
	}
}

func TestEachIdentityCommandRefusesAPathIDOutsideTheContract(t *testing.T) {
	for _, command := range identityCommands() {
		if command.badPath == "" {
			continue
		}
		t.Run(command.name, func(t *testing.T) {
			f, fake := command.edge(t, command.reply, nil)

			requireRejection(t, command.send(t, f, command.badPath, command.baseBody(t)), http.StatusBadRequest, "invalid-request")
			requireHops(t, fake, command.method, 0)
		})
	}
}
