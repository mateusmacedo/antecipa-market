package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRetireProductWithdrawnRetiresAndEmitsTheRetirement(t *testing.T) {
	p := product(domain.ProductActive, at+30*day)

	acc, rej := p.Retire(domain.RetireProduct{Reason: domain.RetireWithdrawn, At: at})

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RetireProductResponse{ProductID: productID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	want := productAt(domain.ProductRetired, at+30*day)
	if got := p.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want the product retired and otherwise unchanged", got)
	}
	requireProductEvents(t, acc.Events(), []kernel.DomainEvent{domain.PartnerProductRetired{
		ProductID: productID, PartnerID: partnerID, Version: 2, At: at,
	}})
}

func TestRetireProductExpiredRetiresAndEmitsTheExpiry(t *testing.T) {
	for name, validUntil := range map[string]kernel.Instant{
		"valid until the instant":  at,
		"valid until a day before": at - day,
	} {
		t.Run(name, func(t *testing.T) {
			p := product(domain.ProductActive, validUntil)

			acc, rej := p.Retire(domain.RetireProduct{Reason: domain.RetireExpired, At: at})

			tb.RequireAccepted(t, rej)
			if s := p.Snapshot(); s.Status != domain.ProductRetired {
				t.Fatalf("Status = %s, want retired", s.Status)
			}
			requireProductEvents(t, acc.Events(), []kernel.DomainEvent{domain.PartnerProductExpired{
				ProductID: productID, PartnerID: partnerID, Version: 2, ValidUntil: validUntil, At: at,
			}})
		})
	}
}

func TestRetireProductRejectsEachPrecondition(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before domain.FinancialProductSnapshot
		reason domain.ProductRetireReason
		code   kernel.Code
	}{
		{"withdrawing a retired product", productAt(domain.ProductRetired, at-day), domain.RetireWithdrawn, domain.CodeFinancialProductNotActive},
		{"expiring a retired product", productAt(domain.ProductRetired, at-day), domain.RetireExpired, domain.CodeFinancialProductNotActive},
		{"expiring within the validity", productAt(domain.ProductActive, at+1), domain.RetireExpired, domain.CodeFinancialProductNotYetExpired},
		{"expiring an indefinite validity", productAt(domain.ProductActive, 0), domain.RetireExpired, domain.CodeFinancialProductNotYetExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := domain.FromFinancialProductSnapshot(tc.before)

			acc, rej := p.Retire(domain.RetireProduct{Reason: tc.reason, At: at})

			tb.RequireRejected(t, acc, rej, tc.code)
			requireProductUnchanged(t, tc.before, p.Snapshot())
		})
	}
}
