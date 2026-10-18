package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRegisterPartnerDeterminism(t *testing.T) {
	p1, p2 := domain.NewPartner(partnerID), domain.NewPartner(partnerID)

	acc1, _ := p1.Register(registerPartnerCommand())
	acc2, _ := p2.Register(registerPartnerCommand())

	if !p1.Snapshot().Equal(p2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestChangeStageDeterminism(t *testing.T) {
	cmd := changeStage(domain.PartnerOffboarding)
	cmd.Reason = "contract terminated"
	p1, p2 := partnerAt(domain.PartnerActive), partnerAt(domain.PartnerActive)

	acc1, _ := p1.ChangeStage(cmd)
	acc2, _ := p2.ChangeStage(cmd)

	if !p1.Snapshot().Equal(p2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestActivateAgreementDeterminism(t *testing.T) {
	a1, a2 := draftAgreement(t), draftAgreement(t)

	acc1, _ := a1.Activate(activateAgreementCommand())
	acc2, _ := a2.Activate(activateAgreementCommand())

	if !a1.Snapshot().Equal(a2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestActivateAgreementLeavesTheCriteriaOfTheCallerUntouched(t *testing.T) {
	cmd := activateAgreementCommand()
	cmd.SatisfiedCriteria = []string{"H18", "H17", "H15", "H14"}

	_, rej := draftAgreement(t).Activate(cmd)

	tb.RequireAccepted(t, rej)
	if cmd.SatisfiedCriteria[0] != "H18" || cmd.SatisfiedCriteria[3] != "H14" {
		t.Fatalf("SatisfiedCriteria = %v, want the order the caller gave", cmd.SatisfiedCriteria)
	}
}

func TestRaiseExpiryAlertDeterminism(t *testing.T) {
	p1, p2 := profileExpiringIn(t, 25*day), profileExpiringIn(t, 25*day)

	acc1, _ := p1.RaiseExpiryAlert(raiseAlert(30))
	acc2, _ := p2.RaiseExpiryAlert(raiseAlert(30))

	if !p1.Snapshot().Equal(p2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestDefineOperationalPolicyDeterminism(t *testing.T) {
	p1, p2 := domain.NewOperationalPolicy(policyID), domain.NewOperationalPolicy(policyID)

	acc1, _ := p1.Define(definePolicyCommand())
	acc2, _ := p2.Define(definePolicyCommand())

	if !p1.Snapshot().Equal(p2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestDeclareCapabilityDeterminism(t *testing.T) {
	c1, c2 := domain.NewCapability(capabilityID), domain.NewCapability(capabilityID)

	acc1, _ := c1.Declare(declareCapabilityCommand())
	acc2, _ := c2.Declare(declareCapabilityCommand())

	if !c1.Snapshot().Equal(c2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestRecordHomologationDeterminism(t *testing.T) {
	c1, c2 := capability(domain.CapabilityStatusDeclared), capability(domain.CapabilityStatusDeclared)
	cmd := domain.RecordHomologation{Results: results(true, withCriteria("H3")...), At: at}

	acc1, _ := c1.RecordHomologation(cmd)
	acc2, _ := c2.RecordHomologation(cmd)

	if !c1.Snapshot().Equal(c2.Snapshot()) || acc1.Response() != acc2.Response() {
		t.Fatal("same inputs must produce the same state")
	}
}

func TestEnableSuspendAndChangeRolloutDeterminism(t *testing.T) {
	for name, decide := range map[string]func(*domain.Capability) []kernel.DomainEvent{
		"enable": func(c *domain.Capability) []kernel.DomainEvent {
			acc, _ := c.Enable(domain.EnableCapability{HasEffectiveAgreement: true, HasEffectiveProfile: true, At: at})
			return acc.Events()
		},
		"suspend": func(c *domain.Capability) []kernel.DomainEvent {
			acc, _ := c.Suspend(domain.SuspendCapability{Reason: "partner incident", Trigger: domain.TriggerManual, At: at})
			return acc.Events()
		},
		"change rollout": func(c *domain.Capability) []kernel.DomainEvent {
			acc, _ := c.ChangeRollout(domain.ChangeRollout{Mode: domain.RolloutPercentage, Percentage: 10, ExcludedTenantIDs: []string{"tenant-x"}, At: at})
			return acc.Events()
		},
	} {
		t.Run(name, func(t *testing.T) {
			status := domain.CapabilityStatusEnabled
			if name == "enable" {
				status = domain.CapabilityStatusHomologated
			}
			c1, c2 := capability(status), capability(status)

			events1, events2 := decide(c1), decide(c2)

			if !c1.Snapshot().Equal(c2.Snapshot()) {
				t.Fatal("same inputs must produce the same state")
			}
			tb.RequireSameEvents(t, events1, events2)
		})
	}
}
