package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func submitEffectCapability() domain.OperationCapability {
	return domain.OperationCapability{
		Operation: "submitEffect", Supported: true, Protocol: domain.ProtocolSyncHTTP, StateQuerySupported: true,
		IdempotencySupport: domain.IdempotencyKeyHeader, AssetTypes: []string{"duplicata"},
		MaxItemsPerRequest: 100, MaxRequestsPerMinute: 600, MaxPayloadBytes: 1 << 20,
		BusinessCalendar: "BR-B3", CutoffTime: "17:00", ProcessingWindow: "08:00-18:00",
		EncumbranceCoverage: domain.CoverageComplete, PositionFreshnessSLA: 300,
		Efficacy: domain.EfficacyByObservation, Quiescence: int64(30 * second), MaxQueries: 5,
		KeyAlphabet: "0123456789abcdef", KeyMaxLength: 64, QueryInterval: 60, AmbiguityDeadline: 3600,
	}
}

func declare(operations ...domain.OperationCapability) domain.DeclareCapabilityProfile {
	return domain.DeclareCapabilityProfile{ConnectorKey: connectorKey, Version: 2, EffectiveFrom: at, Operations: operations}
}

func TestDeclareCapabilityProfileCreatesTheVersion(t *testing.T) {
	p := domain.NewCapabilityProfile(domain.ProfileKeyOf(connectorKey, 2))

	acc, rej := p.Declare(declare(submitEffectCapability()))

	tb.RequireAccepted(t, rej)
	requireNoEvents(t, acc)
	if got, want := acc.Response(), (domain.ProfileResponse{ProfileKey: "prt-1:production:2"}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	s := p.Snapshot()
	if s.ConnectorKey != connectorKey || s.Version != 2 || s.EffectiveFrom != at || len(s.Operations) != 1 || !s.Operations[0].Equal(submitEffectCapability()) {
		t.Fatalf("Snapshot() = %+v", s)
	}
}

func TestDeclareCapabilityProfileRejects(t *testing.T) {
	withDeadline := func(deadline int) domain.OperationCapability {
		o := submitEffectCapability()
		o.AmbiguityDeadline = deadline
		return o
	}
	withoutInterval := submitEffectCapability()
	withoutInterval.QueryInterval = 0
	tests := []struct {
		name string
		cmd  domain.DeclareCapabilityProfile
		code kernel.Code
	}{
		{"without operations", declare(), domain.CodeCapabilityProfileOperationsEmpty},
		{"with a partner declaration the kernel refuses", func() domain.DeclareCapabilityProfile {
			c := declare(submitEffectCapability())
			c.InvalidProfiles = []domain.InvalidProfile{{Operation: "submitEffect", Reason: "key of 8 symbols of 4 bits is below 128 bits"}}
			return c
		}(), domain.CodeCapabilityProfileExternalProfileInvalid},
		{"a write operation without ambiguity deadline", declare(withDeadline(0)), domain.CodeCapabilityProfileAmbiguityDeadlineMissing},
		{"a write operation with state query and no query interval", declare(withoutInterval), domain.CodeCapabilityProfileQueryIntervalMissing},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := domain.NewCapabilityProfile(domain.ProfileKeyOf(connectorKey, 2))
			before := p.Snapshot()

			acc, rej := p.Declare(tc.cmd)

			tb.RequireRejected(t, acc, rej, tc.code)
			if !before.Equal(p.Snapshot()) {
				t.Fatalf("snapshot changed on rejection: %+v", p.Snapshot())
			}
		})
	}
}

func TestAReadOperationNeedsNoAmbiguityDeadline(t *testing.T) {
	read := submitEffectCapability()
	read.Operation, read.AmbiguityDeadline, read.QueryInterval = "queryAsset", 0, 0
	p := domain.NewCapabilityProfile(domain.ProfileKeyOf(connectorKey, 2))

	_, rej := p.Declare(declare(read))

	tb.RequireAccepted(t, rej)
}

func TestEffectiveProfileIsTheNewestVersionAlreadyInForce(t *testing.T) {
	version := func(v int, from kernel.Instant) domain.CapabilityProfileSnapshot {
		return domain.CapabilityProfileSnapshot{Key: domain.ProfileKeyOf(connectorKey, v), ConnectorKey: connectorKey, Version: v, EffectiveFrom: from}
	}
	profiles := []domain.CapabilityProfileSnapshot{version(1, at), version(3, at+2*second), version(2, at+second)}

	tests := []struct {
		at   kernel.Instant
		want int
	}{{at - 1, 0}, {at, 1}, {at + second, 2}, {at + 5*second, 3}}
	for _, tc := range tests {
		got, found := domain.EffectiveProfile(profiles, tc.at)
		if (tc.want == 0) == found || (found && got.Version != tc.want) {
			t.Errorf("EffectiveProfile(%d) = version %d found %v, want %d", tc.at, got.Version, found, tc.want)
		}
	}
}

func TestSupportsNeedsTheOperationSupportedForTheAssetType(t *testing.T) {
	unsupported := submitEffectCapability()
	unsupported.Operation, unsupported.Supported = "requestRelease", false
	anyAsset := submitEffectCapability()
	anyAsset.Operation, anyAsset.AssetTypes = "activateOptIn", nil
	s := domain.CapabilityProfileSnapshot{Operations: []domain.OperationCapability{submitEffectCapability(), unsupported, anyAsset}}

	tests := []struct {
		operation, assetType string
		want                 bool
	}{
		{"submitEffect", "duplicata", true},
		{"submitEffect", "cartao", false},
		{"requestRelease", "duplicata", false},
		{"activateOptIn", "cartao", true},
		{"revokeOptIn", "duplicata", false},
	}
	for _, tc := range tests {
		if _, got := s.Supports(tc.operation, tc.assetType); got != tc.want {
			t.Errorf("Supports(%s, %s) = %v, want %v", tc.operation, tc.assetType, got, tc.want)
		}
	}
}

func TestCapabilityProfileSnapshotIsADeepCopy(t *testing.T) {
	p := domain.NewCapabilityProfile(domain.ProfileKeyOf(connectorKey, 2))
	if _, rej := p.Declare(declare(submitEffectCapability())); rej != nil {
		t.Fatalf("setup: %v", rej)
	}
	held := p.Snapshot()
	held.Operations[0].AssetTypes[0] = "other"

	if got := p.Snapshot().Operations[0].AssetTypes[0]; got != "duplicata" {
		t.Fatalf("the snapshot shares its asset types with the aggregate: %q", got)
	}
	if !domain.FromCapabilityProfileSnapshot(p.Snapshot()).Snapshot().Equal(p.Snapshot()) {
		t.Fatalf("round trip lost state")
	}
}
