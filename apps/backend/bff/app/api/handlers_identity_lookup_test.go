package api_test

import (
	"net/http"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func organizationLookup() identityCommand {
	return identityCommand{
		name:       "findOrganizationByTaxId",
		permission: "identity:find-organization-by-tax-id",
		method:     methodOrganizationByTaxID,
		path:       "/identity/organization-lookups",
		body:       map[string]any{"taxIdType": "cnpj", "taxId": "12345678000195", "jurisdiction": "BR"},
		reply: answer(&servicev1.FindOrganizationByTaxIdResponse{Organization: &servicev1.Organization{
			OrganizationId:      "org-1",
			Kinds:               []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER},
			Jurisdiction:        "BR",
			TaxIdType:           servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
			TaxId:               "12345678000195",
			TaxIdHash:           "hash-1",
			TaxIdHashKeyVersion: 2,
			TaxIdMasked:         "12.***.***/0001-95",
			LegalName:           "Acme Ltda",
			Status:              servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE,
			CreatedAt:           createdAt,
		}}),
		request: &servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ, TaxId: "12345678000195", Jurisdiction: "BR"},
		status:  http.StatusOK,
		want: `{"organizationId":"org-1","kinds":["funder"],"jurisdiction":"BR","taxIdType":"cnpj","taxId":"12345678000195",
			"taxIdHash":"hash-1","taxIdHashKeyVersion":2,"taxIdMasked":"12.***.***/0001-95","legalName":"Acme Ltda",
			"status":"active","createdAt":1767225600123456789}`,
	}
}

func TestALookupAnswersTheOrganizationWithoutForwardingTheKeyNorMarkingAReplay(t *testing.T) {
	lookup := organizationLookup()
	f, fake := lookup.edge(t, lookup.reply, metadata.Pairs(kernelgrpc.ReplayedHeader, "true"))

	rec := lookup.send(t, f, lookup.path, lookup.baseBody(t))

	requireAnswer(t, rec, http.StatusOK, lookup.want)
	if got := rec.Header().Get(api.ReplayedHeader); got != "" {
		t.Fatalf("%s = %q on a lookup, want none: the context neither stores nor replays a read", api.ReplayedHeader, got)
	}
	routed := requireHops(t, fake, lookup.method, 1)[0]
	if !proto.Equal(routed.req.(proto.Message), lookup.request) {
		t.Fatalf("request = %v, want %v", routed.req, lookup.request)
	}
	if key := routed.md.Get(kernelgrpc.IdempotencyKey); len(key) != 0 {
		t.Fatalf("%s = %v on a lookup, want none", kernelgrpc.IdempotencyKey, key)
	}
}

func TestALookupOfAnAbsentOrganizationIs404(t *testing.T) {
	lookup := organizationLookup()
	f, fake := lookup.edge(t, failing(status.Error(codes.NotFound, "no organization with this tax id")), nil)

	requireRejection(t, lookup.send(t, f, lookup.path, lookup.baseBody(t)), http.StatusNotFound, "not-found")
	requireHops(t, fake, lookup.method, 1)
}

func TestTheBodyOfALookupIsBoundByTheSchema(t *testing.T) {
	lookup := organizationLookup()
	body := func(edits ...edit) string { return bodyWith(t, lookup.body, edits...) }
	requireBoundsOf(t, lookup, []bound{
		{"a cpf", body(with("taxIdType", "cpf")),
			routedAs(func(r *servicev1.FindOrganizationByTaxIdRequest) bool {
				return r.GetTaxIdType() == servicev1.TaxIdType_TAX_ID_TYPE_CPF
			})},
		{"a tax id of 1 character", body(with("taxId", "1")), reaches},
		{"a tax id of 32 printable characters", body(with("taxId", printable32)), reaches},
		{"no tax id type", body(without("taxIdType")), nil},
		{"an unknown tax id type", body(with("taxIdType", "rg")), nil},
		{"no tax id", body(without("taxId")), nil},
		{"an empty tax id", body(with("taxId", "")), nil},
		{"a tax id of 33 characters", body(with("taxId", printable32+"3")), nil},
		{"a non-ASCII tax id", body(with("taxId", "1234é")), nil},
		{"a tax id with a control character", body(with("taxId", "1234\x7f")), nil},
		{"no jurisdiction", body(without("jurisdiction")), nil},
		{"an empty jurisdiction", body(with("jurisdiction", "")), nil},
		{"a lowercase jurisdiction", body(with("jurisdiction", "br")), nil},
		{"a jurisdiction of 3 letters", body(with("jurisdiction", "BRA")), nil},
		{"a jurisdiction of 1 letter", body(with("jurisdiction", "B")), nil},
		{"an unknown field", body(with("organizationId", "org-1")), nil},
	})
}
