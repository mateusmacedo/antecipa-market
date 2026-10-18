package application_test

import (
	"context"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func suspendCapability(reason string) application.SuspendCapability {
	return application.SuspendCapability{CapabilityID: testCapabilityID, Reason: reason, Trigger: domain.TriggerManual}
}

func TestSuspendCapabilityWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))

	if _, err := h.service.SuspendCapability(ctx, suspendCapability("partner incident")); err != nil {
		t.Fatalf("SuspendCapability() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"capabilities.Load",
		"capabilities.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestSuspendCapabilitySuspendsByHandAndEnqueuesTheEnvironmentAndVersion(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 2, "cap-1"))

	outcome, err := h.service.SuspendCapability(ctx, suspendCapability("partner incident"))

	if err != nil {
		t.Fatalf("SuspendCapability() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.SuspendCapabilityResponse{CapabilityID: testCapabilityID}) {
		t.Fatalf("Response() = %+v", got)
	}
	want := domain.Suspension{Reason: "partner incident", Trigger: domain.TriggerManual, SuspendedAt: kernel.Instant(testOccurred)}
	if snap, version, err := h.capRead.Load(ctx, testCapabilityID); err != nil || version != 2 || snap.Status != domain.CapabilityStatusSuspended || snap.Suspension != want {
		t.Fatalf("Load() = %+v v%d, %v; want suspended by hand at version 2", snap, version, err)
	}
	entries := h.store.Entries()
	event := domain.CapabilitySuspended{
		CapabilityID: testCapabilityID, PartnerID: testPartnerID, Operation: "register-receivable", AssetType: "duplicata",
		Environment: domain.EnvironmentProduction, Version: 2, Reason: "partner incident", Trigger: domain.TriggerManual, At: kernel.Instant(testOccurred),
	}
	if len(entries) != 1 || entries[0].Event != event || entries[0].Intent.PartitionKey != string(testCapabilityID) {
		t.Fatalf("outbox = %+v, want [%+v] keyed by the capability", entries, event)
	}
}

func TestSuspendCapabilityRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   domain.CapabilitySnapshot
		reason string
		code   kernel.Code
	}{
		{"capability declared", capabilityVersion(testCapabilityID, domain.CapabilityStatusDeclared, 1, ""), "partner incident", domain.CodeCapabilityNotEnabled},
		{"reason empty", capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""), "", domain.CodeCapabilityReasonEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			seedCapability(t, h, ctx, tc.held)

			outcome, err := h.service.SuspendCapability(ctx, suspendCapability(tc.reason))

			if err != nil {
				t.Fatalf("SuspendCapability() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if snap, version, _ := h.capRead.Load(ctx, testCapabilityID); version != 1 || !snap.Equal(tc.held) || h.outboxLen() != 0 {
				t.Fatalf("Load() = %+v v%d with %d outbox entries, want the capability untouched and nothing emitted", snap, version, h.outboxLen())
			}
		})
	}
}
