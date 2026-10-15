package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestARepeatedRegisterReplaysWithoutASecondEffect(t *testing.T) {
	h, instr := newInstrumentedHarness(t)

	first, err := h.service.RegisterOrganization(withKey(t, context.Background(), "k-register"), register())
	if err != nil {
		t.Fatalf("first RegisterOrganization() = %v, want nil", err)
	}
	h.rec.Reset()
	ctx := withKey(t, context.Background(), "k-register")
	again, err := h.service.RegisterOrganization(ctx, register())
	if err != nil {
		t.Fatalf("repeated RegisterOrganization() = %v, want the stored outcome", err)
	}

	if again.Response() != first.Response() {
		t.Fatalf("replay = %+v, want %+v", again.Response(), first.Response())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1: the replay produced a second event", got)
	}
	if len(instr.audits) != 1 {
		t.Fatalf("Audit called %d times, want 1: a replay is not a new fact", len(instr.audits))
	}
	if slices.Contains(h.rec.Observed(), "protector.Protect") {
		t.Fatalf("sequence = %v: the replay reached the protection", h.rec.Observed())
	}
	if outcome, _ := port.IdempotencyOutcomeFrom(ctx); outcome != port.IdempotencyReplayed {
		t.Fatalf("idempotency outcome = %v, want replayed", outcome)
	}
}

func TestARepeatedRefusalReplaysTheRejection(t *testing.T) {
	h := newHarness(t)
	cmd := register()
	cmd.LegalName = ""

	if _, err := h.service.RegisterOrganization(withKey(t, context.Background(), "k-refused"), cmd); err != nil {
		t.Fatalf("first RegisterOrganization() = %v, want a rejection, not an error", err)
	}
	again, err := h.service.RegisterOrganization(withKey(t, context.Background(), "k-refused"), cmd)
	if err != nil {
		t.Fatalf("repeated RegisterOrganization() = %v, want the stored rejection", err)
	}

	if rej, refused := again.Rejection(); !refused || rej.Code() != domain.CodeOrganizationLegalNameEmpty {
		t.Fatalf("replay = %+v, want the rejection %s", again, domain.CodeOrganizationLegalNameEmpty)
	}
}

func requireSecondRegisterRefused(t *testing.T, firstKey, secondKey string, second application.RegisterOrganization, want error) {
	t.Helper()
	h := newHarness(t)
	if _, err := h.service.RegisterOrganization(withKey(t, context.Background(), firstKey), register()); err != nil {
		t.Fatalf("first RegisterOrganization() = %v, want nil", err)
	}

	_, err := h.service.RegisterOrganization(withKey(t, context.Background(), secondKey), second)

	if !errors.Is(err, want) {
		t.Fatalf("second RegisterOrganization() = %v, want %v", err, want)
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1", got)
	}
}

func TestTheSameKeyWithAnotherPayloadIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.RegisterOrganization){
		"kinds":        func(c *application.RegisterOrganization) { c.Kinds = append(c.Kinds, domain.KindFunder) },
		"jurisdiction": func(c *application.RegisterOrganization) { c.Jurisdiction = "PT" },
		"tax id type":  func(c *application.RegisterOrganization) { c.TaxIDType = domain.TaxIDTypeCPF },
		"tax id":       func(c *application.RegisterOrganization) { c.TaxID = cnpj("A1B2C3D40001") },
		"legal name":   func(c *application.RegisterOrganization) { c.LegalName += " II" },
		"trade name":   func(c *application.RegisterOrganization) { c.TradeName = "" },
	} {
		t.Run(name, func(t *testing.T) {
			second := register()
			mutate(&second)
			requireSecondRegisterRefused(t, "k-reused", "k-reused", second, port.ErrIdempotencyMismatch)
		})
	}
}

func TestTheSameKindsInAnotherOrderAreTheSamePayload(t *testing.T) {
	h := newHarness(t)
	first := register()
	first.Kinds = []domain.OrganizationKind{domain.KindClient, domain.KindFunder}
	second := register()
	second.Kinds = []domain.OrganizationKind{domain.KindFunder, domain.KindClient}

	if _, err := h.service.RegisterOrganization(withKey(t, context.Background(), "k-kinds"), first); err != nil {
		t.Fatalf("first RegisterOrganization() = %v, want nil", err)
	}
	if _, err := h.service.RegisterOrganization(withKey(t, context.Background(), "k-kinds"), second); err != nil {
		t.Fatalf("RegisterOrganization() with the kinds reordered = %v, want the replay", err)
	}
}

func TestRegisteringAnExistingOrganizationUnderAnotherKeyAlreadyExists(t *testing.T) {
	requireSecondRegisterRefused(t, "k-first", "k-second", register(), port.ErrAlreadyExists)
}

func TestACommandWaitingPastTheCeilingIsInFlight(t *testing.T) {
	h := newHarness(t, withRegisterError(port.ErrRegisterTimeout))

	_, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register())

	if !errors.Is(err, port.ErrIdempotencyInFlight) {
		t.Fatalf("RegisterOrganization() = %v, want ErrIdempotencyInFlight", err)
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeAnyEffect(t *testing.T) {
	h := newHarness(t)
	ctx := port.WithExecutionContext(context.Background(), execctx.Execution(t))

	_, err := h.service.RegisterOrganization(ctx, register())

	if !errors.Is(err, port.ErrIdempotencyKeyAbsent) {
		t.Fatalf("RegisterOrganization() = %v, want ErrIdempotencyKeyAbsent", err)
	}
	if len(h.protector.calls) != 0 || h.outboxLen() != 0 {
		t.Fatalf("protect calls = %d, outbox = %d; want no effect", len(h.protector.calls), h.outboxLen())
	}
}
