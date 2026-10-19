package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	createdAt = int64(1767225600123456789)
	otherUser = "u-9"
)

type identityRoute struct {
	name       string
	permission string
	method     string
	path       string
	query      bool
	empty      string
	reply      reply
	prepare    func(*fakeIdentity)
	request    proto.Message
	want       string
}

func answer(resp proto.Message) reply {
	return func(any) (any, error) { return resp, nil }
}

func identityRoutes() []identityRoute {
	return []identityRoute{
		{
			name:       "findOrganization",
			permission: "identity:find-organization",
			method:     methodOrganization,
			path:       "/identity/organizations/org-1",
			query:      true,
			reply: answer(&servicev1.FindOrganizationResponse{Organization: &servicev1.Organization{
				OrganizationId: "org-1",
				Kinds:          []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT, servicev1.OrganizationKind_ORGANIZATION_KIND_PLATFORM},
				Jurisdiction:   "BR",
				TaxIdType:      servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
				TaxId:          "12345678000195",
				TaxIdHash:      "hash-1",
				TaxIdMasked:    "12.***.***/0001-95",
				LegalName:      "Acme Ltda",
				TradeName:      "Acme",
				Status:         servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE,
				CreatedAt:      createdAt,
			}}),
			request: &servicev1.FindOrganizationRequest{OrganizationId: "org-1"},
			want: `{"organizationId":"org-1","kinds":["client","platform"],"jurisdiction":"BR","taxIdType":"cnpj",
				"taxId":"12345678000195","taxIdHash":"hash-1","taxIdHashKeyVersion":0,"taxIdMasked":"12.***.***/0001-95",
				"legalName":"Acme Ltda","tradeName":"Acme","status":"active","createdAt":1767225600123456789}`,
		},
		{
			name:       "findPerson",
			permission: "identity:find-person",
			method:     methodPerson,
			path:       "/identity/persons/p-1",
			query:      true,
			reply: answer(&servicev1.FindPersonResponse{Person: &servicev1.Person{
				PersonId:       "p-1",
				FullName:       "Maria Silva",
				DocumentHash:   "doc-hash",
				DocumentMasked: "***.456.789-**",
				CreatedAt:      createdAt,
			}}),
			request: &servicev1.FindPersonRequest{PersonId: "p-1"},
			want: `{"personId":"p-1","fullName":"Maria Silva","documentHash":"doc-hash","documentHashKeyVersion":0,
				"documentMasked":"***.456.789-**","createdAt":1767225600123456789}`,
		},
		{
			name:       "findRepresentation",
			permission: "identity:find-representation",
			method:     methodRepresentation,
			path:       "/identity/representations/r-1",
			query:      true,
			reply: answer(&servicev1.FindRepresentationResponse{Representation: &servicev1.Representation{
				RepresentationId:    "r-1",
				OrganizationId:      "org-1",
				PersonId:            "p-1",
				PowerType:           servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT,
				SourceType:          servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY,
				SourceEvidenceId:    "ev-1",
				ScopeCurrency:       "BRL",
				ScopeMaxAmountMinor: 500000,
				ScopeActTypes:       []string{"sign", "assign"},
				ValidFrom:           createdAt,
				ValidUntil:          createdAt + 1,
				VerificationStatus:  servicev1.RepresentationVerificationStatus_REPRESENTATION_VERIFICATION_STATUS_DECLARED,
				ProposedByUserId:    "u-1",
			}}),
			request: &servicev1.FindRepresentationRequest{RepresentationId: "r-1"},
			want: `{"representationId":"r-1","organizationId":"org-1","personId":"p-1","powerType":"attorney_in_fact",
				"sourceType":"power_of_attorney","sourceEvidenceId":"ev-1","scopeCurrency":"BRL","scopeMaxAmountMinor":500000,
				"scopeActTypes":["sign","assign"],"validFrom":1767225600123456789,"validUntil":1767225600123456790,
				"verificationStatus":"declared","proposedByUserId":"u-1"}`,
		},
		{
			name:       "findBankAccountChange",
			permission: "identity:find-bank-account-change",
			method:     methodBankAccountChange,
			path:       "/identity/bank-account-changes/bac-1",
			query:      true,
			reply: answer(&servicev1.FindBankAccountChangeResponse{BankAccountChange: &servicev1.BankAccountChange{
				BankAccountChangeId:   "bac-1",
				OrganizationId:        "org-1",
				DestinationEvidenceId: "ev-9",
				RequestedByUserId:     "u-1",
				Status:                servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_REQUESTED,
				RequestedAt:           createdAt,
			}}),
			request: &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "bac-1"},
			want: `{"bankAccountChangeId":"bac-1","organizationId":"org-1","destinationEvidenceId":"ev-9",
				"requestedByUserId":"u-1","status":"requested","requestedAt":1767225600123456789}`,
		},
		{
			name:       "findEstablishmentsByOrganization",
			permission: "identity:find-establishments-by-organization",
			method:     methodEstablishmentsByOrganization,
			path:       "/identity/organizations/org-1/establishments",
			empty:      `[]`,
			reply: answer(&servicev1.FindEstablishmentsByOrganizationResponse{Establishments: []*servicev1.Establishment{
				{
					EstablishmentId:     "est-1",
					OrganizationId:      "org-1",
					TaxIdType:           servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
					TaxId:               "12345678000276",
					TaxIdHash:           "hash-2",
					TaxIdHashKeyVersion: 3,
					TaxIdMasked:         "12.***.***/0002-76",
					TradeName:           "Acme Filial",
					Receiver:            true,
					ExternalReferences: []*servicev1.ExternalReference{
						{ProviderId: "prov-1", System: servicev1.ExternalSystem_EXTERNAL_SYSTEM_ACQUIRER, ReferenceType: servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_ASSET, Reference: " MID 01 ", AsOf: 7, ObservedVersion: "v2"},
						{ProviderId: "prov-2", System: servicev1.ExternalSystem_EXTERNAL_SYSTEM_DEPOSITORY, ReferenceType: servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_SETTLEMENT, Reference: "ref-2"},
					},
					CreatedAt: createdAt,
				},
				{
					EstablishmentId: "est-2",
					OrganizationId:  "org-1",
					TaxIdType:       servicev1.TaxIdType_TAX_ID_TYPE_CPF,
				},
			}}),
			request: &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "org-1"},
			want: `[{"establishmentId":"est-1","organizationId":"org-1","taxIdType":"cnpj","taxId":"12345678000276",
				"taxIdHash":"hash-2","taxIdHashKeyVersion":3,"taxIdMasked":"12.***.***/0002-76","tradeName":"Acme Filial",
				"receiver":true,"externalReferences":[
					{"providerId":"prov-1","system":"acquirer","referenceType":"asset","reference":" MID 01 ","asOf":7,"observedVersion":"v2"},
					{"providerId":"prov-2","system":"depository","referenceType":"settlement","reference":"ref-2","asOf":0}],
				"createdAt":1767225600123456789},
				{"establishmentId":"est-2","organizationId":"org-1","taxIdType":"cpf","taxId":"","taxIdHash":"",
				"taxIdHashKeyVersion":0,"taxIdMasked":"","receiver":false,"externalReferences":[],"createdAt":0}]`,
		},
		{
			name:       "findRepresentationsByOrganization",
			permission: "identity:find-representations-by-organization",
			method:     methodRepresentationsByOrganization,
			path:       "/identity/organizations/org-1/representations",
			empty:      `[]`,
			reply: answer(&servicev1.FindRepresentationsByOrganizationResponse{Representations: []*servicev1.Representation{{
				RepresentationId:   "r-2",
				OrganizationId:     "org-1",
				PersonId:           "p-2",
				PowerType:          servicev1.PowerType_POWER_TYPE_LEGAL_REPRESENTATIVE,
				SourceType:         servicev1.SourceType_SOURCE_TYPE_ARTICLES_OF_ASSOCIATION,
				SourceEvidenceId:   "ev-2",
				ValidFrom:          createdAt,
				VerificationStatus: servicev1.RepresentationVerificationStatus_REPRESENTATION_VERIFICATION_STATUS_DECLARED,
				ProposedByUserId:   "u-1",
			}}}),
			request: &servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: "org-1"},
			want: `[{"representationId":"r-2","organizationId":"org-1","personId":"p-2","powerType":"legal_representative",
				"sourceType":"articles_of_association","sourceEvidenceId":"ev-2","validFrom":1767225600123456789,
				"verificationStatus":"declared","proposedByUserId":"u-1"}]`,
		},
		{
			name:       "findMembershipsByOrganization",
			permission: "identity:find-memberships-by-organization",
			method:     methodMembershipsByOrganization,
			path:       "/identity/organizations/org-1/memberships",
			empty:      `[]`,
			reply: answer(&servicev1.FindMembershipsByOrganizationResponse{Memberships: []*servicev1.Membership{
				{
					MembershipId:        "m-1",
					OrganizationId:      "org-1",
					InviteeEmail:        "ana@acme.test",
					Role:                servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR,
					Status:              servicev1.MembershipStatus_MEMBERSHIP_STATUS_INVITED,
					InvitedByUserId:     "u-1",
					InvitedAt:           createdAt,
					InvitationExpiresAt: createdAt + 10,
				},
				{
					MembershipId:        "m-2",
					OrganizationId:      "org-1",
					InviteeEmail:        "rui@acme.test",
					Role:                servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR,
					Status:              servicev1.MembershipStatus_MEMBERSHIP_STATUS_REVOKED,
					InvitedByUserId:     "u-1",
					InvitedAt:           1,
					InvitationExpiresAt: 2,
					UserId:              "u-7",
					PersonId:            "p-7",
					VerificationLevel:   servicev1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED,
					GrantedAt:           3,
					RevokedAt:           4,
					RevocationReason:    "left",
				},
			}}),
			request: &servicev1.FindMembershipsByOrganizationRequest{OrganizationId: "org-1"},
			want: `[{"membershipId":"m-1","organizationId":"org-1","inviteeEmail":"ana@acme.test","role":"operator",
				"status":"invited","invitedByUserId":"u-1","invitedAt":1767225600123456789,"invitationExpiresAt":1767225600123456799},
				{"membershipId":"m-2","organizationId":"org-1","inviteeEmail":"rui@acme.test","role":"auditor","status":"revoked",
				"invitedByUserId":"u-1","invitedAt":1,"invitationExpiresAt":2,"userId":"u-7","personId":"p-7",
				"verificationLevel":"personal_data_checked","grantedAt":3,"revokedAt":4,"revocationReason":"left"}]`,
		},
		{
			name:       "findBankAccountChangesByOrganization",
			permission: "identity:find-bank-account-changes-by-organization",
			method:     methodBankAccountChangesByOrganization,
			path:       "/identity/organizations/org-1/bank-account-changes",
			empty:      `[]`,
			reply: answer(&servicev1.FindBankAccountChangesByOrganizationResponse{BankAccountChanges: []*servicev1.BankAccountChange{{
				BankAccountChangeId:   "bac-2",
				OrganizationId:        "org-1",
				DestinationEvidenceId: "ev-3",
				RequestedByUserId:     "u-1",
				ApprovedByUserId:      "u-2",
				Status:                servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED,
				RequestedAt:           5,
				ApprovedAt:            6,
			}}}),
			request: &servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: "org-1"},
			want: `[{"bankAccountChangeId":"bac-2","organizationId":"org-1","destinationEvidenceId":"ev-3","requestedByUserId":"u-1",
				"approvedByUserId":"u-2","status":"approved","requestedAt":5,"approvedAt":6}]`,
		},
		{
			name:       "findMembershipsByUser",
			permission: "identity:find-memberships-by-user",
			method:     methodMembershipsByUser,
			path:       "/identity/users/u-9/memberships",
			empty:      `[]`,
			reply: answer(&servicev1.FindMembershipsByUserResponse{Memberships: []*servicev1.Membership{{
				MembershipId:        "m-3",
				OrganizationId:      "org-2",
				InviteeEmail:        "u9@acme.test",
				Role:                servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR,
				Status:              servicev1.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE,
				InvitedByUserId:     "u-1",
				InvitedAt:           1,
				InvitationExpiresAt: 2,
				UserId:              otherUser,
				PersonId:            "p-9",
				VerificationLevel:   servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP,
				GrantedAt:           3,
			}}}),
			request: &servicev1.FindMembershipsByUserRequest{UserId: otherUser},
			want: `[{"membershipId":"m-3","organizationId":"org-2","inviteeEmail":"u9@acme.test","role":"operator","status":"active",
				"invitedByUserId":"u-1","invitedAt":1,"invitationExpiresAt":2,"userId":"u-9","personId":"p-9",
				"verificationLevel":"channel_ownership","grantedAt":3}]`,
		},
		{
			name:       "findEffectivePermissions",
			permission: "identity:find-effective-permissions",
			method:     methodPermissions,
			path:       "/identity/users/u-9/permissions",
			empty:      `{"permissions":[]}`,
			prepare: func(f *fakeIdentity) {
				f.grantTo(otherUser, "identity:find-bank-account-change", "identity:request-bank-account-change")
			},
			request: &servicev1.FindEffectivePermissionsRequest{UserId: otherUser},
			want:    `{"permissions":["identity:find-bank-account-change","identity:request-bank-account-change"]}`,
		},
	}
}

func identityEdge(t *testing.T, fake *fakeIdentity, options ...option) fixture {
	t.Helper()
	identity := fake.dial(t, obsclock.System())
	return newFixture(t, append([]option{withIdentity(identity), withAuthenticator(app.NewPermissions(authn.DevAuthenticator{}, identity))}, options...)...)
}

func requireJSON(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	got, expected := jsonOf(t, rec.Body.Bytes()), jsonOf(t, []byte(want))
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("body = %s, want %s", rec.Body.String(), want)
	}
}

func jsonOf(t *testing.T, raw []byte) any {
	t.Helper()
	stream := json.NewDecoder(bytes.NewReader(raw))
	stream.UseNumber()
	var v any
	if err := stream.Decode(&v); err != nil {
		t.Fatalf("%s is not JSON: %v", raw, err)
	}
	return v
}

func requireHops(t *testing.T, fake *fakeIdentity, method string, want int) []hop {
	t.Helper()
	hops := fake.hopsTo(method)
	if len(hops) != want {
		t.Fatalf("%s calls = %d, want %d", method, len(hops), want)
	}
	return hops
}

func TestEachIdentityRouteAnswersTheViewOfTheContract(t *testing.T) {
	for _, route := range identityRoutes() {
		t.Run(route.name, func(t *testing.T) {
			fake := (&fakeIdentity{}).grant(route.permission).replying(route.method, route.reply)
			if route.prepare != nil {
				route.prepare(fake)
			}
			f := identityEdge(t, fake)

			requireJSON(t, f.do(t, http.MethodGet, route.path, nil), route.want)

			calls := 1
			if route.method == methodPermissions {
				calls = 2
			}
			hops := requireHops(t, fake, route.method, calls)
			routed := hops[len(hops)-1]
			if !proto.Equal(routed.req.(proto.Message), route.request) {
				t.Fatalf("%s request = %v, want %v", route.method, routed.req, route.request)
			}
			if tenant := routed.md.Get(kernelgrpc.TenantKey); len(tenant) != 1 || tenant[0] != testTenant {
				t.Fatalf("%s tenant = %v, want the tenant of the request", route.method, tenant)
			}
			permissions := requireHops(t, fake, methodPermissions, calls)
			if asked := permissions[0].req.(*servicev1.FindEffectivePermissionsRequest).GetUserId(); asked != "tester" {
				t.Fatalf("the edge asked the permissions of %q, want the subject of the token", asked)
			}
		})
	}
}

func TestAQueryForAnAbsentResourceIs404(t *testing.T) {
	for _, route := range identityRoutes() {
		if !route.query {
			continue
		}
		t.Run(route.name, func(t *testing.T) {
			fake := (&fakeIdentity{}).grant(route.permission).replying(route.method, func(any) (any, error) {
				return nil, status.Error(codes.NotFound, "absent")
			})
			f := identityEdge(t, fake)

			requireRejection(t, f.do(t, http.MethodGet, route.path, nil), http.StatusNotFound, "not-found")
		})
	}
}

func TestAnIdentityRouteRefusedAtTheEdgeNeverReachesTheContext(t *testing.T) {
	cases := []struct {
		name        string
		headers     []string
		code        string
		permissions int
	}{
		{name: "without the permission", code: "permission-denied", permissions: 1},
		{name: "asserting another tenant", headers: []string{"X-Tenant-ID", "globex"}, code: "identity-mismatch", permissions: 1},
		{name: "without a tenant", headers: []string{"Authorization", tenantlessSubject}, code: "tenant-unresolved", permissions: 0},
	}
	for _, route := range identityRoutes() {
		for _, tc := range cases {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				granted := route.permission
				if tc.code == "permission-denied" {
					granted = "identity:another-operation"
				}
				fake := (&fakeIdentity{}).grant(granted).replying(route.method, route.reply)
				f := identityEdge(t, fake)

				requireRejection(t, f.do(t, http.MethodGet, route.path, nil, tc.headers...), http.StatusForbidden, tc.code)
				requireHops(t, fake, methodPermissions, tc.permissions)
				if route.method != methodPermissions {
					requireHops(t, fake, route.method, 0)
				}
			})
		}
	}
}

func TestAPathIDOutsideTheContractIs400WithoutCallingTheContext(t *testing.T) {
	fake := (&fakeIdentity{}).grant("identity:find-organization")
	f := identityEdge(t, fake)

	requireRejection(t, f.do(t, http.MethodGet, "/identity/organizations/org%201", nil), http.StatusBadRequest, "invalid-request")
	requireHops(t, fake, methodOrganization, 0)
}

func TestAnAnswerTheContractCannotRepresentIs500(t *testing.T) {
	cases := []struct {
		name   string
		answer *servicev1.FindOrganizationResponse
	}{
		{name: "no organization in the envelope", answer: &servicev1.FindOrganizationResponse{}},
		{name: "unspecified status", answer: &servicev1.FindOrganizationResponse{Organization: &servicev1.Organization{
			OrganizationId: "org-1", TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
		}}},
		{name: "a kind the contract does not declare", answer: &servicev1.FindOrganizationResponse{Organization: &servicev1.Organization{
			OrganizationId: "org-1", TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
			Status: servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE, Kinds: []servicev1.OrganizationKind{99},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := (&fakeIdentity{}).grant("identity:find-organization").replying(methodOrganization, answer(tc.answer))
			f := identityEdge(t, fake)

			requireRejection(t, f.do(t, http.MethodGet, "/identity/organizations/org-1", nil), http.StatusInternalServerError, "internal-failure")
		})
	}
}

func TestTheOptionalFieldsOfAViewAppearOnceTheContextFillsThem(t *testing.T) {
	fake := (&fakeIdentity{}).grant("identity:find-bank-account-change").replying(methodBankAccountChange, answer(&servicev1.FindBankAccountChangeResponse{
		BankAccountChange: &servicev1.BankAccountChange{
			BankAccountChangeId: "bac-1", OrganizationId: "org-1", DestinationEvidenceId: "ev-9",
			RequestedByUserId: "u-1", ApprovedByUserId: "u-2",
			Status:      servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED,
			RequestedAt: 1, ApprovedAt: 2,
		},
	}))
	f := identityEdge(t, fake)

	requireJSON(t, f.do(t, http.MethodGet, "/identity/bank-account-changes/bac-1", nil),
		`{"bankAccountChangeId":"bac-1","organizationId":"org-1","destinationEvidenceId":"ev-9","requestedByUserId":"u-1",
		"approvedByUserId":"u-2","status":"approved","requestedAt":1,"approvedAt":2}`)
}

func TestAnEmptyListingIsAnEmptyArray(t *testing.T) {
	for _, route := range identityRoutes() {
		if route.empty == "" {
			continue
		}
		t.Run(route.name, func(t *testing.T) {
			fake := (&fakeIdentity{}).grant(route.permission).grantTo(otherUser)
			f := identityEdge(t, fake)

			rec := f.do(t, http.MethodGet, route.path, nil)

			requireJSON(t, rec, route.empty)
			if body := bytes.TrimSpace(rec.Body.Bytes()); bytes.Contains(body, []byte("null")) {
				t.Fatalf("body = %s, want an empty array, never null", body)
			}
		})
	}
}

func TestValidAtNarrowsTheRepresentationsToAnInstant(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  int64
	}{
		{name: "absent", query: "", want: 0},
		{name: "zero", query: "?validAt=0", want: 0},
		{name: "an instant", query: "?validAt=1767225600123456789", want: createdAt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := (&fakeIdentity{}).grant("identity:find-representations-by-organization")
			f := identityEdge(t, fake)

			requireJSON(t, f.do(t, http.MethodGet, "/identity/organizations/org-1/representations"+tc.query, nil), `[]`)

			hops := requireHops(t, fake, methodRepresentationsByOrganization, 1)
			want := &servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: "org-1", ValidAt: tc.want}
			if !proto.Equal(hops[0].req.(proto.Message), want) {
				t.Fatalf("request = %v, want %v", hops[0].req, want)
			}
		})
	}
}

func TestAValidAtOutsideTheContractIs400WithoutCallingTheContext(t *testing.T) {
	for _, value := range []string{"-1", "abc", "1.5", "", "9223372036854775808"} {
		t.Run(value, func(t *testing.T) {
			fake := (&fakeIdentity{}).grant("identity:find-representations-by-organization")
			f := identityEdge(t, fake)

			rec := f.do(t, http.MethodGet, "/identity/organizations/org-1/representations?validAt="+value, nil)

			requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
			requireHops(t, fake, methodRepresentationsByOrganization, 0)
		})
	}
}

func TestAnElementTheContractCannotRepresentFailsTheWholeListing(t *testing.T) {
	fake := (&fakeIdentity{}).grant("identity:find-memberships-by-organization").replying(methodMembershipsByOrganization,
		answer(&servicev1.FindMembershipsByOrganizationResponse{Memberships: []*servicev1.Membership{{
			MembershipId: "m-1", Role: servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR,
			Status: servicev1.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE, VerificationLevel: 99,
		}}}))
	f := identityEdge(t, fake)

	rec := f.do(t, http.MethodGet, "/identity/organizations/org-1/memberships", nil)

	requireRejection(t, rec, http.StatusInternalServerError, "internal-failure")
}

func TestTheInventoryNamesEveryIdentityRouteUnderTest(t *testing.T) {
	var tested []string
	for _, route := range identityRoutes() {
		tested = append(tested, route.name)
	}
	tested = append(tested, commandRoutesUnderTest...)
	var inventory []string
	for name := range productionRoutes {
		inventory = append(inventory, name)
	}
	slices.Sort(tested)
	slices.Sort(inventory)
	if !slices.Equal(tested, inventory) {
		t.Fatalf("routes under test = %v, inventory = %v", tested, inventory)
	}
}
