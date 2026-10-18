package rpc_test

import (
	"slices"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func effectiveRequest() *servicev1.FindEffectiveCapabilityRequest {
	return &servicev1.FindEffectiveCapabilityRequest{
		PartnerId: "prt-1", Operation: "register-receivable", AssetType: "duplicata", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION,
		TenantId: "tenant-a", AsOf: int64(occurred),
	}
}

func enabledEverywhere(id domain.CapabilityID) domain.CapabilitySnapshot {
	c := capabilityAt(id, domain.CapabilityStatusEnabled)
	c.Rollout = domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 10, AllowlistTenantIDs: []string{"tenant-b"}, ExcludedTenantIDs: []string{"tenant-x"}}
	return c
}

func effectivePlanted() relations {
	agreement := activeAgreement()
	agreement.ValidUntil = 0
	return relations{
		agreements:   agreementsByPartner{"prt-1": {agreement}},
		capabilities: capabilitiesByPartner{"prt-1": {enabledEverywhere("cap-1")}},
		products:     productsByPartner{"prt-1": {productAt("fpr-1", domain.ProductActive, occurred+86_400_000_000_000)}},
		bucket:       9,
	}
}

func TestEveryQueryIsServedWithoutAnIdempotencyKey(t *testing.T) {
	h := newHarness(t, effectivePlanted())
	seedIn(t, h, partnersTable, "prt-1", partnerAt("prt-1", domain.PartnerActive))

	var resp servicev1.FindEffectiveCapabilityResponse
	if err := h.invoke(withoutKey(t), "FindEffectiveCapability", effectiveRequest(), &resp); err != nil {
		t.Fatalf("FindEffectiveCapability() without a key = %v, want served: a query reads no key", err)
	}
	if !resp.GetAllowed() || resp.GetFailedCondition() != servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_UNSPECIFIED {
		t.Fatalf("FindEffectiveCapability() = %v, want allowed", &resp)
	}
	if entries := h.store.Entries(); len(entries) != 0 {
		t.Fatalf("outbox holds %d entries, want a query to write nothing", len(entries))
	}
}

func TestFindEffectiveCapabilityAnswersTheConditionThatFailed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status domain.PartnerStatus
		edit   func(*servicev1.FindEffectiveCapabilityRequest)
		want   servicev1.EffectiveCapabilityCondition
	}{
		{"unknown series", domain.PartnerActive, func(r *servicev1.FindEffectiveCapabilityRequest) { r.AssetType = "cheque" },
			servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_CAPABILITY_NOT_FOUND},
		{"suspended partner", domain.PartnerSuspended, nil, servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PARTNER_NOT_ACTIVE},
		{"excluded tenant", domain.PartnerActive, func(r *servicev1.FindEffectiveCapabilityRequest) { r.TenantId = "tenant-x" },
			servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_NOT_IN_ROLLOUT},
		{"product past its validity", domain.PartnerActive, func(r *servicev1.FindEffectiveCapabilityRequest) {
			r.ProductId, r.AsOf = "fpr-1", int64(occurred)+86_400_000_000_000
		}, servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PRODUCT_NOT_EFFECTIVE},
		{"amount above the product", domain.PartnerActive, func(r *servicev1.FindEffectiveCapabilityRequest) {
			r.ProductId, r.AmountMinor = "fpr-1", proto.Int64(3)
		}, servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PARAMETERS_INCOMPATIBLE},
		{"term below the product", domain.PartnerActive, func(r *servicev1.FindEffectiveCapabilityRequest) {
			r.ProductId, r.TermDays = "fpr-1", proto.Int32(6)
		}, servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PARAMETERS_INCOMPATIBLE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, effectivePlanted())
			seedIn(t, h, partnersTable, "prt-1", partnerAt("prt-1", tc.status))
			req := effectiveRequest()
			if tc.edit != nil {
				tc.edit(req)
			}

			var resp servicev1.FindEffectiveCapabilityResponse
			if err := h.invoke(withoutKey(t), "FindEffectiveCapability", req, &resp); err != nil {
				t.Fatalf("FindEffectiveCapability() = %v", err)
			}

			if resp.GetAllowed() || resp.GetFailedCondition() != tc.want {
				t.Fatalf("FindEffectiveCapability() = %v, want denied with %s", &resp, tc.want)
			}
		})
	}
}

func TestEveryQueryRefusesAMalformedFilterAsInvalidArgument(t *testing.T) {
	h := newHarness(t, relations{})
	effective := func(edit func(*servicev1.FindEffectiveCapabilityRequest)) proto.Message {
		r := effectiveRequest()
		edit(r)
		return r
	}
	eligible := func(edit func(*servicev1.FindEligiblePartnersRequest)) proto.Message {
		r := eligibleRequest()
		edit(r)
		return r
	}
	for _, c := range []struct {
		name   string
		method string
		req    proto.Message
	}{
		{"partner id off the format", "FindPartner", &servicev1.FindPartnerRequest{PartnerId: "prt 1"}},
		{"organization id empty", "FindPartnersByOrganization", &servicev1.FindPartnersByOrganizationRequest{}},
		{"agreements of no partner", "FindAgreementsByPartner", &servicev1.FindAgreementsByPartnerRequest{}},
		{"capabilities of an unknown environment", "FindCapabilitiesByPartner",
			&servicev1.FindCapabilitiesByPartnerRequest{PartnerId: "prt-1", Environment: servicev1.Environment(99)}},
		{"capabilities of an unknown status", "FindCapabilitiesByPartner",
			&servicev1.FindCapabilitiesByPartnerRequest{PartnerId: "prt-1", Status: servicev1.CapabilityStatus(99)}},
		{"effective without an environment", "FindEffectiveCapability", effective(func(r *servicev1.FindEffectiveCapabilityRequest) {
			r.Environment = servicev1.Environment_ENVIRONMENT_UNSPECIFIED
		})},
		{"effective without a tenant", "FindEffectiveCapability", effective(func(r *servicev1.FindEffectiveCapabilityRequest) { r.TenantId = "" })},
		{"effective without an instant", "FindEffectiveCapability", effective(func(r *servicev1.FindEffectiveCapabilityRequest) { r.AsOf = 0 })},
		{"effective with a negative amount", "FindEffectiveCapability", effective(func(r *servicev1.FindEffectiveCapabilityRequest) {
			r.AmountMinor = proto.Int64(-1)
		})},
		{"effective with an operation off ASCII", "FindEffectiveCapability", effective(func(r *servicev1.FindEffectiveCapabilityRequest) {
			r.Operation = "registroç"
		})},
		{"eligible with a negative term", "FindEligiblePartners", eligible(func(r *servicev1.FindEligiblePartnersRequest) { r.TermDays = -1 })},
		{"eligible without a region", "FindEligiblePartners", eligible(func(r *servicev1.FindEligiblePartnersRequest) { r.Region = "" })},
		{"products without an instant", "FindProductsByPartner", &servicev1.FindProductsByPartnerRequest{PartnerId: "prt-1"}},
		{"profile without an environment", "FindIntegrationProfile", &servicev1.FindIntegrationProfileRequest{PartnerId: "prt-1", AsOf: int64(occurred)}},
		{"policy without an instant", "FindOperationalPolicy", &servicev1.FindOperationalPolicyRequest{PartnerId: "prt-1", AsOf: -1}},
		{"certificates without a bound", "FindCertificatesExpiring",
			&servicev1.FindCertificatesExpiringRequest{Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := h.invoke(withoutKey(t), c.method, c.req, responseOf(c.method)); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("%s() = %v, want InvalidArgument", c.method, err)
			}
		})
	}
}

func responseOf(method string) proto.Message {
	return map[string]proto.Message{
		"FindPartner":                &servicev1.FindPartnerResponse{},
		"FindPartnersByOrganization": &servicev1.FindPartnersByOrganizationResponse{},
		"FindAgreementsByPartner":    &servicev1.FindAgreementsByPartnerResponse{},
		"FindCapabilitiesByPartner":  &servicev1.FindCapabilitiesByPartnerResponse{},
		"FindEffectiveCapability":    &servicev1.FindEffectiveCapabilityResponse{},
		"FindEligiblePartners":       &servicev1.FindEligiblePartnersResponse{},
		"FindProductsByPartner":      &servicev1.FindProductsByPartnerResponse{},
		"FindIntegrationProfile":     &servicev1.FindIntegrationProfileResponse{},
		"FindOperationalPolicy":      &servicev1.FindOperationalPolicyResponse{},
		"FindCertificatesExpiring":   &servicev1.FindCertificatesExpiringResponse{},
	}[method]
}

func TestFindPartnerAnswersThePartnerOnTheWire(t *testing.T) {
	h := newHarness(t, relations{})
	offboarding := partnerAt("prt-1", domain.PartnerOffboarding)
	offboarding.StageReason, offboarding.OffboardingStartedAt = "contract terminated", occurred+1
	seedIn(t, h, partnersTable, "prt-1", offboarding)

	var resp servicev1.FindPartnerResponse
	if err := h.invoke(withoutKey(t), "FindPartner", &servicev1.FindPartnerRequest{PartnerId: "prt-1"}, &resp); err != nil {
		t.Fatalf("FindPartner() = %v", err)
	}

	want := &servicev1.Partner{
		PartnerId: "prt-1", OrganizationId: "org-1", Role: servicev1.PartnerRole_PARTNER_ROLE_FUNDER,
		Status: servicev1.PartnerStatus_PARTNER_STATUS_OFFBOARDING, RegisteredAt: int64(occurred), StageChangedAt: int64(occurred),
		StageReason: "contract terminated", OffboardingStartedAt: int64(occurred) + 1,
	}
	if !proto.Equal(resp.GetPartner(), want) {
		t.Fatalf("FindPartner() = %v, want %v", resp.GetPartner(), want)
	}
	err := h.invoke(withoutKey(t), "FindPartner", &servicev1.FindPartnerRequest{PartnerId: "prt-none"}, &servicev1.FindPartnerResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindPartner(prt-none) = %v, want NotFound", err)
	}
}

func TestTheRelationsOfAPartnerAnswerOnTheWire(t *testing.T) {
	registry := partnerAt("prt-2", domain.PartnerProspect)
	registry.Role = domain.RoleRegistry
	suspended := capabilityAt("cap-2", domain.CapabilityStatusSuspended)
	suspended.Version, suspended.SupersedesID, suspended.Suspension = 2, "cap-1", domain.Suspension{Reason: "incident", Trigger: domain.TriggerManual, SuspendedAt: occurred}
	suspended.Homologation = []domain.CriterionResult{{CriterionID: "H1", Passed: true, EvidenceID: "evd-H1", RecordedAt: occurred}}
	h := newHarness(t, relations{
		partners:     partnersByOrganization{"org-1": {partnerAt("prt-1", domain.PartnerActive), registry}},
		agreements:   agreementsByPartner{"prt-1": {activeAgreement()}},
		capabilities: capabilitiesByPartner{"prt-1": {enabledEverywhere("cap-1"), suspended}},
	})

	var partners servicev1.FindPartnersByOrganizationResponse
	var agreements servicev1.FindAgreementsByPartnerResponse
	var capabilities servicev1.FindCapabilitiesByPartnerResponse
	for _, call := range []struct {
		method    string
		req, resp proto.Message
	}{
		{"FindPartnersByOrganization", &servicev1.FindPartnersByOrganizationRequest{OrganizationId: "org-1"}, &partners},
		{"FindAgreementsByPartner", &servicev1.FindAgreementsByPartnerRequest{PartnerId: "prt-1"}, &agreements},
		{"FindCapabilitiesByPartner", &servicev1.FindCapabilitiesByPartnerRequest{
			PartnerId: "prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, Status: servicev1.CapabilityStatus_CAPABILITY_STATUS_SUSPENDED,
		}, &capabilities},
	} {
		if err := h.invoke(withoutKey(t), call.method, call.req, call.resp); err != nil {
			t.Fatalf("%s() = %v", call.method, err)
		}
	}

	if len(partners.GetPartners()) != 2 || partners.GetPartners()[1].GetRole() != servicev1.PartnerRole_PARTNER_ROLE_REGISTRY {
		t.Fatalf("FindPartnersByOrganization() = %v, want both roles of org-1", &partners)
	}
	wantAgreement := &servicev1.PartnerAgreement{
		AgreementId: "agr-1", PartnerId: "prt-1", Version: 1, Status: servicev1.AgreementStatus_AGREEMENT_STATUS_ACTIVE,
		ValidFrom: validFrom, ValidUntil: validUntil, ContractEvidenceId: "evd-1", ActivatedAt: int64(occurred),
	}
	if len(agreements.GetAgreements()) != 1 || !proto.Equal(agreements.GetAgreements()[0], wantAgreement) {
		t.Fatalf("FindAgreementsByPartner() = %v, want %v", &agreements, wantAgreement)
	}
	wantCapability := &servicev1.Capability{
		CapabilityId: "cap-2", PartnerId: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
		Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, Version: 2, SupersedesId: "cap-1", IsWrite: true, StateQuerySupported: true,
		Status:       servicev1.CapabilityStatus_CAPABILITY_STATUS_SUSPENDED,
		Homologation: []*servicev1.CriterionResult{{CriterionId: "H1", Passed: true, EvidenceId: "evd-H1", RecordedAt: int64(occurred)}},
		Rollout:      &servicev1.Rollout{Mode: servicev1.RolloutMode_ROLLOUT_MODE_DISABLED},
		Suspension:   &servicev1.Suspension{Reason: "incident", Trigger: servicev1.SuspensionTrigger_SUSPENSION_TRIGGER_MANUAL, SuspendedAt: int64(occurred)},
	}
	if len(capabilities.GetCapabilities()) != 1 || !proto.Equal(capabilities.GetCapabilities()[0], wantCapability) {
		t.Fatalf("FindCapabilitiesByPartner() = %v, want only %v", &capabilities, wantCapability)
	}
}

func eligibleRequest() *servicev1.FindEligiblePartnersRequest {
	return &servicev1.FindEligiblePartnersRequest{
		AssetType: "duplicata", Operation: "register-receivable", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION,
		TenantId: "tenant-a", AmountMinor: 1, TermDays: 30, Region: "SP", AsOf: int64(occurred),
	}
}

func TestFindEligiblePartnersAnswersTheEligibleFunders(t *testing.T) {
	planted := effectivePlanted()
	product := productAt("fpr-1", domain.ProductActive, occurred+86_400_000_000_000)
	product.Regions = []string{"SP"}
	planted.products = productsByPartner{"prt-1": {product}}
	planted.byRole = partnersByRole{domain.RoleFunder: {partnerAt("prt-1", domain.PartnerActive), partnerAt("prt-2", domain.PartnerActive)}}
	h := newHarness(t, planted)

	var resp servicev1.FindEligiblePartnersResponse
	if err := h.invoke(withoutKey(t), "FindEligiblePartners", eligibleRequest(), &resp); err != nil {
		t.Fatalf("FindEligiblePartners() = %v", err)
	}

	if len(resp.GetPartners()) != 1 || resp.GetPartners()[0].GetPartnerId() != "prt-1" {
		t.Fatalf("FindEligiblePartners() = %v, want prt-1 alone: prt-2 has no capability", &resp)
	}
}

func TestTheVersionedQueriesAnswerTheCurrentVersionOnTheWire(t *testing.T) {
	product := productAt("fpr-1", domain.ProductActive, occurred+86_400_000_000_000)
	product.Regions, product.RequiredCheckIDs = []string{"SP"}, []string{"CHK-01"}
	profile := expiringIn("ipf-1", 25*86_400_000_000_000, 30)
	profile.Protocols, profile.SchemaVersion, profile.CertificateRef = []string{"https"}, "v3", "cert-1"
	policy := domain.OperationalPolicySnapshot{
		ID: "pol-1", PartnerID: "prt-1", Version: 1, EffectiveFrom: occurred, RateLimitPerMinute: 100, BusinessDays: []string{"mon"},
		CutoffTime: "17:00", ProcessingWindow: "08:00-18:00", SlaRef: "sla-gold", Fallback: "manual queue",
		SupportContact: "support@partner.example", IncidentContact: "+55 11 99999-0000",
	}
	h := newHarness(t, relations{
		products:      productsByPartner{"prt-1": {product}},
		profiles:      profilesByPartner{"prt-1": {profile}},
		policies:      policiesByPartner{"prt-1": {policy}},
		byEnvironment: profilesByEnvironment{domain.EnvironmentProduction: {profile, expiringIn("ipf-2", 90*86_400_000_000_000, 0)}},
	})

	var products servicev1.FindProductsByPartnerResponse
	var found servicev1.FindIntegrationProfileResponse
	var policies servicev1.FindOperationalPolicyResponse
	var expiring servicev1.FindCertificatesExpiringResponse
	for _, call := range []struct {
		method    string
		req, resp proto.Message
	}{
		{"FindProductsByPartner", &servicev1.FindProductsByPartnerRequest{PartnerId: "prt-1", AsOf: int64(occurred)}, &products},
		{"FindIntegrationProfile", &servicev1.FindIntegrationProfileRequest{
			PartnerId: "prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, AsOf: int64(occurred),
		}, &found},
		{"FindOperationalPolicy", &servicev1.FindOperationalPolicyRequest{PartnerId: "prt-1", AsOf: int64(occurred)}, &policies},
		{"FindCertificatesExpiring", &servicev1.FindCertificatesExpiringRequest{
			Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, ExpiresBefore: int64(occurred) + 30*86_400_000_000_000,
		}, &expiring},
	} {
		if err := h.invoke(withoutKey(t), call.method, call.req, call.resp); err != nil {
			t.Fatalf("%s() = %v", call.method, err)
		}
	}

	wantProduct := &servicev1.FinancialProduct{
		ProductId: "fpr-1", PartnerId: "prt-1", AgreementId: "agr-1", Code: "fpr-1", Name: "Antecipação de duplicatas", Version: 1,
		Status: servicev1.ProductStatus_PRODUCT_STATUS_ACTIVE, ValidFrom: int64(occurred) - 30*86_400_000_000_000, ValidUntil: int64(occurred) + 86_400_000_000_000,
		AssetTypes: []string{"duplicata"}, Regions: []string{"SP"}, Currency: "BRL", MinAmountMinor: 1, MaxAmountMinor: 2, MinTermDays: 7, MaxTermDays: 180,
		RequiredCheckIds: []string{"CHK-01"},
	}
	if len(products.GetProducts()) != 1 || !proto.Equal(products.GetProducts()[0], wantProduct) {
		t.Fatalf("FindProductsByPartner() = %v, want %v", &products, wantProduct)
	}
	wantProfile := &servicev1.IntegrationProfile{
		ProfileId: "ipf-1", PartnerId: "prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, Version: 1,
		ChangeKind: servicev1.IntegrationChangeKind_INTEGRATION_CHANGE_KIND_INITIAL, Protocols: []string{"https"},
		Endpoint: "https://partner.example/api", SchemaVersion: "v3", SecretRef: "vault://partners/prt-1/api-key", CertificateRef: "cert-1",
		CertificateExpiresAt: int64(occurred) + 25*86_400_000_000_000, EffectiveFrom: int64(occurred), LastAlertedThresholdDays: 30,
	}
	if !proto.Equal(found.GetProfile(), wantProfile) {
		t.Fatalf("FindIntegrationProfile() = %v, want %v", found.GetProfile(), wantProfile)
	}
	wantPolicy := &servicev1.OperationalPolicy{
		PolicyId: "pol-1", PartnerId: "prt-1", Version: 1, EffectiveFrom: int64(occurred), RateLimitPerMinute: 100, BusinessDays: []string{"mon"},
		CutoffTime: "17:00", ProcessingWindow: "08:00-18:00", SlaRef: "sla-gold", Fallback: "manual queue",
		SupportContact: "support@partner.example", IncidentContact: "+55 11 99999-0000",
	}
	if !proto.Equal(policies.GetPolicy(), wantPolicy) {
		t.Fatalf("FindOperationalPolicy() = %v, want %v", policies.GetPolicy(), wantPolicy)
	}
	ids := make([]string, 0, len(expiring.GetProfiles()))
	for _, p := range expiring.GetProfiles() {
		ids = append(ids, p.GetProfileId())
	}
	if !slices.Equal(ids, []string{"ipf-1"}) {
		t.Fatalf("FindCertificatesExpiring() = %v, want ipf-1 alone", ids)
	}
	err := h.invoke(withoutKey(t), "FindIntegrationProfile", &servicev1.FindIntegrationProfileRequest{
		PartnerId: "prt-1", Environment: servicev1.Environment_ENVIRONMENT_HOMOLOGATION, AsOf: int64(occurred),
	}, &servicev1.FindIntegrationProfileResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindIntegrationProfile(homologation) = %v, want NotFound", err)
	}
}
