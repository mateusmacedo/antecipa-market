package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const evaluatedTenant = "tenant-a"

func enabledFor(id domain.CapabilityID, assetType string, mode domain.RolloutMode) domain.CapabilitySnapshot {
	c := capabilityVersion(id, domain.CapabilityStatusEnabled, 1, "")
	c.AssetType, c.Rollout = assetType, domain.Rollout{Mode: mode}
	return c
}

func effectiveAsk() application.FindEffectiveCapability {
	return application.FindEffectiveCapability{
		Partner:     testPartnerID,
		Operation:   "register-receivable",
		AssetType:   "duplicata",
		Environment: domain.EnvironmentProduction,
		TenantID:    evaluatedTenant,
		AsOf:        kernel.Instant(testOccurred),
	}
}

func effectivePartner(t *testing.T, h *harness, ctx context.Context, role domain.PartnerRole, status domain.PartnerStatus) {
	t.Helper()
	p := partnerAt(status)
	p.Role = role
	seed(t, h, ctx, partnerTable, testPartnerID, p)
	holdEffectiveAgreement(h)
}

func TestFindEffectiveCapabilityDeniesTheSuspendedSeriesOutsideAnyUnitOfWork(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	effectivePartner(t, h, ctx, domain.RoleFunder, domain.PartnerActive)
	suspended := enabledFor(testCapabilityID, "duplicata", domain.RolloutEnabled)
	suspended.Status = domain.CapabilityStatusSuspended
	h.caps.hold(testPartnerID, suspended, enabledFor("cap-101", "cheque", domain.RolloutEnabled))

	got, err := h.service.FindEffectiveCapability(ctx, effectiveAsk())

	if err != nil || got.Allowed || got.FailedCondition != domain.ConditionCapabilityNotEnabled {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want denied with capability_not_enabled", got, err)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v, want the query outside any unit of work", h.rec.Observed())
	}
	if h.outboxLen() != 0 {
		t.Fatalf("outbox holds %d entries, want a query to write nothing", h.outboxLen())
	}
}

func TestFindEffectiveCapabilityDeniesTheCapabilityOfASuspendedPartner(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	effectivePartner(t, h, ctx, domain.RoleFunder, domain.PartnerSuspended)
	h.caps.hold(testPartnerID, enabledFor(testCapabilityID, "duplicata", domain.RolloutEnabled))

	got, err := h.service.FindEffectiveCapability(ctx, effectiveAsk())

	if err != nil || got.Allowed || got.FailedCondition != domain.ConditionPartnerNotActive {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want denied with partner_not_active", got, err)
	}
}

func TestFindEffectiveCapabilityLetsTheExclusionPrevailOverTheRollout(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	effectivePartner(t, h, ctx, domain.RoleFunder, domain.PartnerActive)
	capability := enabledFor(testCapabilityID, "duplicata", domain.RolloutEnabled)
	capability.Rollout.ExcludedTenantIDs = []string{evaluatedTenant}
	h.caps.hold(testPartnerID, capability)

	got, err := h.service.FindEffectiveCapability(ctx, effectiveAsk())

	if err != nil || got.Allowed || got.FailedCondition != domain.ConditionNotInRollout {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want denied with not_in_rollout", got, err)
	}
}

func TestFindEffectiveCapabilityAllowsARegistryWithoutReadingAProduct(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	effectivePartner(t, h, ctx, domain.RoleRegistry, domain.PartnerActive)
	capability := enabledFor(testCapabilityID, "duplicata", domain.RolloutPercentage)
	capability.Rollout.Percentage = 10
	h.caps.hold(testPartnerID, capability)
	h.service.Rollout = fixedBuckets{string(testCapabilityID) + ":" + evaluatedTenant: 9}

	got, err := h.service.FindEffectiveCapability(ctx, effectiveAsk())

	if err != nil || !got.Allowed || got.FailedCondition != "" {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want allowed", got, err)
	}
	if slices.Contains(h.rec.Observed(), "financial_products.LoadByPartner") {
		t.Fatalf("observed = %v, want the product left unread without a product id", h.rec.Observed())
	}
}

func TestFindEffectiveCapabilityJudgesTheProductAskedFor(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	effectivePartner(t, h, ctx, domain.RoleFunder, domain.PartnerActive)
	h.caps.hold(testPartnerID, enabledFor(testCapabilityID, "duplicata", domain.RolloutEnabled))
	product := productVersion(testProductID, "receivables-advance", 1, "")
	product.ValidUntil = kernel.Instant(testOccurred) + 30*day
	h.products.hold(testPartnerID, product)
	ask := effectiveAsk()
	ask.ProductID = testProductID
	tooMuch := int64(50_000_001)
	ask.AmountMinor = &tooMuch

	got, err := h.service.FindEffectiveCapability(ctx, ask)

	if err != nil || got.Allowed || got.FailedCondition != domain.ConditionParametersIncompatible {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want denied with parameters_incompatible", got, err)
	}
	if !slices.Equal(h.products.asked, []domain.PartnerID{testPartnerID}) {
		t.Fatalf("products asked for %v, want the partner of the capability", h.products.asked)
	}
}

func TestFindEffectiveCapabilityReadsTheDataOfAnotherTenantAsAbsent(t *testing.T) {
	h := newHarness(t)
	h.caps.err = port.CrossTenantAccess{Object: "capabilities?partner_id=prt-100", ContextTenant: "acme", DataTenant: "globex"}
	seed(t, h, withTenant(t, context.Background(), "globex"), partnerTable, testPartnerID, partnerAt(domain.PartnerActive))

	got, err := h.service.FindEffectiveCapability(withExecution(t, context.Background()), effectiveAsk())

	if err != nil || got.Allowed || got.FailedCondition != domain.ConditionCapabilityNotFound {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want denied with capability_not_found", got, err)
	}
}

func TestFindEffectiveCapabilityFailsClosedOnATechnicalFailure(t *testing.T) {
	h := newHarness(t)
	boom := errors.New("agreements unavailable")
	h.agreements.err = boom
	ctx := withExecution(t, context.Background())
	seed(t, h, ctx, partnerTable, testPartnerID, partnerAt(domain.PartnerActive))
	h.caps.hold(testPartnerID, enabledFor(testCapabilityID, "duplicata", domain.RolloutEnabled))

	got, err := h.service.FindEffectiveCapability(ctx, effectiveAsk())

	if !errors.Is(err, boom) || got.Allowed {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want the failure and no allowance", got, err)
	}
}

func TestFindEffectiveCapabilityReadsTheRelationsOfTheAskedPartnerWhenNobodyHoldsIt(t *testing.T) {
	h := newHarness(t)
	holdEffectiveAgreement(h)
	h.caps.hold(testPartnerID, enabledFor(testCapabilityID, "duplicata", domain.RolloutEnabled))

	got, err := h.service.FindEffectiveCapability(withExecution(t, context.Background()), effectiveAsk())

	if err != nil || got.Allowed || got.FailedCondition != domain.ConditionPartnerNotActive {
		t.Fatalf("FindEffectiveCapability() = %+v, %v; want denied with partner_not_active", got, err)
	}
	if !slices.Equal(h.caps.asked, []domain.PartnerID{testPartnerID}) || !slices.Equal(h.agreements.asked, []domain.PartnerID{testPartnerID}) {
		t.Fatalf("relations asked for %v and %v, want the partner of the query", h.caps.asked, h.agreements.asked)
	}
}

type bucketCall struct {
	capability domain.CapabilityID
	tenant     port.TenantID
}

type recordedBuckets struct {
	bucket int
	calls  []bucketCall
}

func (b *recordedBuckets) Bucket(id domain.CapabilityID, tenant port.TenantID) int {
	b.calls = append(b.calls, bucketCall{id, tenant})
	return b.bucket
}

func TestFindEffectiveCapabilityBucketsTheCurrentVersionByTheTenantOfTheQuery(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bucket int
		want   domain.EffectiveCondition
	}{
		{"bucket below the percentage", 9, ""},
		{"bucket at the percentage", 10, domain.ConditionNotInRollout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			effectivePartner(t, h, ctx, domain.RoleRegistry, domain.PartnerActive)
			older := capabilityVersion(testCapabilityID, domain.CapabilityStatusSuspended, 1, "")
			current := capabilityVersion("cap-101", domain.CapabilityStatusEnabled, 2, testCapabilityID)
			current.Rollout = domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 10}
			h.caps.hold(testPartnerID, older, current)
			buckets := &recordedBuckets{bucket: tc.bucket}
			h.service.Rollout = buckets

			got, err := h.service.FindEffectiveCapability(ctx, effectiveAsk())

			if err != nil || got.Allowed != (tc.want == "") || got.FailedCondition != tc.want {
				t.Fatalf("FindEffectiveCapability() = %+v, %v; want the condition %q", got, err, tc.want)
			}
			if want := []bucketCall{{"cap-101", evaluatedTenant}}; !slices.Equal(buckets.calls, want) || evaluatedTenant == string(testTenant) {
				t.Fatalf("buckets asked %v, want %v: the current version under the tenant of the query, not %s", buckets.calls, want, testTenant)
			}
		})
	}
}

func funderOf(id domain.PartnerID, status domain.PartnerStatus) domain.PartnerSnapshot {
	p := partnerAt(status)
	p.ID = id
	return p
}

func eligibleAsk() application.FindEligiblePartners {
	return application.FindEligiblePartners{
		AssetType: "duplicata", Operation: "register-receivable", Environment: domain.EnvironmentProduction, TenantID: evaluatedTenant,
		AmountMinor: 1_000_000, TermDays: 30, Region: "SP", AsOf: kernel.Instant(testOccurred),
	}
}

func offering(partnerID domain.PartnerID, id domain.ProductID, region string) domain.FinancialProductSnapshot {
	p := productVersion(id, "receivables-advance", 1, "")
	p.PartnerID, p.Regions, p.ValidUntil = partnerID, []string{region}, kernel.Instant(testOccurred)+30*day
	return p
}

func TestFindEligiblePartnersAnswersOnlyTheFundersEligibleToTheRequest(t *testing.T) {
	h := newHarness(t)
	h.byRole.hold(domain.RoleFunder,
		funderOf("prt-eligible", domain.PartnerActive),
		funderOf("prt-other-region", domain.PartnerActive),
		funderOf("prt-suspended", domain.PartnerActive),
		funderOf("prt-without-product", domain.PartnerActive),
	)
	registry := funderOf("prt-registry", domain.PartnerActive)
	registry.Role = domain.RoleRegistry
	h.byRole.hold(domain.RoleRegistry, registry)
	for _, id := range []domain.PartnerID{"prt-eligible", "prt-other-region", "prt-suspended", "prt-without-product", "prt-registry"} {
		h.agreements.hold(id, activeAgreement(domain.AgreementID("agr-"+id), 0))
		capability := enabledFor(domain.CapabilityID("cap-"+id), "duplicata", domain.RolloutEnabled)
		capability.PartnerID = id
		h.caps.hold(id, capability)
	}
	suspended := h.caps.byKey["prt-suspended"][0]
	suspended.ID, suspended.Version, suspended.Status = "cap-prt-suspended-2", 2, domain.CapabilityStatusSuspended
	h.caps.hold("prt-suspended", suspended)
	h.products.hold("prt-eligible", offering("prt-eligible", "fpr-eligible", "SP"))
	h.products.hold("prt-other-region", offering("prt-other-region", "fpr-other-region", "RJ"))
	h.products.hold("prt-suspended", offering("prt-suspended", "fpr-suspended", "SP"))
	h.products.hold("prt-registry", offering("prt-registry", "fpr-registry", "SP"))

	got, err := h.service.FindEligiblePartners(withExecution(t, context.Background()), eligibleAsk())

	if err != nil || len(got) != 1 || got[0].ID != "prt-eligible" {
		t.Fatalf("FindEligiblePartners() = %+v, %v; want only prt-eligible", got, err)
	}
	if !slices.Equal(h.byRole.asked, []domain.PartnerRole{domain.RoleFunder}) {
		t.Fatalf("partners asked by role %v, want only the funders", h.byRole.asked)
	}
	if slices.Contains(h.rec.Observed(), "within") {
		t.Fatalf("observed = %v, want the query outside any unit of work", h.rec.Observed())
	}
}

func TestFindEligiblePartnersAnswersNoneWhenTheFundersAreOfAnotherTenant(t *testing.T) {
	h := newHarness(t)
	h.byRole.err = port.CrossTenantAccess{Object: "partners?role=funder", ContextTenant: "acme", DataTenant: "globex"}

	got, err := h.service.FindEligiblePartners(withExecution(t, context.Background()), eligibleAsk())

	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("FindEligiblePartners() = %v, %v; want an empty answer", got, err)
	}
}
