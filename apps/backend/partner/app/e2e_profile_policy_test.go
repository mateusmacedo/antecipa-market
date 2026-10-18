//go:build integration

package app_test

import (
	"slices"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
)

const day = int64(86_400_000_000_000)

func profileOf(profileID string, expiresIn int64, supersedes string) *servicev1.RegisterIntegrationProfileRequest {
	return &servicev1.RegisterIntegrationProfileRequest{
		ProfileId: profileID, PartnerId: "e2e-prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION,
		ChangeKind: servicev1.IntegrationChangeKind_INTEGRATION_CHANGE_KIND_INITIAL, Protocols: []string{"https", "mtls"},
		Endpoint: "https://partner.example/api", SchemaVersion: "v3", SecretRef: "vault://partners/e2e-prt-1/api-key",
		CertificateRef: "cert-1", CertificateExpiresAt: validFrom + expiresIn, EffectiveFrom: validFrom, SupersedesId: supersedes,
	}
}

func policyOf(policyID, supersedes string) *servicev1.DefineOperationalPolicyRequest {
	return &servicev1.DefineOperationalPolicyRequest{
		PolicyId: policyID, PartnerId: "e2e-prt-1", EffectiveFrom: validFrom, SupersedesId: supersedes, RateLimitPerMinute: 100,
		BusinessDays: []string{"mon", "tue", "wed", "thu", "fri"}, CutoffTime: "17:00", ProcessingWindow: "08:00-18:00",
		SlaRef: "sla-gold", Fallback: "manual queue", SupportContact: "support@partner.example", IncidentContact: "+55 11 99999-0000",
	}
}

func TestRegisterIntegrationProfileEndToEndKeepsTheReferenceAndEmitsNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	first := invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-1", 90*day, ""), &servicev1.RegisterIntegrationProfileResponse{})
	second := invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-2", 90*day, "e2e-ipf-1"), &servicev1.RegisterIntegrationProfileResponse{})

	if first.GetRegistered().GetProfileId() != "e2e-ipf-1" || second.GetRegistered().GetProfileId() != "e2e-ipf-2" {
		t.Fatalf("RegisterIntegrationProfile() = %v and %v, want versions 1 and 2 registered", first, second)
	}
	if got := count(t, pool, "SELECT count(*) FROM integration_profiles WHERE environment = 'production' AND (snapshot->>'version')::int = 1 AND supersedes_id IS NULL AND snapshot->>'secretRef' = 'vault://partners/e2e-prt-1/api-key'"); got != 1 {
		t.Fatalf("first versions holding the secret reference = %d, want 1", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM integration_profiles WHERE (snapshot->>'version')::int = 2 AND supersedes_id = 'e2e-ipf-1'"); got != 1 {
		t.Fatalf("versions 2 superseding e2e-ipf-1 = %d, want 1", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM outbox"); got != 0 {
		t.Fatalf("outbox = %d, want nothing emitted by a profile registration", got)
	}
}

func TestEachRefusalOfTheIntegrationProfileWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-1", 25*day, ""), &servicev1.RegisterIntegrationProfileResponse{})
	invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-far", 90*day, "e2e-ipf-1"), &servicev1.RegisterIntegrationProfileResponse{})
	invoke(t, conn, call(t), "RaiseCertificateExpiryAlert", &servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "e2e-ipf-1", ThresholdDays: 30}, &servicev1.RaiseCertificateExpiryAlertResponse{})

	register := func(edit func(*servicev1.RegisterIntegrationProfileRequest)) func() string {
		return func() string {
			req := profileOf("e2e-ipf-x", 90*day, "e2e-ipf-far")
			edit(req)
			return invoke(t, conn, call(t), "RegisterIntegrationProfile", req, &servicev1.RegisterIntegrationProfileResponse{}).GetRejection().GetCode()
		}
	}
	alert := func(profileID string, threshold int32) func() string {
		return func() string {
			req := &servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: profileID, ThresholdDays: threshold}
			return invoke(t, conn, call(t), "RaiseCertificateExpiryAlert", req, &servicev1.RaiseCertificateExpiryAlertResponse{}).GetRejection().GetCode()
		}
	}
	for _, c := range []struct {
		name string
		got  func() string
		code string
	}{
		{"endpoint empty", register(func(r *servicev1.RegisterIntegrationProfileRequest) { r.Endpoint = "" }),
			"partner-management/integration-profile/endpoint-empty"},
		{"without the secret reference", register(func(r *servicev1.RegisterIntegrationProfileRequest) { r.SecretRef = "" }),
			"partner-management/integration-profile/secret-ref-missing"},
		{"certificate expired before the instant", register(func(r *servicev1.RegisterIntegrationProfileRequest) { r.CertificateExpiresAt = validFrom - day }),
			"partner-management/integration-profile/certificate-expiry-invalid"},
		{"supersedes the profile of another environment", register(func(r *servicev1.RegisterIntegrationProfileRequest) {
			r.Environment = servicev1.Environment_ENVIRONMENT_HOMOLOGATION
		}), "partner-management/integration-profile/supersedes-invalid"},
		{"threshold of 45 days", alert("e2e-ipf-far", 45), "partner-management/integration-profile/invalid-alert-threshold"},
		{"certificate expiring in 90 days", alert("e2e-ipf-far", 60), "partner-management/integration-profile/threshold-not-reached"},
		{"60 days after 30 were alerted", alert("e2e-ipf-1", 60), "partner-management/integration-profile/already-alerted"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.got(); got != c.code {
				t.Fatalf("rejection = %q, want %q", got, c.code)
			}
			if profiles, events := count(t, pool, "SELECT count(*) FROM integration_profiles"), count(t, pool, "SELECT count(*) FROM outbox"); profiles != 2 || events != 1 {
				t.Fatalf("integration_profiles = %d, outbox = %d after a refusal, want the two held profiles and one alert", profiles, events)
			}
		})
	}
}

func TestTheDeterministicKeyOfACertificateAlertLeavesOneEvent(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "RegisterIntegrationProfile", profileOf("e2e-ipf-1", 25*day, ""), &servicev1.RegisterIntegrationProfileResponse{})
	req := &servicev1.RaiseCertificateExpiryAlertRequest{ProfileId: "e2e-ipf-1", ThresholdDays: 30}
	key := "cert-alert.e2e-ipf-1.30.20250817"

	var first, again servicev1.RaiseCertificateExpiryAlertResponse
	var header metadata.MD
	if err := conn.Invoke(callWithKey(t, key), rpc.FullMethod("RaiseCertificateExpiryAlert"), req, &first); err != nil {
		t.Fatalf("RaiseCertificateExpiryAlert() = %v", err)
	}
	if err := conn.Invoke(callWithKey(t, key), rpc.FullMethod("RaiseCertificateExpiryAlert"), req, &again, grpc.Header(&header)); err != nil {
		t.Fatalf("repeated RaiseCertificateExpiryAlert() = %v, want the recorded response", err)
	}

	if !proto.Equal(&first, &again) || again.GetRaised().GetProfileId() != "e2e-ipf-1" ||
		!slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		t.Fatalf("replay = %v with header %v, want the first answer %v marked as replay", &again, header, &first)
	}
	records := outbox(t, pool)
	if len(records) != 1 {
		t.Fatalf("outbox rows = %d, want a single PartnerCertificateExpiring", len(records))
	}
	record := records[0]
	if record.messageType != "com.company.partner.partner-certificate-expiring.v1" || record.destination != application.Destination || record.partitionKey != "e2e-ipf-1" {
		t.Fatalf("outbox record = %s to %s keyed %s, want the certificate-expiring type keyed by the profile", record.messageType, record.destination, record.partitionKey)
	}
	var event eventv1.PartnerCertificateExpiring
	if err := proto.Unmarshal(record.payload, &event); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if event.GetProfileId() != "e2e-ipf-1" || event.GetPartnerId() != "e2e-prt-1" || event.GetEnvironment() != eventv1.Environment_ENVIRONMENT_PRODUCTION ||
		event.GetThresholdDays() != 30 || event.GetCertificateExpiresAt().AsTime().UnixNano() != validFrom+25*day || event.GetAt().AsTime().UnixNano() != validFrom {
		t.Fatalf("event = %v, want the 30-day alert of the production certificate", &event)
	}
	if got := count(t, pool, "SELECT count(*) FROM integration_profiles WHERE (snapshot->>'lastAlertedThresholdDays')::int = 30"); got != 1 {
		t.Fatalf("profiles alerted at 30 days = %d, want 1", got)
	}
}

func TestTwoConcurrentFirstProfilesOfOneEnvironmentLeaveOneProfile(t *testing.T) {
	pool := appkit.OpenPool(t)
	saves := newBarrier(2)
	conn := dial(t, pool, saves)

	got := invokeAtOnce(t, conn, "RegisterIntegrationProfile",
		map[string]proto.Message{"first": profileOf("e2e-ipf-a1", 90*day, ""), "second": profileOf("e2e-ipf-a2", 90*day, "")},
		func(m proto.Message) bool {
			return m.(*servicev1.RegisterIntegrationProfileResponse).GetRegistered() != nil
		},
		func() proto.Message { return &servicev1.RegisterIntegrationProfileResponse{} })

	oneAcceptedOneFailed(t, got)
	refusedByUniqueness(t, saves, "integration_profiles_tenant_id_partner_id_environment_idx")
	if profiles := count(t, pool, "SELECT count(*) FROM integration_profiles"); profiles != 1 {
		t.Fatalf("integration_profiles = %d, want exactly one first version", profiles)
	}
}

func TestDefineOperationalPolicyEndToEndKeepsOneSeriesAndEmitsNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)

	first := invoke(t, conn, call(t), "DefineOperationalPolicy", policyOf("e2e-pol-1", ""), &servicev1.DefineOperationalPolicyResponse{})
	second := invoke(t, conn, call(t), "DefineOperationalPolicy", policyOf("e2e-pol-2", "e2e-pol-1"), &servicev1.DefineOperationalPolicyResponse{})
	stale := invoke(t, conn, call(t), "DefineOperationalPolicy", policyOf("e2e-pol-3", "e2e-pol-1"), &servicev1.DefineOperationalPolicyResponse{})

	if first.GetDefined().GetPolicyId() != "e2e-pol-1" || second.GetDefined().GetPolicyId() != "e2e-pol-2" {
		t.Fatalf("DefineOperationalPolicy() = %v and %v, want versions 1 and 2 defined", first, second)
	}
	if got := stale.GetRejection().GetCode(); got != "partner-management/operational-policy/supersedes-invalid" {
		t.Fatalf("DefineOperationalPolicy(stale supersedes) = %v, want supersedes-invalid", stale)
	}
	if got := count(t, pool, "SELECT count(*) FROM operational_policies WHERE (snapshot->>'version')::int = 2 AND supersedes_id = 'e2e-pol-1' AND (snapshot->>'rateLimitPerMinute')::int = 100"); got != 1 {
		t.Fatalf("policies at version 2 superseding e2e-pol-1 = %d, want 1", got)
	}
	if policies, events := count(t, pool, "SELECT count(*) FROM operational_policies"), count(t, pool, "SELECT count(*) FROM outbox"); policies != 2 || events != 0 {
		t.Fatalf("operational_policies = %d, outbox = %d, want two versions and nothing emitted", policies, events)
	}
}

func TestEachRefusalOfTheOperationalPolicyWritesNothing(t *testing.T) {
	pool := appkit.OpenPool(t)
	conn := dial(t, pool, nil)
	invoke(t, conn, call(t), "DefineOperationalPolicy", policyOf("e2e-pol-1", ""), &servicev1.DefineOperationalPolicyResponse{})

	for _, c := range []struct {
		name string
		edit func(*servicev1.DefineOperationalPolicyRequest)
		code string
	}{
		{"rate limit of zero", func(r *servicev1.DefineOperationalPolicyRequest) { r.RateLimitPerMinute = 0 },
			"partner-management/operational-policy/rate-limit-invalid"},
		{"without the incident contact", func(r *servicev1.DefineOperationalPolicyRequest) { r.IncidentContact = "" },
			"partner-management/operational-policy/contacts-missing"},
		{"another first version of the partner", func(r *servicev1.DefineOperationalPolicyRequest) { r.SupersedesId = "" },
			"partner-management/operational-policy/supersedes-invalid"},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := policyOf("e2e-pol-x", "e2e-pol-1")
			c.edit(req)

			resp := invoke(t, conn, call(t), "DefineOperationalPolicy", req, &servicev1.DefineOperationalPolicyResponse{})

			if got := resp.GetRejection().GetCode(); got != c.code {
				t.Fatalf("rejection = %q, want %q", got, c.code)
			}
			if policies, events := count(t, pool, "SELECT count(*) FROM operational_policies"), count(t, pool, "SELECT count(*) FROM outbox"); policies != 1 || events != 0 {
				t.Fatalf("operational_policies = %d, outbox = %d after a refusal, want the held policy and nothing emitted", policies, events)
			}
		})
	}
}

func TestTwoConcurrentFirstPoliciesOfOnePartnerLeaveOnePolicy(t *testing.T) {
	pool := appkit.OpenPool(t)
	saves := newBarrier(2)
	conn := dial(t, pool, saves)

	got := invokeAtOnce(t, conn, "DefineOperationalPolicy",
		map[string]proto.Message{"first": policyOf("e2e-pol-a1", ""), "second": policyOf("e2e-pol-a2", "")},
		func(m proto.Message) bool { return m.(*servicev1.DefineOperationalPolicyResponse).GetDefined() != nil },
		func() proto.Message { return &servicev1.DefineOperationalPolicyResponse{} })

	oneAcceptedOneFailed(t, got)
	refusedByUniqueness(t, saves, "operational_policies_tenant_id_partner_id_idx")
	if policies := count(t, pool, "SELECT count(*) FROM operational_policies"); policies != 1 {
		t.Fatalf("operational_policies = %d, want exactly one first version", policies)
	}
}
