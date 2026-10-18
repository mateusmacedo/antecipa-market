package domain_test

import (
	"fmt"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const productID = domain.ProductID("fpr-100")

func defineProductCommand() domain.DefineProduct {
	return domain.DefineProduct{
		PartnerID:             partnerID,
		AgreementID:           agreementID,
		Code:                  "receivables-advance",
		Name:                  "Antecipação de duplicatas",
		AssetTypes:            []string{"duplicata", "cheque"},
		Regions:               []string{"SP", "RJ"},
		Currency:              "BRL",
		MinAmountMinor:        100_000,
		MaxAmountMinor:        50_000_000,
		MinTermDays:           7,
		MaxTermDays:           180,
		RequiredCheckIDs:      []string{"CHK-01", "CHK-07"},
		ValidFrom:             at,
		ValidUntil:            at + 365*day,
		HasEffectiveAgreement: true,
		At:                    at,
	}
}

func productAt(status domain.ProductStatus, validUntil kernel.Instant) domain.FinancialProductSnapshot {
	return domain.FinancialProductSnapshot{
		ID:               productID,
		PartnerID:        partnerID,
		AgreementID:      agreementID,
		Code:             "receivables-advance",
		Name:             "Antecipação de duplicatas",
		Version:          2,
		SupersedesID:     "fpr-1",
		Status:           status,
		ValidFrom:        at - 30*day,
		ValidUntil:       validUntil,
		AssetTypes:       []string{"duplicata"},
		Regions:          []string{"SP"},
		Currency:         "BRL",
		MinAmountMinor:   100_000,
		MaxAmountMinor:   50_000_000,
		MinTermDays:      7,
		MaxTermDays:      180,
		RequiredCheckIDs: []string{"CHK-01"},
	}
}

func product(status domain.ProductStatus, validUntil kernel.Instant) *domain.FinancialProduct {
	return domain.FromFinancialProductSnapshot(productAt(status, validUntil))
}

func requireProductUnchanged(t *testing.T, before, after domain.FinancialProductSnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}

// tb.RequireSameEvents compares with ==, which panics on the lists the product
// events carry (testkit v1.0.0-rc.5 tb/domain.go:48), and depguard keeps reflect
// out of the block, so the events are compared by their Go syntax.
func requireProductEvents(t *testing.T, got, want []kernel.DomainEvent) {
	t.Helper()
	if fmt.Sprintf("%#v", got) != fmt.Sprintf("%#v", want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}
