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

func TestRegisterPartnerWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterPartner(withExecution(t, context.Background()), registerPartner()); err != nil {
		t.Fatalf("RegisterPartner() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"directory.FindOrganization",
		"partners.Load",
		"partners.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRegisterPartnerCreatesAProspectFunderAndEnqueuesTheRegistration(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterPartner(ctx, registerPartner())

	if err != nil {
		t.Fatalf("RegisterPartner() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterPartner() rejected with %s, want accepted", rej.Code())
	}
	if got := outcome.Response(); got != (domain.RegisterPartnerResponse{PartnerID: testPartnerID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.partnerRead.Load(ctx, testPartnerID)
	want := domain.PartnerSnapshot{
		ID: testPartnerID, OrganizationID: testOrganizationID, Role: domain.RoleFunder, Status: domain.PartnerProspect,
		RegisteredAt: kernel.Instant(testOccurred), StageChangedAt: kernel.Instant(testOccurred),
	}
	if err != nil || version != 1 || !snap.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", snap, version, err, want)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypePartner || entry.AggregateID != string(testPartnerID) ||
		entry.AggregateVersion != 1 || entry.Intent.Destination != application.Destination ||
		entry.Intent.PartitionKey != string(testPartnerID) {
		t.Fatalf("outbox entry = %+v, want the registration of the partner at version 1 keyed by it", entry)
	}
	event := domain.PartnerRegistered{PartnerID: testPartnerID, OrganizationID: testOrganizationID, Role: domain.RoleFunder, At: kernel.Instant(testOccurred)}
	if got, ok := entry.Event.(domain.PartnerRegistered); !ok || got != event {
		t.Fatalf("event = %+v, want %+v", entry.Event, event)
	}
}

func TestRegisterPartnerAsksTheDirectoryUnderTheTenantOfTheRequest(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterPartner(withTenant(t, context.Background(), "globex"), registerPartner()); err != nil {
		t.Fatalf("RegisterPartner() error = %v, want nil", err)
	}

	if want := []directoryCall{{tenant: "globex", id: testOrganizationID}}; !slices.Equal(h.directory.calls, want) {
		t.Fatalf("FindOrganization calls = %+v, want %+v", h.directory.calls, want)
	}
}

func TestRegisterPartnerRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*harness)
		code    kernel.Code
	}{
		{"organization not found in identity", func(h *harness) { delete(h.directory.kinds, testOrganizationID) },
			domain.CodePartnerOrganizationNotFound},
		{"organization of another kind", func(h *harness) { h.directory.kinds[testOrganizationID] = []domain.OrganizationKind{"client"} },
			domain.CodePartnerOrganizationKindMismatch},
		{"organization and role already registered", nil, domain.CodePartnerAlreadyRegistered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			if tc.prepare != nil {
				tc.prepare(h)
			} else {
				seed(t, h, ctx, partnerTable, "prt-held", domain.PartnerSnapshot{ID: "prt-held", OrganizationID: testOrganizationID, Role: domain.RoleFunder, Status: domain.PartnerOffboarding})
			}

			outcome, err := h.service.RegisterPartner(ctx, registerPartner())

			if err != nil {
				t.Fatalf("RegisterPartner() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
			if _, _, err := h.partnerRead.Load(ctx, testPartnerID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func TestRegisterPartnerAdmitsAnotherRoleOfTheSameOrganization(t *testing.T) {
	h := newHarness(t)
	h.directory.kinds[testOrganizationID] = []domain.OrganizationKind{domain.KindFunder, domain.KindPartner}
	ctx := withExecution(t, context.Background())
	seed(t, h, ctx, partnerTable, "prt-held", domain.PartnerSnapshot{ID: "prt-held", OrganizationID: testOrganizationID, Role: domain.RoleRegistry})

	outcome, err := h.service.RegisterPartner(ctx, registerPartner())

	if err != nil {
		t.Fatalf("RegisterPartner() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterPartner() rejected with %s: the key is the organization and the role together", rej.Code())
	}
}

func TestRegisterPartnerAdmitsTheSameOrganizationAndRoleInAnotherTenant(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withTenant(t, context.Background(), "globex"), partnerTable, "prt-held", domain.PartnerSnapshot{ID: "prt-held", OrganizationID: testOrganizationID, Role: domain.RoleFunder})

	outcome, err := h.service.RegisterPartner(withExecution(t, context.Background()), registerPartner())

	if err != nil {
		t.Fatalf("RegisterPartner() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused || h.outboxLen() != 1 {
		t.Fatalf("outcome = %+v with %d outbox entries, want accepted with one", rej, h.outboxLen())
	}
}
