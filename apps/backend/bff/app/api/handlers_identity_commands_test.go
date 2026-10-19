package api_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	requester           = "u-requester"
	requesterCredential = `Bearer {"sub":"u-requester","tenant":"acme","permissions":[]}`

	requestBankAccountChangePermission = "identity:request-bank-account-change"
	bankAccountChangesPath             = "/identity/organizations/org-1/bank-account-changes"
	bankAccountChangeRejection         = "identity-organization/bank-account-change/organization-not-active"
)

func bankAccountChangeBody(id, evidence string) string {
	raw, _ := json.Marshal(map[string]string{"bankAccountChangeId": id, "destinationEvidenceId": evidence})
	return string(raw)
}

func requested(id string) reply {
	return answer(&servicev1.RequestBankAccountChangeResponse{Result: &servicev1.RequestBankAccountChangeResponse_Requested{
		Requested: &servicev1.RequestedBankAccountChange{BankAccountChangeId: id},
	}})
}

func refused(code, message string) reply {
	return answer(&servicev1.RequestBankAccountChangeResponse{Result: &servicev1.RequestBankAccountChangeResponse_Rejection{
		Rejection: &servicev1.Rejection{Code: code, Message: message},
	}})
}

func failing(err error) reply {
	return func(any) (any, error) { return nil, err }
}

func commandEdge(t *testing.T, granted string, respond reply) (fixture, *fakeIdentity) {
	t.Helper()
	fake := (&fakeIdentity{}).grant(granted).replying(methodRequestBankAccountChange, respond)
	return identityEdge(t, fake), fake
}

func requestBankAccountChange(t *testing.T, f fixture, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	all := append([]string{"Idempotency-Key", "k-1", "Content-Type", "application/json", "Authorization", requesterCredential}, headers...)
	return f.do(t, http.MethodPost, bankAccountChangesPath, strings.NewReader(body), all...)
}

func requireAnswer(t *testing.T, rec *httptest.ResponseRecorder, status int, want string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	if got, expected := jsonOf(t, rec.Body.Bytes()), jsonOf(t, []byte(want)); !reflect.DeepEqual(got, expected) {
		t.Fatalf("body = %s, want %s", rec.Body.String(), want)
	}
}

func TestARequestedBankAccountChangeCarriesTheRequesterTheTenantAndTheDerivedKey(t *testing.T) {
	f, fake := commandEdge(t, requestBankAccountChangePermission, requested("bac-1"))

	rec := requestBankAccountChange(t, f, bankAccountChangeBody("bac-1", "ev-9"))

	requireAnswer(t, rec, http.StatusCreated, `{"bankAccountChangeId":"bac-1"}`)
	if got := rec.Header().Get(api.ReplayedHeader); got != "" {
		t.Fatalf("%s = %q on a first answer, want none", api.ReplayedHeader, got)
	}
	routed := requireHops(t, fake, methodRequestBankAccountChange, 1)[0]
	want := &servicev1.RequestBankAccountChangeRequest{
		BankAccountChangeId: "bac-1", OrganizationId: "org-1", DestinationEvidenceId: "ev-9", RequestedByUserId: requester,
	}
	if !proto.Equal(routed.req.(proto.Message), want) {
		t.Fatalf("request = %v, want %v", routed.req, want)
	}
	if tenant := routed.md.Get(kernelgrpc.TenantKey); !slices.Equal(tenant, []string{testTenant}) {
		t.Fatalf("tenant = %v, want the tenant of the token", tenant)
	}
	if key := routed.md.Get(kernelgrpc.IdempotencyKey); !slices.Equal(key, []string{derived(requester, "k-1")}) {
		t.Fatalf("%s = %v, want the key derived from the subject of the token", kernelgrpc.IdempotencyKey, key)
	}
	permissions := requireHops(t, fake, methodPermissions, 1)
	if asked := permissions[0].req.(*servicev1.FindEffectivePermissionsRequest).GetUserId(); asked != requester {
		t.Fatalf("the edge asked the permissions of %q, want the subject of the token", asked)
	}
}

func TestARejectionOfTheIdentityIs422WithItsCode(t *testing.T) {
	f, fake := commandEdge(t, requestBankAccountChangePermission, refused(bankAccountChangeRejection, "organization must exist and be active"))

	rec := requestBankAccountChange(t, f, bankAccountChangeBody("bac-1", "ev-9"))

	requireAnswer(t, rec, http.StatusUnprocessableEntity,
		`{"code":"`+bankAccountChangeRejection+`","message":"organization must exist and be active"}`)
	requireHops(t, fake, methodRequestBankAccountChange, 1)
}

func TestAReplayedAnswerIsMarkedOnlyBelow500(t *testing.T) {
	replayed := metadata.Pairs(kernelgrpc.ReplayedHeader, "true")
	cases := []struct {
		name    string
		respond reply
		header  metadata.MD
		status  int
		want    string
	}{
		{"requested", requested("bac-1"), replayed, http.StatusCreated, "true"},
		{"refused", refused(bankAccountChangeRejection, "refused"), replayed, http.StatusUnprocessableEntity, "true"},
		{"requested afresh", requested("bac-1"), nil, http.StatusCreated, ""},
		{"internal failure", failing(status.Error(codes.Internal, "boom")), replayed, http.StatusInternalServerError, ""},
		{"unavailable", failing(status.Error(codes.Unavailable, "restarting")), replayed, http.StatusServiceUnavailable, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := (&fakeIdentity{}).grant(requestBankAccountChangePermission).replying(methodRequestBankAccountChange, tc.respond)
			if tc.header != nil {
				fake.sendingHeader(methodRequestBankAccountChange, tc.header)
			}
			f := identityEdge(t, fake)

			rec := requestBankAccountChange(t, f, bankAccountChangeBody("bac-1", "ev-9"))

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.status, rec.Body.String())
			}
			if got := rec.Header().Get(api.ReplayedHeader); got != tc.want {
				t.Fatalf("%s = %q, want %q", api.ReplayedHeader, got, tc.want)
			}
		})
	}
}

func TestTheIdempotencyOutcomeOfTheIdentityReachesTheClient(t *testing.T) {
	cases := []struct {
		name   string
		cause  error
		status int
		code   string
	}{
		{"reused key", ports.ErrIdempotencyMismatch, http.StatusUnprocessableEntity, "reused-idempotency-key"},
		{"key in flight", ports.ErrIdempotencyInFlight, http.StatusConflict, "in-flight-idempotency-key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped, ok := kernelgrpc.IdempotencyStatus(tc.cause)
			if !ok {
				t.Fatalf("IdempotencyStatus(%v) mapped nothing", tc.cause)
			}
			f, _ := commandEdge(t, requestBankAccountChangePermission, failing(mapped))

			requireRejection(t, requestBankAccountChange(t, f, bankAccountChangeBody("bac-1", "ev-9")), tc.status, tc.code)
		})
	}
}

func TestTheBodyOfARequestedBankAccountChangeIsBoundByTheSchema(t *testing.T) {
	accented := strings.Repeat("é", 256)
	cases := []struct {
		name     string
		body     string
		accepted bool
		evidence string
	}{
		{name: "an empty evidence goes to the domain", body: bankAccountChangeBody("bac-1", ""), accepted: true, evidence: ""},
		{name: "an id of 128 characters", body: bankAccountChangeBody(strings.Repeat("a", 128), "ev-9"), accepted: true, evidence: "ev-9"},
		{name: "an evidence of 256 characters", body: bankAccountChangeBody("bac-1", strings.Repeat("e", 256)), accepted: true, evidence: strings.Repeat("e", 256)},
		{name: "an evidence of 256 non-ASCII characters", body: bankAccountChangeBody("bac-1", accented), accepted: true, evidence: accented},
		{name: "the requester in the body", body: `{"bankAccountChangeId":"bac-1","destinationEvidenceId":"ev-9","requestedByUserId":"u-forged"}`},
		{name: "an unknown field", body: `{"bankAccountChangeId":"bac-1","destinationEvidenceId":"ev-9","organizationId":"org-2"}`},
		{name: "no id", body: `{"destinationEvidenceId":"ev-9"}`},
		{name: "no evidence", body: `{"bankAccountChangeId":"bac-1"}`},
		{name: "a null evidence", body: `{"bankAccountChangeId":"bac-1","destinationEvidenceId":null}`},
		{name: "an empty id", body: bankAccountChangeBody("", "ev-9")},
		{name: "an id of 129 characters", body: bankAccountChangeBody(strings.Repeat("a", 129), "ev-9")},
		{name: "an id outside the pattern", body: bankAccountChangeBody("bac 1", "ev-9")},
		{name: "a non-ASCII id", body: bankAccountChangeBody("bác-1", "ev-9")},
		{name: "an evidence of 257 characters", body: bankAccountChangeBody("bac-1", strings.Repeat("e", 257))},
		{name: "an evidence of 257 non-ASCII characters", body: bankAccountChangeBody("bac-1", accented+"é")},
		{name: "a C0 control character", body: bankAccountChangeBody("bac-1", "ev\x07")},
		{name: "DEL", body: bankAccountChangeBody("bac-1", "ev\x7f")},
		{name: "a C1 control character", body: bankAccountChangeBody("bac-1", "ev\u0085")},
		{name: "a number as id", body: `{"bankAccountChangeId":1,"destinationEvidenceId":"ev-9"}`},
		{name: "trailing data", body: bankAccountChangeBody("bac-1", "ev-9") + `{}`},
		{name: "no body", body: ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, fake := commandEdge(t, requestBankAccountChangePermission, requested("bac-1"))

			rec := requestBankAccountChange(t, f, tc.body)

			if !tc.accepted {
				requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
				requireHops(t, fake, methodRequestBankAccountChange, 0)
				return
			}
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
			}
			routed := requireHops(t, fake, methodRequestBankAccountChange, 1)[0].req.(*servicev1.RequestBankAccountChangeRequest)
			if routed.GetDestinationEvidenceId() != tc.evidence || routed.GetRequestedByUserId() != requester {
				t.Fatalf("request = %v, want the evidence %q and the requester of the token", routed, tc.evidence)
			}
		})
	}
}

func TestARequestedBankAccountChangeRefusedAtTheEdgeNeverReachesTheContext(t *testing.T) {
	f, fake := commandEdge(t, requestBankAccountChangePermission, requested("bac-1"))

	rec := f.do(t, http.MethodPost, "/identity/organizations/org%201/bank-account-changes", strings.NewReader(bankAccountChangeBody("bac-1", "ev-9")),
		"Idempotency-Key", "k-1", "Content-Type", "application/json", "Authorization", requesterCredential)

	requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
	requireHops(t, fake, methodRequestBankAccountChange, 0)
	requireHops(t, fake, methodPermissions, 1)
}

func TestAnApprovalTakesNoBody(t *testing.T) {
	requireBounds(t, "approveBankAccountChange", []bound{
		{"no body", "", routedAs(func(r *servicev1.ApproveBankAccountChangeRequest) bool { return r.GetApprovedByUserId() == requester })},
		{"an empty object", "{}", nil},
		{"the approver in the body", `{"approvedByUserId":"u-forged"}`, nil},
		{"null", "null", nil},
		{"a blank", " ", nil},
	})
}

func keyedRoutes() []identityCommand { return append(identityCommands(), organizationLookup()) }

func keyedRoute(t *testing.T, name string) identityCommand {
	t.Helper()
	for _, route := range keyedRoutes() {
		if route.name == name {
			return route
		}
	}
	t.Fatalf("no keyed identity route %s under test", name)
	return identityCommand{}
}

func businessHops(fake *fakeIdentity) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	calls := 0
	for method, hops := range fake.hops {
		if method != methodPermissions {
			calls += len(hops)
		}
	}
	return calls
}

func permissionsBut(own string) []string {
	var others []string
	for _, route := range keyedRoutes() {
		if route.permission != own {
			others = append(others, route.permission)
		}
	}
	return others
}

func TestEveryKeyedIdentityRouteRefusedAtTheEdgeNeverReachesTheContext(t *testing.T) {
	cases := []struct {
		name        string
		key         string
		denied      bool
		status      int
		code        string
		permissions int
	}{
		{name: "without an idempotency key", key: "", status: http.StatusBadRequest, code: "missing-idempotency-key"},
		{name: "with an invalid idempotency key", key: "k 1", status: http.StatusBadRequest, code: "invalid-idempotency-key"},
		{name: "without the permission", key: "k-1", denied: true, status: http.StatusForbidden, code: "permission-denied", permissions: 1},
	}
	for _, route := range keyedRoutes() {
		for _, tc := range cases {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				granted := []string{route.permission}
				if tc.denied {
					granted = permissionsBut(route.permission)
				}
				fake := (&fakeIdentity{}).grant(granted...).replying(route.method, route.reply)
				f := identityEdge(t, fake)

				rec := f.do(t, http.MethodPost, route.path, strings.NewReader(route.baseBody(t)),
					"Idempotency-Key", tc.key, "Content-Type", "application/json", "Authorization", requesterCredential)

				requireRejection(t, rec, tc.status, tc.code)
				if calls := businessHops(fake); calls != 0 {
					t.Fatalf("business calls = %d, want none", calls)
				}
				requireHops(t, fake, methodPermissions, tc.permissions)
			})
		}
	}
}

func TestTheIdempotencyKeyIsJudgedOnlyOnceTheTokenIsVerifiedAndAdmitted(t *testing.T) {
	cases := []struct {
		name    string
		headers []string
		limit   bool
		status  int
		code    string
	}{
		{name: "without token nor key", headers: []string{"Authorization", ""}, status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "with an invalid token and no key", headers: []string{"Authorization", "Bearer not-json"}, status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "without a tenant nor key", headers: []string{"Authorization", tenantlessSubject}, status: http.StatusForbidden, code: "tenant-unresolved"},
		{name: "beyond the admission without a key", headers: []string{"Authorization", requesterCredential}, limit: true, status: http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command := commandNamed(t, "requestBankAccountChange")
			fake := (&fakeIdentity{}).grant(command.permission).replying(command.method, command.reply)
			var options []option
			if tc.limit {
				options = append(options, withLimit(admission.Limit{PerSecond: 0.001, Burst: 1, Concurrency: 10}))
			}
			f := identityEdge(t, fake, options...)
			if tc.limit {
				command.send(t, f, command.path, command.baseBody(t))
			}

			rec := f.do(t, http.MethodPost, command.path, strings.NewReader(command.baseBody(t)),
				append([]string{"Content-Type", "application/json"}, tc.headers...)...)

			if tc.limit {
				requireAdmissionRefused(t, rec)
				requireHops(t, fake, methodPermissions, 1)
				return
			}
			requireRejection(t, rec, tc.status, tc.code)
			requireHops(t, fake, methodPermissions, 0)
			requireHops(t, fake, command.method, 0)
		})
	}
}

func foreignFields(route identityCommand) []string {
	fields := []string{"unknownField"}
	descriptors := route.request.ProtoReflect().Descriptor().Fields()
	for i := range descriptors.Len() {
		if name := descriptors.Get(i).JSONName(); route.body[name] == nil {
			fields = append(fields, name)
		}
	}
	return fields
}

func TestEveryKeyedIdentityRouteRefusesInItsBodyAFieldItDoesNotCarry(t *testing.T) {
	for _, route := range keyedRoutes() {
		base := route.body
		if base == nil {
			base = map[string]any{}
		}
		for _, field := range foreignFields(route) {
			t.Run(route.name+"/"+field, func(t *testing.T) {
				f, fake := route.edge(t, route.reply, nil)

				rec := route.send(t, f, route.path, bodyWith(t, base, with(field, "u-forged")))

				requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
				if calls := businessHops(fake); calls != 0 {
					t.Fatalf("business calls = %d, want none", calls)
				}
			})
		}
	}
}

func closedVocabulary(route, field string) bool {
	switch field {
	case "taxIdType", "kinds", "powerType", "sourceType", "role", "system", "referenceType":
		return true
	}
	return route == "findOrganizationByTaxId" && field == "jurisdiction"
}

func planted(t *testing.T, route identityCommand, seed int) (map[string]any, []string) {
	t.Helper()
	body := cloned(t, route.body)
	var sentinels []string
	next := func() string {
		sentinel := fmt.Sprintf("Zq%02dx%02dW", seed, len(sentinels))
		sentinels = append(sentinels, sentinel)
		return sentinel
	}
	var plant func(map[string]any)
	plant = func(object map[string]any) {
		for _, field := range slices.Sorted(maps.Keys(object)) {
			if closedVocabulary(route.name, field) {
				continue
			}
			switch value := object[field].(type) {
			case string:
				object[field] = next()
			case []any:
				for i, item := range value {
					switch item := item.(type) {
					case string:
						value[i] = next()
					case map[string]any:
						plant(item)
					}
				}
			}
		}
	}
	plant(body)
	return body, sentinels
}

func TestNoValueOfAKeyedBodyReachesTheLogs(t *testing.T) {
	outcomes := []struct {
		name    string
		invalid bool
		respond func(identityCommand) reply
		status  int
	}{
		{name: "accepted", respond: func(route identityCommand) reply { return route.reply }},
		{name: "refused at the edge", invalid: true, respond: func(route identityCommand) reply { return route.reply }, status: http.StatusBadRequest},
		{name: "rejected by the context", respond: func(route identityCommand) reply {
			return answer(route.rejection(&servicev1.Rejection{Code: "identity-organization/probe/refused", Message: "refused"}))
		}, status: http.StatusUnprocessableEntity},
		{name: "unavailable", respond: func(identityCommand) reply { return failing(status.Error(codes.Unavailable, "restarting")) }, status: http.StatusServiceUnavailable},
		{name: "failing", respond: func(identityCommand) reply { return failing(status.Error(codes.Internal, "boom")) }, status: http.StatusInternalServerError},
	}
	for seed, route := range keyedRoutes() {
		if route.body == nil {
			continue
		}
		for _, outcome := range outcomes {
			if outcome.status == http.StatusUnprocessableEntity && route.rejection == nil {
				continue
			}
			t.Run(route.name+"/"+outcome.name, func(t *testing.T) {
				body, sentinels := planted(t, route, seed)
				if outcome.invalid {
					sentinels = append(sentinels, fmt.Sprintf("Zq%02dxUFW", seed))
					body["unknownField"] = sentinels[len(sentinels)-1]
				}
				want := outcome.status
				if want == 0 {
					want = route.status
				}
				f, fake := route.edge(t, outcome.respond(route), nil)

				rec := route.send(t, f, route.path, bodyWith(t, body))

				if rec.Code != want {
					t.Fatalf("status = %d, want %d (body %s)", rec.Code, want, rec.Body.String())
				}
				hops := fake.hopsTo(route.method)
				if outcome.invalid != (len(hops) == 0) {
					t.Fatalf("%s calls = %d, want none only when the edge refuses the body", route.method, len(hops))
				}
				forwarded := ""
				if len(hops) > 0 {
					forwarded = prototext.Format(hops[0].req.(proto.Message))
				}
				exported := fmt.Sprint(onlyAccessLog(t, f))
				for _, sentinel := range sentinels {
					if !outcome.invalid && !strings.Contains(forwarded, sentinel) {
						t.Fatalf("%s did not reach the context (request %s): the probe proves nothing", sentinel, forwarded)
					}
					if strings.Contains(exported, sentinel) || strings.Contains(f.logs.String(), sentinel) {
						t.Fatalf("%s of the body reached the logs: access log %s, emitted %s", sentinel, exported, f.logs.String())
					}
				}
			})
		}
	}
}

func TestTheGrantOfAMembershipIsNoRouteOfTheEdge(t *testing.T) {
	for _, route := range api.Routes(routeBudget) {
		if route.Name == "grantMembership" || strings.HasSuffix(route.Path, "/grant") {
			t.Fatalf("the edge serves %s %s, want GrantMembership outside the edge", route.Name, route.Path)
		}
	}
	fake := (&fakeIdentity{}).grant("identity:grant-membership")
	f := identityEdge(t, fake)

	rec := f.do(t, http.MethodPost, "/identity/memberships/m-1/grant", strings.NewReader(`{"role":"operator"}`),
		"Idempotency-Key", "k-1", "Content-Type", "application/json", "Authorization", requesterCredential)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	if calls := businessHops(fake); calls != 0 {
		t.Fatalf("business calls = %d, want none", calls)
	}
}
