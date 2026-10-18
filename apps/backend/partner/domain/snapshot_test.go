package domain_test

import (
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func offboardedPartner() domain.PartnerSnapshot {
	return domain.PartnerSnapshot{
		ID:                   partnerID,
		OrganizationID:       organizationID,
		Role:                 domain.RoleRegistry,
		Status:               domain.PartnerOffboarding,
		RegisteredAt:         at,
		StageChangedAt:       at + day,
		StageReason:          "contract terminated",
		OffboardingStartedAt: at + day,
	}
}

func activeAgreement() domain.PartnerAgreementSnapshot {
	return domain.PartnerAgreementSnapshot{
		ID:                   agreementID,
		PartnerID:            partnerID,
		Version:              2,
		SupersedesID:         "agr-1",
		Status:               domain.AgreementActive,
		ValidFrom:            at,
		ValidUntil:           at + 365*day,
		ContractEvidenceID:   "evd-1",
		InstrumentTemplateID: "tpl-1",
		ActivatedAt:          at + day,
	}
}

func TestFromPartnerSnapshotRoundTrips(t *testing.T) {
	before := offboardedPartner()

	if got := domain.FromPartnerSnapshot(before).Snapshot(); !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromPartnerAgreementSnapshotRoundTrips(t *testing.T) {
	before := activeAgreement()

	if got := domain.FromPartnerAgreementSnapshot(before).Snapshot(); !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestPartnerEqualTellsEveryFieldApart(t *testing.T) {
	base := offboardedPartner()
	for name, mutate := range map[string]func(*domain.PartnerSnapshot){
		"id":                     func(s *domain.PartnerSnapshot) { s.ID = "prt-other" },
		"organization":           func(s *domain.PartnerSnapshot) { s.OrganizationID = "org-other" },
		"role":                   func(s *domain.PartnerSnapshot) { s.Role = domain.RoleFunder },
		"status":                 func(s *domain.PartnerSnapshot) { s.Status = domain.PartnerActive },
		"registered at":          func(s *domain.PartnerSnapshot) { s.RegisteredAt++ },
		"stage changed at":       func(s *domain.PartnerSnapshot) { s.StageChangedAt++ },
		"stage reason":           func(s *domain.PartnerSnapshot) { s.StageReason = "other" },
		"offboarding started at": func(s *domain.PartnerSnapshot) { s.OffboardingStartedAt++ },
	} {
		changed := base
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func TestPartnerAgreementEqualTellsEveryFieldApart(t *testing.T) {
	base := activeAgreement()
	for name, mutate := range map[string]func(*domain.PartnerAgreementSnapshot){
		"id":                  func(s *domain.PartnerAgreementSnapshot) { s.ID = "agr-other" },
		"partner":             func(s *domain.PartnerAgreementSnapshot) { s.PartnerID = "prt-other" },
		"version":             func(s *domain.PartnerAgreementSnapshot) { s.Version++ },
		"supersedes":          func(s *domain.PartnerAgreementSnapshot) { s.SupersedesID = "" },
		"status":              func(s *domain.PartnerAgreementSnapshot) { s.Status = domain.AgreementDraft },
		"valid from":          func(s *domain.PartnerAgreementSnapshot) { s.ValidFrom++ },
		"valid until":         func(s *domain.PartnerAgreementSnapshot) { s.ValidUntil = 0 },
		"contract evidence":   func(s *domain.PartnerAgreementSnapshot) { s.ContractEvidenceID = "" },
		"instrument template": func(s *domain.PartnerAgreementSnapshot) { s.InstrumentTemplateID = "" },
		"activated at":        func(s *domain.PartnerAgreementSnapshot) { s.ActivatedAt = 0 },
	} {
		changed := base
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func alertedProfileSnapshot() domain.IntegrationProfileSnapshot {
	return domain.IntegrationProfileSnapshot{
		ID:                       profileID,
		PartnerID:                partnerID,
		Environment:              domain.EnvironmentHomologation,
		Version:                  2,
		SupersedesID:             "ipf-1",
		ChangeKind:               domain.ChangeUnannounced,
		Protocols:                []string{"https"},
		Endpoint:                 "https://partner.example/api",
		SchemaVersion:            "v3",
		SecretRef:                "vault://partners/prt-100/api-key",
		CertificateRef:           "cert-100",
		CertificateExpiresAt:     at + 25*day,
		EffectiveFrom:            at,
		LastAlertedThresholdDays: 30,
	}
}

func supersedingPolicy() domain.OperationalPolicySnapshot {
	return domain.OperationalPolicySnapshot{
		ID:                 policyID,
		PartnerID:          partnerID,
		Version:            2,
		SupersedesID:       "pol-1",
		EffectiveFrom:      at,
		RateLimitPerMinute: 100,
		BusinessDays:       []string{"mon", "fri"},
		CutoffTime:         "17:00",
		ProcessingWindow:   "08:00-18:00",
		SlaRef:             "sla-gold",
		Fallback:           "manual queue",
		SupportContact:     "support@partner.example",
		IncidentContact:    "+55 11 99999-0000",
	}
}

func TestFromIntegrationProfileSnapshotRoundTrips(t *testing.T) {
	before := alertedProfileSnapshot()

	if got := domain.FromIntegrationProfileSnapshot(before).Snapshot(); !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromOperationalPolicySnapshotRoundTrips(t *testing.T) {
	before := supersedingPolicy()

	if got := domain.FromOperationalPolicySnapshot(before).Snapshot(); !got.Equal(before) {
		t.Fatal("round trip lost state")
	}
}

func TestIntegrationProfileEqualTellsEveryFieldApart(t *testing.T) {
	base := alertedProfileSnapshot()
	for name, mutate := range map[string]func(*domain.IntegrationProfileSnapshot){
		"id":                     func(s *domain.IntegrationProfileSnapshot) { s.ID = "ipf-other" },
		"partner":                func(s *domain.IntegrationProfileSnapshot) { s.PartnerID = "prt-other" },
		"environment":            func(s *domain.IntegrationProfileSnapshot) { s.Environment = domain.EnvironmentProduction },
		"version":                func(s *domain.IntegrationProfileSnapshot) { s.Version++ },
		"supersedes":             func(s *domain.IntegrationProfileSnapshot) { s.SupersedesID = "" },
		"change kind":            func(s *domain.IntegrationProfileSnapshot) { s.ChangeKind = domain.ChangeAnnounced },
		"protocols":              func(s *domain.IntegrationProfileSnapshot) { s.Protocols = []string{"mtls"} },
		"endpoint":               func(s *domain.IntegrationProfileSnapshot) { s.Endpoint = "https://other.example" },
		"schema version":         func(s *domain.IntegrationProfileSnapshot) { s.SchemaVersion = "v4" },
		"secret reference":       func(s *domain.IntegrationProfileSnapshot) { s.SecretRef = "vault://other" },
		"certificate reference":  func(s *domain.IntegrationProfileSnapshot) { s.CertificateRef = "cert-other" },
		"certificate expiry":     func(s *domain.IntegrationProfileSnapshot) { s.CertificateExpiresAt++ },
		"effective from":         func(s *domain.IntegrationProfileSnapshot) { s.EffectiveFrom++ },
		"last alerted threshold": func(s *domain.IntegrationProfileSnapshot) { s.LastAlertedThresholdDays = 7 },
	} {
		changed := base
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func TestOperationalPolicyEqualTellsEveryFieldApart(t *testing.T) {
	base := supersedingPolicy()
	for name, mutate := range map[string]func(*domain.OperationalPolicySnapshot){
		"id":                func(s *domain.OperationalPolicySnapshot) { s.ID = "pol-other" },
		"partner":           func(s *domain.OperationalPolicySnapshot) { s.PartnerID = "prt-other" },
		"version":           func(s *domain.OperationalPolicySnapshot) { s.Version++ },
		"supersedes":        func(s *domain.OperationalPolicySnapshot) { s.SupersedesID = "" },
		"effective from":    func(s *domain.OperationalPolicySnapshot) { s.EffectiveFrom++ },
		"rate limit":        func(s *domain.OperationalPolicySnapshot) { s.RateLimitPerMinute++ },
		"business days":     func(s *domain.OperationalPolicySnapshot) { s.BusinessDays = []string{"mon"} },
		"cutoff time":       func(s *domain.OperationalPolicySnapshot) { s.CutoffTime = "18:00" },
		"processing window": func(s *domain.OperationalPolicySnapshot) { s.ProcessingWindow = "00:00-23:59" },
		"sla":               func(s *domain.OperationalPolicySnapshot) { s.SlaRef = "sla-silver" },
		"fallback":          func(s *domain.OperationalPolicySnapshot) { s.Fallback = "" },
		"support contact":   func(s *domain.OperationalPolicySnapshot) { s.SupportContact = "other" },
		"incident contact":  func(s *domain.OperationalPolicySnapshot) { s.IncidentContact = "other" },
	} {
		changed := base
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func homologatedCapability() domain.CapabilitySnapshot {
	return domain.CapabilitySnapshot{
		ID:                  capabilityID,
		PartnerID:           partnerID,
		Operation:           "register-receivable",
		AssetType:           "duplicata",
		Environment:         domain.EnvironmentHomologation,
		Version:             2,
		SupersedesID:        "cap-1",
		IsWrite:             true,
		StateQuerySupported: false,
		RestrictionNote:     "reconciled by the daily file",
		CallbackApplicable:  true,
		Status:              domain.CapabilityStatusSuspended,
		Homologation:        []domain.CriterionResult{{CriterionID: "H1", Passed: true, EvidenceID: "evd-1", RecordedAt: at}},
		Rollout: domain.Rollout{
			Mode: domain.RolloutAllowlist, AllowlistTenantIDs: []string{"tenant-a"}, Percentage: 10, ExcludedTenantIDs: []string{"tenant-x"},
		},
		Suspension: domain.Suspension{Reason: "schema v4 rejected", Trigger: domain.TriggerSchemaMismatch, SuspendedAt: at + day},
		EnabledAt:  at,
	}
}

func TestFromCapabilitySnapshotRoundTrips(t *testing.T) {
	before := homologatedCapability()

	if got := domain.FromCapabilitySnapshot(before).Snapshot(); !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestTheCapabilityKeepsItsOwnCopyOfItsLists(t *testing.T) {
	before := homologatedCapability()
	c := domain.FromCapabilitySnapshot(before)

	before.Homologation[0].Passed = false
	before.Rollout.AllowlistTenantIDs[0] = "tenant-z"
	c.Snapshot().Rollout.ExcludedTenantIDs[0] = "tenant-z"

	if got := c.Snapshot(); !got.Equal(homologatedCapability()) {
		t.Fatalf("Snapshot() = %+v, want the state given, untouched by the caller", got)
	}
}

func TestCapabilityEqualTellsEveryFieldApart(t *testing.T) {
	base := homologatedCapability()
	for name, mutate := range map[string]func(*domain.CapabilitySnapshot){
		"id":                    func(s *domain.CapabilitySnapshot) { s.ID = "cap-other" },
		"partner":               func(s *domain.CapabilitySnapshot) { s.PartnerID = "prt-other" },
		"operation":             func(s *domain.CapabilitySnapshot) { s.Operation = "settle" },
		"asset type":            func(s *domain.CapabilitySnapshot) { s.AssetType = "cpr" },
		"environment":           func(s *domain.CapabilitySnapshot) { s.Environment = domain.EnvironmentProduction },
		"version":               func(s *domain.CapabilitySnapshot) { s.Version++ },
		"supersedes":            func(s *domain.CapabilitySnapshot) { s.SupersedesID = "" },
		"is write":              func(s *domain.CapabilitySnapshot) { s.IsWrite = false },
		"state query supported": func(s *domain.CapabilitySnapshot) { s.StateQuerySupported = true },
		"restriction note":      func(s *domain.CapabilitySnapshot) { s.RestrictionNote = "" },
		"callback applicable":   func(s *domain.CapabilitySnapshot) { s.CallbackApplicable = false },
		"status":                func(s *domain.CapabilitySnapshot) { s.Status = domain.CapabilityStatusEnabled },
		"homologation": func(s *domain.CapabilitySnapshot) {
			s.Homologation = []domain.CriterionResult{{CriterionID: "H1", RecordedAt: at}}
		},
		"rollout mode":       func(s *domain.CapabilitySnapshot) { s.Rollout.Mode = domain.RolloutEnabled },
		"rollout allowlist":  func(s *domain.CapabilitySnapshot) { s.Rollout.AllowlistTenantIDs = nil },
		"rollout percentage": func(s *domain.CapabilitySnapshot) { s.Rollout.Percentage++ },
		"rollout excluded":   func(s *domain.CapabilitySnapshot) { s.Rollout.ExcludedTenantIDs = []string{"tenant-y"} },
		"suspension":         func(s *domain.CapabilitySnapshot) { s.Suspension.Trigger = domain.TriggerManual },
		"enabled at":         func(s *domain.CapabilitySnapshot) { s.EnabledAt++ },
	} {
		changed := homologatedCapability()
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}
