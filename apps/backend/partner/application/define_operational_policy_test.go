package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestDefineOperationalPolicyWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.DefineOperationalPolicy(withExecution(t, context.Background()), definePolicy()); err != nil {
		t.Fatalf("DefineOperationalPolicy() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"operational_policies.LoadByPartner",
		"operational_policies.Load",
		"operational_policies.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestDefineOperationalPolicyOpensTheSeriesWithoutAnEvent(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.DefineOperationalPolicy(ctx, definePolicy())

	if err != nil {
		t.Fatalf("DefineOperationalPolicy() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.DefineOperationalPolicyResponse{PolicyID: testPolicyID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.policyRead.Load(ctx, testPolicyID)
	want := domain.OperationalPolicySnapshot{
		ID: testPolicyID, PartnerID: testPartnerID, Version: 1, EffectiveFrom: kernel.Instant(testOccurred),
		RateLimitPerMinute: 100, BusinessDays: []string{"mon", "tue", "wed", "thu", "fri"}, CutoffTime: "17:00",
		ProcessingWindow: "08:00-18:00", SlaRef: "sla-gold", Fallback: "manual queue",
		SupportContact: "support@partner.example", IncidentContact: "+55 11 99999-0000",
	}
	if err != nil || version != 1 || !snap.Equal(want) {
		t.Fatalf("Load() = v%d, %v; want the fields of the command at version 1", version, err)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: defining a policy emits nothing", got)
	}
	if want := []domain.PartnerID{testPartnerID}; !slices.Equal(h.policies.asked, want) {
		t.Fatalf("the series read asked for %v, want %v", h.policies.asked, want)
	}
}

func TestDefineOperationalPolicySupersedesTheLastVersionOfThePartner(t *testing.T) {
	h := newHarness(t)
	h.policies.hold(testPartnerID, policyVersion("pol-2", 2, "pol-1"), policyVersion("pol-1", 1, ""))
	cmd := definePolicy()
	cmd.PolicyID, cmd.SupersedesID = "pol-3", "pol-2"
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.DefineOperationalPolicy(ctx, cmd)

	if err != nil {
		t.Fatalf("DefineOperationalPolicy() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("DefineOperationalPolicy() rejected with %s, want accepted", rej.Code())
	}
	if snap, _, err := h.policyRead.Load(ctx, "pol-3"); err != nil || snap.Version != 3 || snap.SupersedesID != "pol-2" {
		t.Fatalf("Load() = version %d superseding %q, %v; want version 3 superseding pol-2", snap.Version, snap.SupersedesID, err)
	}
}

func TestDefineOperationalPolicyRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*harness, *application.DefineOperationalPolicy)
		code    kernel.Code
	}{
		{"rate limit of zero", func(_ *harness, c *application.DefineOperationalPolicy) { c.RateLimitPerMinute = 0 },
			domain.CodeOperationalPolicyRateLimitInvalid},
		{"without the incident contact", func(_ *harness, c *application.DefineOperationalPolicy) { c.IncidentContact = "" },
			domain.CodeOperationalPolicyContactsMissing},
		{"another first version of the partner", func(h *harness, _ *application.DefineOperationalPolicy) {
			h.policies.hold(testPartnerID, policyVersion("pol-1", 1, ""))
		}, domain.CodeOperationalPolicySupersedesInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			cmd := definePolicy()
			tc.prepare(h, &cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.DefineOperationalPolicy(ctx, cmd)

			if err != nil {
				t.Fatalf("DefineOperationalPolicy() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := h.policyRead.Load(ctx, testPolicyID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func TestAFailedDefinitionNeverCarriesAContact(t *testing.T) {
	cmd := definePolicy()
	for name, options := range map[string][]option{
		"series read": nil,
		"save":        {withTableFaults("operational_policies", serviceskit.Faults{Save: errBoom})},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, options...)
			if options == nil {
				h.policies.err = errBoom
			}

			_, err := h.service.DefineOperationalPolicy(withExecution(t, context.Background()), cmd)

			if err == nil || strings.Contains(err.Error(), cmd.SupportContact) || strings.Contains(err.Error(), cmd.IncidentContact) {
				t.Fatalf("err = %v, want a failure that names the policy and never its contacts", err)
			}
		})
	}
}
