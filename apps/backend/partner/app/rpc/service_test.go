package rpc_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const (
	day        = int64(86_400_000_000_000)
	validFrom  = int64(occurred)
	validUntil = int64(occurred) + 365*day
)

var descriptor = servicev1.File_company_partner_service_v1_partner_service_proto.Services().ByName("PartnerService")

func registerPartnerRequest(partnerID string) *servicev1.RegisterPartnerRequest {
	return &servicev1.RegisterPartnerRequest{PartnerId: partnerID, OrganizationId: "org-1", Role: servicev1.PartnerRole_PARTNER_ROLE_FUNDER}
}

func registerAgreementRequest(agreementID string) *servicev1.RegisterAgreementRequest {
	return &servicev1.RegisterAgreementRequest{
		AgreementId: agreementID, PartnerId: "prt-1", ValidFrom: validFrom, ValidUntil: validUntil, InstrumentTemplateId: "tpl-1",
	}
}

func activateAgreementRequest(agreementID string) *servicev1.ActivateAgreementRequest {
	return &servicev1.ActivateAgreementRequest{
		AgreementId: agreementID, ContractEvidenceId: "evd-1", SatisfiedCriteria: []string{"H14", "H15", "H17", "H18"},
	}
}

func draft(id domain.AgreementID) domain.PartnerAgreementSnapshot {
	return domain.PartnerAgreementSnapshot{
		ID: id, PartnerID: "prt-1", Version: 1, Status: domain.AgreementDraft, ValidFrom: occurred, ValidUntil: occurred + 365*86_400_000_000_000,
	}
}

func registerProfileRequest(profileID string) *servicev1.RegisterIntegrationProfileRequest {
	return &servicev1.RegisterIntegrationProfileRequest{
		ProfileId: profileID, PartnerId: "prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION,
		ChangeKind: servicev1.IntegrationChangeKind_INTEGRATION_CHANGE_KIND_INITIAL, Protocols: []string{"https", "mtls"},
		Endpoint: "https://partner.example/api", SchemaVersion: "v3", SecretRef: "vault://partners/prt-1/api-key",
		CertificateRef: "cert-1", CertificateExpiresAt: int64(occurred) + 90*day, EffectiveFrom: int64(occurred),
	}
}

func definePolicyRequest(policyID string) *servicev1.DefineOperationalPolicyRequest {
	return &servicev1.DefineOperationalPolicyRequest{
		PolicyId: policyID, PartnerId: "prt-1", EffectiveFrom: int64(occurred), RateLimitPerMinute: 100,
		BusinessDays: []string{"mon", "tue", "wed", "thu", "fri"}, CutoffTime: "17:00", ProcessingWindow: "08:00-18:00",
		SlaRef: "sla-gold", Fallback: "manual queue", SupportContact: "support@partner.example", IncidentContact: "+55 11 99999-0000",
	}
}

func expiringIn(id domain.ProfileID, remaining int64, lastAlerted int) domain.IntegrationProfileSnapshot {
	return domain.IntegrationProfileSnapshot{
		ID: id, PartnerID: "prt-1", Environment: domain.EnvironmentProduction, Version: 1, ChangeKind: domain.ChangeInitial,
		Endpoint: "https://partner.example/api", SecretRef: "vault://partners/prt-1/api-key",
		CertificateExpiresAt: occurred + port.Instant(remaining), EffectiveFrom: occurred, LastAlertedThresholdDays: lastAlerted,
	}
}

func TestServiceDescBindsEachRegisteredMethodToTheDescriptor(t *testing.T) {
	if rpc.ServiceDesc.ServiceName != string(descriptor.FullName()) || len(rpc.ServiceDesc.Streams) != 0 {
		t.Fatalf("ServiceDesc = %q with %d streams, want %q and no stream", rpc.ServiceDesc.ServiceName, len(rpc.ServiceDesc.Streams), descriptor.FullName())
	}
	if uncovered := kernelgrpc.Uncovered(&rpc.ServiceDesc, descriptor); len(uncovered) != 0 {
		t.Fatalf("Uncovered() = %v, want every method of %s served", uncovered, descriptor.FullName())
	}
	for _, method := range rpc.ServiceDesc.Methods {
		if descriptor.Methods().ByName(protoreflect.Name(method.MethodName)) == nil {
			t.Errorf("method %s is not a method of %s", method.MethodName, descriptor.FullName())
		}
	}
	if got := len(rpc.Methods()); got != descriptor.Methods().Len() || got != 24 {
		t.Fatalf("Methods() = %v, want the 24 methods of the descriptor", rpc.Methods())
	}
}

func TestEveryCommandIsARegisteredMethod(t *testing.T) {
	want := []string{
		"RegisterPartner", "ChangePartnerStage", "RegisterAgreement", "ActivateAgreement",
		"DeclareCapability", "RecordHomologation", "EnableCapability", "SuspendCapability", "ChangeRollout",
		"RegisterIntegrationProfile", "RaiseCertificateExpiryAlert", "DefineOperationalPolicy",
		"DefineProduct", "RetireProduct",
	}
	if got := rpc.Commands(); !slices.Equal(got, want) {
		t.Fatalf("Commands() = %v, want %v", got, want)
	}
	for _, command := range rpc.Commands() {
		if !slices.Contains(rpc.Methods(), command) {
			t.Errorf("command %s is not registered in ServiceDesc", command)
		}
	}
}

func TestTheServerImplementsTheHandlerType(t *testing.T) {
	handlerType := reflect.TypeOf(rpc.ServiceDesc.HandlerType).Elem()
	if handlerType.Kind() != reflect.Interface || !reflect.TypeOf(rpc.Server{}).Implements(handlerType) {
		t.Fatalf("rpc.Server does not implement %v", handlerType)
	}
}

func TestRegisterPartnerAnswersTheRegisteredPartner(t *testing.T) {
	h := newHarness(t, relations{})

	var resp servicev1.RegisterPartnerResponse
	if err := h.invoke(withTenant(t), "RegisterPartner", registerPartnerRequest("prt-1"), &resp); err != nil {
		t.Fatalf("RegisterPartner() = %v", err)
	}

	if got := resp.GetRegistered().GetPartnerId(); got != "prt-1" {
		t.Fatalf("Registered.PartnerId = %q, want prt-1 (response %v)", got, &resp)
	}
	if entries := h.store.Entries(); len(entries) != 1 {
		t.Fatalf("outbox holds %d entries, want the PartnerRegistered", len(entries))
	}
}

func TestRegisterPartnerAnswersEachRefusalInTheBody(t *testing.T) {
	cases := []struct {
		name    string
		planted relations
		edit    func(*servicev1.RegisterPartnerRequest)
		code    string
	}{
		{"organization unknown to identity", relations{}, func(r *servicev1.RegisterPartnerRequest) { r.OrganizationId = "org-none" },
			"partner-management/partner/organization-not-found"},
		{"registry of a funder organization", relations{}, func(r *servicev1.RegisterPartnerRequest) { r.Role = servicev1.PartnerRole_PARTNER_ROLE_REGISTRY },
			"partner-management/partner/organization-kind-mismatch"},
		{"funder already registered", relations{partners: partnersByOrganization{"org-1": {{ID: "prt-held", OrganizationID: "org-1", Role: domain.RoleFunder}}}},
			func(*servicev1.RegisterPartnerRequest) {}, "partner-management/partner/already-registered"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, c.planted)
			req := registerPartnerRequest("prt-refused")
			c.edit(req)

			var resp servicev1.RegisterPartnerResponse
			if err := h.invoke(withTenant(t), "RegisterPartner", req, &resp); err != nil {
				t.Fatalf("RegisterPartner() = %v", err)
			}

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection code = %q, want %q (response %v)", got, c.code, &resp)
			}
			if len(h.store.Entries()) != 0 {
				t.Fatalf("outbox holds %d entries after a refusal, want none", len(h.store.Entries()))
			}
		})
	}
}

func TestRegisteringTheSamePartnerUnderAnotherKeyAlreadyExists(t *testing.T) {
	h := newHarness(t, relations{})
	var first, second servicev1.RegisterPartnerResponse
	if err := h.invoke(withTenant(t), "RegisterPartner", registerPartnerRequest("prt-1"), &first); err != nil {
		t.Fatalf("RegisterPartner() = %v", err)
	}

	err := h.invoke(withTenant(t), "RegisterPartner", registerPartnerRequest("prt-1"), &second)

	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("second RegisterPartner() = %v, want AlreadyExists", err)
	}
}

func TestRegisterAgreementAnswersTheRegisteredDraftAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{agreements: agreementsByPartner{"prt-2": {draft("agr-0")}}})

	var registered servicev1.RegisterAgreementResponse
	if err := h.invoke(withTenant(t), "RegisterAgreement", registerAgreementRequest("agr-1"), &registered); err != nil {
		t.Fatalf("RegisterAgreement() = %v", err)
	}
	if got := registered.GetRegistered().GetAgreementId(); got != "agr-1" {
		t.Fatalf("Registered.AgreementId = %q, want agr-1 (response %v)", got, &registered)
	}

	for name, c := range map[string]struct {
		req  *servicev1.RegisterAgreementRequest
		code string
	}{
		"valid until before valid from": {&servicev1.RegisterAgreementRequest{AgreementId: "agr-2", PartnerId: "prt-1", ValidFrom: validFrom, ValidUntil: validFrom - 1},
			"partner-management/partner-agreement/validity-invalid"},
		"second first version of the series": {&servicev1.RegisterAgreementRequest{AgreementId: "agr-3", PartnerId: "prt-2", ValidFrom: validFrom},
			"partner-management/partner-agreement/supersedes-invalid"},
	} {
		var resp servicev1.RegisterAgreementResponse
		if err := h.invoke(withTenant(t), "RegisterAgreement", c.req, &resp); err != nil {
			t.Fatalf("%s: RegisterAgreement() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}
}

func TestActivateAgreementAnswersTheActivationAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{})
	h.seedAgreement(t, draft("agr-1"))
	h.seedAgreement(t, draft("agr-2"))

	var activated servicev1.ActivateAgreementResponse
	if err := h.invoke(withTenant(t), "ActivateAgreement", activateAgreementRequest("agr-1"), &activated); err != nil {
		t.Fatalf("ActivateAgreement() = %v", err)
	}
	if got := activated.GetActivated().GetAgreementId(); got != "agr-1" || len(h.store.Entries()) != 1 {
		t.Fatalf("Activated.AgreementId = %q with %d outbox entries, want agr-1 and the PartnerAgreementActivated", got, len(h.store.Entries()))
	}

	for name, c := range map[string]struct {
		req  *servicev1.ActivateAgreementRequest
		code string
	}{
		"already active": {activateAgreementRequest("agr-1"), "partner-management/partner-agreement/not-draft"},
		"evidence missing": {&servicev1.ActivateAgreementRequest{AgreementId: "agr-2", SatisfiedCriteria: []string{"H14", "H15", "H17", "H18"}},
			"partner-management/partner-agreement/contract-evidence-missing"},
		"without H17": {&servicev1.ActivateAgreementRequest{AgreementId: "agr-2", ContractEvidenceId: "evd-1", SatisfiedCriteria: []string{"H14", "H15", "H18"}},
			"partner-management/partner-agreement/contractual-criteria-incomplete"},
	} {
		var resp servicev1.ActivateAgreementResponse
		if err := h.invoke(withTenant(t), "ActivateAgreement", c.req, &resp); err != nil {
			t.Fatalf("%s: ActivateAgreement() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}

	var unknown servicev1.ActivateAgreementResponse
	if err := h.invoke(withTenant(t), "ActivateAgreement", activateAgreementRequest("agr-none"), &unknown); status.Code(err) != codes.NotFound {
		t.Fatalf("ActivateAgreement(agr-none) = %v, want NotFound", err)
	}
}

func TestMalformedRequestsAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, relations{})
	long := strings.Repeat("a", 129)
	partner := func(edit func(*servicev1.RegisterPartnerRequest)) func() error {
		return func() error {
			req := registerPartnerRequest("prt-1")
			edit(req)
			var r servicev1.RegisterPartnerResponse
			return h.invoke(withTenant(t), "RegisterPartner", req, &r)
		}
	}
	agreement := func(edit func(*servicev1.RegisterAgreementRequest)) func() error {
		return func() error {
			req := registerAgreementRequest("agr-1")
			edit(req)
			var r servicev1.RegisterAgreementResponse
			return h.invoke(withTenant(t), "RegisterAgreement", req, &r)
		}
	}
	activation := func(edit func(*servicev1.ActivateAgreementRequest)) func() error {
		return func() error {
			req := activateAgreementRequest("agr-1")
			edit(req)
			var r servicev1.ActivateAgreementResponse
			return h.invoke(withTenant(t), "ActivateAgreement", req, &r)
		}
	}
	for _, c := range []struct {
		name string
		call func() error
	}{
		{"partner id with a space", partner(func(r *servicev1.RegisterPartnerRequest) { r.PartnerId = "prt 1" })},
		{"partner id empty", partner(func(r *servicev1.RegisterPartnerRequest) { r.PartnerId = "" })},
		{"organization id too long", partner(func(r *servicev1.RegisterPartnerRequest) { r.OrganizationId = long })},
		{"unspecified role", partner(func(r *servicev1.RegisterPartnerRequest) { r.Role = servicev1.PartnerRole_PARTNER_ROLE_UNSPECIFIED })},
		{"role outside the enum", partner(func(r *servicev1.RegisterPartnerRequest) { r.Role = 99 })},
		{"agreement id empty", agreement(func(r *servicev1.RegisterAgreementRequest) { r.AgreementId = "" })},
		{"agreement of no partner", agreement(func(r *servicev1.RegisterAgreementRequest) { r.PartnerId = "" })},
		{"negative valid from", agreement(func(r *servicev1.RegisterAgreementRequest) { r.ValidFrom = -1 })},
		{"negative valid until", agreement(func(r *servicev1.RegisterAgreementRequest) { r.ValidUntil = -1 })},
		{"instrument template with a space", agreement(func(r *servicev1.RegisterAgreementRequest) { r.InstrumentTemplateId = "tpl 1" })},
		{"supersedes too long", agreement(func(r *servicev1.RegisterAgreementRequest) { r.SupersedesId = long })},
		{"activation of no agreement", activation(func(r *servicev1.ActivateAgreementRequest) { r.AgreementId = "" })},
		{"evidence with a space", activation(func(r *servicev1.ActivateAgreementRequest) { r.ContractEvidenceId = "evd 1" })},
		{"criterion outside the catalogue", activation(func(r *servicev1.ActivateAgreementRequest) { r.SatisfiedCriteria = []string{"H14", "X1"} })},
		{"too many criteria", activation(func(r *servicev1.ActivateAgreementRequest) { r.SatisfiedCriteria = slices.Repeat([]string{"H14"}, 33) })},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if len(h.store.Entries()) != 0 {
		t.Fatalf("outbox holds %d entries after malformed requests, want none", len(h.store.Entries()))
	}
}

func TestACommandWithoutAKeyIsRefused(t *testing.T) {
	h := newHarness(t, relations{})

	var resp servicev1.RegisterPartnerResponse
	err := h.invoke(withoutKey(t), "RegisterPartner", registerPartnerRequest("prt-1"), &resp)

	if status.Code(err) != codes.InvalidArgument || kernelgrpc.ReasonOf(err) != kernelgrpc.ReasonMissingIdempotencyKey {
		t.Fatalf("RegisterPartner() without a key = %v, want InvalidArgument with %s", err, kernelgrpc.ReasonMissingIdempotencyKey)
	}
}

func TestRegisterIntegrationProfileAnswersTheRegisteredProfileAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{profiles: profilesByPartner{"prt-2": {expiringIn("ipf-0", 90*day, 0)}}})

	var registered servicev1.RegisterIntegrationProfileResponse
	if err := h.invoke(withTenant(t), "RegisterIntegrationProfile", registerProfileRequest("ipf-1"), &registered); err != nil {
		t.Fatalf("RegisterIntegrationProfile() = %v", err)
	}
	if got := registered.GetRegistered().GetProfileId(); got != "ipf-1" || len(h.store.Entries()) != 0 {
		t.Fatalf("Registered.ProfileId = %q with %d outbox entries, want ipf-1 and nothing emitted", got, len(h.store.Entries()))
	}

	for name, c := range map[string]struct {
		edit func(*servicev1.RegisterIntegrationProfileRequest)
		code string
	}{
		"endpoint empty":                          {func(r *servicev1.RegisterIntegrationProfileRequest) { r.Endpoint = "" }, "partner-management/integration-profile/endpoint-empty"},
		"without the secret reference":            {func(r *servicev1.RegisterIntegrationProfileRequest) { r.SecretRef = "" }, "partner-management/integration-profile/secret-ref-missing"},
		"certificate already expired":             {func(r *servicev1.RegisterIntegrationProfileRequest) { r.CertificateExpiresAt = int64(occurred) - day }, "partner-management/integration-profile/certificate-expiry-invalid"},
		"second first version of the environment": {func(r *servicev1.RegisterIntegrationProfileRequest) { r.PartnerId = "prt-2" }, "partner-management/integration-profile/supersedes-invalid"},
	} {
		req := registerProfileRequest("ipf-refused")
		c.edit(req)
		var resp servicev1.RegisterIntegrationProfileResponse
		if err := h.invoke(withTenant(t), "RegisterIntegrationProfile", req, &resp); err != nil {
			t.Fatalf("%s: RegisterIntegrationProfile() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}
}

func TestRaiseCertificateExpiryAlertAnswersTheAlertAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{})
	h.seedProfile(t, expiringIn("ipf-25", 25*day, 0))
	h.seedProfile(t, expiringIn("ipf-90", 90*day, 0))

	var raised servicev1.RaiseCertificateExpiryAlertResponse
	if err := h.invoke(withTenant(t), "RaiseCertificateExpiryAlert", &servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "ipf-25", ThresholdDays: 30}, &raised); err != nil {
		t.Fatalf("RaiseCertificateExpiryAlert() = %v", err)
	}
	if got := raised.GetRaised().GetProfileId(); got != "ipf-25" || len(h.store.Entries()) != 1 {
		t.Fatalf("Raised.ProfileId = %q with %d outbox entries, want ipf-25 and the PartnerCertificateExpiring", got, len(h.store.Entries()))
	}

	for name, c := range map[string]struct {
		req  *servicev1.RaiseCertificateExpiryAlertRequest
		code string
	}{
		"threshold of 45 days":            {&servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "ipf-90", ThresholdDays: 45}, "partner-management/integration-profile/invalid-alert-threshold"},
		"certificate expiring in 90 days": {&servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "ipf-90", ThresholdDays: 60}, "partner-management/integration-profile/threshold-not-reached"},
		"60 days after 30 were alerted":   {&servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "ipf-25", ThresholdDays: 60}, "partner-management/integration-profile/already-alerted"},
	} {
		var resp servicev1.RaiseCertificateExpiryAlertResponse
		if err := h.invoke(withTenant(t), "RaiseCertificateExpiryAlert", c.req, &resp); err != nil {
			t.Fatalf("%s: RaiseCertificateExpiryAlert() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}

	var unknown servicev1.RaiseCertificateExpiryAlertResponse
	err := h.invoke(withTenant(t), "RaiseCertificateExpiryAlert", &servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "ipf-none", ThresholdDays: 30}, &unknown)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("RaiseCertificateExpiryAlert(ipf-none) = %v, want NotFound", err)
	}
}

func TestTheDeterministicKeyOfAnAlertReplaysTheRecordedAnswer(t *testing.T) {
	h := newHarness(t, relations{})
	h.seedProfile(t, expiringIn("ipf-25", 25*day, 0))
	req := &servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "ipf-25", ThresholdDays: 30}

	var first, again servicev1.RaiseCertificateExpiryAlertResponse
	var header metadata.MD
	if err := h.invoke(withKey(t, "cert-alert.ipf-25.30.20250817"), "RaiseCertificateExpiryAlert", req, &first); err != nil {
		t.Fatalf("RaiseCertificateExpiryAlert() = %v", err)
	}
	if err := h.invoke(withKey(t, "cert-alert.ipf-25.30.20250817"), "RaiseCertificateExpiryAlert", req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated RaiseCertificateExpiryAlert() = %v", err)
	}

	if again.GetRaised().GetProfileId() != "ipf-25" || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want Raised ipf-25 marked as replay", &again, header)
	}
	if entries := h.store.Entries(); len(entries) != 1 {
		t.Fatalf("outbox holds %d entries, want the single PartnerCertificateExpiring", len(entries))
	}
}

func TestDefineOperationalPolicyAnswersTheDefinedPolicyAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{policies: policiesByPartner{"prt-2": {{ID: "pol-0", PartnerID: "prt-2", Version: 1}}}})

	var defined servicev1.DefineOperationalPolicyResponse
	if err := h.invoke(withTenant(t), "DefineOperationalPolicy", definePolicyRequest("pol-1"), &defined); err != nil {
		t.Fatalf("DefineOperationalPolicy() = %v", err)
	}
	if got := defined.GetDefined().GetPolicyId(); got != "pol-1" || len(h.store.Entries()) != 0 {
		t.Fatalf("Defined.PolicyId = %q with %d outbox entries, want pol-1 and nothing emitted", got, len(h.store.Entries()))
	}

	for name, c := range map[string]struct {
		edit func(*servicev1.DefineOperationalPolicyRequest)
		code string
	}{
		"rate limit of zero":                  {func(r *servicev1.DefineOperationalPolicyRequest) { r.RateLimitPerMinute = 0 }, "partner-management/operational-policy/rate-limit-invalid"},
		"without the incident contact":        {func(r *servicev1.DefineOperationalPolicyRequest) { r.IncidentContact = "" }, "partner-management/operational-policy/contacts-missing"},
		"second first version of the partner": {func(r *servicev1.DefineOperationalPolicyRequest) { r.PartnerId = "prt-2" }, "partner-management/operational-policy/supersedes-invalid"},
	} {
		req := definePolicyRequest("pol-refused")
		c.edit(req)
		var resp servicev1.DefineOperationalPolicyResponse
		if err := h.invoke(withTenant(t), "DefineOperationalPolicy", req, &resp); err != nil {
			t.Fatalf("%s: DefineOperationalPolicy() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
		if message := resp.GetRejection().GetMessage(); strings.Contains(message, "support@partner.example") || strings.Contains(message, "99999") {
			t.Fatalf("%s: rejection message %q carries a contact", name, message)
		}
	}
}

func TestMalformedProfileAndPolicyRequestsAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, relations{})
	long := strings.Repeat("a", 129)
	profile := func(edit func(*servicev1.RegisterIntegrationProfileRequest)) func() error {
		return func() error {
			req := registerProfileRequest("ipf-1")
			edit(req)
			var r servicev1.RegisterIntegrationProfileResponse
			return h.invoke(withTenant(t), "RegisterIntegrationProfile", req, &r)
		}
	}
	alert := func(req *servicev1.RaiseCertificateExpiryAlertRequest) func() error {
		return func() error {
			var r servicev1.RaiseCertificateExpiryAlertResponse
			return h.invoke(withTenant(t), "RaiseCertificateExpiryAlert", req, &r)
		}
	}
	policy := func(edit func(*servicev1.DefineOperationalPolicyRequest)) func() error {
		return func() error {
			req := definePolicyRequest("pol-1")
			edit(req)
			var r servicev1.DefineOperationalPolicyResponse
			return h.invoke(withTenant(t), "DefineOperationalPolicy", req, &r)
		}
	}
	for _, c := range []struct {
		name string
		call func() error
	}{
		{"profile id empty", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.ProfileId = "" })},
		{"profile id with a colon", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.ProfileId = "ipf:1" })},
		{"profile id over 100 characters", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.ProfileId = strings.Repeat("a", 101) })},
		{"alert of a profile id with a colon", alert(&servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "ipf:1", ThresholdDays: 30})},
		{"alert of a profile id over 100 characters", alert(&servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: strings.Repeat("a", 101), ThresholdDays: 30})},
		{"profile of no partner", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.PartnerId = "" })},
		{"unspecified environment", profile(func(r *servicev1.RegisterIntegrationProfileRequest) {
			r.Environment = servicev1.Environment_ENVIRONMENT_UNSPECIFIED
		})},
		{"environment outside the enum", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.Environment = 99 })},
		{"unspecified change kind", profile(func(r *servicev1.RegisterIntegrationProfileRequest) {
			r.ChangeKind = servicev1.IntegrationChangeKind_INTEGRATION_CHANGE_KIND_UNSPECIFIED
		})},
		{"too many protocols", profile(func(r *servicev1.RegisterIntegrationProfileRequest) {
			r.Protocols = slices.Repeat([]string{"https"}, 17)
		})},
		{"protocol with a control character", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.Protocols = []string{"https\n"} })},
		{"endpoint too long", profile(func(r *servicev1.RegisterIntegrationProfileRequest) {
			r.Endpoint = "https://" + strings.Repeat("a", 2041)
		})},
		{"endpoint outside printable ascii", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.Endpoint = "https://parceiro.exemplo/ação" })},
		{"schema version too long", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.SchemaVersion = strings.Repeat("v", 65) })},
		{"secret reference too long", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.SecretRef = strings.Repeat("s", 257) })},
		{"certificate reference with a tab", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.CertificateRef = "cert\t1" })},
		{"negative certificate expiry", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.CertificateExpiresAt = -1 })},
		{"negative effective from", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.EffectiveFrom = -1 })},
		{"profile supersedes too long", profile(func(r *servicev1.RegisterIntegrationProfileRequest) { r.SupersedesId = long })},
		{"alert of no profile", alert(&servicev1.RaiseCertificateExpiryAlertRequest{ThresholdDays: 30})},
		{"policy id with a space", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.PolicyId = "pol 1" })},
		{"policy of no partner", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.PartnerId = "" })},
		{"negative policy effective from", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.EffectiveFrom = -1 })},
		{"policy supersedes too long", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.SupersedesId = long })},
		{"eight business days", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.BusinessDays = slices.Repeat([]string{"mon"}, 8) })},
		{"cutoff time too long", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.CutoffTime = strings.Repeat("1", 65) })},
		{"processing window with a line break", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.ProcessingWindow = "08:00\n18:00" })},
		{"sla too long", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.SlaRef = strings.Repeat("s", 513) })},
		{"fallback with a control character", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.Fallback = "manual\x00queue" })},
		{"support contact too long", policy(func(r *servicev1.DefineOperationalPolicyRequest) { r.SupportContact = strings.Repeat("ç", 513) })},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := c.call()
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
			if message := status.Convert(err).Message(); strings.Contains(message, "support@partner.example") || strings.Contains(message, "99999") {
				t.Fatalf("refusal %q carries a contact", message)
			}
		})
	}
	if len(h.store.Entries()) != 0 {
		t.Fatalf("outbox holds %d entries after malformed requests, want none", len(h.store.Entries()))
	}
}

func TestTheLongestTextsTheContractAdmitsPass(t *testing.T) {
	h := newHarness(t, relations{})
	profile := registerProfileRequest("ipf-long")
	profile.Endpoint = "https://" + strings.Repeat("a", 2040)
	profile.SecretRef = strings.Repeat("s", 256)
	policy := definePolicyRequest("pol-long")
	policy.SupportContact = strings.Repeat("ç", 512)
	policy.BusinessDays = slices.Repeat([]string{"mon"}, 7)

	var registered servicev1.RegisterIntegrationProfileResponse
	var defined servicev1.DefineOperationalPolicyResponse
	if err := h.invoke(withTenant(t), "RegisterIntegrationProfile", profile, &registered); err != nil || registered.GetRegistered() == nil {
		t.Fatalf("RegisterIntegrationProfile() = %v, %v; want the profile at the ceilings registered", &registered, err)
	}
	if err := h.invoke(withTenant(t), "DefineOperationalPolicy", policy, &defined); err != nil || defined.GetDefined() == nil {
		t.Fatalf("DefineOperationalPolicy() = %v; want the policy at the ceilings defined", err)
	}
}
