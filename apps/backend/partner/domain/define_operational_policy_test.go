package domain_test

import (
	"slices"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestDefineOperationalPolicyOpensTheSeriesAtVersionOneWithoutAnEvent(t *testing.T) {
	p := domain.NewOperationalPolicy(policyID)

	acc, rej := p.Define(definePolicyCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.DefineOperationalPolicyResponse{PolicyID: policyID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %d, want none", len(acc.Events()))
	}
	want := domain.OperationalPolicySnapshot{
		ID:                 policyID,
		PartnerID:          partnerID,
		Version:            1,
		EffectiveFrom:      at,
		RateLimitPerMinute: 100,
		BusinessDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		CutoffTime:         "17:00",
		ProcessingWindow:   "08:00-18:00",
		SlaRef:             "sla-gold",
		Fallback:           "manual queue",
		SupportContact:     "support@partner.example",
		IncidentContact:    "+55 11 99999-0000",
	}
	if !p.Snapshot().Equal(want) {
		t.Fatal("Snapshot() differs from the fields of the command at version 1")
	}
}

func TestDefineOperationalPolicySupersedesTheLastVersionOfThePartner(t *testing.T) {
	cmd := definePolicyCommand()
	cmd.SupersedesID, cmd.Head = "pol-2", domain.PolicyHead{ID: "pol-2", Version: 2}
	p := domain.NewOperationalPolicy("pol-3")

	_, rej := p.Define(cmd)

	tb.RequireAccepted(t, rej)
	if s := p.Snapshot(); s.Version != 3 || s.SupersedesID != "pol-2" {
		t.Fatalf("version %d superseding %q, want version 3 superseding pol-2", s.Version, s.SupersedesID)
	}
}

func TestDefineOperationalPolicyKeepsItsOwnCopyOfTheBusinessDays(t *testing.T) {
	cmd := definePolicyCommand()
	p := domain.NewOperationalPolicy(policyID)
	if _, rej := p.Define(cmd); rej != nil {
		t.Fatalf("Define rejected: %v", rej.Code())
	}

	cmd.BusinessDays[0] = "sun"
	p.Snapshot().BusinessDays[1] = "sat"

	if got := p.Snapshot().BusinessDays; !slices.Equal(got, []string{"mon", "tue", "wed", "thu", "fri"}) {
		t.Fatalf("BusinessDays = %v, want the days of the command, untouched by the caller", got)
	}
}

func TestDefineOperationalPolicyRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.DefineOperationalPolicy)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"rate limit of zero", func(c *domain.DefineOperationalPolicy) { c.RateLimitPerMinute = 0 },
			domain.CodeOperationalPolicyRateLimitInvalid, nil},
		{"negative rate limit", func(c *domain.DefineOperationalPolicy) { c.RateLimitPerMinute = -1 },
			domain.CodeOperationalPolicyRateLimitInvalid, nil},
		{"without the incident contact", func(c *domain.DefineOperationalPolicy) { c.IncidentContact = "" },
			domain.CodeOperationalPolicyContactsMissing, []kernel.Detail{{Key: "missing", Value: "incidentContact"}}},
		{"support contact blank", func(c *domain.DefineOperationalPolicy) { c.SupportContact = "  " },
			domain.CodeOperationalPolicyContactsMissing, []kernel.Detail{{Key: "missing", Value: "supportContact"}}},
		{"without any contact", func(c *domain.DefineOperationalPolicy) { c.SupportContact, c.IncidentContact = "", "" },
			domain.CodeOperationalPolicyContactsMissing, []kernel.Detail{{Key: "missing", Value: "supportContact,incidentContact"}}},
		{"supersedes in an empty series", func(c *domain.DefineOperationalPolicy) { c.SupersedesID = "pol-0" },
			domain.CodeOperationalPolicySupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "pol-0"}}},
		{"another first version of the partner", func(c *domain.DefineOperationalPolicy) {
			c.Head = domain.PolicyHead{ID: "pol-1", Version: 1}
		}, domain.CodeOperationalPolicySupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: ""}}},
		{"supersedes a version that is not the last", func(c *domain.DefineOperationalPolicy) {
			c.SupersedesID, c.Head = "pol-1", domain.PolicyHead{ID: "pol-2", Version: 2}
		}, domain.CodeOperationalPolicySupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "pol-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := definePolicyCommand()
			tt.mutate(&cmd)
			p := domain.NewOperationalPolicy(policyID)
			before := p.Snapshot()

			acc, rej := p.Define(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requirePolicyUnchanged(t, before, p.Snapshot())
		})
	}
}

func TestARefusedPolicyNeverCarriesAContact(t *testing.T) {
	cmd := definePolicyCommand()
	cmd.RateLimitPerMinute, cmd.IncidentContact = 1, ""

	_, rej := domain.NewOperationalPolicy(policyID).Define(cmd)

	text := rej.Message()
	for _, d := range rej.Details() {
		text += d.Key + d.Value
	}
	if strings.Contains(text, cmd.SupportContact) {
		t.Fatalf("rejection %q carries the support contact", rej.Code())
	}
}

func TestDefineOperationalPolicyReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.DefineOperationalPolicy{SupersedesID: "pol-0", At: at}
	want := []kernel.Code{
		domain.CodeOperationalPolicyRateLimitInvalid,
		domain.CodeOperationalPolicyContactsMissing,
		domain.CodeOperationalPolicySupersedesInvalid,
	}
	fixes := []func(*domain.DefineOperationalPolicy){
		func(c *domain.DefineOperationalPolicy) { c.RateLimitPerMinute = 100 },
		func(c *domain.DefineOperationalPolicy) {
			c.SupportContact, c.IncidentContact = "support@partner.example", "+55 11 99999-0000"
		},
		func(*domain.DefineOperationalPolicy) {},
	}
	for i, code := range want {
		acc, rej := domain.NewOperationalPolicy(policyID).Define(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}
