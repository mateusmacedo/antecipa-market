package rpc_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	alphanumericCNPJ = "12ABC34501DE35"
	syntheticCPF     = "123.456.789-09"
)

func registerRequest(organizationID string) *servicev1.RegisterOrganizationRequest {
	return &servicev1.RegisterOrganizationRequest{
		OrganizationId: organizationID,
		Kinds:          []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER, servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT},
		Jurisdiction:   "BR",
		TaxIdType:      servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxId:          alphanumericCNPJ,
		LegalName:      "Recebiveis Sinteticos Ltda",
		TradeName:      "Sintetica",
	}
}

func register(t *testing.T, h *harness, req *servicev1.RegisterOrganizationRequest) *servicev1.RegisterOrganizationResponse {
	t.Helper()
	var resp servicev1.RegisterOrganizationResponse
	if err := h.invoke(withTenant(t), "RegisterOrganization", req, &resp); err != nil {
		t.Fatalf("RegisterOrganization(%s) = %v", req.GetOrganizationId(), err)
	}
	return &resp
}

func TestServiceDescCoversEveryMethodOfTheDescriptor(t *testing.T) {
	service := servicev1.File_company_identity_service_v1_identity_service_proto.Services().ByName("IdentityService")

	if rpc.ServiceDesc.ServiceName != string(service.FullName()) {
		t.Fatalf("ServiceName = %q, want %q", rpc.ServiceDesc.ServiceName, service.FullName())
	}
	if missing := kernelgrpc.Uncovered(&rpc.ServiceDesc, service); len(missing) != 0 || len(rpc.ServiceDesc.Streams) != 0 {
		t.Fatalf("ServiceDesc leaves %v uncovered and declares %d streams, want every unary method of the descriptor",
			missing, len(rpc.ServiceDesc.Streams))
	}
}

func TestTheServerImplementsTheHandlerType(t *testing.T) {
	handlerType := reflect.TypeOf(rpc.ServiceDesc.HandlerType).Elem()
	if handlerType.Kind() != reflect.Interface || !reflect.TypeOf(rpc.Server{}).Implements(handlerType) {
		t.Fatalf("rpc.Server does not implement %v", handlerType)
	}
}

func TestRegisterOrganizationAnswersTheRegisteredOrganizationAndFindReadsIt(t *testing.T) {
	h := newHarness(t, nil)

	resp := register(t, h, registerRequest("org-1"))

	if got := resp.GetRegistered().GetOrganizationId(); got != "org-1" {
		t.Fatalf("Registered.OrganizationId = %q, want org-1 (response %v)", got, resp)
	}
	var found servicev1.FindOrganizationResponse
	if err := h.invoke(withTenant(t), "FindOrganization", &servicev1.FindOrganizationRequest{OrganizationId: "org-1"}, &found); err != nil {
		t.Fatalf("FindOrganization() = %v", err)
	}
	o := found.GetOrganization()
	wantKinds := []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT, servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER}
	if !reflect.DeepEqual(o.GetKinds(), wantKinds) || o.GetJurisdiction() != "BR" || o.GetTaxIdType() != servicev1.TaxIdType_TAX_ID_TYPE_CNPJ {
		t.Fatalf("FindOrganization() = %v, want client and funder, BR, CNPJ", o)
	}
	if o.GetTaxId() != alphanumericCNPJ || o.GetTaxIdHash() != hashOf(testTenant, domain.TaxIDTypeCNPJ, alphanumericCNPJ) || o.GetTaxIdHashKeyVersion() != 1 {
		t.Fatalf("FindOrganization() = %v, want the tax id, its hash in the tenant and key version 1", o)
	}
	if o.GetTaxIdMasked() != "**.***.***/**DE-35" || o.GetLegalName() != "Recebiveis Sinteticos Ltda" || o.GetTradeName() != "Sintetica" {
		t.Fatalf("FindOrganization() = %v, want the mask and both names", o)
	}
	if o.GetStatus() != servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE || o.GetCreatedAt() != int64(occurred) {
		t.Fatalf("FindOrganization() = %v, want ACTIVE created at the domain instant %d in nanoseconds", o, occurred)
	}
}

func TestRegisterOrganizationAnswersEachRefusalInTheBody(t *testing.T) {
	planted := byTaxIDHash{hashOf(testTenant, domain.TaxIDTypeCPF, syntheticCPF): {{ID: "org-held", Jurisdiction: "BR"}}}
	cases := []struct {
		name string
		edit func(*servicev1.RegisterOrganizationRequest)
		code string
	}{
		{"kinds empty", func(r *servicev1.RegisterOrganizationRequest) { r.Kinds = nil }, "identity-organization/organization/kinds-empty"},
		{"jurisdiction of three letters", func(r *servicev1.RegisterOrganizationRequest) { r.Jurisdiction = "BRA" }, "identity-organization/organization/jurisdiction-invalid"},
		{"tax id empty", func(r *servicev1.RegisterOrganizationRequest) { r.TaxId = "" }, "identity-organization/organization/tax-id-empty"},
		{"legal name blank", func(r *servicev1.RegisterOrganizationRequest) { r.LegalName = "  " }, "identity-organization/organization/legal-name-empty"},
		{"tax id held in the jurisdiction", func(r *servicev1.RegisterOrganizationRequest) {
			r.TaxIdType, r.TaxId = servicev1.TaxIdType_TAX_ID_TYPE_CPF, syntheticCPF
		}, "identity-organization/organization/duplicate-tax-id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, planted)
			req := registerRequest("org-refused")
			c.edit(req)

			resp := register(t, h, req)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
			if len(h.store.Entries()) != 0 {
				t.Fatalf("outbox holds %d entries after a refusal, want none", len(h.store.Entries()))
			}
		})
	}
}

func TestRegisteringTheSameOrganizationUnderAnotherKeyAlreadyExists(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))

	var resp servicev1.RegisterOrganizationResponse
	err := h.invoke(withTenant(t), "RegisterOrganization", registerRequest("org-1"), &resp)

	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("second RegisterOrganization() = %v, want AlreadyExists: a retry that never converges is not a conflict", err)
	}
}

func TestFindOrganizationByTaxIDMapsTheOrganizationOfTheJurisdiction(t *testing.T) {
	h := newHarness(t, byTaxIDHash{hashOf(testTenant, domain.TaxIDTypeCNPJ, alphanumericCNPJ): {
		{ID: "org-pt", Jurisdiction: "PT", TaxIDType: domain.TaxIDTypeCNPJ, Status: domain.OrganizationActive},
		{ID: "org-br", Jurisdiction: "BR", TaxIDType: domain.TaxIDTypeCNPJ, Status: domain.OrganizationActive, Kinds: domain.KindsOf(domain.KindPartner)},
	}})

	var resp servicev1.FindOrganizationByTaxIdResponse
	err := h.invoke(withTenant(t), "FindOrganizationByTaxId",
		&servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ, TaxId: "12.abc.345/01de-35", Jurisdiction: "BR"}, &resp)

	if err != nil {
		t.Fatalf("FindOrganizationByTaxId() = %v", err)
	}
	o := resp.GetOrganization()
	if o.GetOrganizationId() != "org-br" || len(o.GetKinds()) != 1 || o.GetKinds()[0] != servicev1.OrganizationKind_ORGANIZATION_KIND_PARTNER {
		t.Fatalf("FindOrganizationByTaxId() = %v, want org-br, a partner", o)
	}
}

func TestFindingAnUnknownOrganizationIsNotFound(t *testing.T) {
	h := newHarness(t, nil)

	var byID servicev1.FindOrganizationResponse
	errByID := h.invoke(withTenant(t), "FindOrganization", &servicev1.FindOrganizationRequest{OrganizationId: "org-none"}, &byID)
	var byTaxID servicev1.FindOrganizationByTaxIdResponse
	errByTaxID := h.invoke(withTenant(t), "FindOrganizationByTaxId",
		&servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CPF, TaxId: syntheticCPF, Jurisdiction: "BR"}, &byTaxID)

	if status.Code(errByID) != codes.NotFound || status.Code(errByTaxID) != codes.NotFound {
		t.Fatalf("FindOrganization() = %v and FindOrganizationByTaxId() = %v, want NotFound for both", errByID, errByTaxID)
	}
}

func TestMalformedRequestsAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, nil)
	long := strings.Repeat("a", 257)

	register := func(edit func(*servicev1.RegisterOrganizationRequest)) func() error {
		return func() error {
			req := registerRequest("org-1")
			edit(req)
			var r servicev1.RegisterOrganizationResponse
			return h.invoke(withTenant(t), "RegisterOrganization", req, &r)
		}
	}
	byTaxID := func(req *servicev1.FindOrganizationByTaxIdRequest) func() error {
		return func() error {
			var r servicev1.FindOrganizationByTaxIdResponse
			return h.invoke(withTenant(t), "FindOrganizationByTaxId", req, &r)
		}
	}
	calls := []struct {
		name string
		call func() error
	}{
		{"organization id with a space", register(func(r *servicev1.RegisterOrganizationRequest) { r.OrganizationId = "org 1" })},
		{"organization id empty", register(func(r *servicev1.RegisterOrganizationRequest) { r.OrganizationId = "" })},
		{"unspecified kind", register(func(r *servicev1.RegisterOrganizationRequest) {
			r.Kinds = []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_UNSPECIFIED}
		})},
		{"kind outside the enum", register(func(r *servicev1.RegisterOrganizationRequest) { r.Kinds = []servicev1.OrganizationKind{99} })},
		{"unspecified tax id type", register(func(r *servicev1.RegisterOrganizationRequest) {
			r.TaxIdType = servicev1.TaxIdType_TAX_ID_TYPE_UNSPECIFIED
		})},
		{"tax id too long", register(func(r *servicev1.RegisterOrganizationRequest) { r.TaxId = strings.Repeat("1", 33) })},
		{"jurisdiction too long", register(func(r *servicev1.RegisterOrganizationRequest) { r.Jurisdiction = "BRAZILIAN" })},
		{"legal name too long", register(func(r *servicev1.RegisterOrganizationRequest) { r.LegalName = long })},
		{"trade name too long", register(func(r *servicev1.RegisterOrganizationRequest) { r.TradeName = long })},
		{"organization to find with a space", func() error {
			var r servicev1.FindOrganizationResponse
			return h.invoke(withTenant(t), "FindOrganization", &servicev1.FindOrganizationRequest{OrganizationId: "org 1"}, &r)
		}},
		{"tax id to find under an unspecified type", byTaxID(&servicev1.FindOrganizationByTaxIdRequest{TaxId: syntheticCPF, Jurisdiction: "BR"})},
		{"empty tax id to find", byTaxID(&servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CPF, Jurisdiction: "BR"})},
		{"jurisdiction to find in lower case", byTaxID(&servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CPF, TaxId: syntheticCPF, Jurisdiction: "br"})},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if len(h.store.Entries()) != 0 {
		t.Fatalf("outbox holds %d entries, want none: no malformed call reached the use case", len(h.store.Entries()))
	}
}

const (
	referenceAsOf = int64(1_755_431_000_000_000_000)
	oneYear       = int64(365 * 24 * 3600 * 1_000_000_000)
	personHash    = "acme|person-document|12345678909"
)

func addRequest(establishmentID, organizationID string) *servicev1.AddEstablishmentRequest {
	return &servicev1.AddEstablishmentRequest{
		EstablishmentId: establishmentID,
		OrganizationId:  organizationID,
		TaxIdType:       servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxId:           alphanumericCNPJ,
		TradeName:       "Loja Sintetica",
		Receiver:        true,
		ExternalReferences: []*servicev1.ExternalReference{{
			ProviderId:      "partner-registry",
			System:          servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY,
			ReferenceType:   servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_ASSET,
			Reference:       "AST-0001/a",
			AsOf:            referenceAsOf,
			ObservedVersion: "v3",
		}},
	}
}

func personRequest(personID string) *servicev1.RegisterPersonRequest {
	return &servicev1.RegisterPersonRequest{PersonId: personID, FullName: "Ana Sintetica", Document: syntheticCPF}
}

func representationRequest(representationID, organizationID, personID string) *servicev1.RegisterRepresentationRequest {
	return &servicev1.RegisterRepresentationRequest{
		RepresentationId:    representationID,
		OrganizationId:      organizationID,
		PersonId:            personID,
		PowerType:           servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT,
		SourceType:          servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY,
		SourceEvidenceId:    "evd-0001",
		ScopeCurrency:       "BRL",
		ScopeMaxAmountMinor: 10_000_000,
		ScopeActTypes:       []string{"assignment", "consent"},
		ValidFrom:           int64(occurred),
		ValidUntil:          int64(occurred) + oneYear,
		ProposedByUserId:    "usr-1",
	}
}

func addEstablishment(t *testing.T, h *harness, req *servicev1.AddEstablishmentRequest) *servicev1.AddEstablishmentResponse {
	t.Helper()
	var resp servicev1.AddEstablishmentResponse
	if err := h.invoke(withTenant(t), "AddEstablishment", req, &resp); err != nil {
		t.Fatalf("AddEstablishment(%s) = %v", req.GetEstablishmentId(), err)
	}
	return &resp
}

func registerPerson(t *testing.T, h *harness, req *servicev1.RegisterPersonRequest) *servicev1.RegisterPersonResponse {
	t.Helper()
	var resp servicev1.RegisterPersonResponse
	if err := h.invoke(withTenant(t), "RegisterPerson", req, &resp); err != nil {
		t.Fatalf("RegisterPerson(%s) = %v", req.GetPersonId(), err)
	}
	return &resp
}

func registerRepresentation(t *testing.T, h *harness, req *servicev1.RegisterRepresentationRequest) *servicev1.RegisterRepresentationResponse {
	t.Helper()
	var resp servicev1.RegisterRepresentationResponse
	if err := h.invoke(withTenant(t), "RegisterRepresentation", req, &resp); err != nil {
		t.Fatalf("RegisterRepresentation(%s) = %v", req.GetRepresentationId(), err)
	}
	return &resp
}

func inTenant(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithTenant(testTenant))
}

func TestAddEstablishmentAnswersTheAddedEstablishmentAndEnqueuesItsEvent(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))

	resp := addEstablishment(t, h, addRequest("est-1", "org-1"))

	if got := resp.GetAdded().GetEstablishmentId(); got != "est-1" {
		t.Fatalf("Added.EstablishmentId = %q, want est-1 (response %v)", got, resp)
	}
	entries := h.store.Entries()
	if len(entries) != 2 {
		t.Fatalf("outbox holds %d entries, want the organization's and the establishment's", len(entries))
	}
	added, ok := entries[1].Event.(domain.EstablishmentAdded)
	want := domain.EstablishmentAdded{
		EstablishmentID: "est-1", OrganizationID: "org-1", TaxIDType: domain.TaxIDTypeCNPJ,
		TaxIDHash: hashOf(testTenant, domain.TaxIDTypeCNPJ, alphanumericCNPJ), Receiver: true, At: occurred,
	}
	if !ok || added != want {
		t.Fatalf("enqueued %v, want %v", entries[1].Event, want)
	}
	stored, _, err := establishmentsTable.Reader(h.store).Load(inTenant(t), "est-1")
	if err != nil {
		t.Fatalf("load est-1: %v", err)
	}
	wantRefs := []domain.ExternalReference{{
		ProviderID: "partner-registry", System: domain.ExternalSystemRegistry, ReferenceType: domain.ExternalReferenceAsset,
		Reference: "AST-0001/a", AsOf: port.Instant(referenceAsOf), ObservedVersion: "v3",
	}}
	if !slices.Equal(stored.ExternalReferences, wantRefs) || stored.TradeName != "Loja Sintetica" || stored.TaxIDMasked != "**.***.***/**DE-35" {
		t.Fatalf("stored %+v, want the reference exactly as the partner sent it, the trade name and the mask", stored)
	}
}

func TestAddEstablishmentAnswersEachRefusalInTheBody(t *testing.T) {
	planted := relations{establishments: establishmentsByTaxIDHash{
		hashOf(testTenant, domain.TaxIDTypeCPF, syntheticCPF): {{ID: "est-held", OrganizationID: "org-1"}},
	}}
	cases := []struct {
		name         string
		organization bool
		edit         func(*servicev1.AddEstablishmentRequest)
		code         string
	}{
		{"organization not found", false, func(*servicev1.AddEstablishmentRequest) {}, "identity-organization/establishment/organization-not-active"},
		{"tax id empty", true, func(r *servicev1.AddEstablishmentRequest) { r.TaxId = "" }, "identity-organization/establishment/tax-id-empty"},
		{"tax id held in the organization", true, func(r *servicev1.AddEstablishmentRequest) {
			r.TaxIdType, r.TaxId = servicev1.TaxIdType_TAX_ID_TYPE_CPF, syntheticCPF
		}, "identity-organization/establishment/duplicate-tax-id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarnessWith(t, planted)
			before := 0
			if c.organization {
				register(t, h, registerRequest("org-1"))
				before = 1
			}
			req := addRequest("est-refused", "org-1")
			c.edit(req)

			resp := addEstablishment(t, h, req)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
			if n := len(h.store.Entries()); n != before {
				t.Fatalf("outbox holds %d entries after a refusal, want %d", n, before)
			}
		})
	}
}

func TestFindEstablishmentsByOrganizationMapsEveryField(t *testing.T) {
	held := domain.EstablishmentSnapshot{
		ID: "est-1", OrganizationID: "org-1", TaxIDType: domain.TaxIDTypeCPF, TaxID: syntheticCPF, TaxIDHash: "hash-1",
		TaxIDHashKeyVersion: 2, TaxIDMasked: "***.***.*89-09", TradeName: "Loja", Receiver: true, CreatedAt: occurred,
		ExternalReferences: []domain.ExternalReference{{
			ProviderID: "partner-depository", System: domain.ExternalSystemDepository, ReferenceType: domain.ExternalReferenceSettlement,
			Reference: "STL 9", AsOf: 7, ObservedVersion: "etag-1",
		}},
	}
	h := newHarnessWith(t, relations{byOrganization: establishmentsByOrganization{"org-1": {held}}})

	var resp servicev1.FindEstablishmentsByOrganizationResponse
	if err := h.invoke(withTenant(t), "FindEstablishmentsByOrganization", &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "org-1"}, &resp); err != nil {
		t.Fatalf("FindEstablishmentsByOrganization() = %v", err)
	}

	want := &servicev1.Establishment{
		EstablishmentId: "est-1", OrganizationId: "org-1", TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CPF, TaxId: syntheticCPF,
		TaxIdHash: "hash-1", TaxIdHashKeyVersion: 2, TaxIdMasked: "***.***.*89-09", TradeName: "Loja", Receiver: true, CreatedAt: int64(occurred),
		ExternalReferences: []*servicev1.ExternalReference{{
			ProviderId: "partner-depository", System: servicev1.ExternalSystem_EXTERNAL_SYSTEM_DEPOSITORY,
			ReferenceType: servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_SETTLEMENT, Reference: "STL 9", AsOf: 7, ObservedVersion: "etag-1",
		}},
	}
	if len(resp.GetEstablishments()) != 1 || !proto.Equal(resp.GetEstablishments()[0], want) {
		t.Fatalf("FindEstablishmentsByOrganization() = %v, want [%v]", resp.GetEstablishments(), want)
	}
}

func TestFindEstablishmentsOfAnOrganizationWithoutAnyIsEmpty(t *testing.T) {
	h := newHarness(t, nil)

	var resp servicev1.FindEstablishmentsByOrganizationResponse
	err := h.invoke(withTenant(t), "FindEstablishmentsByOrganization", &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "org-none"}, &resp)

	if err != nil || len(resp.GetEstablishments()) != 0 {
		t.Fatalf("FindEstablishmentsByOrganization() = %v, %v; want an empty list and no error", resp.GetEstablishments(), err)
	}
}

func TestRegisterPersonAnswersThePersonAndFindReadsOnlyTheHashAndTheMask(t *testing.T) {
	h := newHarness(t, nil)

	resp := registerPerson(t, h, personRequest("per-1"))

	if got := resp.GetRegistered().GetPersonId(); got != "per-1" {
		t.Fatalf("Registered.PersonId = %q, want per-1 (response %v)", got, resp)
	}
	if n := len(h.store.Entries()); n != 0 {
		t.Fatalf("outbox holds %d entries, want none: RegisterPerson emits no event", n)
	}
	var found servicev1.FindPersonResponse
	if err := h.invoke(withTenant(t), "FindPerson", &servicev1.FindPersonRequest{PersonId: "per-1"}, &found); err != nil {
		t.Fatalf("FindPerson() = %v", err)
	}
	want := &servicev1.Person{
		PersonId: "per-1", FullName: "Ana Sintetica", DocumentHash: personHash, DocumentHashKeyVersion: 1,
		DocumentMasked: "***.***.*89-09", CreatedAt: int64(occurred),
	}
	if !proto.Equal(found.GetPerson(), want) {
		t.Fatalf("FindPerson() = %v, want %v", found.GetPerson(), want)
	}
}

func TestThePersonOnTheWireDeclaresNoDocument(t *testing.T) {
	fields := (&servicev1.Person{}).ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		if fields.Get(i).Name() == "document" {
			t.Fatal("Person declares document, want only its hash and mask to leave the context (spec note ‡‡)")
		}
	}
}

func TestRegisterPersonAnswersEachRefusalInTheBody(t *testing.T) {
	planted := relations{documents: personsByDocumentHash{personHash: {{ID: "per-held"}}}}
	cases := []struct {
		name string
		edit func(*servicev1.RegisterPersonRequest)
		code string
	}{
		{"full name blank", func(r *servicev1.RegisterPersonRequest) { r.FullName = "  " }, "identity-organization/person/full-name-empty"},
		{"document empty", func(r *servicev1.RegisterPersonRequest) { r.Document = "" }, "identity-organization/person/document-empty"},
		{"document held in the tenant", func(*servicev1.RegisterPersonRequest) {}, "identity-organization/person/duplicate-document"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarnessWith(t, planted)
			req := personRequest("per-refused")
			c.edit(req)

			resp := registerPerson(t, h, req)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
		})
	}
}

func TestRegisterRepresentationAnswersAndFindReadsItDeclared(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	registerPerson(t, h, personRequest("per-1"))

	resp := registerRepresentation(t, h, representationRequest("rep-1", "org-1", "per-1"))

	if got := resp.GetRegistered().GetRepresentationId(); got != "rep-1" {
		t.Fatalf("Registered.RepresentationId = %q, want rep-1 (response %v)", got, resp)
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries, want only the organization's: RegisterRepresentation emits no event", n)
	}
	var found servicev1.FindRepresentationResponse
	if err := h.invoke(withTenant(t), "FindRepresentation", &servicev1.FindRepresentationRequest{RepresentationId: "rep-1"}, &found); err != nil {
		t.Fatalf("FindRepresentation() = %v", err)
	}
	want := &servicev1.Representation{
		RepresentationId: "rep-1", OrganizationId: "org-1", PersonId: "per-1",
		PowerType: servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT, SourceType: servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY,
		SourceEvidenceId: "evd-0001", ScopeCurrency: "BRL", ScopeMaxAmountMinor: 10_000_000, ScopeActTypes: []string{"assignment", "consent"},
		ValidFrom: int64(occurred), ValidUntil: int64(occurred) + oneYear,
		VerificationStatus: servicev1.RepresentationVerificationStatus_REPRESENTATION_VERIFICATION_STATUS_DECLARED, ProposedByUserId: "usr-1",
	}
	if !proto.Equal(found.GetRepresentation(), want) {
		t.Fatalf("FindRepresentation() = %v, want %v", found.GetRepresentation(), want)
	}
}

func TestRegisterRepresentationAnswersEachRefusalInTheBody(t *testing.T) {
	cases := []struct {
		name                 string
		organization, person bool
		edit                 func(*servicev1.RegisterRepresentationRequest)
		code                 string
	}{
		{"evidence blank", true, true, func(r *servicev1.RegisterRepresentationRequest) { r.SourceEvidenceId = " " },
			"identity-organization/representation/source-evidence-missing"},
		{"valid until equal to valid from", true, true, func(r *servicev1.RegisterRepresentationRequest) { r.ValidUntil = r.ValidFrom },
			"identity-organization/representation/validity-invalid"},
		{"amount without currency", true, true, func(r *servicev1.RegisterRepresentationRequest) { r.ScopeCurrency = "" },
			"identity-organization/representation/scope-invalid"},
		{"organization not found", false, true, func(*servicev1.RegisterRepresentationRequest) {},
			"identity-organization/representation/organization-not-active"},
		{"person not found", true, false, func(*servicev1.RegisterRepresentationRequest) {},
			"identity-organization/representation/person-not-found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, nil)
			if c.organization {
				register(t, h, registerRequest("org-1"))
			}
			if c.person {
				registerPerson(t, h, personRequest("per-1"))
			}
			req := representationRequest("rep-refused", "org-1", "per-1")
			c.edit(req)

			resp := registerRepresentation(t, h, req)

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
		})
	}
}

func TestFindRepresentationsByOrganizationKeepsOnlyThoseInForceAtValidAt(t *testing.T) {
	declared := func(id domain.RepresentationID, from, until port.Instant) domain.RepresentationSnapshot {
		return domain.RepresentationSnapshot{
			ID: id, OrganizationID: "org-1", PersonID: "per-1", PowerType: domain.PowerOperator, SourceType: domain.SourceInternalDelegation,
			SourceEvidenceID: "evd-1", ValidFrom: from, ValidUntil: until, VerificationStatus: domain.RepresentationDeclared,
		}
	}
	h := newHarnessWith(t, relations{representations: representationsByOrganization{
		"org-1": {declared("rep-old", 100, 200), declared("rep-new", 200, 0)},
	}})

	for _, c := range []struct {
		validAt int64
		want    []string
	}{{150, []string{"rep-old"}}, {200, []string{"rep-new"}}, {99, []string{}}, {0, []string{"rep-old", "rep-new"}}} {
		var resp servicev1.FindRepresentationsByOrganizationResponse
		err := h.invoke(withTenant(t), "FindRepresentationsByOrganization",
			&servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: "org-1", ValidAt: c.validAt}, &resp)
		if err != nil {
			t.Fatalf("FindRepresentationsByOrganization(valid at %d) = %v", c.validAt, err)
		}
		got := []string{}
		for _, r := range resp.GetRepresentations() {
			got = append(got, r.GetRepresentationId())
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("FindRepresentationsByOrganization(valid at %d) = %v, want %v", c.validAt, got, c.want)
		}
	}
}

func TestFindingAnUnknownPersonOrRepresentationIsNotFound(t *testing.T) {
	h := newHarness(t, nil)

	var person servicev1.FindPersonResponse
	errPerson := h.invoke(withTenant(t), "FindPerson", &servicev1.FindPersonRequest{PersonId: "per-none"}, &person)
	var representation servicev1.FindRepresentationResponse
	errRepresentation := h.invoke(withTenant(t), "FindRepresentation", &servicev1.FindRepresentationRequest{RepresentationId: "rep-none"}, &representation)

	if status.Code(errPerson) != codes.NotFound || status.Code(errRepresentation) != codes.NotFound {
		t.Fatalf("FindPerson() = %v and FindRepresentation() = %v, want NotFound for both", errPerson, errRepresentation)
	}
}

func TestMalformedRequestsOfTheRelationsAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	long := strings.Repeat("a", 257)
	many := make([]string, 33)
	for i := range many {
		many[i] = "act"
	}

	add := func(edit func(*servicev1.AddEstablishmentRequest)) func() error {
		return func() error {
			req := addRequest("est-1", "org-1")
			edit(req)
			var r servicev1.AddEstablishmentResponse
			return h.invoke(withTenant(t), "AddEstablishment", req, &r)
		}
	}
	person := func(edit func(*servicev1.RegisterPersonRequest)) func() error {
		return func() error {
			req := personRequest("per-1")
			edit(req)
			var r servicev1.RegisterPersonResponse
			return h.invoke(withTenant(t), "RegisterPerson", req, &r)
		}
	}
	representation := func(edit func(*servicev1.RegisterRepresentationRequest)) func() error {
		return func() error {
			req := representationRequest("rep-1", "org-1", "per-1")
			edit(req)
			var r servicev1.RegisterRepresentationResponse
			return h.invoke(withTenant(t), "RegisterRepresentation", req, &r)
		}
	}
	query := func(method string, req, resp proto.Message) func() error {
		return func() error { return h.invoke(withTenant(t), method, req, resp) }
	}
	calls := []struct {
		name string
		call func() error
	}{
		{"establishment id with a space", add(func(r *servicev1.AddEstablishmentRequest) { r.EstablishmentId = "est 1" })},
		{"establishment of an empty organization id", add(func(r *servicev1.AddEstablishmentRequest) { r.OrganizationId = "" })},
		{"establishment under an unspecified tax id type", add(func(r *servicev1.AddEstablishmentRequest) {
			r.TaxIdType = servicev1.TaxIdType_TAX_ID_TYPE_UNSPECIFIED
		})},
		{"establishment tax id too long", add(func(r *servicev1.AddEstablishmentRequest) { r.TaxId = strings.Repeat("1", 33) })},
		{"establishment trade name too long", add(func(r *servicev1.AddEstablishmentRequest) { r.TradeName = long })},
		{"reference to an unspecified system", add(func(r *servicev1.AddEstablishmentRequest) {
			r.ExternalReferences[0].System = servicev1.ExternalSystem_EXTERNAL_SYSTEM_UNSPECIFIED
		})},
		{"reference of a type outside the enum", add(func(r *servicev1.AddEstablishmentRequest) { r.ExternalReferences[0].ReferenceType = 99 })},
		{"reference value too long", add(func(r *servicev1.AddEstablishmentRequest) { r.ExternalReferences[0].Reference = long })},
		{"reference provider too long", add(func(r *servicev1.AddEstablishmentRequest) { r.ExternalReferences[0].ProviderId = long })},
		{"too many references", add(func(r *servicev1.AddEstablishmentRequest) {
			for range 32 {
				r.ExternalReferences = append(r.ExternalReferences, r.ExternalReferences[0])
			}
		})},
		{"person id with a space", person(func(r *servicev1.RegisterPersonRequest) { r.PersonId = "per 1" })},
		{"person full name too long", person(func(r *servicev1.RegisterPersonRequest) { r.FullName = long })},
		{"person document too long", person(func(r *servicev1.RegisterPersonRequest) { r.Document = strings.Repeat("1", 33) })},
		{"representation id empty", representation(func(r *servicev1.RegisterRepresentationRequest) { r.RepresentationId = "" })},
		{"representation of an organization id with a space", representation(func(r *servicev1.RegisterRepresentationRequest) { r.OrganizationId = "org 1" })},
		{"representation of a person id with a space", representation(func(r *servicev1.RegisterRepresentationRequest) { r.PersonId = "per 1" })},
		{"unspecified power type", representation(func(r *servicev1.RegisterRepresentationRequest) {
			r.PowerType = servicev1.PowerType_POWER_TYPE_UNSPECIFIED
		})},
		{"power type outside the enum", representation(func(r *servicev1.RegisterRepresentationRequest) { r.PowerType = 99 })},
		{"unspecified source type", representation(func(r *servicev1.RegisterRepresentationRequest) {
			r.SourceType = servicev1.SourceType_SOURCE_TYPE_UNSPECIFIED
		})},
		{"evidence id too long", representation(func(r *servicev1.RegisterRepresentationRequest) { r.SourceEvidenceId = long })},
		{"currency too long", representation(func(r *servicev1.RegisterRepresentationRequest) { r.ScopeCurrency = "BRAZILIAN" })},
		{"too many act types", representation(func(r *servicev1.RegisterRepresentationRequest) { r.ScopeActTypes = many })},
		{"act type too long", representation(func(r *servicev1.RegisterRepresentationRequest) { r.ScopeActTypes = []string{strings.Repeat("a", 65)} })},
		{"proposer too long", representation(func(r *servicev1.RegisterRepresentationRequest) { r.ProposedByUserId = long })},
		{"establishments of an organization id with a space", query("FindEstablishmentsByOrganization",
			&servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "org 1"}, &servicev1.FindEstablishmentsByOrganizationResponse{})},
		{"person to find with an empty id", query("FindPerson", &servicev1.FindPersonRequest{}, &servicev1.FindPersonResponse{})},
		{"representation to find with a space", query("FindRepresentation",
			&servicev1.FindRepresentationRequest{RepresentationId: "rep 1"}, &servicev1.FindRepresentationResponse{})},
		{"representations of an empty organization id", query("FindRepresentationsByOrganization",
			&servicev1.FindRepresentationsByOrganizationRequest{}, &servicev1.FindRepresentationsByOrganizationResponse{})},
		{"representations valid at a negative instant", query("FindRepresentationsByOrganization",
			&servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: "org-1", ValidAt: -1}, &servicev1.FindRepresentationsByOrganizationResponse{})},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries, want only the organization's: no malformed call reached the use case", n)
	}
}

const (
	oneWeek      = int64(7 * 24 * 3600 * 1_000_000_000)
	inviteeEmail = "ana.sintetica@example.test"
)

func invokeOK[M proto.Message](t *testing.T, h *harness, method string, req proto.Message, resp M) M {
	t.Helper()
	if err := h.invoke(withTenant(t), method, req, resp); err != nil {
		t.Fatalf("%s() = %v", method, err)
	}
	return resp
}

func provisionRequest(subject string) *servicev1.ProvisionUserAccountRequest {
	return &servicev1.ProvisionUserAccountRequest{Subject: subject, PersonId: "per-1"}
}

func suspendRequest(userID string) *servicev1.SuspendUserRequest {
	return &servicev1.SuspendUserRequest{UserId: userID, Reason: "fraud review"}
}

func inviteRequest(membershipID string) *servicev1.InviteUserRequest {
	return &servicev1.InviteUserRequest{
		MembershipId:        membershipID,
		OrganizationId:      "org-1",
		InviteeEmail:        inviteeEmail,
		Role:                servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR,
		InvitedByUserId:     "sub-admin",
		InvitationExpiresAt: int64(occurred) + oneWeek,
	}
}

func grantRequest(membershipID string) *servicev1.GrantMembershipRequest {
	return &servicev1.GrantMembershipRequest{
		MembershipId:      membershipID,
		UserId:            "sub-1",
		PersonId:          "per-1",
		ChannelConfirmed:  true,
		VerificationLevel: servicev1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED,
	}
}

func revokeRequest(membershipID string) *servicev1.RevokeMembershipRequest {
	return &servicev1.RevokeMembershipRequest{MembershipId: membershipID, Reason: "left the company"}
}

func TestProvisionUserAccountActivatesTheSubjectWithoutAnEvent(t *testing.T) {
	h := newHarness(t, nil)

	resp := invokeOK(t, h, "ProvisionUserAccount", provisionRequest("sub-1"), &servicev1.ProvisionUserAccountResponse{})

	if got := resp.GetProvisioned().GetUserId(); got != "sub-1" {
		t.Fatalf("Provisioned.UserId = %q, want sub-1 (response %v)", got, resp)
	}
	if n := len(h.store.Entries()); n != 0 {
		t.Fatalf("outbox holds %d entries, want none: ProvisionUserAccount emits no event", n)
	}
	stored, _, err := userAccountsTable.Reader(h.store).Load(inTenant(t), "sub-1")
	want := domain.UserAccountSnapshot{Subject: "sub-1", PersonID: "per-1", Status: domain.UserAccountActive, ProvisionedAt: occurred}
	if err != nil || stored != want {
		t.Fatalf("stored %+v (%v), want %+v", stored, err, want)
	}
}

func TestProvisionUserAccountLetsAnEmptySubjectReachTheRefusalOfTheDomain(t *testing.T) {
	h := newHarness(t, nil)

	resp := invokeOK(t, h, "ProvisionUserAccount", provisionRequest(""), &servicev1.ProvisionUserAccountResponse{})

	if got := resp.GetRejection().GetCode(); got != "identity-organization/user-account/subject-empty" {
		t.Fatalf("rejection code = %q, want subject-empty (response %v)", got, resp)
	}
}

func TestSuspendUserSuspendsTheAccountAndEnqueuesTheFact(t *testing.T) {
	h := newHarness(t, nil)
	invokeOK(t, h, "ProvisionUserAccount", provisionRequest("sub-1"), &servicev1.ProvisionUserAccountResponse{})

	resp := invokeOK(t, h, "SuspendUser", suspendRequest("sub-1"), &servicev1.SuspendUserResponse{})

	if got := resp.GetSuspended().GetUserId(); got != "sub-1" {
		t.Fatalf("Suspended.UserId = %q, want sub-1 (response %v)", got, resp)
	}
	entries := h.store.Entries()
	want := domain.UserSuspended{UserID: "sub-1", Reason: "fraud review", At: occurred}
	if len(entries) != 1 || entries[0].Event != want {
		t.Fatalf("outbox = %v, want only %v", entries, want)
	}
}

func TestSuspendUserAnswersEachRefusalInTheBody(t *testing.T) {
	cases := []struct {
		name      string
		suspended bool
		edit      func(*servicev1.SuspendUserRequest)
		code      string
	}{
		{"account already suspended", true, func(*servicev1.SuspendUserRequest) {}, "identity-organization/user-account/not-active"},
		{"reason blank", false, func(r *servicev1.SuspendUserRequest) { r.Reason = "  " }, "identity-organization/user-account/reason-empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, nil)
			invokeOK(t, h, "ProvisionUserAccount", provisionRequest("sub-1"), &servicev1.ProvisionUserAccountResponse{})
			before := 0
			if c.suspended {
				invokeOK(t, h, "SuspendUser", suspendRequest("sub-1"), &servicev1.SuspendUserResponse{})
				before = 1
			}
			req := suspendRequest("sub-1")
			c.edit(req)

			resp := invokeOK(t, h, "SuspendUser", req, &servicev1.SuspendUserResponse{})

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
			if n := len(h.store.Entries()); n != before {
				t.Fatalf("outbox holds %d entries after a refusal, want %d", n, before)
			}
		})
	}
}

func TestSuspendingAnUnknownAccountIsNotFound(t *testing.T) {
	h := newHarness(t, nil)

	err := h.invoke(withTenant(t), "SuspendUser", suspendRequest("sub-none"), &servicev1.SuspendUserResponse{})

	if status.Code(err) != codes.NotFound {
		t.Fatalf("SuspendUser(sub-none) = %v, want NotFound", err)
	}
}

func TestInviteUserInvitesWithoutAnEvent(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))

	resp := invokeOK(t, h, "InviteUser", inviteRequest("mbr-1"), &servicev1.InviteUserResponse{})

	if got := resp.GetInvited().GetMembershipId(); got != "mbr-1" {
		t.Fatalf("Invited.MembershipId = %q, want mbr-1 (response %v)", got, resp)
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries, want only the organization's: InviteUser emits no event", n)
	}
	stored, _, err := membershipsTable.Reader(h.store).Load(inTenant(t), "mbr-1")
	want := domain.MembershipSnapshot{
		ID: "mbr-1", OrganizationID: "org-1", InviteeEmail: inviteeEmail, Role: domain.RoleOperator, Status: domain.MembershipStatusInvited,
		InvitedByUserID: "sub-admin", InvitedAt: occurred, InvitationExpiresAt: occurred + port.Instant(oneWeek),
	}
	if err != nil || stored != want {
		t.Fatalf("stored %+v (%v), want %+v", stored, err, want)
	}
}

func TestInviteUserInvitesPlatformOpsIntoAPlatformOrganization(t *testing.T) {
	h := newHarness(t, nil)
	platform := registerRequest("org-1")
	platform.Kinds = []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_PLATFORM}
	register(t, h, platform)
	req := inviteRequest("mbr-1")
	req.Role = servicev1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS

	invokeOK(t, h, "InviteUser", req, &servicev1.InviteUserResponse{})

	stored, _, err := membershipsTable.Reader(h.store).Load(inTenant(t), "mbr-1")
	if err != nil || stored.Role != domain.RolePlatformOps {
		t.Fatalf("stored %+v (%v), want an invitation of platform_ops", stored, err)
	}
}

func TestInviteUserAnswersEachRefusalInTheBody(t *testing.T) {
	funder := registerRequest("org-1")
	funder.Kinds = []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER}
	cases := []struct {
		name         string
		organization *servicev1.RegisterOrganizationRequest
		edit         func(*servicev1.InviteUserRequest)
		code         string
	}{
		{"invitee email blank", registerRequest("org-1"), func(r *servicev1.InviteUserRequest) { r.InviteeEmail = " " },
			"identity-organization/membership/invitee-email-empty"},
		{"expiry at the instant of the request", registerRequest("org-1"), func(r *servicev1.InviteUserRequest) { r.InvitationExpiresAt = int64(occurred) },
			"identity-organization/membership/invitation-expiry-invalid"},
		{"organization not found", nil, func(*servicev1.InviteUserRequest) {}, "identity-organization/membership/organization-not-active"},
		{"operator of a funder", funder, func(*servicev1.InviteUserRequest) {}, "identity-organization/membership/role-kind-mismatch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, nil)
			if c.organization != nil {
				register(t, h, c.organization)
			}
			req := inviteRequest("mbr-refused")
			c.edit(req)

			resp := invokeOK(t, h, "InviteUser", req, &servicev1.InviteUserResponse{})

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
			if _, _, err := membershipsTable.Reader(h.store).Load(inTenant(t), "mbr-refused"); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("load mbr-refused = %v, want nothing written", err)
			}
		})
	}
}

func withInvitation(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	invokeOK(t, h, "ProvisionUserAccount", provisionRequest("sub-1"), &servicev1.ProvisionUserAccountResponse{})
	invokeOK(t, h, "InviteUser", inviteRequest("mbr-1"), &servicev1.InviteUserResponse{})
	return h
}

func TestGrantMembershipActivatesItAndEnqueuesTheGrant(t *testing.T) {
	h := withInvitation(t)

	resp := invokeOK(t, h, "GrantMembership", grantRequest("mbr-1"), &servicev1.GrantMembershipResponse{})

	if got := resp.GetGranted().GetMembershipId(); got != "mbr-1" {
		t.Fatalf("Granted.MembershipId = %q, want mbr-1 (response %v)", got, resp)
	}
	entries := h.store.Entries()
	want := domain.MembershipGranted{
		MembershipID: "mbr-1", OrganizationID: "org-1", UserID: "sub-1", Role: domain.RoleOperator,
		VerificationLevel: domain.VerificationPersonalDataChecked, At: occurred,
	}
	if len(entries) != 2 || entries[1].Event != want {
		t.Fatalf("outbox = %v, want the organization's and %v", entries, want)
	}
}

func TestGrantMembershipAnswersEachRefusalInTheBody(t *testing.T) {
	cases := []struct {
		name    string
		granted bool
		edit    func(*servicev1.GrantMembershipRequest)
		code    string
	}{
		{"already active", true, func(*servicev1.GrantMembershipRequest) {}, "identity-organization/membership/not-invited"},
		{"channel not confirmed", false, func(r *servicev1.GrantMembershipRequest) { r.ChannelConfirmed = false },
			"identity-organization/membership/channel-not-confirmed"},
		{"channel ownership for an operator", false, func(r *servicev1.GrantMembershipRequest) {
			r.VerificationLevel = servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP
		}, "identity-organization/membership/verification-insufficient"},
		{"user without an account", false, func(r *servicev1.GrantMembershipRequest) { r.UserId = "sub-none" },
			"identity-organization/membership/user-not-active"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := withInvitation(t)
			before := 1
			if c.granted {
				invokeOK(t, h, "GrantMembership", grantRequest("mbr-1"), &servicev1.GrantMembershipResponse{})
				before = 2
			}
			req := grantRequest("mbr-1")
			c.edit(req)

			resp := invokeOK(t, h, "GrantMembership", req, &servicev1.GrantMembershipResponse{})

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
			if n := len(h.store.Entries()); n != before {
				t.Fatalf("outbox holds %d entries after a refusal, want %d", n, before)
			}
		})
	}
}

func TestRevokeMembershipRevokesItAndEnqueuesTheRevocation(t *testing.T) {
	h := withInvitation(t)
	invokeOK(t, h, "GrantMembership", grantRequest("mbr-1"), &servicev1.GrantMembershipResponse{})

	resp := invokeOK(t, h, "RevokeMembership", revokeRequest("mbr-1"), &servicev1.RevokeMembershipResponse{})

	if got := resp.GetRevoked().GetMembershipId(); got != "mbr-1" {
		t.Fatalf("Revoked.MembershipId = %q, want mbr-1 (response %v)", got, resp)
	}
	entries := h.store.Entries()
	want := domain.MembershipRevoked{
		MembershipID: "mbr-1", OrganizationID: "org-1", UserID: "sub-1", Role: domain.RoleOperator, Reason: "left the company", At: occurred,
	}
	if len(entries) != 3 || entries[2].Event != want {
		t.Fatalf("outbox = %v, want the organization's, the grant and %v", entries, want)
	}
}

func TestRevokeMembershipRefusesAnInvitationInTheBody(t *testing.T) {
	h := withInvitation(t)

	resp := invokeOK(t, h, "RevokeMembership", revokeRequest("mbr-1"), &servicev1.RevokeMembershipResponse{})

	if got := resp.GetRejection().GetCode(); got != "identity-organization/membership/not-active" {
		t.Fatalf("rejection code = %q, want not-active (response %v)", got, resp)
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries after a refusal, want only the organization's", n)
	}
}

func TestFindMembershipsAnswersThePlatformOpsRole(t *testing.T) {
	held := domain.MembershipSnapshot{
		ID: "mbr-1", OrganizationID: "org-1", InviteeEmail: inviteeEmail, Role: domain.RolePlatformOps, Status: domain.MembershipStatusInvited,
		InvitedByUserID: "sub-admin", InvitedAt: 10, InvitationExpiresAt: 20,
	}
	h := newHarnessWith(t, relations{members: membershipsByOrganization{"org-1": {held}}})

	resp := invokeOK(t, h, "FindMembershipsByOrganization",
		&servicev1.FindMembershipsByOrganizationRequest{OrganizationId: "org-1"}, &servicev1.FindMembershipsByOrganizationResponse{})

	if got := resp.GetMemberships(); len(got) != 1 || got[0].GetRole() != servicev1.MembershipRole_MEMBERSHIP_ROLE_PLATFORM_OPS {
		t.Fatalf("Memberships = %v, want one of MEMBERSHIP_ROLE_PLATFORM_OPS", got)
	}
}

func TestFindMembershipsMapEveryField(t *testing.T) {
	held := domain.MembershipSnapshot{
		ID: "mbr-1", OrganizationID: "org-1", InviteeEmail: inviteeEmail, Role: domain.RoleAuditor, Status: domain.MembershipStatusRevoked,
		InvitedByUserID: "sub-admin", InvitedAt: 10, InvitationExpiresAt: 20, UserID: "sub-1", PersonID: "per-1",
		VerificationLevel: domain.VerificationChannelOwnership, GrantedAt: 15, RevokedAt: 30, RevocationReason: "left the company",
	}
	h := newHarnessWith(t, relations{
		byUser:  membershipsByUser{"sub-1": {held}},
		members: membershipsByOrganization{"org-1": {held}},
	})
	want := &servicev1.Membership{
		MembershipId: "mbr-1", OrganizationId: "org-1", InviteeEmail: inviteeEmail, Role: servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR,
		Status: servicev1.MembershipStatus_MEMBERSHIP_STATUS_REVOKED, InvitedByUserId: "sub-admin", InvitedAt: 10, InvitationExpiresAt: 20,
		UserId: "sub-1", PersonId: "per-1", VerificationLevel: servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP,
		GrantedAt: 15, RevokedAt: 30, RevocationReason: "left the company",
	}

	byUser := invokeOK(t, h, "FindMembershipsByUser", &servicev1.FindMembershipsByUserRequest{UserId: "sub-1"}, &servicev1.FindMembershipsByUserResponse{})
	byOrganization := invokeOK(t, h, "FindMembershipsByOrganization",
		&servicev1.FindMembershipsByOrganizationRequest{OrganizationId: "org-1"}, &servicev1.FindMembershipsByOrganizationResponse{})

	if len(byUser.GetMemberships()) != 1 || !proto.Equal(byUser.GetMemberships()[0], want) {
		t.Fatalf("FindMembershipsByUser() = %v, want [%v]", byUser.GetMemberships(), want)
	}
	if len(byOrganization.GetMemberships()) != 1 || !proto.Equal(byOrganization.GetMemberships()[0], want) {
		t.Fatalf("FindMembershipsByOrganization() = %v, want [%v]", byOrganization.GetMemberships(), want)
	}
}

func TestFindEffectivePermissionsAnswersThePermissionsOfTheActiveRolesOnly(t *testing.T) {
	membership := func(id domain.MembershipID, role domain.Role, s domain.MembershipStatus) domain.MembershipSnapshot {
		return domain.MembershipSnapshot{ID: id, OrganizationID: "org-1", Role: role, Status: s, UserID: "sub-1"}
	}
	h := newHarnessWith(t, relations{byUser: membershipsByUser{"sub-1": {
		membership("mbr-1", domain.RoleOperator, domain.MembershipStatusActive),
		membership("mbr-2", domain.RoleAuditor, domain.MembershipStatusRevoked),
	}}})
	invokeOK(t, h, "ProvisionUserAccount", provisionRequest("sub-1"), &servicev1.ProvisionUserAccountResponse{})

	resp := invokeOK(t, h, "FindEffectivePermissions", &servicev1.FindEffectivePermissionsRequest{UserId: "sub-1"}, &servicev1.FindEffectivePermissionsResponse{})

	want := make([]string, 0, 17)
	for _, p := range domain.RoleOperator.Permissions() {
		want = append(want, string(p))
	}
	if !slices.Equal(resp.GetPermissions(), want) {
		t.Fatalf("FindEffectivePermissions() = %v, want the 17 of operator %v", resp.GetPermissions(), want)
	}
}

func TestFindEffectivePermissionsOfASubjectWithoutAnAccountIsEmpty(t *testing.T) {
	h := newHarnessWith(t, relations{byUser: membershipsByUser{"sub-none": {
		{ID: "mbr-1", OrganizationID: "org-1", Role: domain.RoleOperator, Status: domain.MembershipStatusActive, UserID: "sub-none"},
	}}})

	resp := invokeOK(t, h, "FindEffectivePermissions", &servicev1.FindEffectivePermissionsRequest{UserId: "sub-none"}, &servicev1.FindEffectivePermissionsResponse{})

	if len(resp.GetPermissions()) != 0 {
		t.Fatalf("FindEffectivePermissions(sub-none) = %v, want none: no account, no permission", resp.GetPermissions())
	}
}

func TestMalformedRequestsOfTheAccessAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	long := strings.Repeat("a", 513)

	command := func(method string, req, resp proto.Message) func() error {
		return func() error { return h.invoke(withTenant(t), method, req, resp) }
	}
	provision := func(edit func(*servicev1.ProvisionUserAccountRequest)) func() error {
		req := provisionRequest("sub-1")
		edit(req)
		return command("ProvisionUserAccount", req, &servicev1.ProvisionUserAccountResponse{})
	}
	suspend := func(edit func(*servicev1.SuspendUserRequest)) func() error {
		req := suspendRequest("sub-1")
		edit(req)
		return command("SuspendUser", req, &servicev1.SuspendUserResponse{})
	}
	invite := func(edit func(*servicev1.InviteUserRequest)) func() error {
		req := inviteRequest("mbr-1")
		edit(req)
		return command("InviteUser", req, &servicev1.InviteUserResponse{})
	}
	grant := func(edit func(*servicev1.GrantMembershipRequest)) func() error {
		req := grantRequest("mbr-1")
		edit(req)
		return command("GrantMembership", req, &servicev1.GrantMembershipResponse{})
	}
	revoke := func(edit func(*servicev1.RevokeMembershipRequest)) func() error {
		req := revokeRequest("mbr-1")
		edit(req)
		return command("RevokeMembership", req, &servicev1.RevokeMembershipResponse{})
	}
	calls := []struct {
		name string
		call func() error
	}{
		{"subject with a space", provision(func(r *servicev1.ProvisionUserAccountRequest) { r.Subject = "sub 1" })},
		{"subject too long", provision(func(r *servicev1.ProvisionUserAccountRequest) { r.Subject = strings.Repeat("s", 129) })},
		{"person of the account with a space", provision(func(r *servicev1.ProvisionUserAccountRequest) { r.PersonId = "per 1" })},
		{"user to suspend empty", suspend(func(r *servicev1.SuspendUserRequest) { r.UserId = "" })},
		{"suspension reason too long", suspend(func(r *servicev1.SuspendUserRequest) { r.Reason = long })},
		{"membership id with a space", invite(func(r *servicev1.InviteUserRequest) { r.MembershipId = "mbr 1" })},
		{"invitation to an empty organization id", invite(func(r *servicev1.InviteUserRequest) { r.OrganizationId = "" })},
		{"invitee email too long", invite(func(r *servicev1.InviteUserRequest) { r.InviteeEmail = strings.Repeat("a", 255) })},
		{"unspecified role", invite(func(r *servicev1.InviteUserRequest) { r.Role = servicev1.MembershipRole_MEMBERSHIP_ROLE_UNSPECIFIED })},
		{"role outside the enum", invite(func(r *servicev1.InviteUserRequest) { r.Role = 99 })},
		{"inviter too long", invite(func(r *servicev1.InviteUserRequest) { r.InvitedByUserId = strings.Repeat("a", 257) })},
		{"membership to grant empty", grant(func(r *servicev1.GrantMembershipRequest) { r.MembershipId = "" })},
		{"user of the grant with a space", grant(func(r *servicev1.GrantMembershipRequest) { r.UserId = "sub 1" })},
		{"person of the grant with a space", grant(func(r *servicev1.GrantMembershipRequest) { r.PersonId = "per 1" })},
		{"unspecified verification level", grant(func(r *servicev1.GrantMembershipRequest) {
			r.VerificationLevel = servicev1.VerificationLevel_VERIFICATION_LEVEL_UNSPECIFIED
		})},
		{"membership to revoke with a space", revoke(func(r *servicev1.RevokeMembershipRequest) { r.MembershipId = "mbr 1" })},
		{"revocation reason too long", revoke(func(r *servicev1.RevokeMembershipRequest) { r.Reason = long })},
		{"memberships of an empty user", command("FindMembershipsByUser", &servicev1.FindMembershipsByUserRequest{}, &servicev1.FindMembershipsByUserResponse{})},
		{"memberships of an organization id with a space", command("FindMembershipsByOrganization",
			&servicev1.FindMembershipsByOrganizationRequest{OrganizationId: "org 1"}, &servicev1.FindMembershipsByOrganizationResponse{})},
		{"permissions of a user with a space", command("FindEffectivePermissions",
			&servicev1.FindEffectivePermissionsRequest{UserId: "sub 1"}, &servicev1.FindEffectivePermissionsResponse{})},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries, want only the organization's: no malformed call reached the use case", n)
	}
	if _, _, err := userAccountsTable.Reader(h.store).Load(inTenant(t), "sub-1"); !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("load sub-1 = %v, want no account provisioned by a malformed call", err)
	}
}

func requestChangeRequest(bankAccountChangeID string) *servicev1.RequestBankAccountChangeRequest {
	return &servicev1.RequestBankAccountChangeRequest{
		BankAccountChangeId:   bankAccountChangeID,
		OrganizationId:        "org-1",
		DestinationEvidenceId: "evd-1",
		RequestedByUserId:     "sub-1",
	}
}

func approveChangeRequest(bankAccountChangeID, approvedBy string) *servicev1.ApproveBankAccountChangeRequest {
	return &servicev1.ApproveBankAccountChangeRequest{BankAccountChangeId: bankAccountChangeID, ApprovedByUserId: approvedBy}
}

func withRequestedChange(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	invokeOK(t, h, "RequestBankAccountChange", requestChangeRequest("bac-1"), &servicev1.RequestBankAccountChangeResponse{})
	return h
}

func TestRequestBankAccountChangeRequestsItAndEnqueuesTheRequest(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))

	resp := invokeOK(t, h, "RequestBankAccountChange", requestChangeRequest("bac-1"), &servicev1.RequestBankAccountChangeResponse{})

	if got := resp.GetRequested().GetBankAccountChangeId(); got != "bac-1" {
		t.Fatalf("Requested.BankAccountChangeId = %q, want bac-1 (response %v)", got, resp)
	}
	entries := h.store.Entries()
	want := domain.BankAccountChangeRequested{
		BankAccountChangeID: "bac-1", OrganizationID: "org-1", DestinationEvidenceID: "evd-1", RequestedByUserID: "sub-1", At: occurred,
	}
	if len(entries) != 2 || entries[1].Event != want {
		t.Fatalf("outbox = %v, want the organization's and %v", entries, want)
	}
	stored, _, err := bankAccountChangesTable.Reader(h.store).Load(inTenant(t), "bac-1")
	wantStored := domain.BankAccountChangeSnapshot{
		ID: "bac-1", OrganizationID: "org-1", DestinationEvidenceID: "evd-1", RequestedByUserID: "sub-1",
		Status: domain.BankAccountChangeStatusRequested, RequestedAt: occurred,
	}
	if err != nil || stored != wantStored {
		t.Fatalf("stored %+v (%v), want %+v", stored, err, wantStored)
	}
}

func TestRequestBankAccountChangeAnswersEachRefusalInTheBody(t *testing.T) {
	cases := []struct {
		name         string
		organization bool
		edit         func(*servicev1.RequestBankAccountChangeRequest)
		code         string
	}{
		{"evidence empty", true, func(r *servicev1.RequestBankAccountChangeRequest) { r.DestinationEvidenceId = "" },
			"identity-organization/bank-account-change/destination-evidence-missing"},
		{"evidence blank", true, func(r *servicev1.RequestBankAccountChangeRequest) { r.DestinationEvidenceId = "  " },
			"identity-organization/bank-account-change/destination-evidence-missing"},
		{"organization not found", false, func(*servicev1.RequestBankAccountChangeRequest) {},
			"identity-organization/bank-account-change/organization-not-active"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, nil)
			before := 0
			if c.organization {
				register(t, h, registerRequest("org-1"))
				before = 1
			}
			req := requestChangeRequest("bac-refused")
			c.edit(req)

			resp := invokeOK(t, h, "RequestBankAccountChange", req, &servicev1.RequestBankAccountChangeResponse{})

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
			if n := len(h.store.Entries()); n != before {
				t.Fatalf("outbox holds %d entries after a refusal, want %d", n, before)
			}
			if _, _, err := bankAccountChangesTable.Reader(h.store).Load(inTenant(t), "bac-refused"); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("load bac-refused = %v, want nothing written", err)
			}
		})
	}
}

func TestApproveBankAccountChangeByAnotherUserApprovesItAndEnqueuesTheApproval(t *testing.T) {
	h := withRequestedChange(t)

	resp := invokeOK(t, h, "ApproveBankAccountChange", approveChangeRequest("bac-1", "sub-2"), &servicev1.ApproveBankAccountChangeResponse{})

	if got := resp.GetApproved().GetBankAccountChangeId(); got != "bac-1" {
		t.Fatalf("Approved.BankAccountChangeId = %q, want bac-1 (response %v)", got, resp)
	}
	entries := h.store.Entries()
	want := domain.BankAccountChangeApproved{BankAccountChangeID: "bac-1", OrganizationID: "org-1", ApprovedByUserID: "sub-2", At: occurred}
	if len(entries) != 3 || entries[2].Event != want {
		t.Fatalf("outbox = %v, want the organization's, the request and %v", entries, want)
	}
	found := invokeOK(t, h, "FindBankAccountChange", &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "bac-1"}, &servicev1.FindBankAccountChangeResponse{})
	wantFound := &servicev1.BankAccountChange{
		BankAccountChangeId: "bac-1", OrganizationId: "org-1", DestinationEvidenceId: "evd-1", RequestedByUserId: "sub-1",
		ApprovedByUserId: "sub-2", Status: servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED,
		RequestedAt: int64(occurred), ApprovedAt: int64(occurred),
	}
	if !proto.Equal(found.GetBankAccountChange(), wantFound) {
		t.Fatalf("FindBankAccountChange() = %v, want %v", found.GetBankAccountChange(), wantFound)
	}
}

func TestApproveBankAccountChangeAnswersEachRefusalInTheBody(t *testing.T) {
	cases := []struct {
		name     string
		approved bool
		approver string
		code     string
	}{
		{"approved by the requester", false, "sub-1", "identity-organization/bank-account-change/same-actor-approval"},
		{"already approved", true, "sub-3", "identity-organization/bank-account-change/not-requested"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := withRequestedChange(t)
			before := 2
			if c.approved {
				invokeOK(t, h, "ApproveBankAccountChange", approveChangeRequest("bac-1", "sub-2"), &servicev1.ApproveBankAccountChangeResponse{})
				before = 3
			}
			held, _, err := bankAccountChangesTable.Reader(h.store).Load(inTenant(t), "bac-1")
			if err != nil {
				t.Fatalf("load bac-1: %v", err)
			}

			resp := invokeOK(t, h, "ApproveBankAccountChange", approveChangeRequest("bac-1", c.approver), &servicev1.ApproveBankAccountChangeResponse{})

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, resp)
			}
			if n := len(h.store.Entries()); n != before {
				t.Fatalf("outbox holds %d entries after a refusal, want %d", n, before)
			}
			if after, _, err := bankAccountChangesTable.Reader(h.store).Load(inTenant(t), "bac-1"); err != nil || after != held {
				t.Fatalf("stored %+v (%v) after a refusal, want %+v untouched", after, err, held)
			}
		})
	}
}

func TestApprovingOrFindingAnUnknownBankAccountChangeIsNotFound(t *testing.T) {
	h := newHarness(t, nil)

	errApprove := h.invoke(withTenant(t), "ApproveBankAccountChange", approveChangeRequest("bac-none", "sub-2"), &servicev1.ApproveBankAccountChangeResponse{})
	errFind := h.invoke(withTenant(t), "FindBankAccountChange", &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "bac-none"}, &servicev1.FindBankAccountChangeResponse{})

	if status.Code(errApprove) != codes.NotFound || status.Code(errFind) != codes.NotFound {
		t.Fatalf("ApproveBankAccountChange() = %v and FindBankAccountChange() = %v, want NotFound for both", errApprove, errFind)
	}
}

func TestFindBankAccountChangesByOrganizationMapsEveryField(t *testing.T) {
	requested := domain.BankAccountChangeSnapshot{
		ID: "bac-1", OrganizationID: "org-1", DestinationEvidenceID: "evd-1", RequestedByUserID: "sub-1",
		Status: domain.BankAccountChangeStatusRequested, RequestedAt: 10,
	}
	approved := domain.BankAccountChangeSnapshot{
		ID: "bac-2", OrganizationID: "org-1", DestinationEvidenceID: "evd-2", RequestedByUserID: "sub-1", ApprovedByUserID: "sub-2",
		Status: domain.BankAccountChangeStatusApproved, RequestedAt: 20, ApprovedAt: 30,
	}
	h := newHarnessWith(t, relations{changes: bankAccountChangesByOrganization{"org-1": {requested, approved}}})

	resp := invokeOK(t, h, "FindBankAccountChangesByOrganization",
		&servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: "org-1"}, &servicev1.FindBankAccountChangesByOrganizationResponse{})

	want := []*servicev1.BankAccountChange{
		{
			BankAccountChangeId: "bac-1", OrganizationId: "org-1", DestinationEvidenceId: "evd-1", RequestedByUserId: "sub-1",
			Status: servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_REQUESTED, RequestedAt: 10,
		},
		{
			BankAccountChangeId: "bac-2", OrganizationId: "org-1", DestinationEvidenceId: "evd-2", RequestedByUserId: "sub-1", ApprovedByUserId: "sub-2",
			Status: servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED, RequestedAt: 20, ApprovedAt: 30,
		},
	}
	got := resp.GetBankAccountChanges()
	if len(got) != len(want) || !proto.Equal(got[0], want[0]) || !proto.Equal(got[1], want[1]) {
		t.Fatalf("FindBankAccountChangesByOrganization() = %v, want %v", got, want)
	}
	none := invokeOK(t, h, "FindBankAccountChangesByOrganization",
		&servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: "org-none"}, &servicev1.FindBankAccountChangesByOrganizationResponse{})
	if len(none.GetBankAccountChanges()) != 0 {
		t.Fatalf("FindBankAccountChangesByOrganization(org-none) = %v, want an empty list", none.GetBankAccountChanges())
	}
}

func TestABlankActorOfTheBankAccountChangeIsRefusedBeforeTheUseCase(t *testing.T) {
	h := withRequestedChange(t)

	for _, blank := range []string{"", "   ", "\t"} {
		request := requestChangeRequest("bac-2")
		request.RequestedByUserId = blank
		errRequest := h.invoke(withTenant(t), "RequestBankAccountChange", request, &servicev1.RequestBankAccountChangeResponse{})
		errApprove := h.invoke(withTenant(t), "ApproveBankAccountChange", approveChangeRequest("bac-1", blank), &servicev1.ApproveBankAccountChangeResponse{})

		if status.Code(errRequest) != codes.InvalidArgument || status.Code(errApprove) != codes.InvalidArgument {
			t.Fatalf("actor %q: RequestBankAccountChange() = %v and ApproveBankAccountChange() = %v, want InvalidArgument for both", blank, errRequest, errApprove)
		}
	}
	if n := len(h.store.Entries()); n != 2 {
		t.Fatalf("outbox holds %d entries, want only the organization's and the request: no blank actor reached the use case", n)
	}
	if _, _, err := bankAccountChangesTable.Reader(h.store).Load(inTenant(t), "bac-2"); !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("load bac-2 = %v, want nothing written by a blank requester", err)
	}
	if held, _, err := bankAccountChangesTable.Reader(h.store).Load(inTenant(t), "bac-1"); err != nil || held.Status != domain.BankAccountChangeStatusRequested {
		t.Fatalf("bac-1 = %+v (%v), want it still requested after a blank approver", held, err)
	}
}

func TestMalformedRequestsOfTheBankAccountChangeAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))
	long := strings.Repeat("a", 257)

	command := func(method string, req, resp proto.Message) func() error {
		return func() error { return h.invoke(withTenant(t), method, req, resp) }
	}
	request := func(edit func(*servicev1.RequestBankAccountChangeRequest)) func() error {
		req := requestChangeRequest("bac-1")
		edit(req)
		return command("RequestBankAccountChange", req, &servicev1.RequestBankAccountChangeResponse{})
	}
	approve := func(edit func(*servicev1.ApproveBankAccountChangeRequest)) func() error {
		req := approveChangeRequest("bac-1", "sub-2")
		edit(req)
		return command("ApproveBankAccountChange", req, &servicev1.ApproveBankAccountChangeResponse{})
	}
	calls := []struct {
		name string
		call func() error
	}{
		{"change id with a space", request(func(r *servicev1.RequestBankAccountChangeRequest) { r.BankAccountChangeId = "bac 1" })},
		{"change of an empty organization id", request(func(r *servicev1.RequestBankAccountChangeRequest) { r.OrganizationId = "" })},
		{"evidence too long", request(func(r *servicev1.RequestBankAccountChangeRequest) { r.DestinationEvidenceId = long })},
		{"requester too long", request(func(r *servicev1.RequestBankAccountChangeRequest) { r.RequestedByUserId = long })},
		{"change to approve empty", approve(func(r *servicev1.ApproveBankAccountChangeRequest) { r.BankAccountChangeId = "" })},
		{"approver too long", approve(func(r *servicev1.ApproveBankAccountChangeRequest) { r.ApprovedByUserId = long })},
		{"change to find with a space", command("FindBankAccountChange",
			&servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "bac 1"}, &servicev1.FindBankAccountChangeResponse{})},
		{"changes of an empty organization id", command("FindBankAccountChangesByOrganization",
			&servicev1.FindBankAccountChangesByOrganizationRequest{}, &servicev1.FindBankAccountChangesByOrganizationResponse{})},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries, want only the organization's: no malformed call reached the use case", n)
	}
	if _, _, err := bankAccountChangesTable.Reader(h.store).Load(inTenant(t), "bac-1"); !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("load bac-1 = %v, want no change written by a malformed call", err)
	}
}

const inviteeDomain = "@example.test"

func accented(characters int) string { return strings.Repeat("é", characters) }

func notAccepted(err error, rejection *servicev1.Rejection) error {
	if err == nil && rejection != nil {
		return errors.New("refused in the body: " + rejection.GetCode())
	}
	return err
}

func TestFreeTextIsBoundedInCharactersNotInBytes(t *testing.T) {
	cases := []struct {
		name    string
		ceiling int
		text    func(characters int) string
		call    func(t *testing.T, text string) error
	}{
		{"legal and trade names", 256, accented, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			req := registerRequest("org-1")
			req.LegalName, req.TradeName = text, text
			var r servicev1.RegisterOrganizationResponse
			err := h.invoke(withTenant(t), "RegisterOrganization", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"establishment trade name", 256, accented, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			register(t, h, registerRequest("org-1"))
			req := addRequest("est-1", "org-1")
			req.TradeName = text
			var r servicev1.AddEstablishmentResponse
			err := h.invoke(withTenant(t), "AddEstablishment", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"external reference provider, value and observed version", 256, accented, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			register(t, h, registerRequest("org-1"))
			req := addRequest("est-1", "org-1")
			ref := req.ExternalReferences[0]
			ref.ProviderId, ref.Reference, ref.ObservedVersion = text, text, text
			var r servicev1.AddEstablishmentResponse
			err := h.invoke(withTenant(t), "AddEstablishment", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"person full name", 256, accented, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			req := personRequest("per-1")
			req.FullName = text
			var r servicev1.RegisterPersonResponse
			err := h.invoke(withTenant(t), "RegisterPerson", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"representation source evidence id", 256, accented, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			register(t, h, registerRequest("org-1"))
			registerPerson(t, h, personRequest("per-1"))
			req := representationRequest("rep-1", "org-1", "per-1")
			req.SourceEvidenceId = text
			var r servicev1.RegisterRepresentationResponse
			err := h.invoke(withTenant(t), "RegisterRepresentation", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"suspension reason", 512, accented, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			invokeOK(t, h, "ProvisionUserAccount", provisionRequest("sub-1"), &servicev1.ProvisionUserAccountResponse{})
			req := suspendRequest("sub-1")
			req.Reason = text
			var r servicev1.SuspendUserResponse
			err := h.invoke(withTenant(t), "SuspendUser", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"invitee email", 254, func(characters int) string {
			return accented(characters-len(inviteeDomain)) + inviteeDomain
		}, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			register(t, h, registerRequest("org-1"))
			req := inviteRequest("mbr-1")
			req.InviteeEmail = text
			var r servicev1.InviteUserResponse
			err := h.invoke(withTenant(t), "InviteUser", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"revocation reason", 512, accented, func(t *testing.T, text string) error {
			h := withInvitation(t)
			invokeOK(t, h, "GrantMembership", grantRequest("mbr-1"), &servicev1.GrantMembershipResponse{})
			req := revokeRequest("mbr-1")
			req.Reason = text
			var r servicev1.RevokeMembershipResponse
			err := h.invoke(withTenant(t), "RevokeMembership", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
		{"destination evidence id", 256, accented, func(t *testing.T, text string) error {
			h := newHarness(t, nil)
			register(t, h, registerRequest("org-1"))
			req := requestChangeRequest("bac-1")
			req.DestinationEvidenceId = text
			var r servicev1.RequestBankAccountChangeResponse
			err := h.invoke(withTenant(t), "RequestBankAccountChange", req, &r)
			return notAccepted(err, r.GetRejection())
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			atCeiling := c.text(c.ceiling)
			if n := utf8.RuneCountInString(atCeiling); n != c.ceiling || len(atCeiling) <= c.ceiling {
				t.Fatalf("text holds %d characters in %d bytes, want %d characters in more bytes than that", n, len(atCeiling), c.ceiling)
			}
			if err := c.call(t, atCeiling); err != nil {
				t.Fatalf("%d characters in %d bytes = %v, want accepted", c.ceiling, len(atCeiling), err)
			}
			if err := c.call(t, c.text(c.ceiling+1)); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("%d characters = %v, want InvalidArgument", c.ceiling+1, err)
			}
		})
	}
}

func TestCodesTaxIDsAndActorsStayBoundedInBytes(t *testing.T) {
	h := newHarness(t, nil)
	register(t, h, registerRequest("org-1"))

	command := func(method string, req, resp proto.Message) func() error {
		return func() error { return h.invoke(withTenant(t), method, req, resp) }
	}
	organization := func(edit func(*servicev1.RegisterOrganizationRequest)) func() error {
		req := registerRequest("org-2")
		edit(req)
		return command("RegisterOrganization", req, &servicev1.RegisterOrganizationResponse{})
	}
	representation := func(edit func(*servicev1.RegisterRepresentationRequest)) func() error {
		req := representationRequest("rep-1", "org-1", "per-1")
		edit(req)
		return command("RegisterRepresentation", req, &servicev1.RegisterRepresentationResponse{})
	}
	add := addRequest("est-1", "org-1")
	add.TaxId = accented(17)
	person := personRequest("per-1")
	person.Document = accented(17)
	invite := inviteRequest("mbr-1")
	invite.InvitedByUserId = accented(129)
	change := requestChangeRequest("bac-1")
	change.RequestedByUserId = accented(129)
	calls := []struct {
		name string
		call func() error
	}{
		{"tax id of 17 characters in 34 bytes", organization(func(r *servicev1.RegisterOrganizationRequest) { r.TaxId = accented(17) })},
		{"establishment tax id of 17 characters in 34 bytes", command("AddEstablishment", add, &servicev1.AddEstablishmentResponse{})},
		{"tax id to find of 17 characters in 34 bytes", command("FindOrganizationByTaxId", &servicev1.FindOrganizationByTaxIdRequest{
			TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CPF, TaxId: accented(17), Jurisdiction: "BR",
		}, &servicev1.FindOrganizationByTaxIdResponse{})},
		{"person document of 17 characters in 34 bytes", command("RegisterPerson", person, &servicev1.RegisterPersonResponse{})},
		{"jurisdiction of 5 characters in 10 bytes", organization(func(r *servicev1.RegisterOrganizationRequest) { r.Jurisdiction = accented(5) })},
		{"currency of 5 characters in 10 bytes", representation(func(r *servicev1.RegisterRepresentationRequest) { r.ScopeCurrency = accented(5) })},
		{"act type of 33 characters in 66 bytes", representation(func(r *servicev1.RegisterRepresentationRequest) { r.ScopeActTypes = []string{accented(33)} })},
		{"proposer of 129 characters in 258 bytes", representation(func(r *servicev1.RegisterRepresentationRequest) { r.ProposedByUserId = accented(129) })},
		{"inviter of 129 characters in 258 bytes", command("InviteUser", invite, &servicev1.InviteUserResponse{})},
		{"requester of 129 characters in 258 bytes", command("RequestBankAccountChange", change, &servicev1.RequestBankAccountChangeResponse{})},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if n := len(h.store.Entries()); n != 1 {
		t.Fatalf("outbox holds %d entries, want only the organization's: no value over a byte ceiling reached the use case", n)
	}
}

var controlCharacters = map[string]string{"NUL": "\x00", "tab": "\t", "ESC": "\x1b", "DEL": "\x7f", "NEL": "\u0085"}

func refusedAtTheEdge(t *testing.T, h *harness, method string, req, resp proto.Message, refusal string) {
	t.Helper()
	err := h.invoke(withTenant(t), method, req, resp)
	if status.Code(err) != codes.InvalidArgument || !strings.Contains(status.Convert(err).Message(), refusal) {
		t.Fatalf("%s() = %v, want InvalidArgument saying %q", method, err, refusal)
	}
	if n := len(h.store.Entries()); n != 0 {
		t.Fatalf("outbox holds %d entries, want none: the refusal comes before the use case", n)
	}
}

func TestFreeTextHoldingAControlCharacterIsRefusedBeforeTheUseCase(t *testing.T) {
	cases := []struct {
		name, method string
		req          func(text string) proto.Message
		resp         proto.Message
	}{
		{"legal name", "RegisterOrganization", func(text string) proto.Message {
			r := registerRequest("org-1")
			r.LegalName = text
			return r
		}, &servicev1.RegisterOrganizationResponse{}},
		{"organization trade name", "RegisterOrganization", func(text string) proto.Message {
			r := registerRequest("org-1")
			r.TradeName = text
			return r
		}, &servicev1.RegisterOrganizationResponse{}},
		{"establishment trade name", "AddEstablishment", func(text string) proto.Message {
			r := addRequest("est-1", "org-1")
			r.TradeName = text
			return r
		}, &servicev1.AddEstablishmentResponse{}},
		{"reference provider", "AddEstablishment", func(text string) proto.Message {
			r := addRequest("est-1", "org-1")
			r.ExternalReferences[0].ProviderId = text
			return r
		}, &servicev1.AddEstablishmentResponse{}},
		{"reference value", "AddEstablishment", func(text string) proto.Message {
			r := addRequest("est-1", "org-1")
			r.ExternalReferences[0].Reference = text
			return r
		}, &servicev1.AddEstablishmentResponse{}},
		{"reference observed version", "AddEstablishment", func(text string) proto.Message {
			r := addRequest("est-1", "org-1")
			r.ExternalReferences[0].ObservedVersion = text
			return r
		}, &servicev1.AddEstablishmentResponse{}},
		{"person full name", "RegisterPerson", func(text string) proto.Message {
			r := personRequest("per-1")
			r.FullName = text
			return r
		}, &servicev1.RegisterPersonResponse{}},
		{"source evidence id", "RegisterRepresentation", func(text string) proto.Message {
			r := representationRequest("rep-1", "org-1", "per-1")
			r.SourceEvidenceId = text
			return r
		}, &servicev1.RegisterRepresentationResponse{}},
		{"proposer", "RegisterRepresentation", func(text string) proto.Message {
			r := representationRequest("rep-1", "org-1", "per-1")
			r.ProposedByUserId = text
			return r
		}, &servicev1.RegisterRepresentationResponse{}},
		{"suspension reason", "SuspendUser", func(text string) proto.Message {
			r := suspendRequest("sub-1")
			r.Reason = text
			return r
		}, &servicev1.SuspendUserResponse{}},
		{"invitee email", "InviteUser", func(text string) proto.Message {
			r := inviteRequest("mbr-1")
			r.InviteeEmail = text
			return r
		}, &servicev1.InviteUserResponse{}},
		{"inviter", "InviteUser", func(text string) proto.Message {
			r := inviteRequest("mbr-1")
			r.InvitedByUserId = text
			return r
		}, &servicev1.InviteUserResponse{}},
		{"revocation reason", "RevokeMembership", func(text string) proto.Message {
			r := revokeRequest("mbr-1")
			r.Reason = text
			return r
		}, &servicev1.RevokeMembershipResponse{}},
		{"destination evidence id", "RequestBankAccountChange", func(text string) proto.Message {
			r := requestChangeRequest("bac-1")
			r.DestinationEvidenceId = text
			return r
		}, &servicev1.RequestBankAccountChangeResponse{}},
		{"requester", "RequestBankAccountChange", func(text string) proto.Message {
			r := requestChangeRequest("bac-1")
			r.RequestedByUserId = text
			return r
		}, &servicev1.RequestBankAccountChangeResponse{}},
		{"approver", "ApproveBankAccountChange", func(text string) proto.Message {
			return approveChangeRequest("bac-1", text)
		}, &servicev1.ApproveBankAccountChangeResponse{}},
	}
	for _, c := range cases {
		for name, control := range controlCharacters {
			t.Run(c.name+" with "+name, func(t *testing.T) {
				refusedAtTheEdge(t, newHarness(t, nil), c.method, c.req("Ana"+control+"Sintetica"), c.resp, "control characters")
			})
		}
	}
}

func TestCodesOutsidePrintableASCIIAreRefusedBeforeTheUseCase(t *testing.T) {
	cases := []struct {
		name, method string
		req          func(code string) proto.Message
		resp         proto.Message
	}{
		{"tax id", "RegisterOrganization", func(code string) proto.Message {
			r := registerRequest("org-1")
			r.TaxId = "12ABC345" + code
			return r
		}, &servicev1.RegisterOrganizationResponse{}},
		{"jurisdiction", "RegisterOrganization", func(code string) proto.Message {
			r := registerRequest("org-1")
			r.Jurisdiction = "B" + code
			return r
		}, &servicev1.RegisterOrganizationResponse{}},
		{"tax id to find", "FindOrganizationByTaxId", func(code string) proto.Message {
			return &servicev1.FindOrganizationByTaxIdRequest{TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ, TaxId: "12ABC345" + code, Jurisdiction: "BR"}
		}, &servicev1.FindOrganizationByTaxIdResponse{}},
		{"establishment tax id", "AddEstablishment", func(code string) proto.Message {
			r := addRequest("est-1", "org-1")
			r.TaxId = "12ABC345" + code
			return r
		}, &servicev1.AddEstablishmentResponse{}},
		{"person document", "RegisterPerson", func(code string) proto.Message {
			r := personRequest("per-1")
			r.Document = "123.456.789" + code
			return r
		}, &servicev1.RegisterPersonResponse{}},
		{"scope currency", "RegisterRepresentation", func(code string) proto.Message {
			r := representationRequest("rep-1", "org-1", "per-1")
			r.ScopeCurrency = "BR" + code
			return r
		}, &servicev1.RegisterRepresentationResponse{}},
		{"act type", "RegisterRepresentation", func(code string) proto.Message {
			r := representationRequest("rep-1", "org-1", "per-1")
			r.ScopeActTypes = []string{"assignment", "consent" + code}
			return r
		}, &servicev1.RegisterRepresentationResponse{}},
	}
	for _, c := range cases {
		for name, outside := range map[string]string{"NUL": "\x00", "DEL": "\x7f", "accented letter": "é", "NBSP": " "} {
			t.Run(c.name+" with "+name, func(t *testing.T) {
				refusedAtTheEdge(t, newHarness(t, nil), c.method, c.req(outside), c.resp, "printable ASCII")
			})
		}
	}
}
