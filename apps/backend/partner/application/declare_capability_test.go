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

func TestDeclareCapabilityWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.DeclareCapability(withExecution(t, context.Background()), declareCapability()); err != nil {
		t.Fatalf("DeclareCapability() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"capabilities.LoadByPartner",
		"capabilities.Load",
		"capabilities.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestDeclareCapabilityOpensTheSeriesDeclaredWithTheCallbackAndNoEvent(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.DeclareCapability(ctx, declareCapability())

	if err != nil {
		t.Fatalf("DeclareCapability() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.DeclareCapabilityResponse{CapabilityID: testCapabilityID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.capRead.Load(ctx, testCapabilityID)
	want := domain.CapabilitySnapshot{
		ID: testCapabilityID, PartnerID: testPartnerID, Operation: "register-receivable", AssetType: "duplicata",
		Environment: domain.EnvironmentProduction, Version: 1, IsWrite: true, RestrictionNote: "reconciled by the daily file",
		CallbackApplicable: true, Status: domain.CapabilityStatusDeclared, Rollout: domain.Rollout{Mode: domain.RolloutDisabled},
	}
	if err != nil || version != 1 || !snap.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", snap, version, err, want)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: declaring a capability emits nothing", got)
	}
	if want := []domain.PartnerID{testPartnerID}; !slices.Equal(h.caps.asked, want) {
		t.Fatalf("the series read asked for %v, want %v", h.caps.asked, want)
	}
}

func TestDeclareCapabilitySupersedesTheLastVersionOfItsSeriesOnly(t *testing.T) {
	h := newHarness(t)
	other := capabilityVersion("cap-o3", domain.CapabilityStatusEnabled, 3, "cap-o2")
	other.AssetType = "cpr"
	h.caps.hold(testPartnerID,
		capabilityVersion("cap-1", domain.CapabilityStatusEnabled, 1, ""),
		capabilityVersion("cap-2", domain.CapabilityStatusDeclared, 2, "cap-1"),
		other,
	)
	cmd := declareCapability()
	cmd.CapabilityID, cmd.SupersedesID = "cap-3", "cap-2"
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.DeclareCapability(ctx, cmd)

	if err != nil {
		t.Fatalf("DeclareCapability() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("DeclareCapability() rejected with %s, want accepted", rej.Code())
	}
	if snap, _, err := h.capRead.Load(ctx, "cap-3"); err != nil || snap.Version != 3 || snap.SupersedesID != "cap-2" {
		t.Fatalf("Load() = %+v, %v; want version 3 superseding cap-2", snap, err)
	}
}

func TestDeclareCapabilityOpensAnotherSeriesOfThePartnerAsAFirstVersion(t *testing.T) {
	for name, edit := range map[string]func(*domain.CapabilitySnapshot){
		"another operation":   func(s *domain.CapabilitySnapshot) { s.Operation = "settle" },
		"another asset type":  func(s *domain.CapabilitySnapshot) { s.AssetType = "cpr" },
		"another environment": func(s *domain.CapabilitySnapshot) { s.Environment = domain.EnvironmentHomologation },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			held := capabilityVersion("cap-1", domain.CapabilityStatusEnabled, 1, "")
			edit(&held)
			h.caps.hold(testPartnerID, held)

			outcome, err := h.service.DeclareCapability(withExecution(t, context.Background()), declareCapability())

			if err != nil {
				t.Fatalf("DeclareCapability() error = %v, want nil", err)
			}
			if rej, refused := outcome.Rejection(); refused {
				t.Fatalf("DeclareCapability() rejected with %s: the series is the partner, operation, asset type and environment", rej.Code())
			}
		})
	}
}

func TestDeclareCapabilityRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*harness, *application.DeclareCapability)
		code    kernel.Code
	}{
		{"operation empty", func(_ *harness, c *application.DeclareCapability) { c.Operation = "" },
			domain.CodeCapabilityOperationEmpty},
		{"asset type empty", func(_ *harness, c *application.DeclareCapability) { c.AssetType = "" },
			domain.CodeCapabilityAssetTypeEmpty},
		{"write without state query nor restriction", func(_ *harness, c *application.DeclareCapability) { c.RestrictionNote = "" },
			domain.CodeCapabilityStateQueryRestrictionUndeclared},
		{"another first version of the series", func(h *harness, _ *application.DeclareCapability) {
			h.caps.hold(testPartnerID, capabilityVersion("cap-1", domain.CapabilityStatusDeclared, 1, ""))
		}, domain.CodeCapabilitySupersedesInvalid},
		{"supersedes a capability of another series", func(h *harness, c *application.DeclareCapability) {
			other := capabilityVersion("cap-o1", domain.CapabilityStatusDeclared, 1, "")
			other.Environment = domain.EnvironmentHomologation
			h.caps.hold(testPartnerID, other)
			c.SupersedesID = "cap-o1"
		}, domain.CodeCapabilitySupersedesInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			cmd := declareCapability()
			tc.prepare(h, &cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.DeclareCapability(ctx, cmd)

			if err != nil {
				t.Fatalf("DeclareCapability() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := h.capRead.Load(ctx, testCapabilityID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func TestDeclareCapabilityCountsACrossTenantAccessAsAnEmptySeries(t *testing.T) {
	h := newHarness(t)
	h.caps.err = port.CrossTenantAccess{Object: "capabilities?partner_id=prt-100", ContextTenant: testTenant, DataTenant: "globex"}

	outcome, err := h.service.DeclareCapability(withExecution(t, context.Background()), declareCapability())

	if err != nil {
		t.Fatalf("DeclareCapability() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("DeclareCapability() rejected with %s, want the first version of an empty series", rej.Code())
	}
}
