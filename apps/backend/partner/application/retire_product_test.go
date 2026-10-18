package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRetireProductWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedProduct(t, h, ctx, productVersion(testProductID, "receivables-advance", 1, ""))

	if _, err := h.service.RetireProduct(ctx, retireProduct(domain.RetireWithdrawn)); err != nil {
		t.Fatalf("RetireProduct() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"financial_products.Load",
		"financial_products.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRetireProductEnqueuesTheFactOfItsReason(t *testing.T) {
	now := kernel.Instant(testOccurred)
	for _, tc := range []struct {
		reason domain.ProductRetireReason
		want   kernel.DomainEvent
	}{
		{domain.RetireExpired, domain.PartnerProductExpired{ProductID: testProductID, PartnerID: testPartnerID, Version: 1, ValidUntil: now, At: now}},
		{domain.RetireWithdrawn, domain.PartnerProductRetired{ProductID: testProductID, PartnerID: testPartnerID, Version: 1, At: now}},
	} {
		t.Run(string(tc.reason), func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			seedProduct(t, h, ctx, productVersion(testProductID, "receivables-advance", 1, ""))

			outcome, err := h.service.RetireProduct(ctx, retireProduct(tc.reason))

			if err != nil {
				t.Fatalf("RetireProduct() error = %v, want nil", err)
			}
			if got := outcome.Response(); got != (domain.RetireProductResponse{ProductID: testProductID}) {
				t.Fatalf("Response() = %+v", got)
			}
			if snap, version, err := h.productRead.Load(ctx, testProductID); err != nil || version != 2 || snap.Status != domain.ProductRetired {
				t.Fatalf("Load() = %s v%d, %v; want retired at stored version 2", snap.Status, version, err)
			}
			entries := h.store.Entries()
			if len(entries) != 1 || entries[0].Event != tc.want || entries[0].AggregateType != application.AggregateTypeFinancialProduct ||
				entries[0].AggregateVersion != 2 || entries[0].Intent.PartitionKey != string(testProductID) {
				t.Fatalf("outbox = %+v, want %+v keyed by the product at version 2", entries, tc.want)
			}
		})
	}
}

func TestRetireProductRejectsEachPreconditionWithoutWriting(t *testing.T) {
	retired := productVersion(testProductID, "receivables-advance", 1, "")
	retired.Status = domain.ProductRetired
	inForce := productVersion(testProductID, "receivables-advance", 1, "")
	inForce.ValidUntil = kernel.Instant(testOccurred) + day
	for _, tc := range []struct {
		name   string
		held   domain.FinancialProductSnapshot
		reason domain.ProductRetireReason
		code   kernel.Code
	}{
		{"a retired product", retired, domain.RetireWithdrawn, domain.CodeFinancialProductNotActive},
		{"expired within the validity", inForce, domain.RetireExpired, domain.CodeFinancialProductNotYetExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			seedProduct(t, h, ctx, tc.held)

			outcome, err := h.service.RetireProduct(ctx, retireProduct(tc.reason))

			if err != nil {
				t.Fatalf("RetireProduct() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if snap, version, err := h.productRead.Load(ctx, testProductID); err != nil || version != 1 || !snap.Equal(tc.held) {
				t.Fatalf("Load() = v%d, %v; want the held product untouched", version, err)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a rejection", h.outboxLen())
			}
		})
	}
}

func TestRetiringAnUnknownProductIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.RetireProduct(withExecution(t, context.Background()), retireProduct(domain.RetireWithdrawn))

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("RetireProduct() = %v, want ErrNotFound", err)
	}
}

func TestTheDeterministicKeyOfAnExpiryReplaysTheFirstRetirement(t *testing.T) {
	h := newHarness(t)
	seedProduct(t, h, withExecution(t, context.Background()), productVersion(testProductID, "receivables-advance", 1, ""))
	key := "product-expired." + string(testProductID) + ".20250817"

	first, errFirst := h.service.RetireProduct(withKey(t, context.Background(), key), retireProduct(domain.RetireExpired))
	ctx := withKey(t, context.Background(), key)
	again, errAgain := h.service.RetireProduct(ctx, retireProduct(domain.RetireExpired))

	if errFirst != nil || errAgain != nil || again.Response() != first.Response() {
		t.Fatalf("RetireProduct() = %+v, %v then %+v, %v; want the first response answered twice", first.Response(), errFirst, again.Response(), errAgain)
	}
	if outcome, _ := port.IdempotencyOutcomeFrom(ctx); outcome != port.IdempotencyReplayed {
		t.Fatalf("idempotency outcome = %v, want replayed", outcome)
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want a single PartnerProductExpired", got)
	}
}

func TestEachFailureOfRetireProductCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		wantMsg string
	}{
		{"load", []option{withTableFaults("financial_products", serviceskit.Faults{Load: errBoom})}, "application: retire product fpr-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, "application: retire product fpr-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			ctx := withExecution(t, context.Background())
			seedProduct(t, h, ctx, productVersion(testProductID, "receivables-advance", 1, ""))

			_, err := h.service.RetireProduct(ctx, retireProduct(domain.RetireWithdrawn))

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
		})
	}
}

func TestTheReasonOfRetireProductTakesPartInTheFingerprint(t *testing.T) {
	h := newHarness(t)
	seedProduct(t, h, withExecution(t, context.Background()), productVersion(testProductID, "receivables-advance", 1, ""))
	if _, err := h.service.RetireProduct(withKey(t, context.Background(), "k-reused"), retireProduct(domain.RetireExpired)); err != nil {
		t.Fatalf("first RetireProduct() = %v, want nil", err)
	}

	if _, err := h.service.RetireProduct(withKey(t, context.Background(), "k-reused"), retireProduct(domain.RetireWithdrawn)); !errors.Is(err, port.ErrIdempotencyMismatch) {
		t.Fatalf("second RetireProduct() = %v, want ErrIdempotencyMismatch", err)
	}
}

func TestTheProductResponsesKeepTheirStoredFormatAndRoundTrip(t *testing.T) {
	golden := []byte{0x01, 'a', 0x07, 'f', 'p', 'r', '-', '1', '0', '0'}
	defined := usecase.EncodeOutcome(application.DefineProductCodec, usecase.Accepted(domain.DefineProductResponse{ProductID: "fpr-100"}))
	retired := usecase.EncodeOutcome(application.RetireProductCodec, usecase.Accepted(domain.RetireProductResponse{ProductID: "fpr-100"}))
	if !slices.Equal(defined, golden) || !slices.Equal(retired, golden) {
		t.Fatalf("stored = %x and %x, want %x: an entry written by the previous release must still decode", defined, retired, golden)
	}

	definedBack, errDefined := usecase.DecodeOutcome(application.DefineProductCodec, defined)
	retiredBack, errRetired := usecase.DecodeOutcome(application.RetireProductCodec, retired)

	if errDefined != nil || definedBack.Response().ProductID != "fpr-100" || errRetired != nil || retiredBack.Response().ProductID != "fpr-100" {
		t.Fatalf("decoded = %+v %v, %+v %v; want each response back", definedBack.Response(), errDefined, retiredBack.Response(), errRetired)
	}
}
