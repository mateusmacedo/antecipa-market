package rpc_test

import (
	"slices"
	"strings"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func defineProductRequest(productID string) *servicev1.DefineProductRequest {
	return &servicev1.DefineProductRequest{
		ProductId: productID, PartnerId: "prt-1", AgreementId: "agr-1", Code: "receivables-advance", Name: "Antecipação de duplicatas",
		AssetTypes: []string{"duplicata", "cheque"}, Regions: []string{"SP", "RJ"}, Currency: "BRL",
		MinAmountMinor: 100_000, MaxAmountMinor: 50_000_000, MinTermDays: 7, MaxTermDays: 180,
		RequiredCheckIds: []string{"CHK-01", "CHK-07"}, ValidFrom: validFrom, ValidUntil: validUntil,
	}
}

func productAt(id domain.ProductID, status domain.ProductStatus, validUntil port.Instant) domain.FinancialProductSnapshot {
	return domain.FinancialProductSnapshot{
		ID: id, PartnerID: "prt-1", AgreementID: "agr-1", Code: string(id), Name: "Antecipação de duplicatas", Version: 1,
		Status: status, ValidFrom: occurred - 30*86_400_000_000_000, ValidUntil: validUntil, AssetTypes: []string{"duplicata"},
		Currency: "BRL", MinAmountMinor: 1, MaxAmountMinor: 2, MinTermDays: 7, MaxTermDays: 180,
	}
}

func retireRequest(productID string, reason servicev1.ProductRetireReason) *servicev1.RetireProductRequest {
	return &servicev1.RetireProductRequest{ProductId: productID, Reason: reason}
}

func TestDefineProductAnswersEachVersionAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{
		agreements: agreementsByPartner{"prt-1": {activeAgreement()}},
		products:   productsByPartner{"prt-1": {{ID: "fpr-held", Code: "held-code", Version: 1}}},
	})
	h.seedAgreement(t, activeAgreement())

	var first, next servicev1.DefineProductResponse
	if err := h.invoke(withTenant(t), "DefineProduct", defineProductRequest("fpr-1"), &first); err != nil {
		t.Fatalf("DefineProduct() = %v", err)
	}
	successor := defineProductRequest("fpr-2")
	successor.Code, successor.SupersedesId = "held-code", "fpr-held"
	if err := h.invoke(withTenant(t), "DefineProduct", successor, &next); err != nil {
		t.Fatalf("DefineProduct(successor) = %v", err)
	}
	if first.GetDefined().GetProductId() != "fpr-1" || next.GetDefined().GetProductId() != "fpr-2" {
		t.Fatalf("Defined = %v and %v, want fpr-1 and fpr-2", &first, &next)
	}
	entries := h.store.Entries()
	if len(entries) != 2 {
		t.Fatalf("outbox holds %d entries, want the definition and the change", len(entries))
	}
	if _, defined := entries[0].Event.(domain.PartnerProductDefined); !defined {
		t.Fatalf("first event = %T, want PartnerProductDefined", entries[0].Event)
	}
	if changed, ok := entries[1].Event.(domain.PartnerProductChanged); !ok || changed.Version != 2 || changed.SupersedesID != "fpr-held" {
		t.Fatalf("second event = %+v, want PartnerProductChanged at version 2 superseding fpr-held", entries[1].Event)
	}

	for name, c := range map[string]struct {
		edit func(*servicev1.DefineProductRequest)
		code string
	}{
		"without asset types":              {func(r *servicev1.DefineProductRequest) { r.AssetTypes = nil }, "partner-management/financial-product/asset-types-empty"},
		"minimum amount above the maximum": {func(r *servicev1.DefineProductRequest) { r.MinAmountMinor = r.MaxAmountMinor + 1 }, "partner-management/financial-product/range-invalid"},
		"valid until before valid from":    {func(r *servicev1.DefineProductRequest) { r.ValidUntil = r.ValidFrom - 1 }, "partner-management/financial-product/validity-invalid"},
		"agreement unknown":                {func(r *servicev1.DefineProductRequest) { r.AgreementId = "agr-none" }, "partner-management/financial-product/agreement-not-active"},
		"second first version of the code": {func(r *servicev1.DefineProductRequest) { r.Code = "held-code" }, "partner-management/financial-product/supersedes-invalid"},
	} {
		req := defineProductRequest("fpr-x")
		c.edit(req)
		var resp servicev1.DefineProductResponse
		if err := h.invoke(withTenant(t), "DefineProduct", req, &resp); err != nil {
			t.Fatalf("%s: DefineProduct() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}
}

func TestRetireProductAnswersEachReasonAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{})
	seedIn(t, h, productsTable, "fpr-ended", productAt("fpr-ended", domain.ProductActive, occurred))
	seedIn(t, h, productsTable, "fpr-open", productAt("fpr-open", domain.ProductActive, occurred+86_400_000_000_000))
	seedIn(t, h, productsTable, "fpr-old", productAt("fpr-old", domain.ProductRetired, occurred))

	for _, c := range []struct {
		req  *servicev1.RetireProductRequest
		want proto.Message
	}{
		{retireRequest("fpr-ended", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_EXPIRED), &servicev1.RetiredProduct{ProductId: "fpr-ended"}},
		{retireRequest("fpr-open", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN), &servicev1.RetiredProduct{ProductId: "fpr-open"}},
	} {
		var resp servicev1.RetireProductResponse
		if err := h.invoke(withTenant(t), "RetireProduct", c.req, &resp); err != nil {
			t.Fatalf("RetireProduct(%s) = %v", c.req.GetProductId(), err)
		}
		if !proto.Equal(resp.GetRetired(), c.want) {
			t.Fatalf("RetireProduct(%s) = %v, want %v", c.req.GetProductId(), &resp, c.want)
		}
	}
	entries := h.store.Entries()
	if len(entries) != 2 {
		t.Fatalf("outbox holds %d entries, want the expiry and the retirement", len(entries))
	}
	if _, expired := entries[0].Event.(domain.PartnerProductExpired); !expired {
		t.Fatalf("first event = %T, want PartnerProductExpired", entries[0].Event)
	}
	if _, retired := entries[1].Event.(domain.PartnerProductRetired); !retired {
		t.Fatalf("second event = %T, want PartnerProductRetired", entries[1].Event)
	}

	seedIn(t, h, productsTable, "fpr-later", productAt("fpr-later", domain.ProductActive, occurred+86_400_000_000_000))
	for name, c := range map[string]struct {
		req  *servicev1.RetireProductRequest
		code string
	}{
		"a retired product":           {retireRequest("fpr-old", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN), "partner-management/financial-product/not-active"},
		"expired within the validity": {retireRequest("fpr-later", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_EXPIRED), "partner-management/financial-product/not-yet-expired"},
	} {
		var resp servicev1.RetireProductResponse
		if err := h.invoke(withTenant(t), "RetireProduct", c.req, &resp); err != nil {
			t.Fatalf("%s: RetireProduct() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}

	err := h.invoke(withTenant(t), "RetireProduct", retireRequest("fpr-none", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN), &servicev1.RetireProductResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("RetireProduct(fpr-none) = %v, want NotFound", err)
	}
}

func TestTheDeterministicKeyOfAProductExpiryReplaysTheAnswer(t *testing.T) {
	h := newHarness(t, relations{})
	seedIn(t, h, productsTable, "fpr-1", productAt("fpr-1", domain.ProductActive, occurred))
	req := retireRequest("fpr-1", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_EXPIRED)

	var first, again servicev1.RetireProductResponse
	var header metadata.MD
	if err := h.invoke(withKey(t, "product-expired.fpr-1.20250817"), "RetireProduct", req, &first); err != nil {
		t.Fatalf("RetireProduct() = %v", err)
	}
	if err := h.invoke(withKey(t, "product-expired.fpr-1.20250817"), "RetireProduct", req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated RetireProduct() = %v", err)
	}

	if !proto.Equal(&first, &again) || !slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want %v marked as replay", &again, header, &first)
	}
	if entries := h.store.Entries(); len(entries) != 1 {
		t.Fatalf("outbox holds %d entries, want a single PartnerProductExpired", len(entries))
	}
}

func TestMalformedProductRequestsAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, relations{})
	define := func(edit func(*servicev1.DefineProductRequest)) func() error {
		return func() error {
			req := defineProductRequest("fpr-1")
			edit(req)
			var r servicev1.DefineProductResponse
			return h.invoke(withTenant(t), "DefineProduct", req, &r)
		}
	}
	retire := func(req *servicev1.RetireProductRequest) func() error {
		return func() error {
			var r servicev1.RetireProductResponse
			return h.invoke(withTenant(t), "RetireProduct", req, &r)
		}
	}
	for _, c := range []struct {
		name string
		call func() error
	}{
		{"product id empty", define(func(r *servicev1.DefineProductRequest) { r.ProductId = "" })},
		{"product id with a colon", define(func(r *servicev1.DefineProductRequest) { r.ProductId = "fpr:1" })},
		{"product id over 100 characters", define(func(r *servicev1.DefineProductRequest) { r.ProductId = strings.Repeat("a", 101) })},
		{"retirement of a product id with a colon", retire(retireRequest("fpr:1", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN))},
		{"retirement of a product id over 100 characters", retire(retireRequest(strings.Repeat("a", 101), servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN))},
		{"product of no partner", define(func(r *servicev1.DefineProductRequest) { r.PartnerId = "" })},
		{"product of no agreement", define(func(r *servicev1.DefineProductRequest) { r.AgreementId = "" })},
		{"supersedes too long", define(func(r *servicev1.DefineProductRequest) { r.SupersedesId = strings.Repeat("a", 129) })},
		{"code too long", define(func(r *servicev1.DefineProductRequest) { r.Code = strings.Repeat("c", 65) })},
		{"code outside printable ascii", define(func(r *servicev1.DefineProductRequest) { r.Code = "antecipação" })},
		{"name too long", define(func(r *servicev1.DefineProductRequest) { r.Name = strings.Repeat("ç", 257) })},
		{"name with a control character", define(func(r *servicev1.DefineProductRequest) { r.Name = "Antecipação\x00" })},
		{"too many asset types", define(func(r *servicev1.DefineProductRequest) { r.AssetTypes = slices.Repeat([]string{"duplicata"}, 33) })},
		{"asset type with a line break", define(func(r *servicev1.DefineProductRequest) { r.AssetTypes = []string{"duplicata\n"} })},
		{"too many regions", define(func(r *servicev1.DefineProductRequest) { r.Regions = slices.Repeat([]string{"SP"}, 65) })},
		{"too many required checks", define(func(r *servicev1.DefineProductRequest) { r.RequiredCheckIds = slices.Repeat([]string{"CHK-01"}, 65) })},
		{"currency too long", define(func(r *servicev1.DefineProductRequest) { r.Currency = "BRLBRLBRL" })},
		{"negative valid from", define(func(r *servicev1.DefineProductRequest) { r.ValidFrom = -1 })},
		{"negative valid until", define(func(r *servicev1.DefineProductRequest) { r.ValidUntil = -1 })},
		{"retirement of no product", retire(retireRequest("", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN))},
		{"unspecified reason", retire(retireRequest("fpr-1", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_UNSPECIFIED))},
		{"reason outside the enum", retire(retireRequest("fpr-1", 99))},
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

func TestTheLargestProductTheContractAdmitsIsDefined(t *testing.T) {
	h := newHarness(t, relations{agreements: agreementsByPartner{"prt-1": {activeAgreement()}}})
	h.seedAgreement(t, activeAgreement())
	req := defineProductRequest("fpr-long")
	req.Name = strings.Repeat("ç", 256)
	req.Code, req.Currency = strings.Repeat("c", 64), "BRL-REAL"
	req.AssetTypes = slices.Repeat([]string{strings.Repeat("a", 64)}, 32)
	req.Regions = slices.Repeat([]string{"SP"}, 64)
	req.RequiredCheckIds = slices.Repeat([]string{"CHK-01"}, 64)

	var resp servicev1.DefineProductResponse
	if err := h.invoke(withTenant(t), "DefineProduct", req, &resp); err != nil || resp.GetDefined() == nil {
		t.Fatalf("DefineProduct() = %v, %v; want the product at the ceilings defined", &resp, err)
	}
}
