//go:build integration

package app_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

var applicableCriteria = []string{"H1", "H2", "H3", "H4", "H5", "H6", "H7", "H8", "H10", "H11", "H12"}

func declarationOf(capabilityID, supersedes string) *servicev1.DeclareCapabilityRequest {
	return &servicev1.DeclareCapabilityRequest{
		CapabilityId: capabilityID, PartnerId: "e2e-prt-1", Operation: "register-receivable", AssetType: "duplicata",
		Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, IsWrite: true, StateQuerySupported: true, SupersedesId: supersedes,
	}
}

func passed(criteria ...string) []*servicev1.HomologationResult {
	out := make([]*servicev1.HomologationResult, 0, len(criteria))
	for _, criterion := range criteria {
		out = append(out, &servicev1.HomologationResult{CriterionId: criterion, Passed: true, EvidenceId: "evd-" + criterion})
	}
	return out
}

func stageTo(target servicev1.PartnerStatus, reason string) *servicev1.ChangePartnerStageRequest {
	return &servicev1.ChangePartnerStageRequest{PartnerId: "e2e-prt-1", Target: target, Reason: reason}
}

func changeStage(t *testing.T, conn *grpc.ClientConn, target servicev1.PartnerStatus, reason string) *servicev1.ChangePartnerStageResponse {
	t.Helper()
	return invoke(t, conn, call(t), "ChangePartnerStage", stageTo(target, reason), &servicev1.ChangePartnerStageResponse{})
}

func homologatingPartner(t *testing.T, conn *grpc.ClientConn, pool *pgxpool.Pool) {
	t.Helper()
	invoke(t, conn, call(t), "RegisterPartner", registration("e2e-prt-1"), &servicev1.RegisterPartnerResponse{})
	for _, target := range []servicev1.PartnerStatus{
		servicev1.PartnerStatus_PARTNER_STATUS_CONTRACTING,
		servicev1.PartnerStatus_PARTNER_STATUS_INTEGRATING,
		servicev1.PartnerStatus_PARTNER_STATUS_HOMOLOGATING,
	} {
		if resp := changeStage(t, conn, target, ""); resp.GetChanged().GetStatus() != target {
			t.Fatalf("setup: ChangePartnerStage(%s) = %v", target, resp)
		}
	}
	invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-1", ""), &servicev1.RegisterAgreementResponse{})
	invoke(t, conn, call(t), "ActivateAgreement", activation("e2e-agr-1"), &servicev1.ActivateAgreementResponse{})
	invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-1", 90*day, ""), &servicev1.RegisterIntegrationProfileResponse{})
	if _, err := pool.Exec(t.Context(), "DELETE FROM outbox"); err != nil {
		t.Fatalf("setup: clear outbox: %v", err)
	}
}

func homologatedCapability(t *testing.T, conn *grpc.ClientConn, capabilityID string) {
	t.Helper()
	invoke(t, conn, call(t), "DeclareCapability", declarationOf(capabilityID, ""), &servicev1.DeclareCapabilityResponse{})
	resp := invoke(t, conn, call(t), "RecordHomologation",
		&servicev1.RecordHomologationRequest{CapabilityId: capabilityID, Results: passed(applicableCriteria...)}, &servicev1.RecordHomologationResponse{})
	if resp.GetRecorded().GetStatus() != servicev1.CapabilityStatus_CAPABILITY_STATUS_HOMOLOGATED {
		t.Fatalf("setup: RecordHomologation() = %v, want homologated", resp)
	}
}

func lastRecord(t *testing.T, pool *pgxpool.Pool) enqueued {
	t.Helper()
	records := outbox(t, pool)
	if len(records) == 0 {
		t.Fatal("outbox is empty, want a record")
	}
	return records[len(records)-1]
}

func TestTheCapabilityAndThePartnerWalkTheirLifecycleEndToEnd(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	homologatingPartner(t, conn, pool)
	homologatedCapability(t, conn, "e2e-cap-1")

	if got := count(t, pool, "SELECT count(*) FROM outbox"); got != 0 {
		t.Fatalf("outbox = %d after declaring and homologating, want nothing emitted", got)
	}
	enabled := invoke(t, conn, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
	if enabled.GetEnabled().GetCapabilityId() != "e2e-cap-1" {
		t.Fatalf("EnableCapability() = %v, want e2e-cap-1 enabled", enabled)
	}
	record := lastRecord(t, pool)
	var capabilityEnabled eventv1.CapabilityEnabled
	if err := proto.Unmarshal(record.payload, &capabilityEnabled); err != nil || record.messageType != "com.company.partner.capability-enabled.v1" ||
		record.partitionKey != "e2e-cap-1" || record.destination != application.Destination {
		t.Fatalf("outbox record = %s keyed %s (%v), want the capability-enabled keyed by the capability", record.messageType, record.partitionKey, err)
	}
	if capabilityEnabled.GetEnvironment() != eventv1.Environment_ENVIRONMENT_PRODUCTION || capabilityEnabled.GetVersion() != 1 ||
		capabilityEnabled.GetRolloutMode() != eventv1.RolloutMode_ROLLOUT_MODE_DISABLED || !capabilityEnabled.GetStateQuerySupported() {
		t.Fatalf("event = %v, want version 1 in production, rollout disabled and the state query", &capabilityEnabled)
	}

	if resp := changeStage(t, conn, servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, ""); resp.GetChanged().GetStatus() != servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE {
		t.Fatalf("ChangePartnerStage(active) = %v, want active", resp)
	}
	if got := count(t, pool, "SELECT count(*) FROM outbox"); got != 1 {
		t.Fatalf("outbox = %d after the activation of the partner, want nothing more emitted", got)
	}

	rollout := &servicev1.ChangeRolloutRequest{
		CapabilityId: "e2e-cap-1", Mode: servicev1.RolloutMode_ROLLOUT_MODE_PERCENTAGE, Percentage: 10, ExcludedTenantIds: []string{"tenant-x"},
	}
	invoke(t, conn, call(t), "ChangeRollout", rollout, &servicev1.ChangeRolloutResponse{})
	record = lastRecord(t, pool)
	var rolloutChanged eventv1.CapabilityRolloutChanged
	if err := proto.Unmarshal(record.payload, &rolloutChanged); err != nil || record.messageType != "com.company.partner.capability-rollout-changed.v1" ||
		rolloutChanged.GetMode() != eventv1.RolloutMode_ROLLOUT_MODE_PERCENTAGE || rolloutChanged.GetPercentage() != 10 || record.partitionKey != "e2e-cap-1" {
		t.Fatalf("outbox record = %s %v (%v), want the rollout changed to 10 percent", record.messageType, &rolloutChanged, err)
	}
	if got := count(t, pool, "SELECT count(*) FROM capabilities WHERE snapshot->'rollout'->>'mode' = 'percentage' AND snapshot->'rollout'->'excludedTenantIds' = '[\"tenant-x\"]'::jsonb"); got != 1 {
		t.Fatalf("capabilities with the rollout of 10 percent = %d, want 1", got)
	}

	invoke(t, conn, call(t), "SuspendCapability", &servicev1.SuspendCapabilityRequest{CapabilityId: "e2e-cap-1", Reason: "partner incident"}, &servicev1.SuspendCapabilityResponse{})
	record = lastRecord(t, pool)
	var suspended eventv1.CapabilitySuspended
	if err := proto.Unmarshal(record.payload, &suspended); err != nil || record.messageType != "com.company.partner.capability-suspended.v1" ||
		suspended.GetTrigger() != eventv1.SuspensionTrigger_SUSPENSION_TRIGGER_MANUAL || suspended.GetEnvironment() != eventv1.Environment_ENVIRONMENT_PRODUCTION ||
		suspended.GetVersion() != 1 || suspended.GetReason() != "partner incident" {
		t.Fatalf("outbox record = %s %v (%v), want the manual suspension with environment and version", record.messageType, &suspended, err)
	}

	changeStage(t, conn, servicev1.PartnerStatus_PARTNER_STATUS_SUSPENDED, "contract under review")
	if resp := changeStage(t, conn, servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, ""); resp.GetChanged().GetStatus() != servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE {
		t.Fatalf("ChangePartnerStage(active) from suspended = %v, want active with the capability still suspended", resp)
	}
	if got := count(t, pool, "SELECT count(*) FROM capabilities WHERE status = 'suspended'"); got != 1 {
		t.Fatalf("suspended capabilities = %d after the partner went back to active, want the capability untouched", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM outbox"); got != 3 {
		t.Fatalf("outbox = %d, want the suspension and the return of the partner to emit nothing", got)
	}

	resumed := invoke(t, conn, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
	if resumed.GetEnabled().GetCapabilityId() != "e2e-cap-1" || lastRecord(t, pool).messageType != "com.company.partner.capability-enabled.v1" {
		t.Fatalf("EnableCapability() after a manual suspension = %v, want the resumption without a new homologation", resumed)
	}

	changeStage(t, conn, servicev1.PartnerStatus_PARTNER_STATUS_OFFBOARDING, "contract terminated")
	record = lastRecord(t, pool)
	if record.messageType != "com.company.partner.partner-offboarding-started.v1" || record.partitionKey != "e2e-prt-1" {
		t.Fatalf("outbox record = %s keyed %s, want the offboarding started keyed by the partner", record.messageType, record.partitionKey)
	}
	if got := count(t, pool, "SELECT count(*) FROM capabilities WHERE status = 'enabled'"); got != 1 {
		t.Fatalf("enabled capabilities = %d after the offboarding, want the capability untouched", got)
	}
}

func TestEachRefusalOfTheStageAndTheCapabilityWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterPartner", registration("e2e-prt-1"), &servicev1.RegisterPartnerResponse{})
	invoke(t, conn, call(t), "RegisterPartner", &servicev1.RegisterPartnerRequest{
		PartnerId: "e2e-prt-2", OrganizationId: "org-funder", Role: servicev1.PartnerRole_PARTNER_ROLE_REGISTRY,
	}, &servicev1.RegisterPartnerResponse{})
	invoke(t, conn, call(t), "DeclareCapability", declarationOf("e2e-cap-declared", ""), &servicev1.DeclareCapabilityResponse{})
	other := declarationOf("e2e-cap-other", "")
	other.Environment = servicev1.Environment_ENVIRONMENT_HOMOLOGATION
	invoke(t, conn, call(t), "DeclareCapability", other, &servicev1.DeclareCapabilityResponse{})
	homologated := declarationOf("e2e-cap-homologated", "")
	homologated.Operation = "settle"
	invoke(t, conn, call(t), "DeclareCapability", homologated, &servicev1.DeclareCapabilityResponse{})
	invoke(t, conn, call(t), "RecordHomologation",
		&servicev1.RecordHomologationRequest{CapabilityId: "e2e-cap-homologated", Results: passed(applicableCriteria...)}, &servicev1.RecordHomologationResponse{})
	events := count(t, pool, "SELECT count(*) FROM outbox")

	stage := func(partnerID string, target servicev1.PartnerStatus, reason string) func() string {
		return func() string {
			req := &servicev1.ChangePartnerStageRequest{PartnerId: partnerID, Target: target, Reason: reason}
			return invoke(t, conn, call(t), "ChangePartnerStage", req, &servicev1.ChangePartnerStageResponse{}).GetRejection().GetCode()
		}
	}
	declare := func(edit func(*servicev1.DeclareCapabilityRequest)) func() string {
		return func() string {
			req := declarationOf("e2e-cap-x", "")
			edit(req)
			return invoke(t, conn, call(t), "DeclareCapability", req, &servicev1.DeclareCapabilityResponse{}).GetRejection().GetCode()
		}
	}
	record := func(capabilityID string, results []*servicev1.HomologationResult) func() string {
		return func() string {
			req := &servicev1.RecordHomologationRequest{CapabilityId: capabilityID, Results: results}
			return invoke(t, conn, call(t), "RecordHomologation", req, &servicev1.RecordHomologationResponse{}).GetRejection().GetCode()
		}
	}
	enable := func(capabilityID string) func() string {
		return func() string {
			req := &servicev1.EnableCapabilityRequest{CapabilityId: capabilityID}
			return invoke(t, conn, call(t), "EnableCapability", req, &servicev1.EnableCapabilityResponse{}).GetRejection().GetCode()
		}
	}
	for _, c := range []struct {
		name string
		got  func() string
		code string
	}{
		{"prospect to active", stage("e2e-prt-1", servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, ""), "partner-management/partner/invalid-stage-transition"},
		{"operation empty", declare(func(r *servicev1.DeclareCapabilityRequest) { r.Operation = "" }), "partner-management/capability/operation-empty"},
		{"asset type empty", declare(func(r *servicev1.DeclareCapabilityRequest) { r.AssetType = "" }), "partner-management/capability/asset-type-empty"},
		{"write without state query nor restriction", declare(func(r *servicev1.DeclareCapabilityRequest) { r.StateQuerySupported = false }),
			"partner-management/capability/state-query-restriction-undeclared"},
		{"another first version of the series", declare(func(*servicev1.DeclareCapabilityRequest) {}), "partner-management/capability/supersedes-invalid"},
		{"supersedes a capability of another series", declare(func(r *servicev1.DeclareCapabilityRequest) { r.SupersedesId = "e2e-cap-other" }),
			"partner-management/capability/supersedes-invalid"},
		{"results empty", record("e2e-cap-declared", nil), "partner-management/capability/criteria-empty"},
		{"criterion H13", record("e2e-cap-declared", passed("H13")), "partner-management/capability/unknown-criterion"},
		{"already homologated", record("e2e-cap-homologated", passed("H1")), "partner-management/capability/homologation-closed"},
		{"enabling a declared capability", enable("e2e-cap-declared"), "partner-management/capability/not-homologated"},
		{"enabling without an agreement", enable("e2e-cap-homologated"), "partner-management/capability/agreement-not-active"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.got(); got != c.code {
				t.Fatalf("rejection = %q, want %q", got, c.code)
			}
			if capabilities, outboxed := count(t, pool, "SELECT count(*) FROM capabilities"), count(t, pool, "SELECT count(*) FROM outbox"); capabilities != 3 || outboxed != events {
				t.Fatalf("capabilities = %d, outbox = %d after a refusal, want the three held and %d events", capabilities, outboxed, events)
			}
			if got := count(t, pool, "SELECT count(*) FROM capabilities WHERE capability_id = 'e2e-cap-declared' AND status = 'declared' AND snapshot->'homologation' = '[]'::jsonb"); got != 1 {
				t.Fatalf("e2e-cap-declared changed after a refusal")
			}
		})
	}
}

func TestTheAgreementAndTheProfileGateTheEnablingEndToEnd(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	homologatedCapability(t, conn, "e2e-cap-1")
	invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-1", ""), &servicev1.RegisterAgreementResponse{})
	invoke(t, conn, call(t), "ActivateAgreement", activation("e2e-agr-1"), &servicev1.ActivateAgreementResponse{})
	other := profileOf("e2e-ipf-h1", 90*day, "")
	other.Environment = servicev1.Environment_ENVIRONMENT_HOMOLOGATION
	invoke(t, conn, call(t), "RegisterIntegrationProfile", other, &servicev1.RegisterIntegrationProfileResponse{})

	refused := invoke(t, conn, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
	if got := refused.GetRejection().GetCode(); got != "partner-management/capability/integration-profile-missing" {
		t.Fatalf("EnableCapability() without a production profile = %v, want integration-profile-missing", refused)
	}

	invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-1", 90*day, ""), &servicev1.RegisterIntegrationProfileResponse{})
	enabled := invoke(t, conn, call(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "e2e-cap-1"}, &servicev1.EnableCapabilityResponse{})
	if enabled.GetEnabled().GetCapabilityId() != "e2e-cap-1" {
		t.Fatalf("EnableCapability() with the production profile = %v, want enabled", enabled)
	}
}

func TestThePartnerReachesActiveOnlyWithAnAgreementAndAnEnabledCapability(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterPartner", registration("e2e-prt-1"), &servicev1.RegisterPartnerResponse{})
	for _, target := range []servicev1.PartnerStatus{
		servicev1.PartnerStatus_PARTNER_STATUS_CONTRACTING,
		servicev1.PartnerStatus_PARTNER_STATUS_INTEGRATING,
		servicev1.PartnerStatus_PARTNER_STATUS_HOMOLOGATING,
	} {
		changeStage(t, conn, target, "")
	}

	if got := changeStage(t, conn, servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, "").GetRejection().GetCode(); got != "partner-management/partner/agreement-not-active" {
		t.Fatalf("ChangePartnerStage(active) without an agreement = %q, want agreement-not-active", got)
	}
	invoke(t, conn, call(t), "RegisterAgreement", agreementOf("e2e-agr-1", ""), &servicev1.RegisterAgreementResponse{})
	invoke(t, conn, call(t), "ActivateAgreement", activation("e2e-agr-1"), &servicev1.ActivateAgreementResponse{})
	homologatedCapability(t, conn, "e2e-cap-1")
	if got := changeStage(t, conn, servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE, "").GetRejection().GetCode(); got != "partner-management/partner/no-enabled-capability" {
		t.Fatalf("ChangePartnerStage(active) without an enabled capability = %q, want no-enabled-capability", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM partners WHERE status = 'homologating'"); got != 1 {
		t.Fatalf("homologating partners = %d after the refusals, want the partner untouched", got)
	}
}

func TestTwoConcurrentFirstVersionsOfOneCapabilitySeriesLeaveOneCapability(t *testing.T) {
	pool := appkit.OpenPool(t)
	saves := newBarrier(2)
	conn := dial(t, pool, saves)

	got := invokeAtOnce(t, conn, "DeclareCapability",
		map[string]proto.Message{"first": declarationOf("e2e-cap-a1", ""), "second": declarationOf("e2e-cap-a2", "")},
		func(m proto.Message) bool { return m.(*servicev1.DeclareCapabilityResponse).GetDeclared() != nil },
		func() proto.Message { return &servicev1.DeclareCapabilityResponse{} })

	oneAcceptedOneRejected(t, got, string(domain.CodeCapabilitySupersedesInvalid))
	refusedByUniqueness(t, saves, "capabilities_tenant_id_series_origin_key")
	if capabilities := count(t, pool, "SELECT count(*) FROM capabilities"); capabilities != 1 {
		t.Fatalf("capabilities = %d, want exactly one first version", capabilities)
	}
}

func TestDeclareCapabilityEndToEndKeepsOneSeriesOfVersions(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	first := invoke(t, conn, call(t), "DeclareCapability", declarationOf("e2e-cap-1", ""), &servicev1.DeclareCapabilityResponse{})
	second := invoke(t, conn, call(t), "DeclareCapability", declarationOf("e2e-cap-2", "e2e-cap-1"), &servicev1.DeclareCapabilityResponse{})
	stale := invoke(t, conn, call(t), "DeclareCapability", declarationOf("e2e-cap-3", "e2e-cap-1"), &servicev1.DeclareCapabilityResponse{})

	if first.GetDeclared().GetCapabilityId() != "e2e-cap-1" || second.GetDeclared().GetCapabilityId() != "e2e-cap-2" {
		t.Fatalf("DeclareCapability() = %v and %v, want versions 1 and 2 declared", first, second)
	}
	if got := stale.GetRejection().GetCode(); got != "partner-management/capability/supersedes-invalid" {
		t.Fatalf("DeclareCapability(stale supersedes) = %v, want supersedes-invalid", stale)
	}
	if got := count(t, pool, "SELECT count(*) FROM capabilities WHERE status = 'declared' AND (snapshot->>'version')::int = 2 AND supersedes_id = 'e2e-cap-1' AND snapshot->'rollout'->>'mode' = 'disabled'"); got != 1 {
		t.Fatalf("declared versions 2 superseding e2e-cap-1 with the rollout disabled = %d, want 1", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM outbox"); got != 0 {
		t.Fatalf("outbox = %d, want nothing emitted by a declaration", got)
	}
}
