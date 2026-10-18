//go:build integration

package app_test

import (
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
)

func productOf(productID, supersedes string) *servicev1.DefineProductRequest {
	return &servicev1.DefineProductRequest{
		ProductId: productID, PartnerId: "e2e-prt-1", AgreementId: "e2e-agr-1", Code: "receivables-advance",
		Name: "Antecipação de duplicatas", AssetTypes: []string{"duplicata", "cheque"}, Regions: []string{"SP", "RJ"}, Currency: "BRL",
		MinAmountMinor: 100_000, MaxAmountMinor: 50_000_000, MinTermDays: 7, MaxTermDays: 180,
		RequiredCheckIds: []string{"CHK-01", "CHK-07"}, ValidFrom: validFrom - 30*day, ValidUntil: validFrom, SupersedesId: supersedes,
	}
}

func retirement(productID string, reason servicev1.ProductRetireReason) *servicev1.RetireProductRequest {
	return &servicev1.RetireProductRequest{ProductId: productID, Reason: reason}
}

func withActiveAgreement(t *testing.T, conn *grpc.ClientConn) {
	t.Helper()
	invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-1", ""), &servicev1.RegisterAgreementResponse{})
	invoke(t, conn, call(t), "ActivateAgreement", activation("e2e-agr-1"), &servicev1.ActivateAgreementResponse{})
}

func products(t *testing.T, pool *pgxpool.Pool) (rows, events int) {
	t.Helper()
	return count(t, pool, "SELECT count(*) FROM financial_products"), count(t, pool, "SELECT count(*) FROM outbox")
}

func TestDefineProductEndToEndKeepsOneSeriesAndEmitsEachVersion(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	withActiveAgreement(t, conn)

	first := invoke(t, conn, call(t), "DefineProduct", productOf("e2e-fpr-1", ""), &servicev1.DefineProductResponse{})
	second := invoke(t, conn, call(t), "DefineProduct", productOf("e2e-fpr-2", "e2e-fpr-1"), &servicev1.DefineProductResponse{})
	stale := invoke(t, conn, call(t), "DefineProduct", productOf("e2e-fpr-3", "e2e-fpr-1"), &servicev1.DefineProductResponse{})

	if first.GetDefined().GetProductId() != "e2e-fpr-1" || second.GetDefined().GetProductId() != "e2e-fpr-2" {
		t.Fatalf("DefineProduct() = %v and %v, want versions 1 and 2 defined", first, second)
	}
	if got := stale.GetRejection().GetCode(); got != "partner-management/financial-product/supersedes-invalid" {
		t.Fatalf("DefineProduct(stale supersedes) = %v, want supersedes-invalid", stale)
	}
	if got := count(t, pool, "SELECT count(*) FROM financial_products WHERE status = 'active' AND code = 'receivables-advance' AND (snapshot->>'version')::int = 2 AND supersedes_id = 'e2e-fpr-1' AND valid_until = $1 AND snapshot->'requiredCheckIds' = '[\"CHK-01\",\"CHK-07\"]'::jsonb", validFrom); got != 1 {
		t.Fatalf("active versions 2 superseding e2e-fpr-1 with the required checks = %d, want 1", got)
	}
	records := outbox(t, pool)
	if len(records) != 3 {
		t.Fatalf("outbox rows = %d, want the activation, the definition and the change", len(records))
	}
	defined, changed := records[1], records[2]
	if defined.messageType != "com.company.partner.partner-product-defined.v1" || defined.partitionKey != "e2e-fpr-1" ||
		changed.messageType != "com.company.partner.partner-product-changed.v1" || changed.partitionKey != "e2e-fpr-2" || changed.destination != application.Destination {
		t.Fatalf("outbox = %+v, want the definition keyed by e2e-fpr-1 and the change keyed by e2e-fpr-2", records[1:])
	}
	var event eventv1.PartnerProductChanged
	if err := proto.Unmarshal(changed.payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetProductId() != "e2e-fpr-2" || event.GetSupersedesId() != "e2e-fpr-1" || event.GetVersion() != 2 || event.GetAgreementId() != "e2e-agr-1" ||
		!slices.Equal(event.GetAssetTypes(), []string{"duplicata", "cheque"}) || event.GetMaxAmountMinor() != 50_000_000 || event.GetMaxTermDays() != 180 ||
		event.GetValidUntil().AsTime().UnixNano() != validFrom || event.GetAt().AsTime().UnixNano() != validFrom {
		t.Fatalf("event = %v, want version 2 of the code with its ranges and validity", &event)
	}
}

func TestRetireProductEndToEndEmitsTheFactOfItsReason(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	withActiveAgreement(t, conn)
	withdrawn := productOf("e2e-fpr-w", "")
	withdrawn.Code = "invoice-advance"
	invoke(t, conn, call(t), "DefineProduct", productOf("e2e-fpr-1", ""), &servicev1.DefineProductResponse{})
	invoke(t, conn, call(t), "DefineProduct", withdrawn, &servicev1.DefineProductResponse{})

	retired := invoke(t, conn, call(t), "RetireProduct", retirement("e2e-fpr-w", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN), &servicev1.RetireProductResponse{})

	if retired.GetRetired().GetProductId() != "e2e-fpr-w" {
		t.Fatalf("RetireProduct() = %v, want Retired e2e-fpr-w", retired)
	}
	records := outbox(t, pool)
	last := records[len(records)-1]
	if len(records) != 4 || last.messageType != "com.company.partner.partner-product-retired.v1" || last.partitionKey != "e2e-fpr-w" {
		t.Fatalf("outbox = %+v, want the retirement keyed by e2e-fpr-w after the activation and the two definitions", records)
	}
	var event eventv1.PartnerProductRetired
	if err := proto.Unmarshal(last.payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetProductId() != "e2e-fpr-w" || event.GetPartnerId() != "e2e-prt-1" || event.GetVersion() != 1 || event.GetAt().AsTime().UnixNano() != validFrom {
		t.Fatalf("event = %v, want version 1 of e2e-fpr-w withdrawn at the instant of the command", &event)
	}
	if got := count(t, pool, "SELECT count(*) FROM financial_products WHERE status = 'retired'"); got != 1 {
		t.Fatalf("retired products = %d, want only e2e-fpr-w", got)
	}
}

func TestTheDeterministicKeyOfAProductExpiryLeavesOneEvent(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	withActiveAgreement(t, conn)
	invoke(t, conn, call(t), "DefineProduct", productOf("e2e-fpr-1", ""), &servicev1.DefineProductResponse{})
	req := retirement("e2e-fpr-1", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_EXPIRED)
	key := "product-expired.e2e-fpr-1.20250817"

	var first, again servicev1.RetireProductResponse
	var header metadata.MD
	if err := conn.Invoke(callWithKey(t, key), rpc.FullMethod("RetireProduct"), req, &first); err != nil {
		t.Fatalf("RetireProduct() = %v", err)
	}
	if err := conn.Invoke(callWithKey(t, key), rpc.FullMethod("RetireProduct"), req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated RetireProduct() = %v, want the recorded response", err)
	}

	if !proto.Equal(&first, &again) || again.GetRetired().GetProductId() != "e2e-fpr-1" ||
		!slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want the first answer %v marked as replay", &again, header, &first)
	}
	var expiries []enqueued
	for _, record := range outbox(t, pool) {
		if record.messageType == "com.company.partner.partner-product-expired.v1" {
			expiries = append(expiries, record)
		}
	}
	if len(expiries) != 1 || expiries[0].partitionKey != "e2e-fpr-1" || expiries[0].destination != application.Destination {
		t.Fatalf("expiries = %+v, want a single PartnerProductExpired keyed by the product", expiries)
	}
	var event eventv1.PartnerProductExpired
	if err := proto.Unmarshal(expiries[0].payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetProductId() != "e2e-fpr-1" || event.GetVersion() != 1 || event.GetValidUntil().AsTime().UnixNano() != validFrom || event.GetAt().AsTime().UnixNano() != validFrom {
		t.Fatalf("event = %v, want version 1 expired at the end of its validity", &event)
	}
	if got := count(t, pool, "SELECT count(*) FROM financial_products WHERE status = 'retired' AND version = 2"); got != 1 {
		t.Fatalf("products retired once = %d, want 1", got)
	}
}

func TestEachRefusalOfTheFinancialProductWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	withActiveAgreement(t, conn)
	invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-draft", "e2e-agr-1"), &servicev1.RegisterAgreementResponse{})
	invoke(t, conn, call(t), "DefineProduct", productOf("e2e-fpr-1", ""), &servicev1.DefineProductResponse{})
	open := productOf("e2e-fpr-open", "")
	open.Code, open.ValidUntil = "invoice-advance", validFrom+day
	invoke(t, conn, call(t), "DefineProduct", open, &servicev1.DefineProductResponse{})
	old := productOf("e2e-fpr-old", "")
	old.Code = "cheque-advance"
	invoke(t, conn, call(t), "DefineProduct", old, &servicev1.DefineProductResponse{})
	invoke(t, conn, call(t), "RetireProduct", retirement("e2e-fpr-old", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN), &servicev1.RetireProductResponse{})
	heldRows, heldEvents := products(t, pool)

	define := func(edit func(*servicev1.DefineProductRequest)) func() string {
		return func() string {
			req := productOf("e2e-fpr-x", "e2e-fpr-1")
			edit(req)
			return invoke(t, conn, call(t), "DefineProduct", req, &servicev1.DefineProductResponse{}).GetRejection().GetCode()
		}
	}
	retire := func(productID string, reason servicev1.ProductRetireReason) func() string {
		return func() string {
			return invoke(t, conn, call(t), "RetireProduct", retirement(productID, reason), &servicev1.RetireProductResponse{}).GetRejection().GetCode()
		}
	}
	for _, c := range []struct {
		name string
		got  func() string
		code string
	}{
		{"without asset types", define(func(r *servicev1.DefineProductRequest) { r.AssetTypes = nil }),
			"partner-management/financial-product/asset-types-empty"},
		{"minimum term above the maximum", define(func(r *servicev1.DefineProductRequest) { r.MinTermDays = r.MaxTermDays + 1 }),
			"partner-management/financial-product/range-invalid"},
		{"valid until before valid from", define(func(r *servicev1.DefineProductRequest) { r.ValidUntil = r.ValidFrom - day }),
			"partner-management/financial-product/validity-invalid"},
		{"agreement still draft", define(func(r *servicev1.DefineProductRequest) { r.AgreementId = "e2e-agr-draft" }),
			"partner-management/financial-product/agreement-not-active"},
		{"another first version of the code", define(func(r *servicev1.DefineProductRequest) { r.SupersedesId = "" }),
			"partner-management/financial-product/supersedes-invalid"},
		{"retiring a retired product", retire("e2e-fpr-old", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN),
			"partner-management/financial-product/not-active"},
		{"expiring within the validity", retire("e2e-fpr-open", servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_EXPIRED),
			"partner-management/financial-product/not-yet-expired"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.got(); got != c.code {
				t.Fatalf("rejection = %q, want %q", got, c.code)
			}
			if rows, events := products(t, pool); rows != heldRows || events != heldEvents {
				t.Fatalf("financial_products = %d, outbox = %d after a refusal, want the %d held products and %d events", rows, events, heldRows, heldEvents)
			}
			if got := count(t, pool, "SELECT count(*) FROM financial_products WHERE financial_product_id = 'e2e-fpr-open' AND status = 'active'"); got != 1 {
				t.Fatalf("e2e-fpr-open left active after a refusal")
			}
		})
	}
}

func TestTwoConcurrentFirstVersionsOfOneCodeLeaveOneProduct(t *testing.T) {
	pool := appkit.OpenPool(t)
	withActiveAgreement(t, dial(t, pool, nil))
	saves := newBarrier(2)
	conn := dial(t, pool, saves)

	got := invokeAtOnce(t, conn, "DefineProduct",
		map[string]proto.Message{"first": productOf("e2e-fpr-a1", ""), "second": productOf("e2e-fpr-a2", "")},
		func(m proto.Message) bool { return m.(*servicev1.DefineProductResponse).GetDefined() != nil },
		func() proto.Message { return &servicev1.DefineProductResponse{} })

	oneAcceptedOneFailed(t, got)
	refusedByUniqueness(t, saves, "financial_products_tenant_id_partner_id_code_idx")
	if rows, events := products(t, pool); rows != 1 || events != 2 {
		t.Fatalf("financial_products = %d, outbox = %d, want one first version and its definition after the activation", rows, events)
	}
}
