package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestDefineProductWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedSustainingAgreement(t, h, ctx)

	if _, err := h.service.DefineProduct(ctx, defineProduct()); err != nil {
		t.Fatalf("DefineProduct() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"financial_products.LoadByPartner",
		"partner_agreements.Load",
		"partner_agreements.LoadByPartner",
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

func TestDefineProductOpensTheSeriesAndEnqueuesTheDefinition(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedSustainingAgreement(t, h, ctx)

	outcome, err := h.service.DefineProduct(ctx, defineProduct())

	if err != nil {
		t.Fatalf("DefineProduct() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.DefineProductResponse{ProductID: testProductID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.productRead.Load(ctx, testProductID)
	if err != nil || version != 1 || snap.Version != 1 || snap.Status != domain.ProductActive || snap.SupersedesID != "" ||
		!slices.Equal(snap.RequiredCheckIDs, []string{"CHK-01", "CHK-07"}) {
		t.Fatalf("Load() = %+v v%d, %v; want version 1 active with the required checks", snap, version, err)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeFinancialProduct || entry.AggregateID != string(testProductID) ||
		entry.AggregateVersion != 1 || entry.Intent.Destination != application.Destination || entry.Intent.PartitionKey != string(testProductID) {
		t.Fatalf("outbox entry = %+v, want the definition of the product at version 1 keyed by it", entry)
	}
	if _, defined := entry.Event.(domain.PartnerProductDefined); !defined {
		t.Fatalf("event = %T, want PartnerProductDefined", entry.Event)
	}
	if want := []domain.PartnerID{testPartnerID}; !slices.Equal(h.products.asked, want) {
		t.Fatalf("the series read asked for %v, want %v", h.products.asked, want)
	}
}

func TestDefineProductSupersedesTheLastVersionOfTheCodeAndEnqueuesTheChange(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedSustainingAgreement(t, h, ctx)
	h.products.hold(testPartnerID,
		productVersion("fpr-1", "receivables-advance", 1, ""),
		productVersion("fpr-2", "receivables-advance", 2, "fpr-1"),
		productVersion("fpr-9", "invoice-advance", 4, "fpr-8"),
	)
	cmd := defineProduct()
	cmd.ProductID, cmd.SupersedesID = "fpr-3", "fpr-2"

	outcome, err := h.service.DefineProduct(ctx, cmd)

	if err != nil {
		t.Fatalf("DefineProduct() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("DefineProduct() rejected with %s, want accepted", rej.Code())
	}
	if snap, _, err := h.productRead.Load(ctx, "fpr-3"); err != nil || snap.Version != 3 || snap.SupersedesID != "fpr-2" {
		t.Fatalf("Load() = version %d superseding %q, %v; want version 3 superseding fpr-2", snap.Version, snap.SupersedesID, err)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	changed, ok := entries[0].Event.(domain.PartnerProductChanged)
	if !ok || changed.SupersedesID != "fpr-2" || changed.Version != 3 || !slices.Equal(changed.AssetTypes, []string{"duplicata", "cheque"}) {
		t.Fatalf("event = %+v, want PartnerProductChanged at version 3 superseding fpr-2", entries[0].Event)
	}
}

func TestDefineProductOpensTheSeriesOfAnotherCode(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedSustainingAgreement(t, h, ctx)
	h.products.hold(testPartnerID, productVersion("fpr-9", "invoice-advance", 1, ""))

	outcome, err := h.service.DefineProduct(ctx, defineProduct())

	if err != nil {
		t.Fatalf("DefineProduct() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("DefineProduct() rejected with %s, want the first version of its own code", rej.Code())
	}
}

func agreementSuccessor(validUntil kernel.Instant) domain.PartnerAgreementSnapshot {
	next := activeAgreement("agr-101", validUntil)
	next.Version, next.SupersedesID = 2, testAgreementID
	return next
}

func TestDefineProductRejectsEachPreconditionWithoutWriting(t *testing.T) {
	now := kernel.Instant(testOccurred)
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *harness, context.Context, *application.DefineProduct)
		code    kernel.Code
	}{
		{"without asset types", func(t *testing.T, h *harness, ctx context.Context, c *application.DefineProduct) {
			seedSustainingAgreement(t, h, ctx)
			c.AssetTypes = nil
		}, domain.CodeFinancialProductAssetTypesEmpty},
		{"minimum amount above the maximum", func(t *testing.T, h *harness, ctx context.Context, c *application.DefineProduct) {
			seedSustainingAgreement(t, h, ctx)
			c.MinAmountMinor = c.MaxAmountMinor + 1
		}, domain.CodeFinancialProductRangeInvalid},
		{"valid until before valid from", func(t *testing.T, h *harness, ctx context.Context, c *application.DefineProduct) {
			seedSustainingAgreement(t, h, ctx)
			c.ValidUntil = c.ValidFrom - day
		}, domain.CodeFinancialProductValidityInvalid},
		{"agreement unknown", func(*testing.T, *harness, context.Context, *application.DefineProduct) {},
			domain.CodeFinancialProductAgreementNotActive},
		{"agreement still draft", func(t *testing.T, h *harness, ctx context.Context, _ *application.DefineProduct) {
			seed(t, h, ctx, agreementTable, testAgreementID, draftAgreement(testAgreementID, 1, ""))
		}, domain.CodeFinancialProductAgreementNotActive},
		{"agreement ended at the instant", func(t *testing.T, h *harness, ctx context.Context, _ *application.DefineProduct) {
			seed(t, h, ctx, agreementTable, testAgreementID, activeAgreement(testAgreementID, now))
		}, domain.CodeFinancialProductAgreementNotActive},
		{"agreement of another partner", func(t *testing.T, h *harness, ctx context.Context, c *application.DefineProduct) {
			seedSustainingAgreement(t, h, ctx)
			c.PartnerID = "prt-other"
		}, domain.CodeFinancialProductAgreementNotActive},
		{"agreement superseded by a version in effect", func(t *testing.T, h *harness, ctx context.Context, _ *application.DefineProduct) {
			seedSustainingAgreement(t, h, ctx)
			h.agreements.hold(testPartnerID, agreementSuccessor(0))
		}, domain.CodeFinancialProductAgreementNotActive},
		{"agreement open while a later active version expired", func(t *testing.T, h *harness, ctx context.Context, _ *application.DefineProduct) {
			open := activeAgreement(testAgreementID, 0)
			seed(t, h, ctx, agreementTable, testAgreementID, open)
			h.agreements.hold(testPartnerID, open, agreementSuccessor(now))
		}, domain.CodeFinancialProductAgreementNotActive},
		{"another first version of the code", func(t *testing.T, h *harness, ctx context.Context, _ *application.DefineProduct) {
			seedSustainingAgreement(t, h, ctx)
			h.products.hold(testPartnerID, productVersion("fpr-1", "receivables-advance", 1, ""))
		}, domain.CodeFinancialProductSupersedesInvalid},
		{"supersedes a version that is not the last", func(t *testing.T, h *harness, ctx context.Context, c *application.DefineProduct) {
			seedSustainingAgreement(t, h, ctx)
			h.products.hold(testPartnerID, productVersion("fpr-1", "receivables-advance", 1, ""), productVersion("fpr-2", "receivables-advance", 2, "fpr-1"))
			c.SupersedesID = "fpr-1"
		}, domain.CodeFinancialProductSupersedesInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			cmd := defineProduct()
			tc.prepare(t, h, ctx, &cmd)

			outcome, err := h.service.DefineProduct(ctx, cmd)

			if err != nil {
				t.Fatalf("DefineProduct() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := h.productRead.Load(ctx, testProductID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a rejection", h.outboxLen())
			}
		})
	}
}

func TestEachFailureOfDefineProductCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		wantMsg string
	}{
		{"series read", nil, func(h *harness) { h.products.err = errBoom }, "application: define product fpr-100: boom"},
		{"agreement read", []option{withTableFaults("partner_agreements", serviceskit.Faults{Load: errBoom})}, nil,
			"application: define product fpr-100: boom"},
		{"agreement series read", nil, func(h *harness) { h.agreements.err = errBoom }, "application: define product fpr-100: boom"},
		{"save", []option{withTableFaults("financial_products", serviceskit.Faults{Save: errBoom})}, nil,
			"application: define product fpr-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, nil, "application: define product fpr-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.prepare != nil {
				tc.prepare(h)
			}
			ctx := withExecution(t, context.Background())
			seedSustainingAgreement(t, h, ctx)

			_, err := h.service.DefineProduct(ctx, defineProduct())

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a failure", h.outboxLen())
			}
		})
	}
}

func TestEachFieldOfDefineProductTakesPartInTheFingerprint(t *testing.T) {
	for name, mutate := range map[string]func(*application.DefineProduct){
		"partner":         func(c *application.DefineProduct) { c.PartnerID = "prt-other" },
		"agreement":       func(c *application.DefineProduct) { c.AgreementID = "agr-other" },
		"code":            func(c *application.DefineProduct) { c.Code = "other" },
		"name":            func(c *application.DefineProduct) { c.Name = "other" },
		"asset types":     func(c *application.DefineProduct) { c.AssetTypes = []string{"cheque", "duplicata"} },
		"regions":         func(c *application.DefineProduct) { c.Regions = []string{"SP"} },
		"currency":        func(c *application.DefineProduct) { c.Currency = "USD" },
		"minimum amount":  func(c *application.DefineProduct) { c.MinAmountMinor++ },
		"maximum amount":  func(c *application.DefineProduct) { c.MaxAmountMinor++ },
		"minimum term":    func(c *application.DefineProduct) { c.MinTermDays++ },
		"maximum term":    func(c *application.DefineProduct) { c.MaxTermDays++ },
		"required checks": func(c *application.DefineProduct) { c.RequiredCheckIDs = nil },
		"valid from":      func(c *application.DefineProduct) { c.ValidFrom++ },
		"valid until":     func(c *application.DefineProduct) { c.ValidUntil++ },
		"supersedes":      func(c *application.DefineProduct) { c.SupersedesID = "fpr-0" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			seedSustainingAgreement(t, h, withExecution(t, context.Background()))
			if _, err := h.service.DefineProduct(withKey(t, context.Background(), "k-reused"), defineProduct()); err != nil {
				t.Fatalf("first DefineProduct() = %v, want nil", err)
			}
			cmd := defineProduct()
			mutate(&cmd)

			if _, err := h.service.DefineProduct(withKey(t, context.Background(), "k-reused"), cmd); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second DefineProduct() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}

func TestTheListsOfAProductDoNotShiftIntoTheNextField(t *testing.T) {
	h := newHarness(t)
	seedSustainingAgreement(t, h, withExecution(t, context.Background()))
	first := defineProduct()
	first.AssetTypes, first.Regions = []string{"duplicata", "SP"}, []string{"RJ"}
	second := defineProduct()
	second.AssetTypes, second.Regions = []string{"duplicata"}, []string{"SP", "RJ"}
	if _, err := h.service.DefineProduct(withKey(t, context.Background(), "k-shift"), first); err != nil {
		t.Fatalf("first DefineProduct() = %v, want nil", err)
	}

	if _, err := h.service.DefineProduct(withKey(t, context.Background(), "k-shift"), second); !errors.Is(err, port.ErrIdempotencyMismatch) {
		t.Fatalf("second DefineProduct() = %v, want ErrIdempotencyMismatch", err)
	}
}
