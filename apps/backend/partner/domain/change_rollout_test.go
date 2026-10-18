package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const rolloutAt = at + 5*day

func changeRollout(mode domain.RolloutMode) domain.ChangeRollout {
	return domain.ChangeRollout{Mode: mode, At: rolloutAt}
}

func TestChangeRolloutReplacesTheRolloutAndAnnouncesIt(t *testing.T) {
	s := capabilityAt(domain.CapabilityStatusEnabled)
	s.Rollout = domain.Rollout{Mode: domain.RolloutEnabled}
	c := domain.FromCapabilitySnapshot(s)
	cmd := changeRollout(domain.RolloutPercentage)
	cmd.Percentage, cmd.ExcludedTenantIDs = 10, []string{"tenant-x"}

	acc, rej := c.ChangeRollout(cmd)

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.ChangeRolloutResponse{CapabilityID: capabilityID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	want := domain.CapabilityRolloutChanged{
		CapabilityID: capabilityID,
		PartnerID:    partnerID,
		Operation:    "register-receivable",
		AssetType:    "duplicata",
		Environment:  domain.EnvironmentProduction,
		Version:      1,
		Mode:         domain.RolloutPercentage,
		Percentage:   10,
		At:           rolloutAt,
	}
	if events := acc.Events(); len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want [%+v]", events, want)
	}
	after := s
	after.Rollout = domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 10, ExcludedTenantIDs: []string{"tenant-x"}}
	if got := c.Snapshot(); !got.Equal(after) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, after)
	}
}

func TestChangeRolloutAcceptsTheBoundsOfThePercentageAndAnyStatus(t *testing.T) {
	for _, status := range []domain.CapabilityStatus{domain.CapabilityStatusDeclared, domain.CapabilityStatusSuspended} {
		for _, percentage := range []int{0, 100} {
			cmd := changeRollout(domain.RolloutPercentage)
			cmd.Percentage = percentage

			_, rej := capability(status).ChangeRollout(cmd)

			tb.RequireAccepted(t, rej)
		}
	}
}

func TestChangeRolloutKeepsItsOwnCopyOfTheTenants(t *testing.T) {
	cmd := changeRollout(domain.RolloutAllowlist)
	cmd.AllowlistTenantIDs, cmd.ExcludedTenantIDs = []string{"tenant-a", "tenant-b"}, []string{"tenant-x"}
	c := capability(domain.CapabilityStatusEnabled)
	if _, rej := c.ChangeRollout(cmd); rej != nil {
		t.Fatalf("ChangeRollout rejected: %v", rej)
	}

	cmd.AllowlistTenantIDs[0], cmd.ExcludedTenantIDs[0] = "tenant-z", "tenant-z"
	c.Snapshot().Rollout.AllowlistTenantIDs[1] = "tenant-z"

	got := c.Snapshot().Rollout
	if !slices.Equal(got.AllowlistTenantIDs, []string{"tenant-a", "tenant-b"}) || !slices.Equal(got.ExcludedTenantIDs, []string{"tenant-x"}) {
		t.Fatalf("Rollout = %+v, want the tenants of the command, untouched by the caller", got)
	}
}

func TestChangeRolloutRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		cmd     func() domain.ChangeRollout
		code    kernel.Code
		details []kernel.Detail
	}{
		{"percentage 101", func() domain.ChangeRollout {
			cmd := changeRollout(domain.RolloutPercentage)
			cmd.Percentage = 101
			return cmd
		}, "partner-management/capability/percentage-out-of-range", []kernel.Detail{{Key: "percentage", Value: "101"}}},
		{"negative percentage", func() domain.ChangeRollout {
			cmd := changeRollout(domain.RolloutPercentage)
			cmd.Percentage = -1
			return cmd
		}, "partner-management/capability/percentage-out-of-range", []kernel.Detail{{Key: "percentage", Value: "-1"}}},
		{"allowlist empty", func() domain.ChangeRollout { return changeRollout(domain.RolloutAllowlist) },
			"partner-management/capability/allowlist-empty", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := capability(domain.CapabilityStatusEnabled)
			before := c.Snapshot()

			acc, rej := c.ChangeRollout(tt.cmd())

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireCapabilityUnchanged(t, before, c.Snapshot())
		})
	}
}

func TestChangeRolloutJudgesThePercentageOnlyInPercentageMode(t *testing.T) {
	cmd := changeRollout(domain.RolloutEnabled)
	cmd.Percentage = 101

	_, rej := capability(domain.CapabilityStatusEnabled).ChangeRollout(cmd)

	tb.RequireAccepted(t, rej)
}

func TestTheRolloutIncludesATenantByItsMode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rollout domain.Rollout
		tenant  string
		bucket  int
		want    bool
	}{
		{"disabled", domain.Rollout{Mode: domain.RolloutDisabled}, "tenant-a", 0, false},
		{"enabled", domain.Rollout{Mode: domain.RolloutEnabled}, "tenant-a", 99, true},
		{"allowlisted", domain.Rollout{Mode: domain.RolloutAllowlist, AllowlistTenantIDs: []string{"tenant-a"}}, "tenant-a", 99, true},
		{"outside the allowlist", domain.Rollout{Mode: domain.RolloutAllowlist, AllowlistTenantIDs: []string{"tenant-b"}}, "tenant-a", 0, false},
		{"bucket below the percentage", domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 10}, "tenant-a", 9, true},
		{"bucket at the percentage", domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 10}, "tenant-a", 10, false},
		{"zero percent", domain.Rollout{Mode: domain.RolloutPercentage, Percentage: 0}, "tenant-a", 0, false},
		{"excluded under enabled", domain.Rollout{Mode: domain.RolloutEnabled, ExcludedTenantIDs: []string{"tenant-a"}}, "tenant-a", 0, false},
		{"excluded under the allowlist", domain.Rollout{
			Mode: domain.RolloutAllowlist, AllowlistTenantIDs: []string{"tenant-a"}, ExcludedTenantIDs: []string{"tenant-a"},
		}, "tenant-a", 0, false},
		{"excluded under the percentage", domain.Rollout{
			Mode: domain.RolloutPercentage, Percentage: 100, ExcludedTenantIDs: []string{"tenant-a"},
		}, "tenant-a", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rollout.Includes(tc.tenant, tc.bucket); got != tc.want {
				t.Fatalf("Includes(%s, %d) = %v, want %v", tc.tenant, tc.bucket, got, tc.want)
			}
		})
	}
}
