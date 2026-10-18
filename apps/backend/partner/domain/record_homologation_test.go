package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const recordedAt = at + 2*day

func recordHomologation(r []domain.HomologationResult) domain.RecordHomologation {
	return domain.RecordHomologation{Results: r, At: recordedAt}
}

func TestRecordHomologationHomologatesAWriteWithStateQueryOnceEveryApplicableCriterionPassed(t *testing.T) {
	c := capability(domain.CapabilityStatusDeclared)

	acc, rej := c.RecordHomologation(recordHomologation(results(true, withCriteria("H3")...)))

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RecordHomologationResponse{CapabilityID: capabilityID, Status: domain.CapabilityStatusHomologated}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %+v, want none", acc.Events())
	}
	want := capabilityAt(domain.CapabilityStatusHomologated)
	want.Homologation = recorded(recordedAt, withCriteria("H3")...)
	if got := c.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestRecordHomologationKeepsTheCapabilityDeclaredUntilEveryApplicableCriterionPassed(t *testing.T) {
	for name, given := range map[string][]domain.HomologationResult{
		"without the state query criterion of a write": results(true, alwaysApplicable...),
		"with one criterion failed":                    append(results(true, withCriteria("H3")[1:]...), results(false, "H1")...),
	} {
		t.Run(name, func(t *testing.T) {
			c := capability(domain.CapabilityStatusDeclared)

			acc, rej := c.RecordHomologation(recordHomologation(given))

			tb.RequireAccepted(t, rej)
			if got := acc.Response().Status; got != domain.CapabilityStatusDeclared {
				t.Fatalf("Status = %s, want declared", got)
			}
			if got := len(c.Snapshot().Homologation); got != len(given) {
				t.Fatalf("Homologation holds %d results, want the %d recorded", got, len(given))
			}
		})
	}
}

func TestRecordHomologationDemandsTheCallbackOnlyWhenApplicable(t *testing.T) {
	s := capabilityAt(domain.CapabilityStatusDeclared)
	s.CallbackApplicable = true
	c := domain.FromCapabilitySnapshot(s)

	acc, _ := c.RecordHomologation(recordHomologation(results(true, withCriteria("H3")...)))
	if got := acc.Response().Status; got != domain.CapabilityStatusDeclared {
		t.Fatalf("Status = %s, want declared while the callback criterion is missing", got)
	}

	acc, rej := c.RecordHomologation(recordHomologation(results(true, "H9")))

	tb.RequireAccepted(t, rej)
	if got := acc.Response().Status; got != domain.CapabilityStatusHomologated {
		t.Fatalf("Status = %s, want homologated once H9 passed", got)
	}
}

func TestRecordHomologationReplacesTheResultOfTheSameCriterion(t *testing.T) {
	c := capability(domain.CapabilityStatusDeclared)
	if _, rej := c.RecordHomologation(recordHomologation(results(false, withCriteria("H3")...))); rej != nil {
		t.Fatalf("setup: RecordHomologation rejected: %v", rej)
	}

	acc, rej := c.RecordHomologation(domain.RecordHomologation{Results: results(true, withCriteria("H3")...), At: recordedAt + day})

	tb.RequireAccepted(t, rej)
	if got := acc.Response().Status; got != domain.CapabilityStatusHomologated {
		t.Fatalf("Status = %s, want homologated", got)
	}
	if got, want := c.Snapshot().Homologation, recorded(recordedAt+day, withCriteria("H3")...); !slices.Equal(got, want) {
		t.Fatalf("Homologation = %+v, want one result per criterion, the last one recorded", got)
	}
}

func TestRecordHomologationOfASchemaMismatchCountsOnlyTheResultsAfterTheSuspension(t *testing.T) {
	s := suspendedCapability(domain.TriggerSchemaMismatch)
	s.Homologation = recorded(at, withCriteria("H3")...)
	c := domain.FromCapabilitySnapshot(s)

	partial, rej := c.RecordHomologation(recordHomologation(results(true, alwaysApplicable...)))

	tb.RequireAccepted(t, rej)
	if got := partial.Response().Status; got != domain.CapabilityStatusSuspended {
		t.Fatalf("Status = %s, want suspended: the H3 passed before the suspension does not count", got)
	}

	acc, rej := c.RecordHomologation(recordHomologation(results(true, "H3")))

	tb.RequireAccepted(t, rej)
	if got := acc.Response().Status; got != domain.CapabilityStatusHomologated {
		t.Fatalf("Status = %s, want homologated after every criterion passed again", got)
	}
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %+v, want none", acc.Events())
	}
}

func TestRecordHomologationOfASchemaMismatchIgnoresAResultRecordedAtTheSuspension(t *testing.T) {
	c := domain.FromCapabilitySnapshot(suspendedCapability(domain.TriggerSchemaMismatch))

	acc, rej := c.RecordHomologation(domain.RecordHomologation{Results: results(true, withCriteria("H3")...), At: at + day})

	tb.RequireAccepted(t, rej)
	if got := acc.Response().Status; got != domain.CapabilityStatusSuspended {
		t.Fatalf("Status = %s, want suspended: only results after suspendedAt count", got)
	}
}

func TestRecordHomologationRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		target  domain.CapabilitySnapshot
		results []domain.HomologationResult
		code    kernel.Code
		details []kernel.Detail
	}{
		{"results empty", capabilityAt(domain.CapabilityStatusDeclared), nil,
			"partner-management/capability/criteria-empty", nil},
		{"criterion H13", capabilityAt(domain.CapabilityStatusDeclared), results(true, "H1", "H13"),
			"partner-management/capability/unknown-criterion", []kernel.Detail{{Key: "criterion", Value: "H13"}}},
		{"criterion H0", capabilityAt(domain.CapabilityStatusDeclared), results(true, "H0"),
			"partner-management/capability/unknown-criterion", []kernel.Detail{{Key: "criterion", Value: "H0"}}},
		{"criterion with a leading zero", capabilityAt(domain.CapabilityStatusDeclared), results(true, "H01"),
			"partner-management/capability/unknown-criterion", []kernel.Detail{{Key: "criterion", Value: "H01"}}},
		{"capability homologated", capabilityAt(domain.CapabilityStatusHomologated), results(true, "H1"),
			"partner-management/capability/homologation-closed", nil},
		{"capability enabled", capabilityAt(domain.CapabilityStatusEnabled), results(true, "H1"),
			"partner-management/capability/homologation-closed", nil},
		{"capability suspended by hand", suspendedCapability(domain.TriggerManual), results(true, "H1"),
			"partner-management/capability/homologation-closed", nil},
		{"capability suspended by offboarding", suspendedCapability(domain.TriggerOffboarding), results(true, "H1"),
			"partner-management/capability/homologation-closed", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := domain.FromCapabilitySnapshot(tt.target)
			before := c.Snapshot()

			acc, rej := c.RecordHomologation(recordHomologation(tt.results))

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireCapabilityUnchanged(t, before, c.Snapshot())
		})
	}
}

func TestRecordHomologationReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	c := capability(domain.CapabilityStatusHomologated)

	for _, step := range []struct {
		results []domain.HomologationResult
		code    kernel.Code
	}{
		{nil, "partner-management/capability/criteria-empty"},
		{results(true, "H13"), "partner-management/capability/unknown-criterion"},
		{results(true, "H1"), "partner-management/capability/homologation-closed"},
	} {
		acc, rej := c.RecordHomologation(recordHomologation(step.results))
		tb.RequireRejected(t, acc, rej, step.code)
	}
}

func TestRecordHomologationKeepsItsOwnCopyOfTheResults(t *testing.T) {
	c := capability(domain.CapabilityStatusDeclared)
	given := results(true, "H1", "H2")
	if _, rej := c.RecordHomologation(recordHomologation(given)); rej != nil {
		t.Fatalf("RecordHomologation rejected: %v", rej)
	}

	given[0].Passed = false
	c.Snapshot().Homologation[1].Passed = false

	if got := c.Snapshot().Homologation; !got[0].Passed || !got[1].Passed {
		t.Fatalf("Homologation = %+v, want the results recorded, untouched by the caller", got)
	}
}
