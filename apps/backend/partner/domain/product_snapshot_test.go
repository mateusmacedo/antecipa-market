package domain_test

import (
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestFromFinancialProductSnapshotRoundTrips(t *testing.T) {
	before := productAt(domain.ProductRetired, at+day)

	if got := domain.FromFinancialProductSnapshot(before).Snapshot(); !got.Equal(before) {
		t.Fatalf("round trip = %+v, want %+v", got, before)
	}
}

func TestTheFinancialProductKeepsItsOwnCopyOfItsLists(t *testing.T) {
	before := productAt(domain.ProductActive, at+day)
	p := domain.FromFinancialProductSnapshot(before)

	before.AssetTypes[0], before.Regions[0], before.RequiredCheckIDs[0] = "x", "x", "x"

	if got := p.Snapshot(); !got.Equal(productAt(domain.ProductActive, at+day)) {
		t.Fatalf("Snapshot() = %+v, want the lists it was built from, untouched by the caller", got)
	}
}

func TestFinancialProductEqualTellsEveryFieldApart(t *testing.T) {
	base := productAt(domain.ProductActive, at+day)
	for name, mutate := range map[string]func(*domain.FinancialProductSnapshot){
		"id":              func(s *domain.FinancialProductSnapshot) { s.ID = "fpr-other" },
		"partner":         func(s *domain.FinancialProductSnapshot) { s.PartnerID = "prt-other" },
		"agreement":       func(s *domain.FinancialProductSnapshot) { s.AgreementID = "agr-other" },
		"code":            func(s *domain.FinancialProductSnapshot) { s.Code = "other" },
		"name":            func(s *domain.FinancialProductSnapshot) { s.Name = "other" },
		"version":         func(s *domain.FinancialProductSnapshot) { s.Version++ },
		"supersedes":      func(s *domain.FinancialProductSnapshot) { s.SupersedesID = "" },
		"status":          func(s *domain.FinancialProductSnapshot) { s.Status = domain.ProductRetired },
		"valid from":      func(s *domain.FinancialProductSnapshot) { s.ValidFrom++ },
		"valid until":     func(s *domain.FinancialProductSnapshot) { s.ValidUntil++ },
		"asset types":     func(s *domain.FinancialProductSnapshot) { s.AssetTypes = []string{"cheque"} },
		"regions":         func(s *domain.FinancialProductSnapshot) { s.Regions = []string{"RJ"} },
		"currency":        func(s *domain.FinancialProductSnapshot) { s.Currency = "USD" },
		"minimum amount":  func(s *domain.FinancialProductSnapshot) { s.MinAmountMinor++ },
		"maximum amount":  func(s *domain.FinancialProductSnapshot) { s.MaxAmountMinor++ },
		"minimum term":    func(s *domain.FinancialProductSnapshot) { s.MinTermDays++ },
		"maximum term":    func(s *domain.FinancialProductSnapshot) { s.MaxTermDays++ },
		"required checks": func(s *domain.FinancialProductSnapshot) { s.RequiredCheckIDs = []string{"CHK-02"} },
	} {
		changed := productAt(domain.ProductActive, at+day)
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func TestDefineProductDeterminism(t *testing.T) {
	p1, p2 := domain.NewFinancialProduct(productID), domain.NewFinancialProduct(productID)

	acc1, _ := p1.Define(defineProductCommand())
	acc2, _ := p2.Define(defineProductCommand())

	if !p1.Snapshot().Equal(p2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	requireProductEvents(t, acc1.Events(), acc2.Events())
}

func TestRetireProductDeterminism(t *testing.T) {
	p1, p2 := product(domain.ProductActive, at), product(domain.ProductActive, at)

	acc1, _ := p1.Retire(domain.RetireProduct{Reason: domain.RetireExpired, At: at})
	acc2, _ := p2.Retire(domain.RetireProduct{Reason: domain.RetireExpired, At: at})

	if !p1.Snapshot().Equal(p2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	requireProductEvents(t, acc1.Events(), acc2.Events())
}

func TestAnAgreementSustainsAProductWhenItIsTheCurrentVersionInEffectForThePartner(t *testing.T) {
	open := agreementVersion(1, domain.AgreementActive, at, 0)
	for _, tc := range []struct {
		name       string
		agreement  domain.PartnerAgreementSnapshot
		agreements []domain.PartnerAgreementSnapshot
		want       bool
	}{
		{"active of indefinite validity", open, []domain.PartnerAgreementSnapshot{open}, true},
		{"active until after the instant", agreementVersion(1, domain.AgreementActive, at, at+2*day),
			[]domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at, at+2*day)}, true},
		{"active until the instant", agreementVersion(1, domain.AgreementActive, at, at+day),
			[]domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at, at+day)}, false},
		{"active from after the instant", agreementVersion(1, domain.AgreementActive, at+2*day, 0),
			[]domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementActive, at+2*day, 0)}, false},
		{"draft", agreementVersion(1, domain.AgreementDraft, at, 0),
			[]domain.PartnerAgreementSnapshot{agreementVersion(1, domain.AgreementDraft, at, 0)}, false},
		{"of another partner", func() domain.PartnerAgreementSnapshot {
			a := open
			a.PartnerID = "prt-other"
			return a
		}(), []domain.PartnerAgreementSnapshot{open}, false},
		{"not found", domain.PartnerAgreementSnapshot{}, []domain.PartnerAgreementSnapshot{open}, false},
		{"superseded by a version in effect", open,
			[]domain.PartnerAgreementSnapshot{open, agreementVersion(2, domain.AgreementActive, at, 0)}, false},
		{"open while a later active version expired", open,
			[]domain.PartnerAgreementSnapshot{open, agreementVersion(2, domain.AgreementActive, at, at+day)}, false},
		{"followed by a version still draft", open,
			[]domain.PartnerAgreementSnapshot{open, agreementVersion(2, domain.AgreementDraft, at, 0)}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.AgreementSustains(tc.agreement, tc.agreements, partnerID, at+day); got != tc.want {
				t.Fatalf("AgreementSustains() = %v, want %v", got, tc.want)
			}
		})
	}
}
