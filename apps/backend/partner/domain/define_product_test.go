package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestDefineProductOpensTheSeriesAtVersionOneAndEmitsTheDefinition(t *testing.T) {
	p := domain.NewFinancialProduct(productID)

	acc, rej := p.Define(defineProductCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.DefineProductResponse{ProductID: productID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	want := domain.FinancialProductSnapshot{
		ID:               productID,
		PartnerID:        partnerID,
		AgreementID:      agreementID,
		Code:             "receivables-advance",
		Name:             "Antecipação de duplicatas",
		Version:          1,
		Status:           domain.ProductActive,
		ValidFrom:        at,
		ValidUntil:       at + 365*day,
		AssetTypes:       []string{"duplicata", "cheque"},
		Regions:          []string{"SP", "RJ"},
		Currency:         "BRL",
		MinAmountMinor:   100_000,
		MaxAmountMinor:   50_000_000,
		MinTermDays:      7,
		MaxTermDays:      180,
		RequiredCheckIDs: []string{"CHK-01", "CHK-07"},
	}
	if got := p.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
	requireProductEvents(t, acc.Events(), []kernel.DomainEvent{domain.PartnerProductDefined{
		ProductID:      productID,
		PartnerID:      partnerID,
		AgreementID:    agreementID,
		Code:           "receivables-advance",
		Version:        1,
		ValidFrom:      at,
		ValidUntil:     at + 365*day,
		AssetTypes:     []string{"duplicata", "cheque"},
		Regions:        []string{"SP", "RJ"},
		Currency:       "BRL",
		MinAmountMinor: 100_000,
		MaxAmountMinor: 50_000_000,
		MinTermDays:    7,
		MaxTermDays:    180,
		At:             at,
	}})
}

func TestDefineProductSupersedesTheLastVersionOfTheSeriesAndEmitsTheChange(t *testing.T) {
	cmd := defineProductCommand()
	cmd.SupersedesID, cmd.Head = "fpr-2", domain.ProductHead{ID: "fpr-2", Version: 2}
	p := domain.NewFinancialProduct("fpr-3")

	acc, rej := p.Define(cmd)

	tb.RequireAccepted(t, rej)
	if s := p.Snapshot(); s.Version != 3 || s.SupersedesID != "fpr-2" || s.Status != domain.ProductActive {
		t.Fatalf("version %d superseding %q in %s, want version 3 superseding fpr-2, active", s.Version, s.SupersedesID, s.Status)
	}
	requireProductEvents(t, acc.Events(), []kernel.DomainEvent{domain.PartnerProductChanged{
		ProductID:      "fpr-3",
		PartnerID:      partnerID,
		AgreementID:    agreementID,
		Code:           "receivables-advance",
		Version:        3,
		SupersedesID:   "fpr-2",
		ValidFrom:      at,
		ValidUntil:     at + 365*day,
		AssetTypes:     []string{"duplicata", "cheque"},
		Regions:        []string{"SP", "RJ"},
		Currency:       "BRL",
		MinAmountMinor: 100_000,
		MaxAmountMinor: 50_000_000,
		MinTermDays:    7,
		MaxTermDays:    180,
		At:             at,
	}})
}

func TestDefineProductAcceptsTheBoundsOfItsRanges(t *testing.T) {
	for name, mutate := range map[string]func(*domain.DefineProduct){
		"a single amount and term": func(c *domain.DefineProduct) {
			c.MinAmountMinor, c.MaxAmountMinor, c.MinTermDays, c.MaxTermDays = 500_000, 500_000, 30, 30
		},
		"an indefinite validity": func(c *domain.DefineProduct) { c.ValidUntil = 0 },
		"no region":              func(c *domain.DefineProduct) { c.Regions = nil },
	} {
		t.Run(name, func(t *testing.T) {
			cmd := defineProductCommand()
			mutate(&cmd)

			_, rej := domain.NewFinancialProduct(productID).Define(cmd)

			tb.RequireAccepted(t, rej)
		})
	}
}

func TestDefineProductKeepsItsOwnCopyOfTheLists(t *testing.T) {
	cmd := defineProductCommand()
	p := domain.NewFinancialProduct(productID)
	acc, rej := p.Define(cmd)
	if rej != nil {
		t.Fatalf("Define rejected: %v", rej.Code())
	}

	cmd.AssetTypes[0], cmd.Regions[0], cmd.RequiredCheckIDs[0] = "x", "x", "x"
	s := p.Snapshot()
	s.AssetTypes[1], s.Regions[1], s.RequiredCheckIDs[1] = "y", "y", "y"
	defined := acc.Events()[0].(domain.PartnerProductDefined)

	got := p.Snapshot()
	if !slices.Equal(got.AssetTypes, []string{"duplicata", "cheque"}) || !slices.Equal(got.Regions, []string{"SP", "RJ"}) ||
		!slices.Equal(got.RequiredCheckIDs, []string{"CHK-01", "CHK-07"}) {
		t.Fatalf("Snapshot() = %+v, want the lists of the command, untouched by the caller", got)
	}
	if !slices.Equal(defined.AssetTypes, []string{"duplicata", "cheque"}) || !slices.Equal(defined.Regions, []string{"SP", "RJ"}) {
		t.Fatalf("event = %+v, want the lists of the command, untouched by the caller", defined)
	}
}

func TestDefineProductRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.DefineProduct)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"without asset types", func(c *domain.DefineProduct) { c.AssetTypes = nil },
			domain.CodeFinancialProductAssetTypesEmpty, nil},
		{"minimum amount above the maximum", func(c *domain.DefineProduct) { c.MinAmountMinor = c.MaxAmountMinor + 1 },
			domain.CodeFinancialProductRangeInvalid, nil},
		{"minimum term above the maximum", func(c *domain.DefineProduct) { c.MinTermDays = c.MaxTermDays + 1 },
			domain.CodeFinancialProductRangeInvalid, nil},
		{"valid until before valid from", func(c *domain.DefineProduct) { c.ValidUntil = c.ValidFrom - 1 },
			domain.CodeFinancialProductValidityInvalid, nil},
		{"valid until equal to valid from", func(c *domain.DefineProduct) { c.ValidUntil = c.ValidFrom },
			domain.CodeFinancialProductValidityInvalid, nil},
		{"agreement not active or not in effect", func(c *domain.DefineProduct) { c.HasEffectiveAgreement = false },
			domain.CodeFinancialProductAgreementNotActive, nil},
		{"supersedes in an empty series", func(c *domain.DefineProduct) { c.SupersedesID = "fpr-0" },
			domain.CodeFinancialProductSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "fpr-0"}}},
		{"another first version of the code", func(c *domain.DefineProduct) {
			c.Head = domain.ProductHead{ID: "fpr-1", Version: 1}
		}, domain.CodeFinancialProductSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: ""}}},
		{"supersedes a version that is not the last", func(c *domain.DefineProduct) {
			c.SupersedesID, c.Head = "fpr-1", domain.ProductHead{ID: "fpr-2", Version: 2}
		}, domain.CodeFinancialProductSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "fpr-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := defineProductCommand()
			tt.mutate(&cmd)
			p := domain.NewFinancialProduct(productID)
			before := p.Snapshot()

			acc, rej := p.Define(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireProductUnchanged(t, before, p.Snapshot())
		})
	}
}

func TestDefineProductReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := defineProductCommand()
	cmd.AssetTypes, cmd.MinTermDays, cmd.ValidUntil, cmd.HasEffectiveAgreement, cmd.SupersedesID = nil, cmd.MaxTermDays+1, cmd.ValidFrom, false, "fpr-0"
	want := []kernel.Code{
		domain.CodeFinancialProductAssetTypesEmpty,
		domain.CodeFinancialProductRangeInvalid,
		domain.CodeFinancialProductValidityInvalid,
		domain.CodeFinancialProductAgreementNotActive,
		domain.CodeFinancialProductSupersedesInvalid,
	}
	fixes := []func(*domain.DefineProduct){
		func(c *domain.DefineProduct) { c.AssetTypes = []string{"duplicata"} },
		func(c *domain.DefineProduct) { c.MinTermDays = 7 },
		func(c *domain.DefineProduct) { c.ValidUntil = 0 },
		func(c *domain.DefineProduct) { c.HasEffectiveAgreement = true },
		func(*domain.DefineProduct) {},
	}
	for i, code := range want {
		acc, rej := domain.NewFinancialProduct(productID).Define(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}
