package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestDeclareCapabilityOpensTheSeriesDeclaredWithTheRolloutDisabled(t *testing.T) {
	cmd := declareCapabilityCommand()
	cmd.StateQuerySupported, cmd.RestrictionNote, cmd.CallbackApplicable = false, "no state query: reconciled by the daily file", true
	c := domain.NewCapability(capabilityID)

	acc, rej := c.Declare(cmd)

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.DeclareCapabilityResponse{CapabilityID: capabilityID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %+v, want none", acc.Events())
	}
	want := domain.CapabilitySnapshot{
		ID:                 capabilityID,
		PartnerID:          partnerID,
		Operation:          "register-receivable",
		AssetType:          "duplicata",
		Environment:        domain.EnvironmentProduction,
		Version:            1,
		IsWrite:            true,
		RestrictionNote:    "no state query: reconciled by the daily file",
		CallbackApplicable: true,
		Status:             domain.CapabilityStatusDeclared,
		Rollout:            domain.Rollout{Mode: domain.RolloutDisabled},
	}
	if got := c.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestDeclareCapabilitySupersedesTheLastVersionOfTheSeries(t *testing.T) {
	cmd := declareCapabilityCommand()
	cmd.SupersedesID, cmd.Head = "cap-2", domain.CapabilityHead{ID: "cap-2", Version: 2}
	c := domain.NewCapability("cap-3")

	_, rej := c.Declare(cmd)

	tb.RequireAccepted(t, rej)
	if s := c.Snapshot(); s.Version != 3 || s.SupersedesID != "cap-2" || s.Status != domain.CapabilityStatusDeclared {
		t.Fatalf("Snapshot() = %+v, want version 3 superseding cap-2, declared", s)
	}
}

func TestDeclareCapabilityRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.DeclareCapability)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"operation empty", func(c *domain.DeclareCapability) { c.Operation = "" },
			"partner-management/capability/operation-empty", nil},
		{"operation blank", func(c *domain.DeclareCapability) { c.Operation = "  " },
			"partner-management/capability/operation-empty", nil},
		{"asset type empty", func(c *domain.DeclareCapability) { c.AssetType = "" },
			"partner-management/capability/asset-type-empty", nil},
		{"write without state query nor restriction", func(c *domain.DeclareCapability) { c.StateQuerySupported = false },
			"partner-management/capability/state-query-restriction-undeclared", nil},
		{"write without state query and a blank restriction", func(c *domain.DeclareCapability) {
			c.StateQuerySupported, c.RestrictionNote = false, " "
		}, "partner-management/capability/state-query-restriction-undeclared", nil},
		{"supersedes in an empty series", func(c *domain.DeclareCapability) { c.SupersedesID = "cap-0" },
			"partner-management/capability/supersedes-invalid", []kernel.Detail{{Key: "supersedes", Value: "cap-0"}}},
		{"another first version of the series", func(c *domain.DeclareCapability) {
			c.Head = domain.CapabilityHead{ID: "cap-1", Version: 1}
		}, "partner-management/capability/supersedes-invalid", []kernel.Detail{{Key: "supersedes", Value: ""}}},
		{"supersedes a version that is not the last", func(c *domain.DeclareCapability) {
			c.SupersedesID, c.Head = "cap-1", domain.CapabilityHead{ID: "cap-2", Version: 2}
		}, "partner-management/capability/supersedes-invalid", []kernel.Detail{{Key: "supersedes", Value: "cap-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := declareCapabilityCommand()
			tt.mutate(&cmd)
			c := domain.NewCapability(capabilityID)
			before := c.Snapshot()

			acc, rej := c.Declare(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireCapabilityUnchanged(t, before, c.Snapshot())
		})
	}
}

func TestDeclareCapabilityAdmitsAReadWithoutStateQueryOrRestriction(t *testing.T) {
	cmd := declareCapabilityCommand()
	cmd.IsWrite, cmd.StateQuerySupported = false, false

	_, rej := domain.NewCapability(capabilityID).Declare(cmd)

	tb.RequireAccepted(t, rej)
}

func TestDeclareCapabilityReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.DeclareCapability{IsWrite: true, SupersedesID: "cap-0", At: at}
	want := []kernel.Code{
		"partner-management/capability/operation-empty",
		"partner-management/capability/asset-type-empty",
		"partner-management/capability/state-query-restriction-undeclared",
		"partner-management/capability/supersedes-invalid",
	}
	fixes := []func(*domain.DeclareCapability){
		func(c *domain.DeclareCapability) { c.Operation = "register-receivable" },
		func(c *domain.DeclareCapability) { c.AssetType = "duplicata" },
		func(c *domain.DeclareCapability) { c.RestrictionNote = "reconciled by the daily file" },
		func(*domain.DeclareCapability) {},
	}
	for i, code := range want {
		acc, rej := domain.NewCapability(capabilityID).Declare(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}
