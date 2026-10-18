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

func seedDraft(t *testing.T, h *harness, ctx context.Context) {
	t.Helper()
	seed(t, h, ctx, agreementTable, testAgreementID, draftAgreement(testAgreementID, 1, ""))
}

func TestActivateAgreementWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedDraft(t, h, ctx)

	if _, err := h.service.ActivateAgreement(ctx, activateAgreement()); err != nil {
		t.Fatalf("ActivateAgreement() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"partner_agreements.Load",
		"partner_agreements.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestActivateAgreementActivatesTheDraftAndEnqueuesTheActivation(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedDraft(t, h, ctx)

	outcome, err := h.service.ActivateAgreement(ctx, activateAgreement())

	if err != nil {
		t.Fatalf("ActivateAgreement() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.ActivateAgreementResponse{AgreementID: testAgreementID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.agreeRead.Load(ctx, testAgreementID)
	if err != nil || version != 2 || snap.Status != domain.AgreementActive || snap.ContractEvidenceID != "evd-1" || snap.ActivatedAt != kernel.Instant(testOccurred) {
		t.Fatalf("Load() = %+v v%d, %v; want active at version 2 with the evidence", snap, version, err)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypePartnerAgreement || entry.AggregateID != string(testAgreementID) ||
		entry.AggregateVersion != 2 || entry.Intent.PartitionKey != string(testAgreementID) {
		t.Fatalf("outbox entry = %+v, want the activation of the agreement at version 2 keyed by it", entry)
	}
	event := domain.PartnerAgreementActivated{
		AgreementID: testAgreementID, PartnerID: testPartnerID, Version: 1,
		ValidFrom: kernel.Instant(testOccurred), ValidUntil: kernel.Instant(testOccurred) + 365*day, At: kernel.Instant(testOccurred),
	}
	if got, ok := entry.Event.(domain.PartnerAgreementActivated); !ok || got != event {
		t.Fatalf("event = %+v, want %+v", entry.Event, event)
	}
}

func TestActivateAgreementRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*application.ActivateAgreement)
		seeded func() domain.PartnerAgreementSnapshot
		code   kernel.Code
	}{
		{"agreement already active", func(*application.ActivateAgreement) {}, func() domain.PartnerAgreementSnapshot {
			s := draftAgreement(testAgreementID, 1, "")
			s.Status = domain.AgreementActive
			return s
		}, domain.CodePartnerAgreementNotDraft},
		{"evidence missing", func(c *application.ActivateAgreement) { c.ContractEvidenceID = "" }, nil,
			domain.CodePartnerAgreementContractEvidenceMissing},
		{"without H17", func(c *application.ActivateAgreement) { c.SatisfiedCriteria = []string{"H14", "H15", "H18"} }, nil,
			domain.CodePartnerAgreementContractualCriteriaIncomplete},
		{"without H16 when the regulatory check applies", func(c *application.ActivateAgreement) { c.RegulatoryCheckApplicable = true }, nil,
			domain.CodePartnerAgreementContractualCriteriaIncomplete},
		{"validity already over", func(*application.ActivateAgreement) {}, func() domain.PartnerAgreementSnapshot {
			s := draftAgreement(testAgreementID, 1, "")
			s.ValidFrom, s.ValidUntil = kernel.Instant(testOccurred)-30*day, kernel.Instant(testOccurred)
			return s
		}, domain.CodePartnerAgreementValidityInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			held := draftAgreement(testAgreementID, 1, "")
			if tc.seeded != nil {
				held = tc.seeded()
			}
			seed(t, h, ctx, agreementTable, testAgreementID, held)
			cmd := activateAgreement()
			tc.mutate(&cmd)

			outcome, err := h.service.ActivateAgreement(ctx, cmd)

			if err != nil {
				t.Fatalf("ActivateAgreement() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if snap, version, _ := h.agreeRead.Load(ctx, testAgreementID); version != 1 || !snap.Equal(held) || h.outboxLen() != 0 {
				t.Fatalf("Load() = %+v v%d with %d outbox entries, want the seeded agreement untouched", snap, version, h.outboxLen())
			}
		})
	}
}

func TestActivatingAnUnknownAgreementIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.ActivateAgreement(withExecution(t, context.Background()), activateAgreement())

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("ActivateAgreement() = %v, want ErrNotFound", err)
	}
}
