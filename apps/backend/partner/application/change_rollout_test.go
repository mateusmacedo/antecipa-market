package application_test

import (
	"context"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func changeRollout() application.ChangeRollout {
	return application.ChangeRollout{
		CapabilityID: testCapabilityID, Mode: domain.RolloutPercentage, Percentage: 10, ExcludedTenantIDs: []string{"tenant-x"},
	}
}

func TestChangeRolloutWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))

	if _, err := h.service.ChangeRollout(ctx, changeRollout()); err != nil {
		t.Fatalf("ChangeRollout() error = %v, want nil", err)
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

func TestChangeRolloutReplacesTheRolloutAndEnqueuesTheChange(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	held := capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, "")
	held.Rollout = domain.Rollout{Mode: domain.RolloutEnabled}
	seedCapability(t, h, ctx, held)

	outcome, err := h.service.ChangeRollout(ctx, changeRollout())

	if err != nil {
		t.Fatalf("ChangeRollout() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.ChangeRolloutResponse{CapabilityID: testCapabilityID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.capRead.Load(ctx, testCapabilityID)
	want := domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 10, ExcludedTenantIDs: []string{"tenant-x"}}
	if err != nil || version != 2 || !slices.Equal(snap.Rollout.ExcludedTenantIDs, want.ExcludedTenantIDs) || snap.Rollout.Mode != want.Mode || snap.Rollout.Percentage != 10 {
		t.Fatalf("Load() = %+v v%d, %v; want the rollout %+v at version 2", snap, version, err, want)
	}
	event := domain.CapabilityRolloutChanged{
		CapabilityID: testCapabilityID, PartnerID: testPartnerID, Operation: "register-receivable", AssetType: "duplicata",
		Environment: domain.EnvironmentProduction, Version: 1, Mode: domain.RolloutPercentage, Percentage: 10, At: kernel.Instant(testOccurred),
	}
	if entries := h.store.Entries(); len(entries) != 1 || entries[0].Event != event {
		t.Fatalf("outbox = %+v, want [%+v]", entries, event)
	}
}

func TestChangeRolloutRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*application.ChangeRollout)
		code kernel.Code
	}{
		{"percentage 101", func(c *application.ChangeRollout) { c.Percentage = 101 }, domain.CodeCapabilityPercentageOutOfRange},
		{"allowlist empty", func(c *application.ChangeRollout) { c.Mode = domain.RolloutAllowlist }, domain.CodeCapabilityAllowlistEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			held := capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, "")
			seedCapability(t, h, ctx, held)
			cmd := changeRollout()
			tc.edit(&cmd)

			outcome, err := h.service.ChangeRollout(ctx, cmd)

			if err != nil {
				t.Fatalf("ChangeRollout() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if snap, version, _ := h.capRead.Load(ctx, testCapabilityID); version != 1 || !snap.Equal(held) || h.outboxLen() != 0 {
				t.Fatalf("Load() = %+v v%d with %d outbox entries, want the capability untouched and nothing emitted", snap, version, h.outboxLen())
			}
		})
	}
}
