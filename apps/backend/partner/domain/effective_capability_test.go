package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const evaluatedTenant = "tenant-a"

func effectiveFacts() domain.EffectiveFacts {
	enabled := capabilityAt(domain.CapabilityStatusEnabled)
	enabled.Rollout = domain.Rollout{Mode: domain.RolloutEnabled}
	partner := partnerAt(domain.PartnerActive).Snapshot()
	return domain.EffectiveFacts{
		Partner:      partner,
		PartnerFound: true,
		Agreements:   []domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at-day, 0)},
		Capabilities: []domain.CapabilitySnapshot{enabled},
		Products:     []domain.FinancialProductSnapshot{productAt(domain.ProductActive, at+30*day)},
		Bucket:       func(domain.CapabilityID) int { return 50 },
	}
}

func effectiveQuery() domain.EffectiveQuery {
	return domain.EffectiveQuery{
		Operation:   "register-receivable",
		AssetType:   "duplicata",
		Environment: domain.EnvironmentProduction,
		TenantID:    evaluatedTenant,
		AsOf:        at,
	}
}

func amount(v int64) *int64 { return &v }

func term(v int) *int { return &v }

func TestTheEffectiveCapabilityAnswersTheFirstConditionThatFails(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts func(*domain.EffectiveFacts)
		query func(*domain.EffectiveQuery)
		want  domain.EffectiveCondition
	}{
		{"no capability of the series, even with the partner suspended", func(f *domain.EffectiveFacts) {
			f.Capabilities[0].AssetType = "cheque"
			f.Partner.Status = domain.PartnerSuspended
		}, nil, domain.ConditionCapabilityNotFound},
		{"capability of another environment", nil, func(q *domain.EffectiveQuery) { q.Environment = domain.EnvironmentHomologation },
			domain.ConditionCapabilityNotFound},
		{"partner suspended with the capability enabled", func(f *domain.EffectiveFacts) { f.Partner.Status = domain.PartnerSuspended },
			nil, domain.ConditionPartnerNotActive},
		{"partner nobody holds", func(f *domain.EffectiveFacts) { f.PartnerFound = false }, nil, domain.ConditionPartnerNotActive},
		{"agreement still draft", func(f *domain.EffectiveFacts) { f.Agreements[0].Status = domain.AgreementDraft },
			nil, domain.ConditionAgreementNotEffective},
		{"agreement expired at the instant", func(f *domain.EffectiveFacts) { f.Agreements[0].ValidUntil = at },
			nil, domain.ConditionAgreementNotEffective},
		{"current version suspended", func(f *domain.EffectiveFacts) {
			suspended := f.Capabilities[0]
			suspended.ID, suspended.Version, suspended.Status = "cap-101", 2, domain.CapabilityStatusSuspended
			f.Capabilities = append(f.Capabilities, suspended)
		}, nil, domain.ConditionCapabilityNotEnabled},
		{"only homologated", func(f *domain.EffectiveFacts) { f.Capabilities[0].Status = domain.CapabilityStatusHomologated },
			nil, domain.ConditionCapabilityNotEnabled},
		{"product retired", func(f *domain.EffectiveFacts) { f.Products[0].Status = domain.ProductRetired },
			func(q *domain.EffectiveQuery) { q.ProductID = productID }, domain.ConditionProductNotEffective},
		{"product past its validity", func(f *domain.EffectiveFacts) { f.Products[0].ValidUntil = at },
			func(q *domain.EffectiveQuery) { q.ProductID = productID }, domain.ConditionProductNotEffective},
		{"product superseded by a version in effect", func(f *domain.EffectiveFacts) {
			next := f.Products[0]
			next.ID, next.Version, next.SupersedesID = "fpr-101", 3, productID
			f.Products = append(f.Products, next)
		}, func(q *domain.EffectiveQuery) { q.ProductID = productID }, domain.ConditionProductNotEffective},
		{"product of nobody", nil, func(q *domain.EffectiveQuery) { q.ProductID = "fpr-none" }, domain.ConditionProductNotEffective},
		{"tenant excluded from a rollout enabled", func(f *domain.EffectiveFacts) {
			f.Capabilities[0].Rollout.ExcludedTenantIDs = []string{evaluatedTenant}
		}, nil, domain.ConditionNotInRollout},
		{"tenant outside the percentage", func(f *domain.EffectiveFacts) {
			f.Capabilities[0].Rollout = domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 50}
		}, nil, domain.ConditionNotInRollout},
		{"amount above the product", nil, func(q *domain.EffectiveQuery) {
			q.ProductID, q.AmountMinor = productID, amount(50_000_001)
		}, domain.ConditionParametersIncompatible},
		{"term below the product", nil, func(q *domain.EffectiveQuery) {
			q.ProductID, q.TermDays = productID, term(6)
		}, domain.ConditionParametersIncompatible},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts, query := effectiveFacts(), effectiveQuery()
			if tc.facts != nil {
				tc.facts(&facts)
			}
			if tc.query != nil {
				tc.query(&query)
			}

			got := domain.ResolveEffectiveCapability(facts, query)

			if got.Allowed || got.FailedCondition != tc.want {
				t.Fatalf("ResolveEffectiveCapability() = %+v, want denied with %s", got, tc.want)
			}
		})
	}
}

func TestTheEffectiveCapabilityAllowsWithoutEvaluatingTheProductWhenNoneIsAsked(t *testing.T) {
	facts := effectiveFacts()
	facts.Partner.Role = domain.RoleRegistry
	facts.Products = nil
	query := effectiveQuery()
	query.AmountMinor, query.TermDays = amount(1), term(1)

	if got := domain.ResolveEffectiveCapability(facts, query); !got.Allowed || got.FailedCondition != "" {
		t.Fatalf("ResolveEffectiveCapability() = %+v, want allowed without a product", got)
	}
}

func TestTheEffectiveCapabilityAllowsAProductInEffectWithCompatibleParameters(t *testing.T) {
	query := effectiveQuery()
	query.ProductID, query.AmountMinor, query.TermDays = productID, amount(100_000), term(180)

	if got := domain.ResolveEffectiveCapability(effectiveFacts(), query); !got.Allowed {
		t.Fatalf("ResolveEffectiveCapability() = %+v, want allowed", got)
	}
}

func TestTheRolloutBucketIsAskedForTheCurrentVersion(t *testing.T) {
	facts := effectiveFacts()
	older := facts.Capabilities[0]
	current := older
	current.ID, current.Version, current.SupersedesID = "cap-101", 2, older.ID
	current.Rollout = domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 10}
	facts.Capabilities = []domain.CapabilitySnapshot{current, older}
	var asked []domain.CapabilityID
	facts.Bucket = func(id domain.CapabilityID) int {
		asked = append(asked, id)
		return 9
	}

	got := domain.ResolveEffectiveCapability(facts, effectiveQuery())

	if !got.Allowed || len(asked) != 1 || asked[0] != "cap-101" {
		t.Fatalf("ResolveEffectiveCapability() = %+v asking %v, want allowed by the bucket of cap-101", got, asked)
	}
}

func TestEffectiveConditionsKeepTheirPublishedValues(t *testing.T) {
	for _, value := range []struct {
		got  domain.EffectiveCondition
		want string
	}{
		{domain.ConditionCapabilityNotFound, "capability_not_found"},
		{domain.ConditionPartnerNotActive, "partner_not_active"},
		{domain.ConditionAgreementNotEffective, "agreement_not_effective"},
		{domain.ConditionCapabilityNotEnabled, "capability_not_enabled"},
		{domain.ConditionProductNotEffective, "product_not_effective"},
		{domain.ConditionNotInRollout, "not_in_rollout"},
		{domain.ConditionParametersIncompatible, "parameters_incompatible"},
	} {
		if string(value.got) != value.want {
			t.Errorf("condition = %q, want %q", value.got, value.want)
		}
	}
}

func TestAFunderServesTheRequestByAProductInEffectThatCoversIt(t *testing.T) {
	request := domain.ProductRequest{AssetType: "duplicata", Region: "SP", AmountMinor: 1_000_000, TermDays: 30, AsOf: at}
	inEffect := productAt(domain.ProductActive, at+30*day)
	for _, tc := range []struct {
		name     string
		products func() []domain.FinancialProductSnapshot
		want     bool
	}{
		{"product in effect covering the request", func() []domain.FinancialProductSnapshot { return []domain.FinancialProductSnapshot{inEffect} }, true},
		{"no product", func() []domain.FinancialProductSnapshot { return nil }, false},
		{"product retired", func() []domain.FinancialProductSnapshot {
			p := inEffect
			p.Status = domain.ProductRetired
			return []domain.FinancialProductSnapshot{p}
		}, false},
		{"product not yet valid", func() []domain.FinancialProductSnapshot {
			p := inEffect
			p.ValidFrom = at + day
			return []domain.FinancialProductSnapshot{p}
		}, false},
		{"another region", func() []domain.FinancialProductSnapshot {
			p := inEffect
			p.Regions = []string{"RJ"}
			return []domain.FinancialProductSnapshot{p}
		}, false},
		{"another asset type", func() []domain.FinancialProductSnapshot {
			p := inEffect
			p.AssetTypes = []string{"cheque"}
			return []domain.FinancialProductSnapshot{p}
		}, false},
		{"amount below the range", func() []domain.FinancialProductSnapshot {
			p := inEffect
			p.MinAmountMinor = 2_000_000
			return []domain.FinancialProductSnapshot{p}
		}, false},
		{"term above the range", func() []domain.FinancialProductSnapshot {
			p := inEffect
			p.MaxTermDays = 29
			return []domain.FinancialProductSnapshot{p}
		}, false},
		{"a compatible version superseded by an incompatible one", func() []domain.FinancialProductSnapshot {
			next := inEffect
			next.ID, next.Version, next.Regions = "fpr-101", 3, []string{"RJ"}
			return []domain.FinancialProductSnapshot{inEffect, next}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.ServesRequest(tc.products(), request); got != tc.want {
				t.Fatalf("ServesRequest() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTheCurrentVersionsAreTheHighestValidAtTheInstant(t *testing.T) {
	product := func(id domain.ProductID, code string, version int, validFrom kernel.Instant) domain.FinancialProductSnapshot {
		return domain.FinancialProductSnapshot{ID: id, PartnerID: partnerID, Code: code, Version: version, ValidFrom: validFrom}
	}
	products := []domain.FinancialProductSnapshot{
		product("fpr-1", "receivables-advance", 1, at-2*day),
		product("fpr-2", "receivables-advance", 2, at),
		product("fpr-3", "receivables-advance", 3, at+day),
		product("fpr-4", "invoice-advance", 1, at-day),
	}
	current := domain.CurrentProducts(products, at)
	if len(current) != 2 || current[0].ID != "fpr-4" || current[1].ID != "fpr-2" {
		t.Fatalf("CurrentProducts() = %+v, want fpr-4 and fpr-2, ordered by code", current)
	}

	profile := func(id domain.ProfileID, environment domain.Environment, version int, effectiveFrom kernel.Instant) domain.IntegrationProfileSnapshot {
		return domain.IntegrationProfileSnapshot{ID: id, PartnerID: partnerID, Environment: environment, Version: version, EffectiveFrom: effectiveFrom}
	}
	profiles := []domain.IntegrationProfileSnapshot{
		profile("ipf-1", domain.EnvironmentProduction, 1, at-day),
		profile("ipf-2", domain.EnvironmentProduction, 2, at+day),
		profile("ipf-3", domain.EnvironmentHomologation, 1, at-day),
	}
	if got, found := domain.CurrentProfile(profiles, domain.EnvironmentProduction, at); !found || got.ID != "ipf-1" {
		t.Fatalf("CurrentProfile() = %+v, %v; want ipf-1, the version 2 not yet effective", got, found)
	}
	if _, found := domain.CurrentProfile(profiles, domain.EnvironmentDevelopment, at); found {
		t.Fatal("CurrentProfile(development) found a profile, want none")
	}

	policies := []domain.OperationalPolicySnapshot{
		{ID: "pol-1", PartnerID: partnerID, Version: 1, EffectiveFrom: at - day},
		{ID: "pol-2", PartnerID: partnerID, Version: 2, EffectiveFrom: at},
	}
	if got, found := domain.CurrentPolicy(policies, at); !found || got.ID != "pol-2" {
		t.Fatalf("CurrentPolicy() = %+v, %v; want pol-2", got, found)
	}
	if _, found := domain.CurrentPolicy(policies, at-2*day); found {
		t.Fatal("CurrentPolicy() before every version found a policy, want none")
	}
}
