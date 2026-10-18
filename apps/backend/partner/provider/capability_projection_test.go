package provider_test

import (
	"strconv"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestCapabilitiesMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "capability.golden")
	if f.Identity.Context != "partner" || f.Identity.Aggregate != "capability" || len(f.Cases) != 25 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runCapability)
}

func runCapability(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	before := capabilityFromState(t, c.StateBefore)
	at := kernel.Instant(integer(t, c.Command["at"]))
	response := func(id domain.CapabilityID) domainkit.Fields { return domainkit.Fields{"capability": string(id)} }
	switch c.Command["upr"] {
	case "declare":
		s := capabilitySubject(func(cap *domain.Capability) (kernel.Accepted[domain.DeclareCapabilityResponse], *kernel.Rejection) {
			return cap.Declare(domain.DeclareCapability{
				PartnerID:           domain.PartnerID(c.Command["partner"]),
				Operation:           c.Command["operation"],
				AssetType:           c.Command["asset_type"],
				Environment:         domain.Environment(c.Command["environment"]),
				IsWrite:             boolean(t, c.Command["is_write"]),
				StateQuerySupported: boolean(t, c.Command["state_query_supported"]),
				RestrictionNote:     c.Command["restriction_note"],
				CallbackApplicable:  boolean(t, c.Command["callback_applicable"]),
				SupersedesID:        domain.CapabilityID(c.Command["supersedes"]),
				Head:                domain.CapabilityHead{ID: domain.CapabilityID(c.Command["head"]), Version: int(integer(t, c.Command["head_version"]))},
				At:                  at,
			})
		}, func(r domain.DeclareCapabilityResponse) domainkit.Fields { return response(r.CapabilityID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "record-homologation":
		s := capabilitySubject(func(cap *domain.Capability) (kernel.Accepted[domain.RecordHomologationResponse], *kernel.Rejection) {
			return cap.RecordHomologation(domain.RecordHomologation{Results: homologationResultsFrom(t, c.Command["results"]), At: at})
		}, func(r domain.RecordHomologationResponse) domainkit.Fields {
			return domainkit.Fields{"capability": string(r.CapabilityID), "status": string(r.Status)}
		})
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "enable":
		s := capabilitySubject(func(cap *domain.Capability) (kernel.Accepted[domain.EnableCapabilityResponse], *kernel.Rejection) {
			return cap.Enable(domain.EnableCapability{
				HasEffectiveAgreement: boolean(t, c.Command["has_effective_agreement"]),
				HasEffectiveProfile:   boolean(t, c.Command["has_effective_profile"]),
				At:                    at,
			})
		}, func(r domain.EnableCapabilityResponse) domainkit.Fields { return response(r.CapabilityID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "suspend":
		s := capabilitySubject(func(cap *domain.Capability) (kernel.Accepted[domain.SuspendCapabilityResponse], *kernel.Rejection) {
			return cap.Suspend(domain.SuspendCapability{Reason: c.Command["reason"], Trigger: domain.SuspensionTrigger(c.Command["trigger"]), At: at})
		}, func(r domain.SuspendCapabilityResponse) domainkit.Fields { return response(r.CapabilityID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	case "change-rollout":
		s := capabilitySubject(func(cap *domain.Capability) (kernel.Accepted[domain.ChangeRolloutResponse], *kernel.Rejection) {
			return cap.ChangeRollout(domain.ChangeRollout{
				Mode:               domain.RolloutMode(c.Command["mode"]),
				AllowlistTenantIDs: listFrom(c.Command["allowlist"]),
				Percentage:         int(integer(t, c.Command["percentage"])),
				ExcludedTenantIDs:  listFrom(c.Command["excluded"]),
				At:                 at,
			})
		}, func(r domain.ChangeRolloutResponse) domainkit.Fields { return response(r.CapabilityID) })
		return domainkit.Run(before, s), domainkit.ReadTwice(before, s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func capabilitySubject[R any](decide func(*domain.Capability) (kernel.Accepted[R], *kernel.Rejection), response func(R) domainkit.Fields) domainkit.Subject[*domain.Capability, R] {
	return domainkit.Subject[*domain.Capability, R]{
		Decide:   decide,
		Response: response,
		Event:    capabilityEvent,
		Snapshot: capabilityFields,
		Clone:    func(c *domain.Capability) *domain.Capability { return domain.FromCapabilitySnapshot(c.Snapshot()) },
	}
}

func capabilityFields(c *domain.Capability) domainkit.Fields {
	s := c.Snapshot()
	homologation := make([]string, 0, len(s.Homologation))
	for _, r := range s.Homologation {
		homologation = append(homologation, strings.Join([]string{r.CriterionID, strconv.FormatBool(r.Passed), r.EvidenceID, instant(r.RecordedAt)}, ":"))
	}
	return domainkit.Fields{
		"capability":            string(s.ID),
		"partner":               string(s.PartnerID),
		"operation":             s.Operation,
		"asset_type":            s.AssetType,
		"environment":           string(s.Environment),
		"version":               strconv.Itoa(s.Version),
		"supersedes":            string(s.SupersedesID),
		"is_write":              strconv.FormatBool(s.IsWrite),
		"state_query_supported": strconv.FormatBool(s.StateQuerySupported),
		"restriction_note":      s.RestrictionNote,
		"callback_applicable":   strconv.FormatBool(s.CallbackApplicable),
		"status":                string(s.Status),
		"homologation":          strings.Join(homologation, ","),
		"rollout_mode":          string(s.Rollout.Mode),
		"rollout_allowlist":     strings.Join(s.Rollout.AllowlistTenantIDs, ","),
		"rollout_percentage":    strconv.Itoa(s.Rollout.Percentage),
		"rollout_excluded":      strings.Join(s.Rollout.ExcludedTenantIDs, ","),
		"suspension_reason":     s.Suspension.Reason,
		"suspension_trigger":    string(s.Suspension.Trigger),
		"suspended_at":          instant(s.Suspension.SuspendedAt),
		"enabled_at":            instant(s.EnabledAt),
	}
}

func capabilityFromState(t *testing.T, f domainkit.Fields) *domain.Capability {
	t.Helper()
	var homologation []domain.CriterionResult
	for _, entry := range listFrom(f["homologation"]) {
		parts := strings.Split(entry, ":")
		if len(parts) != 4 {
			t.Fatalf("homologation entry %q is not criterion:passed:evidence:recordedAt", entry)
		}
		homologation = append(homologation, domain.CriterionResult{
			CriterionID: parts[0], Passed: boolean(t, parts[1]), EvidenceID: parts[2], RecordedAt: kernel.Instant(integer(t, parts[3])),
		})
	}
	return domain.FromCapabilitySnapshot(domain.CapabilitySnapshot{
		ID:                  domain.CapabilityID(f["capability"]),
		PartnerID:           domain.PartnerID(f["partner"]),
		Operation:           f["operation"],
		AssetType:           f["asset_type"],
		Environment:         domain.Environment(f["environment"]),
		Version:             int(integer(t, f["version"])),
		SupersedesID:        domain.CapabilityID(f["supersedes"]),
		IsWrite:             boolean(t, f["is_write"]),
		StateQuerySupported: boolean(t, f["state_query_supported"]),
		RestrictionNote:     f["restriction_note"],
		CallbackApplicable:  boolean(t, f["callback_applicable"]),
		Status:              domain.CapabilityStatus(f["status"]),
		Homologation:        homologation,
		Rollout: domain.Rollout{
			Mode:               domain.RolloutMode(f["rollout_mode"]),
			AllowlistTenantIDs: listFrom(f["rollout_allowlist"]),
			Percentage:         int(integer(t, f["rollout_percentage"])),
			ExcludedTenantIDs:  listFrom(f["rollout_excluded"]),
		},
		Suspension: domain.Suspension{
			Reason:      f["suspension_reason"],
			Trigger:     domain.SuspensionTrigger(f["suspension_trigger"]),
			SuspendedAt: kernel.Instant(integer(t, f["suspended_at"])),
		},
		EnabledAt: kernel.Instant(integer(t, f["enabled_at"])),
	})
}

func homologationResultsFrom(t *testing.T, listed string) []domain.HomologationResult {
	t.Helper()
	var results []domain.HomologationResult
	for _, entry := range listFrom(listed) {
		parts := strings.Split(entry, ":")
		if len(parts) != 3 {
			t.Fatalf("result %q is not criterion:passed:evidence", entry)
		}
		results = append(results, domain.HomologationResult{CriterionID: parts[0], Passed: boolean(t, parts[1]), EvidenceID: parts[2]})
	}
	return results
}

func capabilityEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch event := e.(type) {
	case domain.CapabilityEnabled:
		return event.EventName(), domainkit.Fields{
			"capability":            string(event.CapabilityID),
			"partner":               string(event.PartnerID),
			"operation":             event.Operation,
			"asset_type":            event.AssetType,
			"environment":           string(event.Environment),
			"version":               strconv.Itoa(event.Version),
			"rollout_mode":          string(event.RolloutMode),
			"state_query_supported": strconv.FormatBool(event.StateQuerySupported),
			"restriction_note":      event.RestrictionNote,
			"at":                    instant(event.At),
		}
	case domain.CapabilitySuspended:
		return event.EventName(), domainkit.Fields{
			"capability":  string(event.CapabilityID),
			"partner":     string(event.PartnerID),
			"operation":   event.Operation,
			"asset_type":  event.AssetType,
			"environment": string(event.Environment),
			"version":     strconv.Itoa(event.Version),
			"reason":      event.Reason,
			"trigger":     string(event.Trigger),
			"at":          instant(event.At),
		}
	case domain.CapabilityRolloutChanged:
		return event.EventName(), domainkit.Fields{
			"capability":  string(event.CapabilityID),
			"partner":     string(event.PartnerID),
			"operation":   event.Operation,
			"asset_type":  event.AssetType,
			"environment": string(event.Environment),
			"version":     strconv.Itoa(event.Version),
			"mode":        string(event.Mode),
			"percentage":  strconv.Itoa(event.Percentage),
			"at":          instant(event.At),
		}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}
