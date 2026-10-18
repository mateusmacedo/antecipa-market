package provider_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

// In provider rather than domain because the domain block may not read files,
// and tb.LoadProjection finds no go.work above a testkit pinned in the module
// cache (testkit v1.0.0-rc.5 tb/tb.go:16-32).
func projectionFixture(t *testing.T, name string) tb.ProjectionFixture {
	t.Helper()
	raw, err := os.ReadFile("../contract/fixtures/projection/v1/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	f, err := tb.DecodeProjection(raw)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return f
}

func TestPartnersMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "partner.golden")
	if f.Identity.Context != "partner" || f.Identity.Aggregate != "partner" || len(f.Cases) != 12 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runPartner)
}

func TestPartnerAgreementsMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "partner-agreement.golden")
	if f.Identity.Context != "partner" || f.Identity.Aggregate != "partner-agreement" || len(f.Cases) != 10 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runAgreement)
}

func TestIntegrationProfilesMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "integration-profile.golden")
	if f.Identity.Context != "partner" || f.Identity.Aggregate != "integration-profile" || len(f.Cases) != 10 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runIntegrationProfile)
}

func TestOperationalPoliciesMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "operational-policy.golden")
	if f.Identity.Context != "partner" || f.Identity.Aggregate != "operational-policy" || len(f.Cases) != 5 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runOperationalPolicy)
}

func runPartner(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	before := partnerFromState(t, c.StateBefore)
	switch c.Command["upr"] {
	case "register":
		s := partnerSubject(func(p *domain.Partner) (kernel.Accepted[domain.RegisterPartnerResponse], *kernel.Rejection) {
			return p.Register(domain.RegisterPartner{
				OrganizationID:    domain.OrganizationID(c.Command["organization"]),
				Role:              domain.PartnerRole(c.Command["role"]),
				OrganizationFound: boolean(t, c.Command["organization_found"]),
				OrganizationKinds: kindsFrom(c.Command["organization_kinds"]),
				AlreadyRegistered: boolean(t, c.Command["already_registered"]),
				At:                kernel.Instant(integer(t, c.Command["at"])),
			})
		}, func(r domain.RegisterPartnerResponse) domainkit.Fields {
			return domainkit.Fields{"partner": string(r.PartnerID)}
		})
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "change-stage":
		s := partnerSubject(func(p *domain.Partner) (kernel.Accepted[domain.ChangePartnerStageResponse], *kernel.Rejection) {
			return p.ChangeStage(domain.ChangePartnerStage{
				Target:                domain.PartnerStatus(c.Command["target"]),
				Reason:                c.Command["reason"],
				HasEffectiveAgreement: boolean(t, c.Command["has_effective_agreement"]),
				HasEnabledCapability:  boolean(t, c.Command["has_enabled_capability"]),
				At:                    kernel.Instant(integer(t, c.Command["at"])),
			})
		}, func(r domain.ChangePartnerStageResponse) domainkit.Fields {
			return domainkit.Fields{"partner": string(r.PartnerID), "status": string(r.Status)}
		})
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func partnerSubject[R any](decide func(*domain.Partner) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.Partner, R] {
	return domainkit.Subject[*domain.Partner, R]{
		Decide:   decide,
		Response: response,
		Event:    partnerEvent,
		Snapshot: partnerState,
		Clone:    func(p *domain.Partner) *domain.Partner { return domain.FromPartnerSnapshot(p.Snapshot()) },
	}
}

func partnerState(p *domain.Partner) domainkit.Fields {
	s := p.Snapshot()
	return domainkit.Fields{
		"partner":                string(s.ID),
		"organization":           string(s.OrganizationID),
		"role":                   string(s.Role),
		"status":                 string(s.Status),
		"registered_at":          instant(s.RegisteredAt),
		"stage_changed_at":       instant(s.StageChangedAt),
		"stage_reason":           s.StageReason,
		"offboarding_started_at": instant(s.OffboardingStartedAt),
	}
}

func partnerFromState(t *testing.T, f domainkit.Fields) *domain.Partner {
	t.Helper()
	return domain.FromPartnerSnapshot(domain.PartnerSnapshot{
		ID:                   domain.PartnerID(f["partner"]),
		OrganizationID:       domain.OrganizationID(f["organization"]),
		Role:                 domain.PartnerRole(f["role"]),
		Status:               domain.PartnerStatus(f["status"]),
		RegisteredAt:         kernel.Instant(integer(t, f["registered_at"])),
		StageChangedAt:       kernel.Instant(integer(t, f["stage_changed_at"])),
		StageReason:          f["stage_reason"],
		OffboardingStartedAt: kernel.Instant(integer(t, f["offboarding_started_at"])),
	})
}

func partnerEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch event := e.(type) {
	case domain.PartnerRegistered:
		return event.EventName(), domainkit.Fields{
			"partner":      string(event.PartnerID),
			"organization": string(event.OrganizationID),
			"role":         string(event.Role),
			"at":           instant(event.At),
		}
	case domain.PartnerOffboardingStarted:
		return event.EventName(), domainkit.Fields{
			"partner":      string(event.PartnerID),
			"organization": string(event.OrganizationID),
			"reason":       event.Reason,
			"at":           instant(event.At),
		}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func runAgreement(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	before := agreementFromState(t, c.StateBefore)
	switch c.Command["upr"] {
	case "register":
		s := agreementSubject(func(a *domain.PartnerAgreement) (kernel.Accepted[domain.RegisterAgreementResponse], *kernel.Rejection) {
			return a.Register(domain.RegisterAgreement{
				PartnerID:            domain.PartnerID(c.Command["partner"]),
				ValidFrom:            kernel.Instant(integer(t, c.Command["valid_from"])),
				ValidUntil:           kernel.Instant(integer(t, c.Command["valid_until"])),
				InstrumentTemplateID: c.Command["instrument_template"],
				SupersedesID:         domain.AgreementID(c.Command["supersedes"]),
				Head:                 domain.AgreementHead{ID: domain.AgreementID(c.Command["head"]), Version: int(integer(t, c.Command["head_version"]))},
				At:                   kernel.Instant(integer(t, c.Command["at"])),
			})
		}, func(r domain.RegisterAgreementResponse) domainkit.Fields {
			return domainkit.Fields{"agreement": string(r.AgreementID)}
		})
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "activate":
		s := agreementSubject(func(a *domain.PartnerAgreement) (kernel.Accepted[domain.ActivateAgreementResponse], *kernel.Rejection) {
			return a.Activate(domain.ActivateAgreement{
				ContractEvidenceID:        c.Command["contract_evidence"],
				SatisfiedCriteria:         listFrom(c.Command["satisfied_criteria"]),
				RegulatoryCheckApplicable: boolean(t, c.Command["regulatory_check_applicable"]),
				At:                        kernel.Instant(integer(t, c.Command["at"])),
			})
		}, func(r domain.ActivateAgreementResponse) domainkit.Fields {
			return domainkit.Fields{"agreement": string(r.AgreementID)}
		})
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func agreementSubject[R any](decide func(*domain.PartnerAgreement) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.PartnerAgreement, R] {
	return domainkit.Subject[*domain.PartnerAgreement, R]{
		Decide:   decide,
		Response: response,
		Event:    agreementEvent,
		Snapshot: agreementState,
		Clone: func(a *domain.PartnerAgreement) *domain.PartnerAgreement {
			return domain.FromPartnerAgreementSnapshot(a.Snapshot())
		},
	}
}

func agreementState(a *domain.PartnerAgreement) domainkit.Fields {
	s := a.Snapshot()
	return domainkit.Fields{
		"agreement":           string(s.ID),
		"partner":             string(s.PartnerID),
		"version":             strconv.Itoa(s.Version),
		"supersedes":          string(s.SupersedesID),
		"status":              string(s.Status),
		"valid_from":          instant(s.ValidFrom),
		"valid_until":         instant(s.ValidUntil),
		"contract_evidence":   s.ContractEvidenceID,
		"instrument_template": s.InstrumentTemplateID,
		"activated_at":        instant(s.ActivatedAt),
	}
}

func agreementFromState(t *testing.T, f domainkit.Fields) *domain.PartnerAgreement {
	t.Helper()
	return domain.FromPartnerAgreementSnapshot(domain.PartnerAgreementSnapshot{
		ID:                   domain.AgreementID(f["agreement"]),
		PartnerID:            domain.PartnerID(f["partner"]),
		Version:              int(integer(t, f["version"])),
		SupersedesID:         domain.AgreementID(f["supersedes"]),
		Status:               domain.AgreementStatus(f["status"]),
		ValidFrom:            kernel.Instant(integer(t, f["valid_from"])),
		ValidUntil:           kernel.Instant(integer(t, f["valid_until"])),
		ContractEvidenceID:   f["contract_evidence"],
		InstrumentTemplateID: f["instrument_template"],
		ActivatedAt:          kernel.Instant(integer(t, f["activated_at"])),
	})
}

func agreementEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	activated, ok := e.(domain.PartnerAgreementActivated)
	if !ok {
		return e.EventName(), domainkit.Fields{}
	}
	return activated.EventName(), domainkit.Fields{
		"agreement":   string(activated.AgreementID),
		"partner":     string(activated.PartnerID),
		"version":     strconv.Itoa(activated.Version),
		"valid_from":  instant(activated.ValidFrom),
		"valid_until": instant(activated.ValidUntil),
		"at":          instant(activated.At),
	}
}

func runIntegrationProfile(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	before := profileFromState(t, c.StateBefore)
	response := func(id domain.ProfileID) domainkit.Fields { return domainkit.Fields{"profile": string(id)} }
	switch c.Command["upr"] {
	case "register":
		s := profileSubject(func(p *domain.IntegrationProfile) (kernel.Accepted[domain.RegisterIntegrationProfileResponse], *kernel.Rejection) {
			return p.Register(domain.RegisterIntegrationProfile{
				PartnerID:            domain.PartnerID(c.Command["partner"]),
				Environment:          domain.Environment(c.Command["environment"]),
				ChangeKind:           domain.IntegrationChangeKind(c.Command["change_kind"]),
				Protocols:            listFrom(c.Command["protocols"]),
				Endpoint:             c.Command["endpoint"],
				SchemaVersion:        c.Command["schema_version"],
				SecretRef:            c.Command["secret_ref"],
				CertificateRef:       c.Command["certificate_ref"],
				CertificateExpiresAt: kernel.Instant(integer(t, c.Command["certificate_expires_at"])),
				EffectiveFrom:        kernel.Instant(integer(t, c.Command["effective_from"])),
				SupersedesID:         domain.ProfileID(c.Command["supersedes"]),
				Head:                 domain.ProfileHead{ID: domain.ProfileID(c.Command["head"]), Version: int(integer(t, c.Command["head_version"]))},
				At:                   kernel.Instant(integer(t, c.Command["at"])),
			})
		}, func(r domain.RegisterIntegrationProfileResponse) domainkit.Fields { return response(r.ProfileID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "raise-expiry-alert":
		s := profileSubject(func(p *domain.IntegrationProfile) (kernel.Accepted[domain.RaiseCertificateExpiryAlertResponse], *kernel.Rejection) {
			return p.RaiseExpiryAlert(domain.RaiseCertificateExpiryAlert{
				ThresholdDays: int(integer(t, c.Command["threshold_days"])),
				At:            kernel.Instant(integer(t, c.Command["at"])),
			})
		}, func(r domain.RaiseCertificateExpiryAlertResponse) domainkit.Fields { return response(r.ProfileID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func profileSubject[R any](decide func(*domain.IntegrationProfile) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.IntegrationProfile, R] {
	return domainkit.Subject[*domain.IntegrationProfile, R]{
		Decide:   decide,
		Response: response,
		Event:    profileEvent,
		Snapshot: profileState,
		Clone: func(p *domain.IntegrationProfile) *domain.IntegrationProfile {
			return domain.FromIntegrationProfileSnapshot(p.Snapshot())
		},
	}
}

func profileState(p *domain.IntegrationProfile) domainkit.Fields {
	s := p.Snapshot()
	return domainkit.Fields{
		"profile":                     string(s.ID),
		"partner":                     string(s.PartnerID),
		"environment":                 string(s.Environment),
		"version":                     strconv.Itoa(s.Version),
		"supersedes":                  string(s.SupersedesID),
		"change_kind":                 string(s.ChangeKind),
		"protocols":                   strings.Join(s.Protocols, ","),
		"endpoint":                    s.Endpoint,
		"schema_version":              s.SchemaVersion,
		"secret_ref":                  s.SecretRef,
		"certificate_ref":             s.CertificateRef,
		"certificate_expires_at":      instant(s.CertificateExpiresAt),
		"effective_from":              instant(s.EffectiveFrom),
		"last_alerted_threshold_days": strconv.Itoa(s.LastAlertedThresholdDays),
	}
}

func profileFromState(t *testing.T, f domainkit.Fields) *domain.IntegrationProfile {
	t.Helper()
	return domain.FromIntegrationProfileSnapshot(domain.IntegrationProfileSnapshot{
		ID:                       domain.ProfileID(f["profile"]),
		PartnerID:                domain.PartnerID(f["partner"]),
		Environment:              domain.Environment(f["environment"]),
		Version:                  int(integer(t, f["version"])),
		SupersedesID:             domain.ProfileID(f["supersedes"]),
		ChangeKind:               domain.IntegrationChangeKind(f["change_kind"]),
		Protocols:                listFrom(f["protocols"]),
		Endpoint:                 f["endpoint"],
		SchemaVersion:            f["schema_version"],
		SecretRef:                f["secret_ref"],
		CertificateRef:           f["certificate_ref"],
		CertificateExpiresAt:     kernel.Instant(integer(t, f["certificate_expires_at"])),
		EffectiveFrom:            kernel.Instant(integer(t, f["effective_from"])),
		LastAlertedThresholdDays: int(integer(t, f["last_alerted_threshold_days"])),
	})
}

func profileEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	expiring, ok := e.(domain.PartnerCertificateExpiring)
	if !ok {
		return e.EventName(), domainkit.Fields{}
	}
	return expiring.EventName(), domainkit.Fields{
		"profile":                string(expiring.ProfileID),
		"partner":                string(expiring.PartnerID),
		"environment":            string(expiring.Environment),
		"certificate_expires_at": instant(expiring.CertificateExpiresAt),
		"threshold_days":         strconv.Itoa(expiring.ThresholdDays),
		"at":                     instant(expiring.At),
	}
}

func runOperationalPolicy(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	before := policyFromState(t, c.StateBefore)
	if c.Command["upr"] != "define" {
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
	}
	s := domainkit.Subject[*domain.OperationalPolicy, domain.DefineOperationalPolicyResponse]{
		Decide: func(p *domain.OperationalPolicy) (kernel.Accepted[domain.DefineOperationalPolicyResponse], *kernel.Rejection) {
			return p.Define(domain.DefineOperationalPolicy{
				PartnerID:          domain.PartnerID(c.Command["partner"]),
				EffectiveFrom:      kernel.Instant(integer(t, c.Command["effective_from"])),
				SupersedesID:       domain.PolicyID(c.Command["supersedes"]),
				Head:               domain.PolicyHead{ID: domain.PolicyID(c.Command["head"]), Version: int(integer(t, c.Command["head_version"]))},
				RateLimitPerMinute: int(integer(t, c.Command["rate_limit_per_minute"])),
				BusinessDays:       listFrom(c.Command["business_days"]),
				CutoffTime:         c.Command["cutoff_time"],
				ProcessingWindow:   c.Command["processing_window"],
				SlaRef:             c.Command["sla"],
				Fallback:           c.Command["fallback"],
				SupportContact:     c.Command["support_contact"],
				IncidentContact:    c.Command["incident_contact"],
				At:                 kernel.Instant(integer(t, c.Command["at"])),
			})
		},
		Response: func(r domain.DefineOperationalPolicyResponse) domainkit.Fields {
			return domainkit.Fields{"policy": string(r.PolicyID)}
		},
		Event:    func(e kernel.DomainEvent) (string, domainkit.Fields) { return e.EventName(), domainkit.Fields{} },
		Snapshot: policyState,
		Clone: func(p *domain.OperationalPolicy) *domain.OperationalPolicy {
			return domain.FromOperationalPolicySnapshot(p.Snapshot())
		},
	}
	return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
}

func policyState(p *domain.OperationalPolicy) domainkit.Fields {
	s := p.Snapshot()
	return domainkit.Fields{
		"policy":                string(s.ID),
		"partner":               string(s.PartnerID),
		"version":               strconv.Itoa(s.Version),
		"supersedes":            string(s.SupersedesID),
		"effective_from":        instant(s.EffectiveFrom),
		"rate_limit_per_minute": strconv.Itoa(s.RateLimitPerMinute),
		"business_days":         strings.Join(s.BusinessDays, ","),
		"cutoff_time":           s.CutoffTime,
		"processing_window":     s.ProcessingWindow,
		"sla":                   s.SlaRef,
		"fallback":              s.Fallback,
		"support_contact":       s.SupportContact,
		"incident_contact":      s.IncidentContact,
	}
}

func policyFromState(t *testing.T, f domainkit.Fields) *domain.OperationalPolicy {
	t.Helper()
	return domain.FromOperationalPolicySnapshot(domain.OperationalPolicySnapshot{
		ID:                 domain.PolicyID(f["policy"]),
		PartnerID:          domain.PartnerID(f["partner"]),
		Version:            int(integer(t, f["version"])),
		SupersedesID:       domain.PolicyID(f["supersedes"]),
		EffectiveFrom:      kernel.Instant(integer(t, f["effective_from"])),
		RateLimitPerMinute: int(integer(t, f["rate_limit_per_minute"])),
		BusinessDays:       listFrom(f["business_days"]),
		CutoffTime:         f["cutoff_time"],
		ProcessingWindow:   f["processing_window"],
		SlaRef:             f["sla"],
		Fallback:           f["fallback"],
		SupportContact:     f["support_contact"],
		IncidentContact:    f["incident_contact"],
	})
}

func instant(at kernel.Instant) string { return strconv.FormatInt(int64(at), 10) }

func integer(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("integer %q: %v", s, err)
	}
	return n
}

func boolean(t *testing.T, s string) bool {
	t.Helper()
	b, err := strconv.ParseBool(s)
	if err != nil {
		t.Fatalf("boolean %q: %v", s, err)
	}
	return b
}

func listFrom(listed string) []string {
	if listed == "" {
		return nil
	}
	return strings.Split(listed, ",")
}

func kindsFrom(listed string) []domain.OrganizationKind {
	var kinds []domain.OrganizationKind
	for _, kind := range listFrom(listed) {
		kinds = append(kinds, domain.OrganizationKind(kind))
	}
	return kinds
}
