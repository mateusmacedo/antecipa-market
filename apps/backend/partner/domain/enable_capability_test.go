package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const enabledAt = at + 3*day

func enableCapability() domain.EnableCapability {
	return domain.EnableCapability{HasEffectiveAgreement: true, HasEffectiveProfile: true, At: enabledAt}
}

func TestEnableCapabilityEnablesAHomologatedCapabilityAndAnnouncesIt(t *testing.T) {
	s := capabilityAt(domain.CapabilityStatusHomologated)
	s.StateQuerySupported, s.RestrictionNote = false, "reconciled by the daily file"
	c := domain.FromCapabilitySnapshot(s)

	acc, rej := c.Enable(enableCapability())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.EnableCapabilityResponse{CapabilityID: capabilityID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	want := domain.CapabilityEnabled{
		CapabilityID:        capabilityID,
		PartnerID:           partnerID,
		Operation:           "register-receivable",
		AssetType:           "duplicata",
		Environment:         domain.EnvironmentProduction,
		Version:             1,
		RolloutMode:         domain.RolloutDisabled,
		StateQuerySupported: false,
		RestrictionNote:     "reconciled by the daily file",
		At:                  enabledAt,
	}
	if events := acc.Events(); len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want [%+v]", events, want)
	}
	after := s
	after.Status, after.EnabledAt = domain.CapabilityStatusEnabled, enabledAt
	if got := c.Snapshot(); !got.Equal(after) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, after)
	}
}

func TestEnableCapabilityResumesASuspensionWithoutANewHomologation(t *testing.T) {
	for _, trigger := range []domain.SuspensionTrigger{domain.TriggerManual, domain.TriggerOffboarding} {
		t.Run(string(trigger), func(t *testing.T) {
			s := suspendedCapability(trigger)
			s.Homologation = recorded(at, withCriteria("H3")...)
			c := domain.FromCapabilitySnapshot(s)

			acc, rej := c.Enable(enableCapability())

			tb.RequireAccepted(t, rej)
			if len(acc.Events()) != 1 {
				t.Fatalf("Events() = %+v, want the CapabilityStatusEnabled", acc.Events())
			}
			got := c.Snapshot()
			if got.Status != domain.CapabilityStatusEnabled || got.EnabledAt != enabledAt || got.Suspension != (domain.Suspension{}) || len(got.Homologation) != len(s.Homologation) {
				t.Fatalf("Snapshot() = %+v, want enabled at %d, no suspension and the homologation kept", got, enabledAt)
			}
		})
	}
}

func TestEnableCapabilityRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name   string
		target domain.CapabilitySnapshot
		mutate func(*domain.EnableCapability)
		code   kernel.Code
	}{
		{"capability declared", capabilityAt(domain.CapabilityStatusDeclared), func(*domain.EnableCapability) {},
			"partner-management/capability/not-homologated"},
		{"capability already enabled", capabilityAt(domain.CapabilityStatusEnabled), func(*domain.EnableCapability) {},
			"partner-management/capability/not-homologated"},
		{"suspended by a schema mismatch", suspendedCapability(domain.TriggerSchemaMismatch), func(*domain.EnableCapability) {},
			"partner-management/capability/rehomologation-required"},
		{"without an agreement in effect", capabilityAt(domain.CapabilityStatusHomologated), func(c *domain.EnableCapability) { c.HasEffectiveAgreement = false },
			"partner-management/capability/agreement-not-active"},
		{"without a profile of the environment", capabilityAt(domain.CapabilityStatusHomologated), func(c *domain.EnableCapability) { c.HasEffectiveProfile = false },
			"partner-management/capability/integration-profile-missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := enableCapability()
			tt.mutate(&cmd)
			c := domain.FromCapabilitySnapshot(tt.target)
			before := c.Snapshot()

			acc, rej := c.Enable(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			requireCapabilityUnchanged(t, before, c.Snapshot())
		})
	}
}

func TestEnableCapabilityReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.EnableCapability{At: enabledAt}
	for _, step := range []struct {
		target domain.CapabilitySnapshot
		code   kernel.Code
	}{
		{capabilityAt(domain.CapabilityStatusDeclared), "partner-management/capability/not-homologated"},
		{suspendedCapability(domain.TriggerSchemaMismatch), "partner-management/capability/rehomologation-required"},
		{suspendedCapability(domain.TriggerManual), "partner-management/capability/agreement-not-active"},
	} {
		acc, rej := domain.FromCapabilitySnapshot(step.target).Enable(cmd)
		tb.RequireRejected(t, acc, rej, step.code)
	}
	cmd.HasEffectiveAgreement = true
	acc, rej := capability(domain.CapabilityStatusHomologated).Enable(cmd)
	tb.RequireRejected(t, acc, rej, "partner-management/capability/integration-profile-missing")
}
