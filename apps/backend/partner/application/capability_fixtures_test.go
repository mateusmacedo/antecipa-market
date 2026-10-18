package application_test

import (
	"context"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

var homologationCriteria = []string{"H1", "H2", "H3", "H4", "H5", "H6", "H7", "H8", "H10", "H11", "H12"}

func declareCapability() application.DeclareCapability {
	return application.DeclareCapability{
		CapabilityID:        testCapabilityID,
		PartnerID:           testPartnerID,
		Operation:           "register-receivable",
		AssetType:           "duplicata",
		Environment:         domain.EnvironmentProduction,
		IsWrite:             true,
		StateQuerySupported: false,
		RestrictionNote:     "reconciled by the daily file",
		CallbackApplicable:  true,
	}
}

func capabilityVersion(id domain.CapabilityID, status domain.CapabilityStatus, version int, supersedes domain.CapabilityID) domain.CapabilitySnapshot {
	return domain.CapabilitySnapshot{
		ID:                  id,
		PartnerID:           testPartnerID,
		Operation:           "register-receivable",
		AssetType:           "duplicata",
		Environment:         domain.EnvironmentProduction,
		Version:             version,
		SupersedesID:        supersedes,
		IsWrite:             true,
		StateQuerySupported: true,
		Status:              status,
		Rollout:             domain.Rollout{Mode: domain.RolloutDisabled},
	}
}

func seedCapability(t *testing.T, h *harness, ctx context.Context, s domain.CapabilitySnapshot) {
	t.Helper()
	seed(t, h, ctx, capTable, s.ID, s)
}

func suspendedBy(trigger domain.SuspensionTrigger) domain.CapabilitySnapshot {
	s := capabilityVersion(testCapabilityID, domain.CapabilityStatusSuspended, 1, "")
	s.Suspension = domain.Suspension{Reason: "under review", Trigger: trigger, SuspendedAt: kernel.Instant(testOccurred) - day}
	return s
}

func homologationOf(passed bool, criteria ...string) []domain.HomologationResult {
	out := make([]domain.HomologationResult, 0, len(criteria))
	for _, criterion := range criteria {
		out = append(out, domain.HomologationResult{CriterionID: criterion, Passed: passed, EvidenceID: "evd-" + criterion})
	}
	return out
}

func activeAgreement(id domain.AgreementID, validUntil kernel.Instant) domain.PartnerAgreementSnapshot {
	a := draftAgreement(id, 1, "")
	a.Status, a.ValidUntil, a.ContractEvidenceID, a.ActivatedAt = domain.AgreementActive, validUntil, "evd-1", kernel.Instant(testOccurred)
	return a
}

func partnerAt(status domain.PartnerStatus) domain.PartnerSnapshot {
	return domain.PartnerSnapshot{
		ID: testPartnerID, OrganizationID: testOrganizationID, Role: domain.RoleFunder, Status: status,
		RegisteredAt: kernel.Instant(testOccurred), StageChangedAt: kernel.Instant(testOccurred),
	}
}
