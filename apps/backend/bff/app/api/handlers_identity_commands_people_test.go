package api_test

import (
	"net/http"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func personBody() map[string]any {
	return map[string]any{"personId": "p-1", "fullName": "Maria Silva", "document": "12345678909"}
}

func userAccountBody() map[string]any {
	return map[string]any{"subject": "kc:sub-1", "personId": "p-1"}
}

func peopleCommands() []identityCommand {
	return []identityCommand{
		{
			name:       "registerPerson",
			permission: "identity:register-person",
			method:     methodRegisterPerson,
			path:       "/identity/persons",
			body:       personBody(),
			reply: answer(&servicev1.RegisterPersonResponse{Result: &servicev1.RegisterPersonResponse_Registered{
				Registered: &servicev1.RegisteredPerson{PersonId: "p-1"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.RegisterPersonResponse{Result: &servicev1.RegisterPersonResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.RegisterPersonRequest{PersonId: "p-1", FullName: "Maria Silva", Document: "12345678909"},
			status:  http.StatusCreated,
			want:    `{"personId":"p-1"}`,
		},
		{
			name:       "provisionUserAccount",
			permission: "identity:provision-user-account",
			method:     methodProvisionUserAccount,
			path:       "/identity/users",
			body:       userAccountBody(),
			reply: answer(&servicev1.ProvisionUserAccountResponse{Result: &servicev1.ProvisionUserAccountResponse_Provisioned{
				Provisioned: &servicev1.ProvisionedUserAccount{UserId: "u-1"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.ProvisionUserAccountResponse{Result: &servicev1.ProvisionUserAccountResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.ProvisionUserAccountRequest{Subject: "kc:sub-1", PersonId: "p-1"},
			status:  http.StatusCreated,
			want:    `{"userId":"u-1"}`,
		},
		{
			name:       "suspendUser",
			permission: "identity:suspend-user",
			method:     methodSuspendUser,
			path:       "/identity/users/u-9/suspend",
			badPath:    "/identity/users/u%209/suspend",
			body:       map[string]any{"reason": "fraud suspected"},
			reply: answer(&servicev1.SuspendUserResponse{Result: &servicev1.SuspendUserResponse_Suspended{
				Suspended: &servicev1.SuspendedUser{UserId: "u-9"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.SuspendUserResponse{Result: &servicev1.SuspendUserResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.SuspendUserRequest{UserId: "u-9", Reason: "fraud suspected"},
			status:  http.StatusOK,
			want:    `{"userId":"u-9"}`,
		},
	}
}

func TestTheBodyOfARegisteredPersonIsBoundByTheSchema(t *testing.T) {
	body := func(edits ...edit) string { return bodyWith(t, personBody(), edits...) }
	person := func(holds func(*servicev1.RegisterPersonRequest) bool) func(proto.Message) bool {
		return routedAs(holds)
	}
	requireBounds(t, "registerPerson", []bound{
		{"an empty full name goes to the domain", body(with("fullName", "")),
			person(func(r *servicev1.RegisterPersonRequest) bool { return r.GetFullName() == "" })},
		{"an empty document goes to the domain", body(with("document", "")),
			person(func(r *servicev1.RegisterPersonRequest) bool { return r.GetDocument() == "" })},
		{"a full name of 256 non-ASCII characters", body(with("fullName", nonASCII256)), reaches},
		{"a document of 32 printable characters", body(with("document", printable32)),
			person(func(r *servicev1.RegisterPersonRequest) bool { return r.GetDocument() == printable32 })},
		{"no person id", body(without("personId")), nil},
		{"a person id of 129 characters", body(with("personId", identifier128+"i")), nil},
		{"no full name", body(without("fullName")), nil},
		{"a full name of 257 characters", body(with("fullName", nonASCII256+"é")), nil},
		{"a full name with a control character", body(with("fullName", "Maria\nSilva")), nil},
		{"no document", body(without("document")), nil},
		{"a null document", body(with("document", nil)), nil},
		{"a document of 33 characters", body(with("document", printable32+"0")), nil},
		{"a non-ASCII document", body(with("document", "123.456.789-0é")), nil},
		{"a document with a control character", body(with("document", "123\x1f")), nil},
		{"an unknown field", body(with("documentHash", "h")), nil},
	})
}

func TestTheDocumentOfARegisteredPersonNeverComesBack(t *testing.T) {
	command := commandNamed(t, "registerPerson")
	f, _ := command.edge(t, command.reply, nil)

	rec := command.send(t, f, command.path, command.baseBody(t))

	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), "12345678909") {
		t.Fatalf("status = %d and body = %s, want 201 without the document", rec.Code, rec.Body.String())
	}
}

func TestTheBodyOfAProvisionedUserAccountIsBoundByTheSchema(t *testing.T) {
	body := func(edits ...edit) string { return bodyWith(t, userAccountBody(), edits...) }
	account := func(holds func(*servicev1.ProvisionUserAccountRequest) bool) func(proto.Message) bool {
		return routedAs(holds)
	}
	requireBounds(t, "provisionUserAccount", []bound{
		{"an empty subject goes to the domain", body(with("subject", "")),
			account(func(r *servicev1.ProvisionUserAccountRequest) bool { return r.GetSubject() == "" })},
		{"a subject of 128 characters", body(with("subject", identifier128)), reaches},
		{"a subject of every allowed character", body(with("subject", "Az09._:-")), reaches},
		{"no person", body(without("personId")),
			account(func(r *servicev1.ProvisionUserAccountRequest) bool { return r.GetPersonId() == "" })},
		{"no subject", body(without("subject")), nil},
		{"a null subject", body(with("subject", nil)), nil},
		{"a subject of 129 characters", body(with("subject", identifier128+"i")), nil},
		{"a subject with a space", body(with("subject", "kc sub")), nil},
		{"a subject with a slash", body(with("subject", "kc/sub")), nil},
		{"a non-ASCII subject", body(with("subject", "kcé")), nil},
		{"an empty person id", body(with("personId", "")), nil},
		{"a person id outside the pattern", body(with("personId", "p 1")), nil},
		{"an unknown field", body(with("userId", "u-forged")), nil},
	})
}

func TestTheBodyOfASuspendedUserIsBoundByTheSchema(t *testing.T) {
	requireBounds(t, "suspendUser", reasonBounds(t, map[string]any{"reason": "fraud suspected"}))
}
