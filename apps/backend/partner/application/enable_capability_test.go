package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func enableCapability() application.EnableCapability {
	return application.EnableCapability{CapabilityID: testCapabilityID}
}

func holdAgreementAndProfile(h *harness) {
	h.agreements.hold(testPartnerID, activeAgreement(testAgreementID, kernel.Instant(testOccurred)+365*day))
	h.profiles.hold(testPartnerID, profileVersion(testProfileID, domain.EnvironmentProduction, 1, "", 90*day))
}

func TestEnableCapabilityWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	holdAgreementAndProfile(h)
	ctx := withExecution(t, context.Background())
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""))

	if _, err := h.service.EnableCapability(ctx, enableCapability()); err != nil {
		t.Fatalf("EnableCapability() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"capabilities.Load",
		"partner_agreements.LoadByPartner",
		"integration_profiles.LoadByPartner",
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

func TestEnableCapabilityEnablesAndEnqueuesTheCapabilityEnabled(t *testing.T) {
	h := newHarness(t)
	holdAgreementAndProfile(h)
	ctx := withExecution(t, context.Background())
	held := capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, "")
	held.StateQuerySupported, held.RestrictionNote = false, "reconciled by the daily file"
	seedCapability(t, h, ctx, held)

	outcome, err := h.service.EnableCapability(ctx, enableCapability())

	if err != nil {
		t.Fatalf("EnableCapability() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.EnableCapabilityResponse{CapabilityID: testCapabilityID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.capRead.Load(ctx, testCapabilityID)
	if err != nil || version != 2 || snap.Status != domain.CapabilityStatusEnabled || snap.EnabledAt != kernel.Instant(testOccurred) {
		t.Fatalf("Load() = %+v v%d, %v; want enabled at version 2", snap, version, err)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeCapability || entry.AggregateID != string(testCapabilityID) ||
		entry.AggregateVersion != 2 || entry.Intent.PartitionKey != string(testCapabilityID) {
		t.Fatalf("outbox entry = %+v, want the enabling of the capability at version 2 keyed by it", entry)
	}
	event := domain.CapabilityEnabled{
		CapabilityID: testCapabilityID, PartnerID: testPartnerID, Operation: "register-receivable", AssetType: "duplicata",
		Environment: domain.EnvironmentProduction, Version: 1, RolloutMode: domain.RolloutDisabled,
		RestrictionNote: "reconciled by the daily file", At: kernel.Instant(testOccurred),
	}
	if got, ok := entry.Event.(domain.CapabilityEnabled); !ok || got != event {
		t.Fatalf("event = %+v, want %+v", entry.Event, event)
	}
	if want := []domain.PartnerID{testPartnerID}; !slices.Equal(h.agreements.asked, want) || !slices.Equal(h.profiles.asked, want) {
		t.Fatalf("the reads asked for %v and %v, want the partner of the capability", h.agreements.asked, h.profiles.asked)
	}
}

func TestEnableCapabilityResumesAManualSuspensionWithoutANewHomologation(t *testing.T) {
	h := newHarness(t)
	holdAgreementAndProfile(h)
	ctx := withExecution(t, context.Background())
	seedCapability(t, h, ctx, suspendedBy(domain.TriggerManual))

	outcome, err := h.service.EnableCapability(ctx, enableCapability())

	if err != nil {
		t.Fatalf("EnableCapability() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("EnableCapability() rejected with %s, want the resumption", rej.Code())
	}
	if snap, _, _ := h.capRead.Load(ctx, testCapabilityID); snap.Status != domain.CapabilityStatusEnabled || snap.Suspension != (domain.Suspension{}) || h.outboxLen() != 1 {
		t.Fatalf("Load() = %+v with %d outbox entries, want enabled without suspension and one CapabilityEnabled", snap, h.outboxLen())
	}
}

func TestEnableCapabilityRejectsEachPreconditionWithoutWriting(t *testing.T) {
	expired := kernel.Instant(testOccurred)
	for _, tc := range []struct {
		name    string
		held    domain.CapabilitySnapshot
		prepare func(*harness)
		code    kernel.Code
	}{
		{"capability declared", capabilityVersion(testCapabilityID, domain.CapabilityStatusDeclared, 1, ""), holdAgreementAndProfile,
			domain.CodeCapabilityNotHomologated},
		{"suspended by a schema mismatch", suspendedBy(domain.TriggerSchemaMismatch), holdAgreementAndProfile,
			domain.CodeCapabilityRehomologationRequired},
		{"without any agreement", capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""), func(h *harness) {
			h.profiles.hold(testPartnerID, profileVersion(testProfileID, domain.EnvironmentProduction, 1, "", 90*day))
		}, domain.CodeCapabilityAgreementNotActive},
		{"with a draft agreement", capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""), func(h *harness) {
			h.agreements.hold(testPartnerID, draftAgreement(testAgreementID, 1, ""))
			h.profiles.hold(testPartnerID, profileVersion(testProfileID, domain.EnvironmentProduction, 1, "", 90*day))
		}, domain.CodeCapabilityAgreementNotActive},
		{"with an expired agreement", capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""), func(h *harness) {
			h.agreements.hold(testPartnerID, activeAgreement(testAgreementID, expired))
			h.profiles.hold(testPartnerID, profileVersion(testProfileID, domain.EnvironmentProduction, 1, "", 90*day))
		}, domain.CodeCapabilityAgreementNotActive},
		{"with a profile of another environment", capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""), func(h *harness) {
			h.agreements.hold(testPartnerID, activeAgreement(testAgreementID, 0))
			h.profiles.hold(testPartnerID, profileVersion(testProfileID, domain.EnvironmentHomologation, 1, "", 90*day))
		}, domain.CodeCapabilityIntegrationProfileMissing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.prepare(h)
			ctx := withExecution(t, context.Background())
			seedCapability(t, h, ctx, tc.held)

			outcome, err := h.service.EnableCapability(ctx, enableCapability())

			if err != nil {
				t.Fatalf("EnableCapability() error = %v, want a rejection", err)
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

func TestEnablingAnUnknownCapabilityIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.EnableCapability(withExecution(t, context.Background()), enableCapability())

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("EnableCapability() = %v, want ErrNotFound", err)
	}
	if len(h.agreements.asked) != 0 || len(h.profiles.asked) != 0 {
		t.Fatalf("the reads asked for %v and %v, want none for a capability that does not exist", h.agreements.asked, h.profiles.asked)
	}
}
