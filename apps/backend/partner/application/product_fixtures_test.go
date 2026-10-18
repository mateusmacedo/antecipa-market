package application_test

import (
	"context"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func defineProduct() application.DefineProduct {
	return application.DefineProduct{
		ProductID:        testProductID,
		PartnerID:        testPartnerID,
		AgreementID:      testAgreementID,
		Code:             "receivables-advance",
		Name:             "Antecipação de duplicatas",
		AssetTypes:       []string{"duplicata", "cheque"},
		Regions:          []string{"SP", "RJ"},
		Currency:         "BRL",
		MinAmountMinor:   100_000,
		MaxAmountMinor:   50_000_000,
		MinTermDays:      7,
		MaxTermDays:      180,
		RequiredCheckIDs: []string{"CHK-01", "CHK-07"},
		ValidFrom:        kernel.Instant(testOccurred),
		ValidUntil:       kernel.Instant(testOccurred) + 365*day,
	}
}

func productVersion(id domain.ProductID, code string, version int, supersedes domain.ProductID) domain.FinancialProductSnapshot {
	return domain.FinancialProductSnapshot{
		ID: id, PartnerID: testPartnerID, AgreementID: testAgreementID, Code: code, Name: "Antecipação de duplicatas",
		Version: version, SupersedesID: supersedes, Status: domain.ProductActive,
		ValidFrom: kernel.Instant(testOccurred) - 30*day, ValidUntil: kernel.Instant(testOccurred),
		AssetTypes: []string{"duplicata"}, Currency: "BRL", MinAmountMinor: 100_000, MaxAmountMinor: 50_000_000, MinTermDays: 7, MaxTermDays: 180,
	}
}

func seedProduct(t *testing.T, h *harness, ctx context.Context, s domain.FinancialProductSnapshot) {
	t.Helper()
	seed(t, h, ctx, productTable, s.ID, s)
}

func seedSustainingAgreement(t *testing.T, h *harness, ctx context.Context) {
	t.Helper()
	agreement := activeAgreement(testAgreementID, kernel.Instant(testOccurred)+365*day)
	seed(t, h, ctx, agreementTable, testAgreementID, agreement)
	h.agreements.hold(testPartnerID, agreement)
}

func retireProduct(reason domain.ProductRetireReason) application.RetireProduct {
	return application.RetireProduct{ProductID: testProductID, Reason: reason}
}
