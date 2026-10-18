package application_test

import (
	"context"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func recordHomologation(results []domain.HomologationResult) application.RecordHomologation {
	return application.RecordHomologation{CapabilityID: testCapabilityID, Results: results}
}

func TestRecordHomologationWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusDeclared, 1, ""))

	if _, err := h.service.RecordHomologation(ctx, recordHomologation(homologationOf(true, "H1"))); err != nil {
		t.Fatalf("RecordHomologation() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"capabilities.Load",
		"capabilities.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRecordHomologationHomologatesOnceEveryApplicableCriterionPassed(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusDeclared, 1, ""))

	outcome, err := h.service.RecordHomologation(ctx, recordHomologation(homologationOf(true, homologationCriteria...)))

	if err != nil {
		t.Fatalf("RecordHomologation() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.RecordHomologationResponse{CapabilityID: testCapabilityID, Status: domain.CapabilityStatusHomologated}) {
		t.Fatalf("Response() = %+v, want homologated", got)
	}
	snap, version, err := h.capRead.Load(ctx, testCapabilityID)
	if err != nil || version != 2 || snap.Status != domain.CapabilityStatusHomologated || len(snap.Homologation) != len(homologationCriteria) ||
		snap.Homologation[0].RecordedAt != kernel.Instant(testOccurred) {
		t.Fatalf("Load() = %+v v%d, %v; want homologated at version 2 with every result recorded at the instant", snap, version, err)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: recording homologation emits nothing", got)
	}
}

func TestRecordHomologationReopensACapabilitySuspendedByASchemaMismatch(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	held := suspendedBy(domain.TriggerSchemaMismatch)
	for _, criterion := range homologationCriteria {
		held.Homologation = append(held.Homologation, domain.CriterionResult{CriterionID: criterion, Passed: true, RecordedAt: held.Suspension.SuspendedAt - day})
	}
	seedCapability(t, h, ctx, held)

	outcome, err := h.service.RecordHomologation(ctx, recordHomologation(homologationOf(true, homologationCriteria...)))

	if err != nil {
		t.Fatalf("RecordHomologation() error = %v, want nil", err)
	}
	if got := outcome.Response().Status; got != domain.CapabilityStatusHomologated {
		t.Fatalf("Status = %s, want homologated after the criteria passed again", got)
	}
	snap, version, err := h.capRead.Load(ctx, testCapabilityID)
	if err != nil || version != 2 || snap.Status != domain.CapabilityStatusHomologated || len(snap.Homologation) != len(homologationCriteria) {
		t.Fatalf("Load() = %+v v%d, %v; want homologated at version 2 with every criterion", snap, version, err)
	}
	for _, result := range snap.Homologation {
		if result.RecordedAt != kernel.Instant(testOccurred) || !result.Passed {
			t.Fatalf("result = %+v, want passed and recorded at the instant of the command", result)
		}
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: recording homologation emits nothing", got)
	}
}

func TestRecordHomologationRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		held    domain.CapabilitySnapshot
		results []domain.HomologationResult
		code    kernel.Code
	}{
		{"results empty", capabilityVersion(testCapabilityID, domain.CapabilityStatusDeclared, 1, ""), nil,
			domain.CodeCapabilityCriteriaEmpty},
		{"criterion H13", capabilityVersion(testCapabilityID, domain.CapabilityStatusDeclared, 1, ""), homologationOf(true, "H13"),
			domain.CodeCapabilityUnknownCriterion},
		{"capability homologated", capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""), homologationOf(true, "H1"),
			domain.CodeCapabilityHomologationClosed},
		{"capability suspended by hand", suspendedBy(domain.TriggerManual), homologationOf(true, "H1"),
			domain.CodeCapabilityHomologationClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			seedCapability(t, h, ctx, tc.held)

			outcome, err := h.service.RecordHomologation(ctx, recordHomologation(tc.results))

			if err != nil {
				t.Fatalf("RecordHomologation() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if snap, version, err := h.capRead.Load(ctx, testCapabilityID); err != nil || version != 1 || !snap.Equal(tc.held) {
				t.Fatalf("Load() = %+v v%d, %v; want the capability untouched", snap, version, err)
			}
		})
	}
}
