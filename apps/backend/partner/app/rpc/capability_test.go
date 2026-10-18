package rpc_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func partnerAt(id domain.PartnerID, s domain.PartnerStatus) domain.PartnerSnapshot {
	return domain.PartnerSnapshot{ID: id, OrganizationID: "org-1", Role: domain.RoleFunder, Status: s, RegisteredAt: occurred, StageChangedAt: occurred}
}

func capabilityAt(id domain.CapabilityID, s domain.CapabilityStatus) domain.CapabilitySnapshot {
	return domain.CapabilitySnapshot{
		ID: id, PartnerID: "prt-1", Operation: "register-receivable", AssetType: "duplicata", Environment: domain.EnvironmentProduction,
		Version: 1, IsWrite: true, StateQuerySupported: true, Status: s, Rollout: domain.Rollout{Mode: domain.RolloutDisabled},
	}
}

func activeAgreement() domain.PartnerAgreementSnapshot {
	a := draft("agr-1")
	a.Status, a.ContractEvidenceID, a.ActivatedAt = domain.AgreementActive, "evd-1", occurred
	return a
}

func declareRequest(capabilityID string) *servicev1.DeclareCapabilityRequest {
	return &servicev1.DeclareCapabilityRequest{
		CapabilityId: capabilityID, PartnerId: "prt-1", Operation: "register-receivable", AssetType: "duplicata",
		Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, IsWrite: true, StateQuerySupported: false,
		RestrictionNote: "reconciled by the daily file", CallbackApplicable: true,
	}
}

func every(criteria ...string) []*servicev1.HomologationResult {
	out := make([]*servicev1.HomologationResult, 0, len(criteria))
	for _, criterion := range criteria {
		out = append(out, &servicev1.HomologationResult{CriterionId: criterion, Passed: true, EvidenceId: "evd-" + criterion})
	}
	return out
}

func TestChangePartnerStageAnswersTheNewStageAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{
		agreements:   agreementsByPartner{"prt-1": {activeAgreement()}},
		capabilities: capabilitiesByPartner{"prt-1": {capabilityAt("cap-1", domain.CapabilityStatusEnabled)}},
	})
	seedIn(t, h, partnersTable, "prt-1", partnerAt("prt-1", domain.PartnerHomologating))
	seedIn(t, h, partnersTable, "prt-2", partnerAt("prt-2", domain.PartnerProspect))
	seedIn(t, h, partnersTable, "prt-3", partnerAt("prt-3", domain.PartnerHomologating))

	var changed servicev1.ChangePartnerStageResponse
	if err := h.invoke(withTenant(t), "ChangePartnerStage", &servicev1.ChangePartnerStageRequest{PartnerId: "prt-1", Target: servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE}, &changed); err != nil {
		t.Fatalf("ChangePartnerStage() = %v", err)
	}
	if got := changed.GetChanged(); got.GetPartnerId() != "prt-1" || got.GetStatus() != servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE {
		t.Fatalf("Changed = %v, want prt-1 active (response %v)", got, &changed)
	}

	for name, c := range map[string]struct {
		req  *servicev1.ChangePartnerStageRequest
		code string
	}{
		"prospect to active": {&servicev1.ChangePartnerStageRequest{PartnerId: "prt-2", Target: servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE},
			"partner-management/partner/invalid-stage-transition"},
		"suspension without reason": {&servicev1.ChangePartnerStageRequest{PartnerId: "prt-1", Target: servicev1.PartnerStatus_PARTNER_STATUS_SUSPENDED},
			"partner-management/partner/reason-empty"},
		"active without an agreement": {&servicev1.ChangePartnerStageRequest{PartnerId: "prt-3", Target: servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE},
			"partner-management/partner/agreement-not-active"},
	} {
		var resp servicev1.ChangePartnerStageResponse
		if err := h.invoke(withTenant(t), "ChangePartnerStage", c.req, &resp); err != nil {
			t.Fatalf("%s: ChangePartnerStage() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}

	err := h.invoke(withTenant(t), "ChangePartnerStage", &servicev1.ChangePartnerStageRequest{PartnerId: "prt-none", Target: servicev1.PartnerStatus_PARTNER_STATUS_CONTRACTING},
		&servicev1.ChangePartnerStageResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("ChangePartnerStage(prt-none) = %v, want NotFound", err)
	}
}

func TestOffboardingAPartnerOverTheServiceEmitsTheOffboardingStarted(t *testing.T) {
	h := newHarness(t, relations{})
	seedIn(t, h, partnersTable, "prt-1", partnerAt("prt-1", domain.PartnerActive))

	var resp servicev1.ChangePartnerStageResponse
	req := &servicev1.ChangePartnerStageRequest{PartnerId: "prt-1", Target: servicev1.PartnerStatus_PARTNER_STATUS_OFFBOARDING, Reason: "contract terminated"}
	if err := h.invoke(withTenant(t), "ChangePartnerStage", req, &resp); err != nil {
		t.Fatalf("ChangePartnerStage() = %v", err)
	}

	if resp.GetChanged().GetStatus() != servicev1.PartnerStatus_PARTNER_STATUS_OFFBOARDING || len(h.store.Entries()) != 1 {
		t.Fatalf("response %v with %d outbox entries, want offboarding and the PartnerOffboardingStarted", &resp, len(h.store.Entries()))
	}
}

func TestDeclareCapabilityAnswersTheDeclaredCapabilityAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{capabilities: capabilitiesByPartner{"prt-2": {capabilityAt("cap-0", domain.CapabilityStatusDeclared)}}})

	var declared servicev1.DeclareCapabilityResponse
	if err := h.invoke(withTenant(t), "DeclareCapability", declareRequest("cap-1"), &declared); err != nil {
		t.Fatalf("DeclareCapability() = %v", err)
	}
	if got := declared.GetDeclared().GetCapabilityId(); got != "cap-1" || len(h.store.Entries()) != 0 {
		t.Fatalf("Declared.CapabilityId = %q with %d outbox entries, want cap-1 and nothing emitted", got, len(h.store.Entries()))
	}

	for name, c := range map[string]struct {
		edit func(*servicev1.DeclareCapabilityRequest)
		code string
	}{
		"operation empty":  {func(r *servicev1.DeclareCapabilityRequest) { r.Operation = "" }, "partner-management/capability/operation-empty"},
		"asset type empty": {func(r *servicev1.DeclareCapabilityRequest) { r.AssetType = "" }, "partner-management/capability/asset-type-empty"},
		"write without state query nor restriction": {func(r *servicev1.DeclareCapabilityRequest) { r.RestrictionNote = "" },
			"partner-management/capability/state-query-restriction-undeclared"},
		"another first version of the series": {func(r *servicev1.DeclareCapabilityRequest) { r.PartnerId = "prt-2" },
			"partner-management/capability/supersedes-invalid"},
	} {
		req := declareRequest("cap-x")
		c.edit(req)
		var resp servicev1.DeclareCapabilityResponse
		if err := h.invoke(withTenant(t), "DeclareCapability", req, &resp); err != nil {
			t.Fatalf("%s: DeclareCapability() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}
}

func TestRecordHomologationAnswersTheStatusAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{})
	seedIn(t, h, capsTable, "cap-1", capabilityAt("cap-1", domain.CapabilityStatusDeclared))
	seedIn(t, h, capsTable, "cap-2", capabilityAt("cap-2", domain.CapabilityStatusHomologated))
	seedIn(t, h, capsTable, "cap-3", capabilityAt("cap-3", domain.CapabilityStatusDeclared))

	var recorded servicev1.RecordHomologationResponse
	req := &servicev1.RecordHomologationRequest{CapabilityId: "cap-1", Results: every("H1", "H2", "H3", "H4", "H5", "H6", "H7", "H8", "H10", "H11", "H12")}
	if err := h.invoke(withTenant(t), "RecordHomologation", req, &recorded); err != nil {
		t.Fatalf("RecordHomologation() = %v", err)
	}
	if got := recorded.GetRecorded(); got.GetCapabilityId() != "cap-1" || got.GetStatus() != servicev1.CapabilityStatus_CAPABILITY_STATUS_HOMOLOGATED {
		t.Fatalf("Recorded = %v, want cap-1 homologated", got)
	}

	for name, c := range map[string]struct {
		req  *servicev1.RecordHomologationRequest
		code string
	}{
		"results empty":           {&servicev1.RecordHomologationRequest{CapabilityId: "cap-3"}, "partner-management/capability/criteria-empty"},
		"criterion H13":           {&servicev1.RecordHomologationRequest{CapabilityId: "cap-3", Results: every("H13")}, "partner-management/capability/unknown-criterion"},
		"criterion of no catalog": {&servicev1.RecordHomologationRequest{CapabilityId: "cap-3", Results: every("X1")}, "partner-management/capability/unknown-criterion"},
		"already homologated":     {&servicev1.RecordHomologationRequest{CapabilityId: "cap-2", Results: every("H1")}, "partner-management/capability/homologation-closed"},
	} {
		var resp servicev1.RecordHomologationResponse
		if err := h.invoke(withTenant(t), "RecordHomologation", c.req, &resp); err != nil {
			t.Fatalf("%s: RecordHomologation() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}
}

func TestEnableCapabilityAnswersTheEnablingAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{
		agreements: agreementsByPartner{"prt-1": {activeAgreement()}},
		profiles:   profilesByPartner{"prt-1": {expiringIn("ipf-1", 90*day, 0)}},
	})
	seedIn(t, h, capsTable, "cap-1", capabilityAt("cap-1", domain.CapabilityStatusHomologated))
	seedIn(t, h, capsTable, "cap-2", capabilityAt("cap-2", domain.CapabilityStatusDeclared))
	mismatch := capabilityAt("cap-3", domain.CapabilityStatusSuspended)
	mismatch.Suspension = domain.Suspension{Reason: "schema v4", Trigger: domain.TriggerSchemaMismatch, SuspendedAt: occurred - port.Instant(day)}
	seedIn(t, h, capsTable, "cap-3", mismatch)
	elsewhere := capabilityAt("cap-4", domain.CapabilityStatusHomologated)
	elsewhere.PartnerID = "prt-2"
	seedIn(t, h, capsTable, "cap-4", elsewhere)

	var enabled servicev1.EnableCapabilityResponse
	if err := h.invoke(withTenant(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "cap-1"}, &enabled); err != nil {
		t.Fatalf("EnableCapability() = %v", err)
	}
	if got := enabled.GetEnabled().GetCapabilityId(); got != "cap-1" || len(h.store.Entries()) != 1 {
		t.Fatalf("Enabled.CapabilityId = %q with %d outbox entries, want cap-1 and the CapabilityEnabled", got, len(h.store.Entries()))
	}

	for id, code := range map[string]string{
		"cap-2": "partner-management/capability/not-homologated",
		"cap-3": "partner-management/capability/rehomologation-required",
		"cap-4": "partner-management/capability/agreement-not-active",
	} {
		var resp servicev1.EnableCapabilityResponse
		if err := h.invoke(withTenant(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: id}, &resp); err != nil {
			t.Fatalf("%s: EnableCapability() = %v", id, err)
		}
		if got := resp.GetRejection().GetCode(); got != code {
			t.Fatalf("%s: rejection code = %q, want %q", id, got, code)
		}
	}
}

func TestEnablingWithoutAProfileOfTheEnvironmentIsRefused(t *testing.T) {
	h := newHarness(t, relations{agreements: agreementsByPartner{"prt-1": {activeAgreement()}}})
	seedIn(t, h, capsTable, "cap-1", capabilityAt("cap-1", domain.CapabilityStatusHomologated))

	var resp servicev1.EnableCapabilityResponse
	if err := h.invoke(withTenant(t), "EnableCapability", &servicev1.EnableCapabilityRequest{CapabilityId: "cap-1"}, &resp); err != nil {
		t.Fatalf("EnableCapability() = %v", err)
	}

	if got := resp.GetRejection().GetCode(); got != "partner-management/capability/integration-profile-missing" {
		t.Fatalf("rejection code = %q, want integration-profile-missing", got)
	}
}

func TestSuspendCapabilityOverTheServiceSuspendsByHand(t *testing.T) {
	h := newHarness(t, relations{})
	seedIn(t, h, capsTable, "cap-1", capabilityAt("cap-1", domain.CapabilityStatusEnabled))
	seedIn(t, h, capsTable, "cap-2", capabilityAt("cap-2", domain.CapabilityStatusDeclared))

	var suspended servicev1.SuspendCapabilityResponse
	if err := h.invoke(withTenant(t), "SuspendCapability", &servicev1.SuspendCapabilityRequest{CapabilityId: "cap-1", Reason: "partner incident"}, &suspended); err != nil {
		t.Fatalf("SuspendCapability() = %v", err)
	}
	entries := h.store.Entries()
	if suspended.GetSuspended().GetCapabilityId() != "cap-1" || len(entries) != 1 {
		t.Fatalf("response %v with %d outbox entries, want cap-1 suspended and one CapabilitySuspended", &suspended, len(entries))
	}
	if event, ok := entries[0].Event.(domain.CapabilitySuspended); !ok || event.Trigger != domain.TriggerManual {
		t.Fatalf("event = %+v, want the manual trigger the service fixes", entries[0].Event)
	}

	for name, c := range map[string]struct {
		req  *servicev1.SuspendCapabilityRequest
		code string
	}{
		"capability declared":           {&servicev1.SuspendCapabilityRequest{CapabilityId: "cap-2", Reason: "partner incident"}, "partner-management/capability/not-enabled"},
		"declared and without a reason": {&servicev1.SuspendCapabilityRequest{CapabilityId: "cap-2"}, "partner-management/capability/not-enabled"},
	} {
		var resp servicev1.SuspendCapabilityResponse
		if err := h.invoke(withTenant(t), "SuspendCapability", c.req, &resp); err != nil {
			t.Fatalf("%s: SuspendCapability() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}
}

func TestSuspendingWithoutAReasonIsRefused(t *testing.T) {
	h := newHarness(t, relations{})
	seedIn(t, h, capsTable, "cap-1", capabilityAt("cap-1", domain.CapabilityStatusEnabled))

	var resp servicev1.SuspendCapabilityResponse
	if err := h.invoke(withTenant(t), "SuspendCapability", &servicev1.SuspendCapabilityRequest{CapabilityId: "cap-1", Reason: " "}, &resp); err != nil {
		t.Fatalf("SuspendCapability() = %v", err)
	}

	if got := resp.GetRejection().GetCode(); got != "partner-management/capability/reason-empty" {
		t.Fatalf("rejection code = %q, want reason-empty", got)
	}
}

func TestChangeRolloutAnswersTheChangeAndEachRefusal(t *testing.T) {
	h := newHarness(t, relations{})
	seedIn(t, h, capsTable, "cap-1", capabilityAt("cap-1", domain.CapabilityStatusEnabled))

	var changed servicev1.ChangeRolloutResponse
	req := &servicev1.ChangeRolloutRequest{
		CapabilityId: "cap-1", Mode: servicev1.RolloutMode_ROLLOUT_MODE_PERCENTAGE, Percentage: 10, ExcludedTenantIds: []string{"tenant-x"},
	}
	if err := h.invoke(withTenant(t), "ChangeRollout", req, &changed); err != nil {
		t.Fatalf("ChangeRollout() = %v", err)
	}
	if got := changed.GetChanged().GetCapabilityId(); got != "cap-1" || len(h.store.Entries()) != 1 {
		t.Fatalf("Changed.CapabilityId = %q with %d outbox entries, want cap-1 and the CapabilityRolloutChanged", got, len(h.store.Entries()))
	}

	for name, c := range map[string]struct {
		req  *servicev1.ChangeRolloutRequest
		code string
	}{
		"percentage 101": {&servicev1.ChangeRolloutRequest{CapabilityId: "cap-1", Mode: servicev1.RolloutMode_ROLLOUT_MODE_PERCENTAGE, Percentage: 101},
			"partner-management/capability/percentage-out-of-range"},
		"allowlist empty": {&servicev1.ChangeRolloutRequest{CapabilityId: "cap-1", Mode: servicev1.RolloutMode_ROLLOUT_MODE_ALLOWLIST},
			"partner-management/capability/allowlist-empty"},
	} {
		var resp servicev1.ChangeRolloutResponse
		if err := h.invoke(withTenant(t), "ChangeRollout", c.req, &resp); err != nil {
			t.Fatalf("%s: ChangeRollout() = %v", name, err)
		}
		if got := resp.GetRejection().GetCode(); got != c.code {
			t.Fatalf("%s: rejection code = %q, want %q", name, got, c.code)
		}
	}
}

func TestMalformedStageAndCapabilityRequestsAreRefusedBeforeTheUseCase(t *testing.T) {
	h := newHarness(t, relations{})
	long := strings.Repeat("a", 129)
	tooManyTenants := make([]string, 1001)
	for i := range tooManyTenants {
		tooManyTenants[i] = fmt.Sprintf("tenant-%d", i)
	}
	stage := func(edit func(*servicev1.ChangePartnerStageRequest)) func() error {
		return func() error {
			req := &servicev1.ChangePartnerStageRequest{PartnerId: "prt-1", Target: servicev1.PartnerStatus_PARTNER_STATUS_SUSPENDED, Reason: "under review"}
			edit(req)
			return h.invoke(withTenant(t), "ChangePartnerStage", req, &servicev1.ChangePartnerStageResponse{})
		}
	}
	declare := func(edit func(*servicev1.DeclareCapabilityRequest)) func() error {
		return func() error {
			req := declareRequest("cap-1")
			edit(req)
			return h.invoke(withTenant(t), "DeclareCapability", req, &servicev1.DeclareCapabilityResponse{})
		}
	}
	homologation := func(edit func(*servicev1.RecordHomologationRequest)) func() error {
		return func() error {
			req := &servicev1.RecordHomologationRequest{CapabilityId: "cap-1", Results: every("H1")}
			edit(req)
			return h.invoke(withTenant(t), "RecordHomologation", req, &servicev1.RecordHomologationResponse{})
		}
	}
	rollout := func(edit func(*servicev1.ChangeRolloutRequest)) func() error {
		return func() error {
			req := &servicev1.ChangeRolloutRequest{CapabilityId: "cap-1", Mode: servicev1.RolloutMode_ROLLOUT_MODE_ALLOWLIST, AllowlistTenantIds: []string{"tenant-a"}}
			edit(req)
			return h.invoke(withTenant(t), "ChangeRollout", req, &servicev1.ChangeRolloutResponse{})
		}
	}
	for _, c := range []struct {
		name string
		call func() error
	}{
		{"stage of no partner", stage(func(r *servicev1.ChangePartnerStageRequest) { r.PartnerId = "" })},
		{"unspecified target", stage(func(r *servicev1.ChangePartnerStageRequest) {
			r.Target = servicev1.PartnerStatus_PARTNER_STATUS_UNSPECIFIED
		})},
		{"prospect as a target", stage(func(r *servicev1.ChangePartnerStageRequest) {
			r.Target = servicev1.PartnerStatus_PARTNER_STATUS_PROSPECT
		})},
		{"target outside the enum", stage(func(r *servicev1.ChangePartnerStageRequest) { r.Target = 99 })},
		{"reason with a control character", stage(func(r *servicev1.ChangePartnerStageRequest) { r.Reason = "under\nreview" })},
		{"capability id too long", declare(func(r *servicev1.DeclareCapabilityRequest) { r.CapabilityId = long })},
		{"declaration of no partner", declare(func(r *servicev1.DeclareCapabilityRequest) { r.PartnerId = "" })},
		{"operation outside printable ASCII", declare(func(r *servicev1.DeclareCapabilityRequest) { r.Operation = "registro-recebível" })},
		{"asset type too long", declare(func(r *servicev1.DeclareCapabilityRequest) { r.AssetType = strings.Repeat("a", 65) })},
		{"unspecified environment", declare(func(r *servicev1.DeclareCapabilityRequest) {
			r.Environment = servicev1.Environment_ENVIRONMENT_UNSPECIFIED
		})},
		{"restriction note too long", declare(func(r *servicev1.DeclareCapabilityRequest) { r.RestrictionNote = strings.Repeat("a", 513) })},
		{"supersedes with a space", declare(func(r *servicev1.DeclareCapabilityRequest) { r.SupersedesId = "cap 0" })},
		{"homologation of no capability", homologation(func(r *servicev1.RecordHomologationRequest) { r.CapabilityId = "" })},
		{"more results than criteria", homologation(func(r *servicev1.RecordHomologationRequest) { r.Results = slices.Repeat(every("H1"), 13) })},
		{"criterion too long", homologation(func(r *servicev1.RecordHomologationRequest) { r.Results = every(strings.Repeat("H", 65)) })},
		{"evidence outside printable ASCII", homologation(func(r *servicev1.RecordHomologationRequest) { r.Results[0].EvidenceId = "evd\t1" })},
		{"enabling of no capability", func() error {
			return h.invoke(withTenant(t), "EnableCapability", &servicev1.EnableCapabilityRequest{}, &servicev1.EnableCapabilityResponse{})
		}},
		{"suspension of no capability", func() error {
			return h.invoke(withTenant(t), "SuspendCapability", &servicev1.SuspendCapabilityRequest{Reason: "partner incident"}, &servicev1.SuspendCapabilityResponse{})
		}},
		{"suspension reason with a control character", func() error {
			req := &servicev1.SuspendCapabilityRequest{CapabilityId: "cap-1", Reason: "partner\x00incident"}
			return h.invoke(withTenant(t), "SuspendCapability", req, &servicev1.SuspendCapabilityResponse{})
		}},
		{"rollout of no capability", rollout(func(r *servicev1.ChangeRolloutRequest) { r.CapabilityId = "" })},
		{"unspecified mode", rollout(func(r *servicev1.ChangeRolloutRequest) { r.Mode = servicev1.RolloutMode_ROLLOUT_MODE_UNSPECIFIED })},
		{"tenant with a space", rollout(func(r *servicev1.ChangeRolloutRequest) { r.AllowlistTenantIds = []string{"tenant a"} })},
		{"tenant repeated", rollout(func(r *servicev1.ChangeRolloutRequest) { r.ExcludedTenantIds = []string{"tenant-x", "tenant-x"} })},
		{"too many tenants", rollout(func(r *servicev1.ChangeRolloutRequest) { r.AllowlistTenantIds = tooManyTenants })},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
	if len(h.store.Entries()) != 0 {
		t.Fatalf("outbox holds %d entries after malformed requests, want none", len(h.store.Entries()))
	}
}
