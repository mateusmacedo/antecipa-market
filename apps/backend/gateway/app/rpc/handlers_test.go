package rpc_test

import (
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
)

func registerRequest() *servicev1.RegisterConnectorRequest {
	return &servicev1.RegisterConnectorRequest{
		PartnerId: "prt-1", Environment: servicev1.Environment_ENVIRONMENT_PRODUCTION, CredentialRef: "vault://prt-1", IntegrationVersion: 1,
	}
}

func submitEffect() *servicev1.OperationCapability {
	return &servicev1.OperationCapability{
		Operation: "submitEffect", Supported: true, Protocol: servicev1.Protocol_PROTOCOL_SYNC_HTTP, StateQuerySupported: true,
		IdempotencySupport: servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_KEY_HEADER, AssetTypes: []string{"duplicata"},
		Efficacy: servicev1.Efficacy_EFFICACY_ON_SUBMIT, Quiescence: 30_000_000_000, MaxQueries: 3,
		KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32, QueryInterval: 60, AmbiguityDeadline: 3600,
		EncumbranceCoverage: servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_UNAVAILABLE,
	}
}

func declareWith(op *servicev1.OperationCapability) *servicev1.DeclareCapabilityProfileRequest {
	return &servicev1.DeclareCapabilityProfileRequest{
		ConnectorId: "prt-1:production", Version: 1, EffectiveFrom: int64(occurred), Operations: []*servicev1.OperationCapability{op},
	}
}

func grantRequest() *servicev1.GrantQueryAuthorizationRequest {
	return &servicev1.GrantQueryAuthorizationRequest{
		ConsentId: "cns-1", OrganizationId: "org-1", ConsentVersion: 1, AssetTypes: []string{"duplicata"},
		PeriodFromDate: "2026-01-01", PeriodUntilDate: "2026-12-31",
	}
}

func createRequest(key string) *servicev1.CreateExternalCommandRequest {
	return &servicev1.CreateExternalCommandRequest{
		IdempotencyKey: key, TenantId: "tnt-1", ConnectorId: "prt-1:production", Operation: "submitEffect", AssetType: "duplicata",
		BusinessKey: "eff-1", PayloadVersion: 1, Payload: `{"amount":100}`, PayloadHash: "h1",
	}
}

func (h *harness) ready(t *testing.T) {
	t.Helper()
	var registered servicev1.RegisterConnectorResponse
	if err := h.invoke(withKey(t), "RegisterConnector", registerRequest(), &registered); err != nil || registered.GetRegistered() == nil {
		t.Fatalf("RegisterConnector() = %v, %v", &registered, err)
	}
	var enabled servicev1.EnableOperationResponse
	if err := h.invoke(withKey(t), "EnableOperation", &servicev1.EnableOperationRequest{
		ConnectorKey: "prt-1:production", CapabilityId: "cap-1", Operation: "submitEffect", StateQuerySupported: true, Version: 1,
	}, &enabled); err != nil || enabled.GetEnabled() == nil {
		t.Fatalf("EnableOperation() = %v, %v", &enabled, err)
	}
	var declared servicev1.DeclareCapabilityProfileResponse
	if err := h.invoke(withKey(t), "DeclareCapabilityProfile", &servicev1.DeclareCapabilityProfileRequest{
		ConnectorId: "prt-1:production", Version: 1, EffectiveFrom: int64(occurred), Operations: []*servicev1.OperationCapability{submitEffect()},
	}, &declared); err != nil || declared.GetDeclared().GetProfileKey() != "prt-1:production:1" {
		t.Fatalf("DeclareCapabilityProfile() = %v, %v", &declared, err)
	}
}

func TestTheExternalCommandFlowsThroughTheEdge(t *testing.T) {
	h := newHarness(t)
	h.ready(t)

	var created servicev1.CreateExternalCommandResponse
	if err := h.invoke(withKey(t), "CreateExternalCommand", createRequest("idem-1"), &created); err != nil {
		t.Fatalf("CreateExternalCommand() = %v", err)
	}
	if got := created.GetCreated(); got.GetIdempotencyKey() != "idem-1" || got.GetStatus() != servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_CREATED {
		t.Fatalf("CreateExternalCommand() = %v", &created)
	}

	var found servicev1.FindExternalCommandResponse
	if err := h.invoke(withoutKey(t), "FindExternalCommand", &servicev1.FindExternalCommandRequest{IdempotencyKey: "idem-1"}, &found); err != nil {
		t.Fatalf("FindExternalCommand() = %v", err)
	}
	if c := found.GetCommand(); c.GetProfile().GetProfileVersion() != 1 || c.GetPayloadHash() != "h1" || c.GetSeriesKey() != "prt-1:production:submitEffect:eff-1" {
		t.Fatalf("FindExternalCommand() = %v", c)
	}

	var cancelled servicev1.CancelExternalCommandResponse
	if err := h.invoke(withKey(t), "CancelExternalCommand", &servicev1.CancelExternalCommandRequest{IdempotencyKey: "idem-1"}, &cancelled); err != nil || cancelled.GetCancelled() == nil {
		t.Fatalf("CancelExternalCommand() = %v, %v", &cancelled, err)
	}
	var listed servicev1.FindExternalCommandsByStatusResponse
	if err := h.invoke(withoutKey(t), "FindExternalCommandsByStatus", &servicev1.FindExternalCommandsByStatusRequest{
		Status: servicev1.ExternalCommandStatus_EXTERNAL_COMMAND_STATUS_CANCELLED, ConnectorId: "prt-1:production",
	}, &listed); err != nil || len(listed.GetCommands()) != 1 {
		t.Fatalf("FindExternalCommandsByStatus() = %v, %v", &listed, err)
	}
}

func TestTheCommandsOfABusinessKeyAreFoundThroughTheEdge(t *testing.T) {
	h := newHarness(t)
	h.ready(t)
	var created servicev1.CreateExternalCommandResponse
	if err := h.invoke(withKey(t), "CreateExternalCommand", createRequest("idem-1"), &created); err != nil || created.GetCreated() == nil {
		t.Fatalf("CreateExternalCommand() = %v, %v", &created, err)
	}

	var found servicev1.FindExternalCommandsByBusinessKeyResponse
	err := h.invoke(withoutKey(t), "FindExternalCommandsByBusinessKey", &servicev1.FindExternalCommandsByBusinessKeyRequest{
		ConnectorId: "prt-1:production", BusinessKey: "eff-1",
	}, &found)

	if err != nil || len(found.GetCommands()) != 1 || found.GetCommands()[0].GetIdempotencyKey() != "idem-1" {
		t.Fatalf("FindExternalCommandsByBusinessKey(eff-1) = %v, %v; want the command idem-1", &found, err)
	}
	var none servicev1.FindExternalCommandsByBusinessKeyResponse
	if err := h.invoke(withoutKey(t), "FindExternalCommandsByBusinessKey", &servicev1.FindExternalCommandsByBusinessKeyRequest{
		ConnectorId: "prt-1:production", BusinessKey: "eff-2",
	}, &none); err != nil || len(none.GetCommands()) != 0 {
		t.Fatalf("FindExternalCommandsByBusinessKey(eff-2) = %v, %v; want no command", &none, err)
	}
}

func TestADomainRefusalTravelsInTheResponse(t *testing.T) {
	h := newHarness(t)

	var created servicev1.CreateExternalCommandResponse
	err := h.invoke(withKey(t), "CreateExternalCommand", createRequest("idem-1"), &created)

	if err != nil || created.GetRejection().GetCode() != "receivables-gateway/external-command/capability-not-enabled" {
		t.Fatalf("CreateExternalCommand() = %v, %v; want the capability-not-enabled rejection", &created, err)
	}
	var resolved servicev1.ResolveUnknownOutcomeResponse
	err = h.invoke(withKey(t), "ResolveUnknownOutcome", &servicev1.ResolveUnknownOutcomeRequest{
		IdempotencyKey: "idem-1", Resolution: servicev1.Resolution_RESOLUTION_CONFIRMED, QueryInteractionId: "int-1", Justification: "j",
	}, &resolved)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("ResolveUnknownOutcome() over no command = %v, want NotFound", err)
	}
}

func TestTheEdgeRefusesAMalformedRequest(t *testing.T) {
	tooLong := strings.Repeat("k", 101)
	cases := map[string]func(h *harness) error{
		"an unspecified environment": func(h *harness) error {
			req := registerRequest()
			req.Environment = servicev1.Environment_ENVIRONMENT_UNSPECIFIED
			return h.invoke(withKey(t), "RegisterConnector", req, &servicev1.RegisterConnectorResponse{})
		},
		"an idempotency key the sweep could not extend": func(h *harness) error {
			return h.invoke(withKey(t), "CreateExternalCommand", createRequest(tooLong), &servicev1.CreateExternalCommandResponse{})
		},
		"an unspecified cause": func(h *harness) error {
			return h.invoke(withKey(t), "SuspendOperation", &servicev1.SuspendOperationRequest{ConnectorKey: "prt-1:production"}, &servicev1.SuspendOperationResponse{})
		},
		"an unspecified resolution": func(h *harness) error {
			return h.invoke(withKey(t), "ResolveUnknownOutcome", &servicev1.ResolveUnknownOutcomeRequest{IdempotencyKey: "idem-1"}, &servicev1.ResolveUnknownOutcomeResponse{})
		},
		"an unspecified status": func(h *harness) error {
			return h.invoke(withoutKey(t), "FindExternalCommandsByStatus", &servicev1.FindExternalCommandsByStatusRequest{}, &servicev1.FindExternalCommandsByStatusResponse{})
		},
		"an unspecified protocol": func(h *harness) error {
			op := submitEffect()
			op.Protocol = servicev1.Protocol_PROTOCOL_UNSPECIFIED
			return h.invoke(withKey(t), "DeclareCapabilityProfile", declareWith(op), &servicev1.DeclareCapabilityProfileResponse{})
		},
		"an unspecified encumbrance coverage": func(h *harness) error {
			op := submitEffect()
			op.EncumbranceCoverage = servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_UNSPECIFIED
			return h.invoke(withKey(t), "DeclareCapabilityProfile", declareWith(op), &servicev1.DeclareCapabilityProfileResponse{})
		},
		"a text carrying a NUL, which jsonb refuses": func(h *harness) error {
			req := registerRequest()
			req.CredentialRef = "vault://\x00"
			return h.invoke(withKey(t), "RegisterConnector", req, &servicev1.RegisterConnectorResponse{})
		},
		"a partner id its connector key could not hold": func(h *harness) error {
			req := registerRequest()
			req.PartnerId = strings.Repeat("p", 120)
			return h.invoke(withKey(t), "RegisterConnector", req, &servicev1.RegisterConnectorResponse{})
		},
		"an empty payload hash": func(h *harness) error {
			req := createRequest("idem-1")
			req.PayloadHash = ""
			return h.invoke(withKey(t), "CreateExternalCommand", req, &servicev1.CreateExternalCommandResponse{})
		},
		"a grant without asset types": func(h *harness) error {
			req := grantRequest()
			req.AssetTypes = nil
			return h.invoke(withKey(t), "GrantQueryAuthorization", req, &servicev1.GrantQueryAuthorizationResponse{})
		},
		"a grant period that is not a date": func(h *harness) error {
			req := grantRequest()
			req.PeriodFromDate = "01/01/2026"
			return h.invoke(withKey(t), "GrantQueryAuthorization", req, &servicev1.GrantQueryAuthorizationResponse{})
		},
		"an effective instant of zero": func(h *harness) error {
			return h.invoke(withoutKey(t), "FindEffectiveCapabilityProfile", &servicev1.FindEffectiveCapabilityProfileRequest{
				ConnectorId: "prt-1:production",
			}, &servicev1.FindEffectiveCapabilityProfileResponse{})
		},
		"a business key outside its alphabet": func(h *harness) error {
			return h.invoke(withoutKey(t), "FindExternalCommandsByBusinessKey", &servicev1.FindExternalCommandsByBusinessKeyRequest{
				ConnectorId: "prt-1:production", BusinessKey: "eff 1",
			}, &servicev1.FindExternalCommandsByBusinessKeyResponse{})
		},
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(newHarness(t)); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("call = %v, want InvalidArgument", err)
			}
		})
	}
}

func TestTheQueriesReadTheConnectorAndItsProfile(t *testing.T) {
	h := newHarness(t)
	h.ready(t)

	var connector servicev1.FindConnectorResponse
	if err := h.invoke(withoutKey(t), "FindConnector", &servicev1.FindConnectorRequest{ConnectorKey: "prt-1:production"}, &connector); err != nil {
		t.Fatalf("FindConnector() = %v", err)
	}
	if c := connector.GetConnector(); c.GetEnvironment() != servicev1.Environment_ENVIRONMENT_PRODUCTION || len(c.GetCapabilities()) != 1 {
		t.Fatalf("FindConnector() = %v", c)
	}
	var effective servicev1.FindEffectiveCapabilityProfileResponse
	if err := h.invoke(withoutKey(t), "FindEffectiveCapabilityProfile", &servicev1.FindEffectiveCapabilityProfileRequest{
		ConnectorId: "prt-1:production", EffectiveAt: int64(occurred),
	}, &effective); err != nil || effective.GetProfile().GetOperations()[0].GetEfficacy() != servicev1.Efficacy_EFFICACY_ON_SUBMIT {
		t.Fatalf("FindEffectiveCapabilityProfile() = %v, %v", &effective, err)
	}
	var missing servicev1.FindCapabilityProfileResponse
	err := h.invoke(withoutKey(t), "FindCapabilityProfile", &servicev1.FindCapabilityProfileRequest{ConnectorId: "prt-1:production", Version: 9}, &missing)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindCapabilityProfile(9) = %v, want NotFound", err)
	}
}

func TestTheSweepAnswersItsCounts(t *testing.T) {
	h := newHarness(t)

	var swept servicev1.SweepExternalCommandsResponse
	if err := h.invoke(withKey(t), "SweepExternalCommands", &servicev1.SweepExternalCommandsRequest{}, &swept); err != nil {
		t.Fatalf("SweepExternalCommands() = %v", err)
	}
	if swept.GetResendRequested() != 0 || swept.GetAmbiguityExpired() != 0 || swept.GetFailed() != 0 {
		t.Fatalf("SweepExternalCommands() = %v, want nothing to do", &swept)
	}
}

func TestTheQueryAuthorizationIsProjectedThroughTheEdge(t *testing.T) {
	h := newHarness(t)

	var granted servicev1.GrantQueryAuthorizationResponse
	if err := h.invoke(withKey(t), "GrantQueryAuthorization", &servicev1.GrantQueryAuthorizationRequest{
		ConsentId: "cns-1", OrganizationId: "org-1", ConsentVersion: 1, AssetTypes: []string{"duplicata"},
		PeriodFromDate: "2026-01-01", PeriodUntilDate: "2026-12-31",
	}, &granted); err != nil || granted.GetGranted().GetConsentId() != "cns-1" {
		t.Fatalf("GrantQueryAuthorization() = %v, %v", &granted, err)
	}
	var revoked servicev1.RevokeQueryAuthorizationResponse
	if err := h.invoke(withKey(t), "RevokeQueryAuthorization", &servicev1.RevokeQueryAuthorizationRequest{ConsentId: "cns-1", EffectiveAt: int64(occurred)}, &revoked); err != nil || revoked.GetRevoked() == nil {
		t.Fatalf("RevokeQueryAuthorization() = %v, %v", &revoked, err)
	}
	var found servicev1.FindQueryAuthorizationByOrganizationResponse
	if err := h.invoke(withoutKey(t), "FindQueryAuthorizationByOrganization", &servicev1.FindQueryAuthorizationByOrganizationRequest{OrganizationId: "org-1"}, &found); err != nil {
		t.Fatalf("FindQueryAuthorizationByOrganization() = %v", err)
	}
	if a := found.GetAuthorizations(); len(a) != 1 || a[0].GetStatus() != servicev1.AuthorizationStatus_AUTHORIZATION_STATUS_REVOKED {
		t.Fatalf("FindQueryAuthorizationByOrganization() = %v", a)
	}
}
