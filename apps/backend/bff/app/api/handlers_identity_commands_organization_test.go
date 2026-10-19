package api_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const asOf = int64(1767225600123456789)

var (
	nonASCII256   = strings.Repeat("é", 256)
	printable32   = strings.Repeat("~", 32)
	identifier128 = strings.Repeat("i", 128)
)

func organizationBody() map[string]any {
	return map[string]any{
		"organizationId": "org-1", "kinds": []string{"client", "funder"}, "jurisdiction": "BR",
		"taxIdType": "cnpj", "taxId": "12345678000195", "legalName": "Acme Ltda", "tradeName": "Acme",
	}
}

func establishmentBody() map[string]any {
	return map[string]any{
		"establishmentId": "est-1", "taxIdType": "cpf", "taxId": "12345678909", "tradeName": "Loja", "receiver": true,
		"externalReferences": []any{externalReference()},
	}
}

func externalReference() map[string]any {
	return map[string]any{
		"providerId": "prov-1", "system": "acquirer", "referenceType": "asset", "reference": "ref-1", "asOf": asOf, "observedVersion": "v2",
	}
}

func organizationCommands() []identityCommand {
	return []identityCommand{
		{
			name:       "registerOrganization",
			permission: "identity:register-organization",
			method:     methodRegisterOrganization,
			path:       "/identity/organizations",
			body:       organizationBody(),
			reply: answer(&servicev1.RegisterOrganizationResponse{Result: &servicev1.RegisterOrganizationResponse_Registered{
				Registered: &servicev1.RegisteredOrganization{OrganizationId: "org-1"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.RegisterOrganizationResponse{Result: &servicev1.RegisterOrganizationResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.RegisterOrganizationRequest{
				OrganizationId: "org-1",
				Kinds:          []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT, servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER},
				Jurisdiction:   "BR",
				TaxIdType:      servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
				TaxId:          "12345678000195",
				LegalName:      "Acme Ltda",
				TradeName:      "Acme",
			},
			status: http.StatusCreated,
			want:   `{"organizationId":"org-1"}`,
		},
		{
			name:       "addEstablishment",
			permission: "identity:add-establishment",
			method:     methodAddEstablishment,
			path:       "/identity/organizations/org-1/establishments",
			badPath:    "/identity/organizations/org%201/establishments",
			body:       establishmentBody(),
			reply: answer(&servicev1.AddEstablishmentResponse{Result: &servicev1.AddEstablishmentResponse_Added{
				Added: &servicev1.AddedEstablishment{EstablishmentId: "est-1"},
			}}),
			rejection: func(r *servicev1.Rejection) proto.Message {
				return &servicev1.AddEstablishmentResponse{Result: &servicev1.AddEstablishmentResponse_Rejection{Rejection: r}}
			},
			request: &servicev1.AddEstablishmentRequest{
				EstablishmentId: "est-1",
				OrganizationId:  "org-1",
				TaxIdType:       servicev1.TaxIdType_TAX_ID_TYPE_CPF,
				TaxId:           "12345678909",
				TradeName:       "Loja",
				Receiver:        true,
				ExternalReferences: []*servicev1.ExternalReference{{
					ProviderId:      "prov-1",
					System:          servicev1.ExternalSystem_EXTERNAL_SYSTEM_ACQUIRER,
					ReferenceType:   servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_ASSET,
					Reference:       "ref-1",
					AsOf:            asOf,
					ObservedVersion: "v2",
				}},
			},
			status: http.StatusCreated,
			want:   `{"establishmentId":"est-1"}`,
		},
	}
}

func TestTheBodyOfARegisteredOrganizationIsBoundByTheSchema(t *testing.T) {
	body := func(edits ...edit) string { return bodyWith(t, organizationBody(), edits...) }
	organization := func(holds func(*servicev1.RegisterOrganizationRequest) bool) func(proto.Message) bool {
		return routedAs(holds)
	}
	everyKind := []servicev1.OrganizationKind{
		servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT, servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER,
		servicev1.OrganizationKind_ORGANIZATION_KIND_PARTNER, servicev1.OrganizationKind_ORGANIZATION_KIND_PLATFORM,
	}
	requireBounds(t, "registerOrganization", []bound{
		{"every kind", body(with("kinds", []string{"client", "funder", "partner", "platform"})),
			organization(func(r *servicev1.RegisterOrganizationRequest) bool { return slices.Equal(r.GetKinds(), everyKind) })},
		{"no kind goes to the domain", body(with("kinds", []string{})),
			organization(func(r *servicev1.RegisterOrganizationRequest) bool { return len(r.GetKinds()) == 0 })},
		{"a cpf", body(with("taxIdType", "cpf")),
			organization(func(r *servicev1.RegisterOrganizationRequest) bool {
				return r.GetTaxIdType() == servicev1.TaxIdType_TAX_ID_TYPE_CPF
			})},
		{"an empty jurisdiction goes to the domain", body(with("jurisdiction", "")),
			organization(func(r *servicev1.RegisterOrganizationRequest) bool { return r.GetJurisdiction() == "" })},
		{"a jurisdiction of 8 printable characters", body(with("jurisdiction", "BR-SP ~1")), reaches},
		{"an empty tax id goes to the domain", body(with("taxId", "")),
			organization(func(r *servicev1.RegisterOrganizationRequest) bool { return r.GetTaxId() == "" })},
		{"a tax id of 32 printable characters", body(with("taxId", printable32)), reaches},
		{"an empty legal name goes to the domain", body(with("legalName", "")),
			organization(func(r *servicev1.RegisterOrganizationRequest) bool { return r.GetLegalName() == "" })},
		{"a legal name of 256 non-ASCII characters", body(with("legalName", nonASCII256)), reaches},
		{"no trade name", body(without("tradeName")),
			organization(func(r *servicev1.RegisterOrganizationRequest) bool { return r.GetTradeName() == "" })},
		{"an organization id of 128 characters", body(with("organizationId", identifier128)), reaches},
		{"no organization id", body(without("organizationId")), nil},
		{"an empty organization id", body(with("organizationId", "")), nil},
		{"an organization id of 129 characters", body(with("organizationId", identifier128+"i")), nil},
		{"no kinds", body(without("kinds")), nil},
		{"null kinds", body(with("kinds", nil)), nil},
		{"five kinds", body(with("kinds", []string{"client", "funder", "partner", "platform", "client"})), nil},
		{"a repeated kind", body(with("kinds", []string{"client", "client"})), nil},
		{"an unknown kind", body(with("kinds", []string{"bank"})), nil},
		{"a kind out of case", body(with("kinds", []string{"CLIENT"})), nil},
		{"no jurisdiction", body(without("jurisdiction")), nil},
		{"a jurisdiction of 9 characters", body(with("jurisdiction", "BR-SP ~12")), nil},
		{"a non-ASCII jurisdiction", body(with("jurisdiction", "BRé")), nil},
		{"a jurisdiction with a control character", body(with("jurisdiction", "B\x07")), nil},
		{"no tax id type", body(without("taxIdType")), nil},
		{"a tax id type out of case", body(with("taxIdType", "CNPJ")), nil},
		{"an unknown tax id type", body(with("taxIdType", "rg")), nil},
		{"no tax id", body(without("taxId")), nil},
		{"a tax id of 33 characters", body(with("taxId", printable32+"3")), nil},
		{"a non-ASCII tax id", body(with("taxId", "1234é")), nil},
		{"no legal name", body(without("legalName")), nil},
		{"a legal name of 257 characters", body(with("legalName", nonASCII256+"é")), nil},
		{"a legal name with a control character", body(with("legalName", "Acme\u0085")), nil},
		{"a trade name of 257 characters", body(with("tradeName", strings.Repeat("t", 257))), nil},
		{"a trade name with a control character", body(with("tradeName", "Acme\x7f")), nil},
		{"an unknown field", body(with("status", "active")), nil},
	})
}

func TestTheBodyOfAnAddedEstablishmentIsBoundByTheSchema(t *testing.T) {
	body := func(edits ...edit) string { return bodyWith(t, establishmentBody(), edits...) }
	reference := func(edits ...edit) map[string]any { return cloned(t, externalReference(), edits...) }
	references := func(items ...any) edit { return with("externalReferences", items) }
	establishment := func(holds func(*servicev1.AddEstablishmentRequest) bool) func(proto.Message) bool {
		return routedAs(holds)
	}
	everySystem := []any{
		reference(with("system", "registry"), with("referenceType", "asset")),
		reference(with("system", "bookkeeper"), with("referenceType", "effect")),
		reference(with("system", "acquirer"), with("referenceType", "authorization")),
		reference(with("system", "funder"), with("referenceType", "command")),
		reference(with("system", "depository"), with("referenceType", "settlement")),
	}
	requireBounds(t, "addEstablishment", []bound{
		{"every system and reference type", body(references(everySystem...)),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool {
				var systems []servicev1.ExternalSystem
				var types []servicev1.ExternalReferenceType
				for _, ref := range r.GetExternalReferences() {
					systems = append(systems, ref.GetSystem())
					types = append(types, ref.GetReferenceType())
				}
				return slices.Equal(systems, []servicev1.ExternalSystem{
					servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY, servicev1.ExternalSystem_EXTERNAL_SYSTEM_BOOKKEEPER,
					servicev1.ExternalSystem_EXTERNAL_SYSTEM_ACQUIRER, servicev1.ExternalSystem_EXTERNAL_SYSTEM_FUNDER,
					servicev1.ExternalSystem_EXTERNAL_SYSTEM_DEPOSITORY,
				}) && slices.Equal(types, []servicev1.ExternalReferenceType{
					servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_ASSET, servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_EFFECT,
					servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_AUTHORIZATION, servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_COMMAND,
					servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_SETTLEMENT,
				})
			})},
		{"a cnpj", body(with("taxIdType", "cnpj")),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool {
				return r.GetTaxIdType() == servicev1.TaxIdType_TAX_ID_TYPE_CNPJ
			})},
		{"not a receiver", body(with("receiver", false)),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool { return !r.GetReceiver() })},
		{"no external reference", body(without("externalReferences")),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool { return len(r.GetExternalReferences()) == 0 })},
		{"an empty list of external references", body(with("externalReferences", []any{})), reaches},
		{"null external references", body(with("externalReferences", nil)), nil},
		{"32 external references", body(with("externalReferences", repeated(32, any(externalReference())))),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool { return len(r.GetExternalReferences()) == 32 })},
		{"a reference without observed version", body(references(reference(without("observedVersion")))),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool {
				return r.GetExternalReferences()[0].GetObservedVersion() == ""
			})},
		{"an empty provider goes to the domain", body(references(reference(with("providerId", "")))),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool {
				return r.GetExternalReferences()[0].GetProviderId() == ""
			})},
		{"texts of 256 non-ASCII characters", body(references(reference(with("providerId", nonASCII256), with("reference", nonASCII256), with("observedVersion", nonASCII256)))), reaches},
		{"an empty tax id goes to the domain", body(with("taxId", "")),
			establishment(func(r *servicev1.AddEstablishmentRequest) bool { return r.GetTaxId() == "" })},
		{"no trade name", body(without("tradeName")), reaches},
		{"no establishment id", body(without("establishmentId")), nil},
		{"an establishment id outside the pattern", body(with("establishmentId", "est 1")), nil},
		{"no tax id type", body(without("taxIdType")), nil},
		{"an unknown tax id type", body(with("taxIdType", "rg")), nil},
		{"no tax id", body(without("taxId")), nil},
		{"a tax id of 33 characters", body(with("taxId", printable32+"3")), nil},
		{"a trade name of 257 characters", body(with("tradeName", strings.Repeat("t", 257))), nil},
		{"no receiver", body(without("receiver")), nil},
		{"a null receiver", body(with("receiver", nil)), nil},
		{"a receiver as text", body(with("receiver", "true")), nil},
		{"33 external references", body(with("externalReferences", repeated(33, any(externalReference())))), nil},
		{"a null external reference", body(references(nil)), nil},
		{"a reference without provider", body(references(reference(without("providerId")))), nil},
		{"a reference without system", body(references(reference(without("system")))), nil},
		{"a reference without type", body(references(reference(without("referenceType")))), nil},
		{"a reference without value", body(references(reference(without("reference")))), nil},
		{"a reference without instant", body(references(reference(without("asOf")))), nil},
		{"an unknown system", body(references(reference(with("system", "bank")))), nil},
		{"an unknown reference type", body(references(reference(with("referenceType", "invoice")))), nil},
		{"a provider of 257 characters", body(references(reference(with("providerId", nonASCII256+"é")))), nil},
		{"a reference of 257 characters", body(references(reference(with("reference", strings.Repeat("r", 257))))), nil},
		{"an observed version of 257 characters", body(references(reference(with("observedVersion", strings.Repeat("v", 257))))), nil},
		{"a provider with a control character", body(references(reference(with("providerId", "prov\x00")))), nil},
		{"an instant as text", body(references(reference(with("asOf", "1767225600")))), nil},
		{"a fractional instant", body(references(reference(with("asOf", 1.5)))), nil},
		{"an unknown field in a reference", body(references(reference(with("tenant", "acme")))), nil},
		{"the organization in the body", body(with("organizationId", "org-2")), nil},
	})
}
