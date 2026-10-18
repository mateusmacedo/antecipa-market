package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const changedAt = at + day

func changeStage(target domain.PartnerStatus) domain.ChangePartnerStage {
	return domain.ChangePartnerStage{Target: target, At: changedAt}
}

func TestChangeStageWalksTheEntryOneStageAtATime(t *testing.T) {
	for _, step := range []struct{ from, to domain.PartnerStatus }{
		{domain.PartnerProspect, domain.PartnerContracting},
		{domain.PartnerContracting, domain.PartnerIntegrating},
		{domain.PartnerIntegrating, domain.PartnerHomologating},
	} {
		t.Run(string(step.from)+" to "+string(step.to), func(t *testing.T) {
			p := partnerAt(step.from)

			acc, rej := p.ChangeStage(changeStage(step.to))

			tb.RequireAccepted(t, rej)
			if got, want := acc.Response(), (domain.ChangePartnerStageResponse{PartnerID: partnerID, Status: step.to}); got != want {
				t.Fatalf("Response() = %+v, want %+v", got, want)
			}
			if len(acc.Events()) != 0 {
				t.Fatalf("Events() = %+v, want none", acc.Events())
			}
			if s := p.Snapshot(); s.Status != step.to || s.StageChangedAt != changedAt {
				t.Fatalf("Snapshot() = %+v, want %s changed at %d", s, step.to, changedAt)
			}
		})
	}
}

func TestChangeStageActivatesAHomologatingPartnerWithAnEffectiveAgreementAndAnEnabledCapability(t *testing.T) {
	p := partnerAt(domain.PartnerHomologating)
	cmd := changeStage(domain.PartnerActive)
	cmd.HasEffectiveAgreement, cmd.HasEnabledCapability = true, true

	acc, rej := p.ChangeStage(cmd)

	tb.RequireAccepted(t, rej)
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %+v, want none", acc.Events())
	}
	if s := p.Snapshot(); s.Status != domain.PartnerActive || s.StageChangedAt != changedAt {
		t.Fatalf("Snapshot() = %+v, want active changed at %d", s, changedAt)
	}
}

func TestChangeStageSuspendsAnActivePartnerKeepingTheReason(t *testing.T) {
	p := partnerAt(domain.PartnerActive)
	cmd := changeStage(domain.PartnerSuspended)
	cmd.Reason = "contract under review"

	acc, rej := p.ChangeStage(cmd)

	tb.RequireAccepted(t, rej)
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %+v, want none: suspending the partner emits nothing", acc.Events())
	}
	want := partnerAt(domain.PartnerActive).Snapshot()
	want.Status, want.StageChangedAt, want.StageReason = domain.PartnerSuspended, changedAt, "contract under review"
	if got := p.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestChangeStageReturnsASuspendedPartnerToActiveWithAnEffectiveAgreementAlone(t *testing.T) {
	p := partnerAt(domain.PartnerSuspended)
	cmd := changeStage(domain.PartnerActive)
	cmd.HasEffectiveAgreement = true

	acc, rej := p.ChangeStage(cmd)

	tb.RequireAccepted(t, rej)
	if len(acc.Events()) != 0 || p.Snapshot().Status != domain.PartnerActive {
		t.Fatalf("outcome = %+v with %+v, want active and nothing emitted", acc.Events(), p.Snapshot())
	}
}

func TestChangeStageStartsTheOffboarding(t *testing.T) {
	for _, from := range []domain.PartnerStatus{domain.PartnerActive, domain.PartnerSuspended} {
		t.Run(string(from), func(t *testing.T) {
			p := partnerAt(from)
			cmd := changeStage(domain.PartnerOffboarding)
			cmd.Reason = "contract terminated"

			acc, rej := p.ChangeStage(cmd)

			tb.RequireAccepted(t, rej)
			want := domain.PartnerOffboardingStarted{
				PartnerID: partnerID, OrganizationID: organizationID, Reason: "contract terminated", At: changedAt,
			}
			if events := acc.Events(); len(events) != 1 || events[0] != want {
				t.Fatalf("Events() = %+v, want [%+v]", events, want)
			}
			if s := p.Snapshot(); s.Status != domain.PartnerOffboarding || s.OffboardingStartedAt != changedAt || s.StageReason != "contract terminated" {
				t.Fatalf("Snapshot() = %+v, want offboarding started at %d with the reason", s, changedAt)
			}
		})
	}
}

func TestChangeStageKeepsThePreviousReasonWhenNoneIsInformed(t *testing.T) {
	suspended := partnerAt(domain.PartnerSuspended).Snapshot()
	suspended.StageReason = "contract under review"
	p := domain.FromPartnerSnapshot(suspended)
	cmd := changeStage(domain.PartnerActive)
	cmd.HasEffectiveAgreement = true

	_, rej := p.ChangeStage(cmd)

	tb.RequireAccepted(t, rej)
	if got := p.Snapshot().StageReason; got != "contract under review" {
		t.Fatalf("StageReason = %q, want the reason of the suspension kept", got)
	}
}

func TestChangeStageRejectsEachTransitionOutsideTheTable(t *testing.T) {
	for _, step := range []struct{ from, to domain.PartnerStatus }{
		{domain.PartnerProspect, domain.PartnerActive},
		{domain.PartnerProspect, domain.PartnerIntegrating},
		{domain.PartnerProspect, domain.PartnerProspect},
		{domain.PartnerContracting, domain.PartnerProspect},
		{domain.PartnerHomologating, domain.PartnerIntegrating},
		{domain.PartnerHomologating, domain.PartnerSuspended},
		{domain.PartnerActive, domain.PartnerHomologating},
		{domain.PartnerActive, domain.PartnerActive},
		{domain.PartnerSuspended, domain.PartnerSuspended},
		{domain.PartnerOffboarding, domain.PartnerActive},
		{domain.PartnerOffboarding, domain.PartnerOffboarding},
		{domain.PartnerActive, "archived"},
		{"", domain.PartnerContracting},
	} {
		t.Run(string(step.from)+" to "+string(step.to), func(t *testing.T) {
			p := partnerAt(step.from)
			before := p.Snapshot()
			cmd := changeStage(step.to)
			cmd.Reason, cmd.HasEffectiveAgreement, cmd.HasEnabledCapability = "any", true, true

			acc, rej := p.ChangeStage(cmd)

			tb.RequireRejected(t, acc, rej, "partner-management/partner/invalid-stage-transition")
			want := []kernel.Detail{{Key: "from", Value: string(step.from)}, {Key: "to", Value: string(step.to)}}
			if got := rej.Details(); !slices.Equal(got, want) {
				t.Fatalf("Details() = %v, want %v", got, want)
			}
			requirePartnerUnchanged(t, before, p.Snapshot())
		})
	}
}

func TestChangeStageRejectsEachMissingFact(t *testing.T) {
	tests := []struct {
		name   string
		from   domain.PartnerStatus
		mutate func(*domain.ChangePartnerStage)
		code   kernel.Code
	}{
		{"suspension without a reason", domain.PartnerActive, func(c *domain.ChangePartnerStage) { c.Target = domain.PartnerSuspended },
			"partner-management/partner/reason-empty"},
		{"suspension with a blank reason", domain.PartnerActive, func(c *domain.ChangePartnerStage) { c.Target, c.Reason = domain.PartnerSuspended, " \t" },
			"partner-management/partner/reason-empty"},
		{"offboarding without a reason", domain.PartnerSuspended, func(c *domain.ChangePartnerStage) { c.Target = domain.PartnerOffboarding },
			"partner-management/partner/reason-empty"},
		{"homologating to active without an effective agreement", domain.PartnerHomologating,
			func(c *domain.ChangePartnerStage) { c.Target, c.HasEnabledCapability = domain.PartnerActive, true },
			"partner-management/partner/agreement-not-active"},
		{"suspended to active without an effective agreement", domain.PartnerSuspended,
			func(c *domain.ChangePartnerStage) { c.Target, c.HasEnabledCapability = domain.PartnerActive, true },
			"partner-management/partner/agreement-not-active"},
		{"homologating to active without an enabled capability", domain.PartnerHomologating,
			func(c *domain.ChangePartnerStage) { c.Target, c.HasEffectiveAgreement = domain.PartnerActive, true },
			"partner-management/partner/no-enabled-capability"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := partnerAt(tt.from)
			before := p.Snapshot()
			cmd := changeStage("")
			tt.mutate(&cmd)

			acc, rej := p.ChangeStage(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			requirePartnerUnchanged(t, before, p.Snapshot())
		})
	}
}

func TestChangeStageReportsTheFactsInTheDeclaredOrder(t *testing.T) {
	p := partnerAt(domain.PartnerHomologating)
	cmd := changeStage(domain.PartnerActive)
	want := []kernel.Code{"partner-management/partner/agreement-not-active", "partner-management/partner/no-enabled-capability"}
	fixes := []func(*domain.ChangePartnerStage){
		func(c *domain.ChangePartnerStage) { c.HasEffectiveAgreement = true },
		func(*domain.ChangePartnerStage) {},
	}
	for i, code := range want {
		acc, rej := p.ChangeStage(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}
