package application_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// simulatorOperation is the declaration of the partner simulator (04:G12): a
// key header partner, observed by state query, with a 128-bit hexadecimal key.
func simulatorOperation(operation string) domain.OperationCapability {
	return domain.OperationCapability{
		Operation: operation, Supported: true, Protocol: domain.ProtocolSyncHTTP, StateQuerySupported: true,
		IdempotencySupport: domain.IdempotencyKeyHeader, AssetTypes: []string{"duplicata"},
		EncumbranceCoverage: domain.CoverageComplete, Efficacy: domain.EfficacyOnSubmit, Quiescence: int64(30 * second),
		MaxQueries: 3, KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32, QueryInterval: 60, AmbiguityDeadline: 3600,
	}
}

func declareProfile(operations ...domain.OperationCapability) application.DeclareCapabilityProfile {
	return application.DeclareCapabilityProfile{ConnectorKey: testConnector, Version: 2, EffectiveFrom: testOccurred, Operations: operations}
}

func (h *harness) declared(t *testing.T, operations ...domain.OperationCapability) {
	t.Helper()
	must(h.service.DeclareCapabilityProfile(withExecution(t), declareProfile(operations...))).accepted(t)
}

func TestDeclareCapabilityProfileCreatesAnImmutableVersion(t *testing.T) {
	h := newHarness(t)

	got := must(h.service.DeclareCapabilityProfile(withExecution(t), declareProfile(simulatorOperation("submitEffect")))).accepted(t)

	if got.ProfileKey != "prt-1:production:2" {
		t.Fatalf("ProfileKey = %q", got.ProfileKey)
	}
	profile, err := h.service.FindCapabilityProfile(withExecution(t), testConnector, 2)
	if err != nil || profile.Version != 2 || profile.EffectiveFrom != testOccurred || len(profile.Operations) != 1 {
		t.Fatalf("FindCapabilityProfile() = %+v, %v", profile, err)
	}
}

func TestDeclareCapabilityProfileRefuses(t *testing.T) {
	shortKey := simulatorOperation("submitEffect")
	shortKey.KeyMaxLength = 31
	noDeadline := simulatorOperation("submitEffect")
	noDeadline.AmbiguityDeadline = 0
	noInterval := simulatorOperation("submitEffect")
	noInterval.QueryInterval = 0
	tests := []struct {
		name string
		cmd  application.DeclareCapabilityProfile
		code kernel.Code
	}{
		{"without operations", declareProfile(), domain.CodeCapabilityProfileOperationsEmpty},
		{"with a key that does not reach 128 bits in its alphabet", declareProfile(shortKey), domain.CodeCapabilityProfileExternalProfileInvalid},
		{"a write operation without ambiguity deadline", declareProfile(noDeadline), domain.CodeCapabilityProfileAmbiguityDeadlineMissing},
		{"a write operation with state query and no query interval", declareProfile(noInterval), domain.CodeCapabilityProfileQueryIntervalMissing},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			outcome, err := h.service.DeclareCapabilityProfile(withExecution(t), tc.cmd)

			must(outcome, err).refused(t, string(tc.code))
		})
	}
}

func TestDeclareCapabilityProfileRefusesAVersionAlreadyDeclared(t *testing.T) {
	h := newHarness(t)
	h.declared(t, simulatorOperation("submitEffect"))

	outcome, err := h.service.DeclareCapabilityProfile(withExecution(t), declareProfile(simulatorOperation("requestRelease")))

	must(outcome, err).refused(t, string(domain.CodeCapabilityProfileAlreadyDeclared))
}

func TestFindEffectiveCapabilityProfileReadsTheVersionInForce(t *testing.T) {
	h := newHarness(t)
	h.declared(t, simulatorOperation("submitEffect"))
	later := declareProfile(simulatorOperation("submitEffect"))
	later.Version, later.EffectiveFrom = 3, testOccurred+10*second
	must(h.service.DeclareCapabilityProfile(withExecution(t), later)).accepted(t)

	before, err := h.service.FindEffectiveCapabilityProfile(withExecution(t), testConnector, testOccurred+second)
	if err != nil || before.Version != 2 {
		t.Fatalf("effective before version 3 = %+v, %v", before, err)
	}
	after, err := h.service.FindEffectiveCapabilityProfile(withExecution(t), testConnector, testOccurred+10*second)
	if err != nil || after.Version != 3 {
		t.Fatalf("effective after version 3 = %+v, %v", after, err)
	}
	if _, err := h.service.FindEffectiveCapabilityProfile(withExecution(t), testConnector, testOccurred-1); err == nil {
		t.Fatalf("an instant before every version found a profile")
	}
}
