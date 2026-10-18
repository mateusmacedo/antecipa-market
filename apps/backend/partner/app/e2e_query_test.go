//go:build integration

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
)

const (
	tenantB         = "globex"
	evaluatedTenant = "tenant-a"
)

func queryAs(t *testing.T, tenant string) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, tenant)
}

type onboarding struct {
	partnerID, organizationID string
	role                      servicev1.PartnerRole
}

func onboard(t *testing.T, conn *grpc.ClientConn, o onboarding) {
	t.Helper()
	invoke(t, conn, call(t), "RegisterPartner",
		&servicev1.RegisterPartnerRequest{PartnerId: o.partnerID, OrganizationId: o.organizationID, Role: o.role}, &servicev1.RegisterPartnerResponse{})
	for _, target := range []servicev1.PartnerStatus{
		servicev1.PartnerStatus_PARTNER_STATUS_CONTRACTING,
		servicev1.PartnerStatus_PARTNER_STATUS_INTEGRATING,
		servicev1.PartnerStatus_PARTNER_STATUS_HOMOLOGATING,
	} {
		invoke(t, conn, call(t), "ChangePartnerStage", &servicev1.ChangePartnerStageRequest{PartnerId: o.partnerID, Target: target}, &servicev1.ChangePartnerStageResponse{})
	}
	agreement := agreementOf("agr-"+o.partnerID, "")
	agreement.PartnerId = o.partnerID
	invoke(t, conn, call(t), "RegisterAgreement", agreement, &servicev1.RegisterAgreementResponse{})
	invoke(t, conn, call(t), "ActivateAgreement", activation(agreement.AgreementId), &servicev1.ActivateAgreementResponse{})
	profile := profileOf("ipf-"+o.partnerID, 90*day, "")
	profile.PartnerId = o.partnerID
	invoke(t, conn, call(t), "RegisterIntegrationProfile", profile, &servicev1.RegisterIntegrationProfileResponse{})
	capability := declarationOf("cap-"+o.partnerID, "")
	capability.PartnerId = o.partnerID
	invoke(t, conn, call(t), "DeclareCapability", capability, &servicev1.DeclareCapabilityResponse{})
	invoke(t, conn, call(t), "RecordHomologation",
		&servicev1.RecordHomologationRequest{CapabilityId: capability.CapabilityId, Results: passed(applicableCriteria...)}, &servicev1.RecordHomologationResponse{})
	invoke(t, conn, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: capability.CapabilityId}, &servicev1.EnableCapabilityResponse{})
	invoke(t, conn, call(t), "ChangeRollout",
		&servicev1.ChangeRolloutRequest{CapabilityId: capability.CapabilityId, Mode: servicev1.RolloutMode_ROLLOUT_MODE_ENABLED}, &servicev1.ChangeRolloutResponse{})
	changed := invoke(t, conn, call(t), "ChangePartnerStage",
		&servicev1.ChangePartnerStageRequest{PartnerId: o.partnerID, Target: servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE}, &servicev1.ChangePartnerStageResponse{})
	if changed.GetChanged().GetStatus() != servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE {
		t.Fatalf("setup: %s did not reach active: %v", o.partnerID, changed)
	}
}

func offer(t *testing.T, conn *grpc.ClientConn, partnerID, region string) string {
	t.Helper()
	product := productOf("fpr-"+partnerID, "")
	product.PartnerId, product.AgreementId, product.Regions, product.ValidUntil = partnerID, "agr-"+partnerID, []string{region}, validFrom+30*day
	resp := invoke(t, conn, call(t), "DefineProduct", product, &servicev1.DefineProductResponse{})
	if resp.GetDefined() == nil {
		t.Fatalf("setup: DefineProduct(%s) = %v", product.ProductId, resp)
	}
	return product.ProductId
}

func effectiveOf(t *testing.T, conn *grpc.ClientConn, ctx context.Context, req *servicev1.FindEffectiveCapabilityRequest) *servicev1.FindEffectiveCapabilityResponse {
	t.Helper()
	return invoke(t, conn, ctx, "FindEffectiveCapability", req, &servicev1.FindEffectiveCapabilityResponse{})
}

func effectiveRequest(partnerID string) *servicev1.FindEffectiveCapabilityRequest {
	return &servicev1.FindEffectiveCapabilityRequest{
		PartnerId: partnerID, Operation: "register-receivable", AssetType: "duplicata",
		Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, TenantId: evaluatedTenant, AsOf: validFrom,
	}
}

func eligibleRequest() *servicev1.FindEligiblePartnersRequest {
	return &servicev1.FindEligiblePartnersRequest{
		AssetType: "duplicata", Operation: "register-receivable", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION,
		TenantId: evaluatedTenant, AmountMinor: 1_000_000, TermDays: 30, Region: "SP", AsOf: validFrom,
	}
}

func TestTheEffectiveCapabilityResolvesOverPostgresOutsideAnyUnitOfWork(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	onboard(t, conn, onboarding{"e2e-prt-reg", "org-partner", servicev1.PartnerRole_PARTNER_ROLE_REGISTRY})
	onboard(t, conn, onboarding{"e2e-prt-fund", "org-funder", servicev1.PartnerRole_PARTNER_ROLE_FUNDER})
	product := offer(t, conn, "e2e-prt-fund", "SP")
	events := count(t, pool, "SELECT count(*) FROM outbox")

	allowed := effectiveOf(t, conn, queryAs(t, tenantA), effectiveRequest("e2e-prt-reg"))
	if !allowed.GetAllowed() {
		t.Fatalf("FindEffectiveCapability(registry, no product) = %v, want allowed", allowed)
	}
	withProduct := effectiveRequest("e2e-prt-fund")
	withProduct.ProductId, withProduct.AmountMinor, withProduct.TermDays = product, proto.Int64(1_000_000), proto.Int32(30)
	if resp := effectiveOf(t, conn, queryAs(t, tenantA), withProduct); !resp.GetAllowed() {
		t.Fatalf("FindEffectiveCapability(funder, compatible product) = %v, want allowed", resp)
	}
	withProduct.AmountMinor = proto.Int64(50_000_001)
	if resp := effectiveOf(t, conn, queryAs(t, tenantA), withProduct); resp.GetAllowed() ||
		resp.GetFailedCondition() != servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PARAMETERS_INCOMPATIBLE {
		t.Fatalf("FindEffectiveCapability(amount above the product) = %v, want parameters_incompatible", resp)
	}

	invoke(t, conn, call(t), "ChangeRollout", &servicev1.ChangeRolloutRequest{
		CapabilityId: "cap-e2e-prt-reg", Mode: servicev1.RolloutMode_ROLLOUT_MODE_ENABLED, ExcludedTenantIds: []string{evaluatedTenant},
	}, &servicev1.ChangeRolloutResponse{})
	if resp := effectiveOf(t, conn, queryAs(t, tenantA), effectiveRequest("e2e-prt-reg")); resp.GetFailedCondition() !=
		servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_NOT_IN_ROLLOUT {
		t.Fatalf("FindEffectiveCapability(excluded tenant) = %v, want not_in_rollout", resp)
	}
	invoke(t, conn, call(t), "ChangePartnerStage", &servicev1.ChangePartnerStageRequest{
		PartnerId: "e2e-prt-fund", Target: servicev1.PartnerStatus_PARTNER_STATUS_SUSPENDED, Reason: "contract under review",
	}, &servicev1.ChangePartnerStageResponse{})
	if resp := effectiveOf(t, conn, queryAs(t, tenantA), effectiveRequest("e2e-prt-fund")); resp.GetFailedCondition() !=
		servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_PARTNER_NOT_ACTIVE {
		t.Fatalf("FindEffectiveCapability(suspended partner) = %v, want partner_not_active", resp)
	}
	if got := count(t, pool, "SELECT count(*) FROM outbox"); got != events+1 {
		t.Fatalf("outbox = %d, want only the rollout change written since the queries began", got-events)
	}
}

func TestASuspendedSeriesIsNotEnabledBesideAnEnabledOneOverPostgres(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	onboard(t, conn, onboarding{"e2e-prt-1", "org-funder", servicev1.PartnerRole_PARTNER_ROLE_FUNDER})
	cheque := declarationOf("e2e-cap-cheque", "")
	cheque.AssetType = "cheque"
	invoke(t, conn, call(t), "DeclareCapability", cheque, &servicev1.DeclareCapabilityResponse{})
	invoke(t, conn, call(t), "RecordHomologation",
		&servicev1.RecordHomologationRequest{CapabilityId: "e2e-cap-cheque", Results: passed(applicableCriteria...)}, &servicev1.RecordHomologationResponse{})
	invoke(t, conn, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-cheque"}, &servicev1.EnableCapabilityResponse{})
	invoke(t, conn, call(t), "SuspendCapability",
		&servicev1.SuspendCapabilityRequest{CapabilityId: "cap-e2e-prt-1", Reason: "partner incident"}, &servicev1.SuspendCapabilityResponse{})

	resp := effectiveOf(t, conn, queryAs(t, tenantA), effectiveRequest("e2e-prt-1"))

	if resp.GetAllowed() || resp.GetFailedCondition() != servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_CAPABILITY_NOT_ENABLED {
		t.Fatalf("FindEffectiveCapability(suspended duplicata) = %v, want capability_not_enabled", resp)
	}
	cheques := effectiveRequest("e2e-prt-1")
	cheques.AssetType = "cheque"
	if resp := effectiveOf(t, conn, queryAs(t, tenantA), cheques); resp.GetAllowed() ||
		resp.GetFailedCondition() != servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_NOT_IN_ROLLOUT {
		t.Fatalf("FindEffectiveCapability(cheque, rollout disabled) = %v, want not_in_rollout", resp)
	}
}

func TestFindEligiblePartnersAnswersOnlyTheEligibleFundersOverPostgres(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	onboard(t, conn, onboarding{"e2e-prt-sp", "org-funder", servicev1.PartnerRole_PARTNER_ROLE_FUNDER})
	onboard(t, conn, onboarding{"e2e-prt-rj", "org-funder-2", servicev1.PartnerRole_PARTNER_ROLE_FUNDER})
	onboard(t, conn, onboarding{"e2e-prt-reg", "org-partner", servicev1.PartnerRole_PARTNER_ROLE_REGISTRY})
	offer(t, conn, "e2e-prt-sp", "SP")
	offer(t, conn, "e2e-prt-rj", "RJ")
	offer(t, conn, "e2e-prt-reg", "SP")

	resp := invoke(t, conn, queryAs(t, tenantA), "FindEligiblePartners", eligibleRequest(), &servicev1.FindEligiblePartnersResponse{})

	if len(resp.GetPartners()) != 1 || resp.GetPartners()[0].GetPartnerId() != "e2e-prt-sp" ||
		resp.GetPartners()[0].GetRole() != servicev1.PartnerRole_PARTNER_ROLE_FUNDER {
		t.Fatalf("FindEligiblePartners() = %v, want e2e-prt-sp alone", resp)
	}
}

func TestTheQueriesOfAnotherTenantFindNothingOfThisOne(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	onboard(t, conn, onboarding{"e2e-prt-1", "org-funder", servicev1.PartnerRole_PARTNER_ROLE_FUNDER})
	offer(t, conn, "e2e-prt-1", "SP")
	elsewhere := queryAs(t, tenantB)

	err := conn.Invoke(elsewhere, rpc.FullMethod("FindPartner"), &servicev1.FindPartnerRequest{PartnerId: "e2e-prt-1"}, &servicev1.FindPartnerResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindPartner() from %s = %v, want NotFound", tenantB, err)
	}
	err = conn.Invoke(elsewhere, rpc.FullMethod("FindIntegrationProfile"), &servicev1.FindIntegrationProfileRequest{
		PartnerId: "e2e-prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, AsOf: validFrom,
	}, &servicev1.FindIntegrationProfileResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindIntegrationProfile() from %s = %v, want NotFound", tenantB, err)
	}
	partners := invoke(t, conn, elsewhere, "FindPartnersByOrganization", &servicev1.FindPartnersByOrganizationRequest{OrganizationId: "org-funder"},
		&servicev1.FindPartnersByOrganizationResponse{})
	agreements := invoke(t, conn, elsewhere, "FindAgreementsByPartner", &servicev1.FindAgreementsByPartnerRequest{PartnerId: "e2e-prt-1"},
		&servicev1.FindAgreementsByPartnerResponse{})
	products := invoke(t, conn, elsewhere, "FindProductsByPartner", &servicev1.FindProductsByPartnerRequest{PartnerId: "e2e-prt-1", AsOf: validFrom},
		&servicev1.FindProductsByPartnerResponse{})
	eligible := invoke(t, conn, elsewhere, "FindEligiblePartners", eligibleRequest(), &servicev1.FindEligiblePartnersResponse{})
	expiring := invoke(t, conn, elsewhere, "FindCertificatesExpiring", &servicev1.FindCertificatesExpiringRequest{
		Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, ExpiresBefore: validFrom + 365*day,
	}, &servicev1.FindCertificatesExpiringResponse{})
	if n := len(partners.GetPartners()) + len(agreements.GetAgreements()) + len(products.GetProducts()) + len(eligible.GetPartners()) +
		len(expiring.GetProfiles()); n != 0 {
		t.Fatalf("queries from %s answered %d items, want nothing of %s", tenantB, n, tenantA)
	}
	if resp := effectiveOf(t, conn, elsewhere, effectiveRequest("e2e-prt-1")); resp.GetAllowed() ||
		resp.GetFailedCondition() != servicev1.EffectiveCapabilityCondition_EFFECTIVE_CAPABILITY_CONDITION_CAPABILITY_NOT_FOUND {
		t.Fatalf("FindEffectiveCapability() from %s = %v, want capability_not_found", tenantB, resp)
	}
	if got := invoke(t, conn, queryAs(t, tenantA), "FindProductsByPartner", &servicev1.FindProductsByPartnerRequest{PartnerId: "e2e-prt-1", AsOf: validFrom},
		&servicev1.FindProductsByPartnerResponse{}); len(got.GetProducts()) != 1 {
		t.Fatalf("FindProductsByPartner() from %s = %v, want the product of the tenant", tenantA, got)
	}
}

func TestTheVersionedQueriesAnswerTheCurrentVersionOverPostgres(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-1", 25*day, ""), &servicev1.RegisterIntegrationProfileResponse{})
	invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-2", 90*day, "e2e-ipf-1"), &servicev1.RegisterIntegrationProfileResponse{})
	ofAnotherPartner := profileOf("e2e-ipf-3", 25*day, "")
	ofAnotherPartner.PartnerId = "e2e-prt-2"
	invoke(t, conn, call(t), "RegisterIntegrationProfile", ofAnotherPartner, &servicev1.RegisterIntegrationProfileResponse{})
	invoke(t, conn, call(t), "DefineOperationalPolicy", policyOf("e2e-pol-1", ""), &servicev1.DefineOperationalPolicyResponse{})
	invoke(t, conn, call(t), "DefineOperationalPolicy", policyOf("e2e-pol-2", "e2e-pol-1"), &servicev1.DefineOperationalPolicyResponse{})

	profile := invoke(t, conn, queryAs(t, tenantA), "FindIntegrationProfile", &servicev1.FindIntegrationProfileRequest{
		PartnerId: "e2e-prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, AsOf: validFrom,
	}, &servicev1.FindIntegrationProfileResponse{})
	policy := invoke(t, conn, queryAs(t, tenantA), "FindOperationalPolicy", &servicev1.FindOperationalPolicyRequest{PartnerId: "e2e-prt-1", AsOf: validFrom},
		&servicev1.FindOperationalPolicyResponse{})
	expiring := invoke(t, conn, queryAs(t, tenantA), "FindCertificatesExpiring", &servicev1.FindCertificatesExpiringRequest{
		Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, ExpiresBefore: validFrom + 30*day,
	}, &servicev1.FindCertificatesExpiringResponse{})

	if profile.GetProfile().GetProfileId() != "e2e-ipf-2" || profile.GetProfile().GetVersion() != 2 ||
		profile.GetProfile().GetSecretRef() != "vault://partners/e2e-prt-1/api-key" {
		t.Fatalf("FindIntegrationProfile() = %v, want version 2 with the secret reference", profile)
	}
	if policy.GetPolicy().GetPolicyId() != "e2e-pol-2" || policy.GetPolicy().GetRateLimitPerMinute() != 100 {
		t.Fatalf("FindOperationalPolicy() = %v, want version 2", policy)
	}
	ids := make([]string, 0, len(expiring.GetProfiles()))
	for _, p := range expiring.GetProfiles() {
		ids = append(ids, p.GetProfileId())
	}
	if !slices.Equal(ids, []string{"e2e-ipf-3"}) {
		t.Fatalf("FindCertificatesExpiring() = %v, want e2e-ipf-3, the current version that expires before the bound, without the superseded e2e-ipf-1", ids)
	}
}

func TestEveryPublishedTypeIsKeyedByTheIDOfItsAggregate(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	alerted := profileOf("e2e-ipf-alert", 25*day, "")
	alerted.Environment = servicev1.Environment_ENVIRONMENT_HOMOLOGATION
	invoke(t, conn, call(t), "RegisterIntegrationProfile", alerted, &servicev1.RegisterIntegrationProfileResponse{})
	onboard(t, conn, onboarding{"e2e-prt-1", "org-funder", servicev1.PartnerRole_PARTNER_ROLE_FUNDER})
	invoke(t, conn, call(t), "RaiseCertificateExpiryAlert", &servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "e2e-ipf-alert", ThresholdDays: 30},
		&servicev1.RaiseCertificateExpiryAlertResponse{})
	invoke(t, conn, call(t), "SuspendCapability", &servicev1.SuspendCapabilityRequest{CapabilityId: "cap-e2e-prt-1", Reason: "incident"},
		&servicev1.SuspendCapabilityResponse{})
	first := productOf("e2e-fpr-1", "")
	first.AgreementId = "agr-e2e-prt-1"
	second := productOf("e2e-fpr-2", "e2e-fpr-1")
	second.AgreementId = "agr-e2e-prt-1"
	withdrawn := productOf("e2e-fpr-w", "")
	withdrawn.AgreementId, withdrawn.Code = "agr-e2e-prt-1", "invoice-advance"
	for _, product := range []*servicev1.DefineProductRequest{first, second, withdrawn} {
		invoke(t, conn, call(t), "DefineProduct", product, &servicev1.DefineProductResponse{})
	}
	invoke(t, conn, call(t), "RetireProduct", retirement("e2e-fpr-2", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_EXPIRED), &servicev1.RetireProductResponse{})
	invoke(t, conn, call(t), "RetireProduct", retirement("e2e-fpr-w", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN), &servicev1.RetireProductResponse{})
	invoke(t, conn, call(t), "ChangePartnerStage", &servicev1.ChangePartnerStageRequest{
		PartnerId: "e2e-prt-1", Target: servicev1.PartnerStatus_PARTNER_STATUS_OFFBOARDING, Reason: "contract terminated",
	}, &servicev1.ChangePartnerStageResponse{})

	aggregateOf := map[string]func([]byte) (string, error){
		"com.company.partner.partner-registered.v1":           idOf(&eventv1.PartnerRegistered{}, (*eventv1.PartnerRegistered).GetPartnerId),
		"com.company.partner.partner-agreement-activated.v1":  idOf(&eventv1.PartnerAgreementActivated{}, (*eventv1.PartnerAgreementActivated).GetAgreementId),
		"com.company.partner.capability-enabled.v1":           idOf(&eventv1.CapabilityEnabled{}, (*eventv1.CapabilityEnabled).GetCapabilityId),
		"com.company.partner.capability-suspended.v1":         idOf(&eventv1.CapabilitySuspended{}, (*eventv1.CapabilitySuspended).GetCapabilityId),
		"com.company.partner.capability-rollout-changed.v1":   idOf(&eventv1.CapabilityRolloutChanged{}, (*eventv1.CapabilityRolloutChanged).GetCapabilityId),
		"com.company.partner.partner-product-defined.v1":      idOf(&eventv1.PartnerProductDefined{}, (*eventv1.PartnerProductDefined).GetProductId),
		"com.company.partner.partner-product-changed.v1":      idOf(&eventv1.PartnerProductChanged{}, (*eventv1.PartnerProductChanged).GetProductId),
		"com.company.partner.partner-product-expired.v1":      idOf(&eventv1.PartnerProductExpired{}, (*eventv1.PartnerProductExpired).GetProductId),
		"com.company.partner.partner-product-retired.v1":      idOf(&eventv1.PartnerProductRetired{}, (*eventv1.PartnerProductRetired).GetProductId),
		"com.company.partner.partner-certificate-expiring.v1": idOf(&eventv1.PartnerCertificateExpiring{}, (*eventv1.PartnerCertificateExpiring).GetProfileId),
		"com.company.partner.partner-offboarding-started.v1":  idOf(&eventv1.PartnerOffboardingStarted{}, (*eventv1.PartnerOffboardingStarted).GetPartnerId),
	}
	seen := map[string]bool{}
	for _, record := range outbox(t, pool) {
		decode, known := aggregateOf[record.messageType]
		if !known {
			t.Fatalf("outbox carries %s, a type outside the eleven of the contract", record.messageType)
		}
		id, err := decode(record.payload)
		if err != nil || id == "" || record.partitionKey != id {
			t.Fatalf("%s keyed %q, aggregate %q (%v); want the partition key equal to the id of the aggregate", record.messageType, record.partitionKey, id, err)
		}
		seen[record.messageType] = true
	}
	if len(seen) != len(aggregateOf) {
		t.Fatalf("outbox carries %d of the %d published types: %v", len(seen), len(aggregateOf), seen)
	}
}

func idOf[E proto.Message](event E, id func(E) string) func([]byte) (string, error) {
	return func(payload []byte) (string, error) {
		fresh := event.ProtoReflect().New().Interface().(E)
		if err := proto.Unmarshal(payload, fresh); err != nil {
			return "", err
		}
		return id(fresh), nil
	}
}
