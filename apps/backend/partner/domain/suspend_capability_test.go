package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const suspendedAt = at + 4*day

func suspendCapability(reason string) domain.SuspendCapability {
	return domain.SuspendCapability{Reason: reason, Trigger: domain.TriggerManual, At: suspendedAt}
}

func TestSuspendCapabilitySuspendsAnEnabledCapabilityByHandAndAnnouncesIt(t *testing.T) {
	s := capabilityAt(domain.CapabilityStatusEnabled)
	s.Version, s.SupersedesID, s.EnabledAt = 2, "cap-1", at
	c := domain.FromCapabilitySnapshot(s)

	acc, rej := c.Suspend(suspendCapability("partner incident"))

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.SuspendCapabilityResponse{CapabilityID: capabilityID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	want := domain.CapabilitySuspended{
		CapabilityID: capabilityID,
		PartnerID:    partnerID,
		Operation:    "register-receivable",
		AssetType:    "duplicata",
		Environment:  domain.EnvironmentProduction,
		Version:      2,
		Reason:       "partner incident",
		Trigger:      domain.TriggerManual,
		At:           suspendedAt,
	}
	if events := acc.Events(); len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want [%+v]", events, want)
	}
	after := s
	after.Status = domain.CapabilityStatusSuspended
	after.Suspension = domain.Suspension{Reason: "partner incident", Trigger: domain.TriggerManual, SuspendedAt: suspendedAt}
	if got := c.Snapshot(); !got.Equal(after) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, after)
	}
}

func TestSuspendCapabilityKeepsTheTriggerOfAnInternalFlow(t *testing.T) {
	cmd := suspendCapability("schema v4 rejected")
	cmd.Trigger = domain.TriggerSchemaMismatch
	c := capability(domain.CapabilityStatusEnabled)

	_, rej := c.Suspend(cmd)

	tb.RequireAccepted(t, rej)
	if got := c.Snapshot().Suspension.Trigger; got != domain.TriggerSchemaMismatch {
		t.Fatalf("Trigger = %s, want schema_mismatch", got)
	}
}

func TestSuspendCapabilityRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name   string
		target domain.CapabilitySnapshot
		reason string
		code   kernel.Code
	}{
		{"capability declared", capabilityAt(domain.CapabilityStatusDeclared), "partner incident", "partner-management/capability/not-enabled"},
		{"capability homologated", capabilityAt(domain.CapabilityStatusHomologated), "partner incident", "partner-management/capability/not-enabled"},
		{"capability already suspended", suspendedCapability(domain.TriggerManual), "partner incident", "partner-management/capability/not-enabled"},
		{"reason empty", capabilityAt(domain.CapabilityStatusEnabled), "", "partner-management/capability/reason-empty"},
		{"reason blank", capabilityAt(domain.CapabilityStatusEnabled), "   ", "partner-management/capability/reason-empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := domain.FromCapabilitySnapshot(tt.target)
			before := c.Snapshot()

			acc, rej := c.Suspend(suspendCapability(tt.reason))

			tb.RequireRejected(t, acc, rej, tt.code)
			requireCapabilityUnchanged(t, before, c.Snapshot())
		})
	}
}

func TestSuspendCapabilityReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	acc, rej := capability(domain.CapabilityStatusDeclared).Suspend(suspendCapability(""))
	tb.RequireRejected(t, acc, rej, "partner-management/capability/not-enabled")

	acc, rej = capability(domain.CapabilityStatusEnabled).Suspend(suspendCapability(""))
	tb.RequireRejected(t, acc, rej, "partner-management/capability/reason-empty")
}
