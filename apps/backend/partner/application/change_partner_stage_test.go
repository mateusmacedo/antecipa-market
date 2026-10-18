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

func changeStage(target domain.PartnerStatus, reason string) application.ChangePartnerStage {
	return application.ChangePartnerStage{PartnerID: testPartnerID, Target: target, Reason: reason}
}

func seedPartner(t *testing.T, h *harness, ctx context.Context, status domain.PartnerStatus) {
	t.Helper()
	seed(t, h, ctx, partnerTable, testPartnerID, partnerAt(status))
}

func holdEffectiveAgreement(h *harness) {
	h.agreements.hold(testPartnerID, activeAgreement(testAgreementID, 0))
}

func TestChangePartnerStageToActiveWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	holdEffectiveAgreement(h)
	h.caps.hold(testPartnerID, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))
	ctx := withExecution(t, context.Background())
	seedPartner(t, h, ctx, domain.PartnerHomologating)

	if _, err := h.service.ChangePartnerStage(ctx, changeStage(domain.PartnerActive, "")); err != nil {
		t.Fatalf("ChangePartnerStage() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"partners.Load",
		"partner_agreements.LoadByPartner",
		"capabilities.LoadByPartner",
		"partners.Load",
		"partners.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestChangePartnerStageActivatesAHomologatingPartnerWithoutEmitting(t *testing.T) {
	h := newHarness(t)
	holdEffectiveAgreement(h)
	enabledCheque := capabilityVersion("cap-2", domain.CapabilityStatusEnabled, 1, "")
	enabledCheque.AssetType = "cheque"
	h.caps.hold(testPartnerID, capabilityVersion("cap-1", domain.CapabilityStatusSuspended, 1, ""), enabledCheque)
	ctx := withExecution(t, context.Background())
	seedPartner(t, h, ctx, domain.PartnerHomologating)

	outcome, err := h.service.ChangePartnerStage(ctx, changeStage(domain.PartnerActive, ""))

	if err != nil {
		t.Fatalf("ChangePartnerStage() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.ChangePartnerStageResponse{PartnerID: testPartnerID, Status: domain.PartnerActive}) {
		t.Fatalf("Response() = %+v, want active", got)
	}
	if snap, version, err := h.partnerRead.Load(ctx, testPartnerID); err != nil || version != 2 || snap.Status != domain.PartnerActive || snap.StageChangedAt != kernel.Instant(testOccurred) {
		t.Fatalf("Load() = %+v v%d, %v; want active, changed at the instant, at version 2", snap, version, err)
	}
	if h.outboxLen() != 0 {
		t.Fatalf("outbox = %d entries, want none", h.outboxLen())
	}
}

func TestSuspendingThePartnerLeavesItsCapabilitiesUntouched(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedPartner(t, h, ctx, domain.PartnerActive)
	enabled := capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, "")
	seedCapability(t, h, ctx, enabled)

	outcome, err := h.service.ChangePartnerStage(ctx, changeStage(domain.PartnerSuspended, "contract under review"))

	if err != nil {
		t.Fatalf("ChangePartnerStage() error = %v, want nil", err)
	}
	if got := outcome.Response().Status; got != domain.PartnerSuspended {
		t.Fatalf("Status = %s, want suspended", got)
	}
	if snap, version, _ := h.capRead.Load(ctx, testCapabilityID); version != 1 || !snap.Equal(enabled) {
		t.Fatalf("capability = %+v v%d, want it still enabled at version 1", snap, version)
	}
	if h.outboxLen() != 0 || len(h.agreements.asked) != 0 || len(h.caps.asked) != 0 {
		t.Fatalf("outbox = %d, reads = %v and %v; want nothing emitted and nothing read for a suspension", h.outboxLen(), h.agreements.asked, h.caps.asked)
	}
}

func TestReturningASuspendedPartnerToActiveReadsOnlyTheAgreement(t *testing.T) {
	h := newHarness(t)
	holdEffectiveAgreement(h)
	ctx := withExecution(t, context.Background())
	seedPartner(t, h, ctx, domain.PartnerSuspended)
	suspended := suspendedBy(domain.TriggerManual)
	seedCapability(t, h, ctx, suspended)

	outcome, err := h.service.ChangePartnerStage(ctx, changeStage(domain.PartnerActive, ""))

	if err != nil {
		t.Fatalf("ChangePartnerStage() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("ChangePartnerStage() rejected with %s, want active again", rej.Code())
	}
	if snap, version, _ := h.capRead.Load(ctx, testCapabilityID); version != 1 || !snap.Equal(suspended) {
		t.Fatalf("capability = %+v v%d, want it still suspended at version 1", snap, version)
	}
	if len(h.caps.asked) != 0 || h.outboxLen() != 0 {
		t.Fatalf("capabilities asked %v with %d outbox entries, want no capability read and nothing emitted", h.caps.asked, h.outboxLen())
	}
}

func TestOffboardingThePartnerEnqueuesTheOffboardingStarted(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedPartner(t, h, ctx, domain.PartnerActive)

	if _, err := h.service.ChangePartnerStage(ctx, changeStage(domain.PartnerOffboarding, "contract terminated")); err != nil {
		t.Fatalf("ChangePartnerStage() error = %v, want nil", err)
	}

	event := domain.PartnerOffboardingStarted{
		PartnerID: testPartnerID, OrganizationID: testOrganizationID, Reason: "contract terminated", At: kernel.Instant(testOccurred),
	}
	entries := h.store.Entries()
	if len(entries) != 1 || entries[0].Event != event || entries[0].AggregateType != application.AggregateTypePartner || entries[0].Intent.PartitionKey != string(testPartnerID) {
		t.Fatalf("outbox = %+v, want [%+v] keyed by the partner", entries, event)
	}
}

func TestChangePartnerStageRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		from    domain.PartnerStatus
		cmd     application.ChangePartnerStage
		prepare func(*harness)
		code    kernel.Code
	}{
		{"prospect to active", domain.PartnerProspect, changeStage(domain.PartnerActive, ""), func(h *harness) {
			holdEffectiveAgreement(h)
			h.caps.hold(testPartnerID, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))
		}, domain.CodePartnerInvalidStageTransition},
		{"suspension without reason", domain.PartnerActive, changeStage(domain.PartnerSuspended, " "), func(*harness) {},
			domain.CodePartnerReasonEmpty},
		{"homologating without an active agreement", domain.PartnerHomologating, changeStage(domain.PartnerActive, ""), func(h *harness) {
			h.agreements.hold(testPartnerID, draftAgreement(testAgreementID, 1, ""))
			h.caps.hold(testPartnerID, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))
		}, domain.CodePartnerAgreementNotActive},
		{"suspended with an expired agreement", domain.PartnerSuspended, changeStage(domain.PartnerActive, ""), func(h *harness) {
			h.agreements.hold(testPartnerID, activeAgreement(testAgreementID, kernel.Instant(testOccurred)))
		}, domain.CodePartnerAgreementNotActive},
		{"homologating without an enabled capability", domain.PartnerHomologating, changeStage(domain.PartnerActive, ""), func(h *harness) {
			holdEffectiveAgreement(h)
			h.caps.hold(testPartnerID, capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""))
		}, domain.CodePartnerNoEnabledCapability},
		{"homologating whose enabled capability was superseded by a suspended version", domain.PartnerHomologating, changeStage(domain.PartnerActive, ""), func(h *harness) {
			holdEffectiveAgreement(h)
			h.caps.hold(testPartnerID,
				capabilityVersion("cap-1", domain.CapabilityStatusEnabled, 1, ""),
				capabilityVersion("cap-2", domain.CapabilityStatusSuspended, 2, "cap-1"),
			)
		}, domain.CodePartnerNoEnabledCapability},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.prepare(h)
			ctx := withExecution(t, context.Background())
			seedPartner(t, h, ctx, tc.from)

			outcome, err := h.service.ChangePartnerStage(ctx, tc.cmd)

			if err != nil {
				t.Fatalf("ChangePartnerStage() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if snap, version, _ := h.partnerRead.Load(ctx, testPartnerID); version != 1 || snap.Status != tc.from || h.outboxLen() != 0 {
				t.Fatalf("Load() = %+v v%d with %d outbox entries, want the partner untouched and nothing emitted", snap, version, h.outboxLen())
			}
		})
	}
}

func TestChangingTheStageOfAnUnknownPartnerIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.ChangePartnerStage(withExecution(t, context.Background()), changeStage(domain.PartnerActive, ""))

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("ChangePartnerStage() = %v, want ErrNotFound", err)
	}
	if len(h.agreements.asked) != 0 || len(h.caps.asked) != 0 {
		t.Fatalf("the reads asked for %v and %v, want none for a partner that does not exist", h.agreements.asked, h.caps.asked)
	}
}
