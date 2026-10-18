package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const capabilityID = domain.CapabilityID("cap-100")

func declareCapabilityCommand() domain.DeclareCapability {
	return domain.DeclareCapability{
		PartnerID:           partnerID,
		Operation:           "register-receivable",
		AssetType:           "duplicata",
		Environment:         domain.EnvironmentProduction,
		IsWrite:             true,
		StateQuerySupported: true,
		At:                  at,
	}
}

func capabilityAt(status domain.CapabilityStatus) domain.CapabilitySnapshot {
	return domain.CapabilitySnapshot{
		ID:                  capabilityID,
		PartnerID:           partnerID,
		Operation:           "register-receivable",
		AssetType:           "duplicata",
		Environment:         domain.EnvironmentProduction,
		Version:             1,
		IsWrite:             true,
		StateQuerySupported: true,
		Status:              status,
		Rollout:             domain.Rollout{Mode: domain.RolloutDisabled},
	}
}

func capability(status domain.CapabilityStatus) *domain.Capability {
	return domain.FromCapabilitySnapshot(capabilityAt(status))
}

func suspendedCapability(trigger domain.SuspensionTrigger) domain.CapabilitySnapshot {
	s := capabilityAt(domain.CapabilityStatusSuspended)
	s.EnabledAt = at
	s.Suspension = domain.Suspension{Reason: "under review", Trigger: trigger, SuspendedAt: at + day}
	return s
}

func results(passed bool, criteria ...string) []domain.HomologationResult {
	out := make([]domain.HomologationResult, 0, len(criteria))
	for _, criterion := range criteria {
		out = append(out, domain.HomologationResult{CriterionID: criterion, Passed: passed, EvidenceID: "evd-" + criterion})
	}
	return out
}

func recorded(recordedAt kernel.Instant, criteria ...string) []domain.CriterionResult {
	out := make([]domain.CriterionResult, 0, len(criteria))
	for _, criterion := range criteria {
		out = append(out, domain.CriterionResult{CriterionID: criterion, Passed: true, EvidenceID: "evd-" + criterion, RecordedAt: recordedAt})
	}
	return out
}

var alwaysApplicable = []string{"H1", "H2", "H4", "H5", "H6", "H7", "H8", "H10", "H11", "H12"}

func withCriteria(extra ...string) []string {
	return append(append([]string{}, alwaysApplicable...), extra...)
}

func requireCapabilityUnchanged(t *testing.T, before, after domain.CapabilitySnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
