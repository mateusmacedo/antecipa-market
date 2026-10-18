package domain

import "slices"

type RolloutMode string

const (
	RolloutDisabled   RolloutMode = "disabled"
	RolloutAllowlist  RolloutMode = "allowlist"
	RolloutPercentage RolloutMode = "percentage"
	RolloutEnabled    RolloutMode = "enabled"
)

type Rollout struct {
	Mode               RolloutMode
	AllowlistTenantIDs []string
	Percentage         int
	ExcludedTenantIDs  []string
}

func (r Rollout) Includes(tenantID string, bucket int) bool {
	if slices.Contains(r.ExcludedTenantIDs, tenantID) {
		return false
	}
	switch r.Mode {
	case RolloutEnabled:
		return true
	case RolloutAllowlist:
		return slices.Contains(r.AllowlistTenantIDs, tenantID)
	case RolloutPercentage:
		return bucket < r.Percentage
	default:
		return false
	}
}

func (r Rollout) clone() Rollout {
	r.AllowlistTenantIDs = slices.Clone(r.AllowlistTenantIDs)
	r.ExcludedTenantIDs = slices.Clone(r.ExcludedTenantIDs)
	return r
}

func (r Rollout) equal(other Rollout) bool {
	return r.Mode == other.Mode &&
		slices.Equal(r.AllowlistTenantIDs, other.AllowlistTenantIDs) &&
		r.Percentage == other.Percentage &&
		slices.Equal(r.ExcludedTenantIDs, other.ExcludedTenantIDs)
}
