package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

var technicalCriteria = []string{"H1", "H2", "H3", "H4", "H5", "H6", "H7", "H8", "H9", "H10", "H11", "H12"}

const (
	stateQueryCriterion = "H3"
	callbackCriterion   = "H9"
)

func (c *Capability) RecordHomologation(cmd RecordHomologation) (kernel.Accepted[RecordHomologationResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*Capability).clone, func(next *Capability) (kernel.Accepted[RecordHomologationResponse], *kernel.Rejection) {
		unknown := slices.IndexFunc(cmd.Results, func(r HomologationResult) bool { return !slices.Contains(technicalCriteria, r.CriterionID) })
		switch {
		case len(cmd.Results) == 0:
			return kernel.Refuse[RecordHomologationResponse](CodeCapabilityCriteriaEmpty, "at least one criterion result is required")
		case unknown >= 0:
			return kernel.Refuse[RecordHomologationResponse](CodeCapabilityUnknownCriterion, "the technical criteria are H1 to H12",
				kernel.Detail{Key: "criterion", Value: cmd.Results[unknown].CriterionID},
			)
		case next.status != CapabilityStatusDeclared && !next.suspendedBy(TriggerSchemaMismatch):
			return kernel.Refuse[RecordHomologationResponse](CodeCapabilityHomologationClosed,
				"only a declared capability, or one suspended by a schema mismatch, records homologation")
		}
		for _, result := range cmd.Results {
			next.record(CriterionResult{CriterionID: result.CriterionID, Passed: result.Passed, EvidenceID: result.EvidenceID, RecordedAt: cmd.At})
		}
		if next.homologated() {
			next.status = CapabilityStatusHomologated
		}
		return kernel.Accept(RecordHomologationResponse{CapabilityID: next.id, Status: next.status}), nil
	})
}

func (c *Capability) record(result CriterionResult) {
	if i := slices.IndexFunc(c.homologation, func(r CriterionResult) bool { return r.CriterionID == result.CriterionID }); i >= 0 {
		c.homologation[i] = result
		return
	}
	c.homologation = append(c.homologation, result)
}

func (c *Capability) homologated() bool {
	counts := func(r CriterionResult) bool {
		return r.Passed && (c.status != CapabilityStatusSuspended || r.RecordedAt > c.suspension.SuspendedAt)
	}
	for _, criterion := range c.applicableCriteria() {
		if !slices.ContainsFunc(c.homologation, func(r CriterionResult) bool { return r.CriterionID == criterion && counts(r) }) {
			return false
		}
	}
	return true
}

func (c *Capability) applicableCriteria() []string {
	return slices.DeleteFunc(slices.Clone(technicalCriteria), func(criterion string) bool {
		switch criterion {
		case stateQueryCriterion:
			return !c.isWrite || !c.stateQuerySupported
		case callbackCriterion:
			return !c.callbackApplicable
		default:
			return false
		}
	})
}
