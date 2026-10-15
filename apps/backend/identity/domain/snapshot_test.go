package domain_test

import (
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestFromOrganizationSnapshotRoundTrips(t *testing.T) {
	before := newRegisteredOrganization(t).Snapshot()

	got := domain.FromOrganizationSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromOrganizationSnapshotRestoresTheStatus(t *testing.T) {
	registered := newRegisteredOrganization(t).Snapshot()

	if got := domain.FromOrganizationSnapshot(registered).Snapshot().Status; got != domain.OrganizationActive {
		t.Fatalf("Status = %v, want OrganizationActive", got)
	}
}

func TestEqualTellsEveryFieldApart(t *testing.T) {
	base := newRegisteredOrganization(t).Snapshot()
	for name, mutate := range map[string]func(*domain.OrganizationSnapshot){
		"id":            func(s *domain.OrganizationSnapshot) { s.ID = "org-other" },
		"kinds":         func(s *domain.OrganizationSnapshot) { s.Kinds = domain.KindsOf(domain.KindFunder) },
		"jurisdiction":  func(s *domain.OrganizationSnapshot) { s.Jurisdiction = "PT" },
		"tax id type":   func(s *domain.OrganizationSnapshot) { s.TaxIDType = domain.TaxIDTypeCPF },
		"tax id":        func(s *domain.OrganizationSnapshot) { s.TaxID = numericCNPJ },
		"tax id hash":   func(s *domain.OrganizationSnapshot) { s.TaxIDHash = "other" },
		"key version":   func(s *domain.OrganizationSnapshot) { s.TaxIDHashKeyVersion = 2 },
		"tax id masked": func(s *domain.OrganizationSnapshot) { s.TaxIDMasked = "other" },
		"legal name":    func(s *domain.OrganizationSnapshot) { s.LegalName = "other" },
		"trade name":    func(s *domain.OrganizationSnapshot) { s.TradeName = "other" },
		"status":        func(s *domain.OrganizationSnapshot) { s.Status = domain.OrganizationSuspended },
		"created at":    func(s *domain.OrganizationSnapshot) { s.CreatedAt++ },
	} {
		changed := base
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}
