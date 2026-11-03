package rpc_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func internal(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.AutoKey(), execctx.WithTenant(testTenant))
}

func (h *harness) registerConnector(t *testing.T) {
	t.Helper()
	var registered servicev1.RegisterConnectorResponse
	if err := h.invoke(withKey(t), "RegisterConnector", registerRequest(), &registered); err != nil || registered.GetRegistered() == nil {
		t.Fatalf("setup: RegisterConnector() = %v, %v", &registered, err)
	}
}

// concludedCapture records the capture of a suspended submission and concludes
// it as the normalization would, so the edge has an interaction to read.
func (h *harness) concludedCapture(t *testing.T, outcome domain.InteractionStatus) {
	t.Helper()
	broken := domain.Mismatch{Kind: domain.MismatchStructure, Detail: "root is not an object", PartnerDeclaredVersion: "2.1"}
	recorded, err := h.service.RecordExternalInteraction(internal(t), application.RecordExternalInteraction{
		InteractionID: "int-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", Direction: domain.DirectionOutbound,
		Source: domain.InteractionSubmitResponse, ContentType: "application/json", CommandID: "idem-1",
		Request:      domain.RawPayload{Ref: "interactions/prt-1:production/int-1/request", VersionID: "vq", Hash: "hq"},
		Response:     domain.RawPayload{Ref: "interactions/prt-1:production/int-1/response", VersionID: "vs", Hash: "hs"},
		CapturedStep: domain.StepSuspend, Mismatch: broken,
	})
	if _, refused := recorded.Rejection(); err != nil || refused {
		t.Fatalf("setup: RecordExternalInteraction() = %+v, %v", recorded, err)
	}
	cmd := application.ConcludeInteraction{InteractionID: "int-1", Revision: 1, Outcome: outcome, Mismatch: broken}
	if outcome == domain.InteractionNormalized {
		cmd = application.ConcludeInteraction{InteractionID: "int-1", Revision: 1, Outcome: outcome, ReadingHash: "rh-1", Finding: domain.FindingApplied}
	}
	concluded, err := h.service.ConcludeInteraction(internal(t), cmd)
	if _, refused := concluded.Rejection(); err != nil || refused {
		t.Fatalf("setup: ConcludeInteraction() = %+v, %v", concluded, err)
	}
}

func TestFindExternalInteractionAnswersTheInteractionOnTheWire(t *testing.T) {
	h := newHarness(t)
	h.registerConnector(t)
	h.concludedCapture(t, domain.InteractionNormalized)

	var found servicev1.FindExternalInteractionResponse
	err := h.invoke(withoutKey(t), "FindExternalInteraction", &servicev1.FindExternalInteractionRequest{InteractionId: "int-1"}, &found)

	i := found.GetInteraction()
	if err != nil || i.GetStatus() != servicev1.InteractionStatus_INTERACTION_STATUS_NORMALIZED || i.GetFinding() != servicev1.Finding_FINDING_APPLIED ||
		i.GetCapturedStep() != servicev1.Step_STEP_SUSPEND || i.GetSource() != servicev1.InteractionSource_INTERACTION_SOURCE_SUBMIT_RESPONSE ||
		i.GetDirection() != servicev1.InteractionDirection_INTERACTION_DIRECTION_OUTBOUND || i.GetResponseVersionId() != "vs" ||
		i.GetNormalizationRevision() != 1 || i.GetCommandId() != "idem-1" {
		t.Fatalf("FindExternalInteraction() = %v, %v", i, err)
	}
}

func TestReprocessInteractionOpensANewRevision(t *testing.T) {
	h := newHarness(t)
	h.registerConnector(t)
	h.concludedCapture(t, domain.InteractionSchemaMismatch)

	var resp servicev1.ReprocessInteractionResponse
	err := h.invoke(withKey(t), "ReprocessInteraction", &servicev1.ReprocessInteractionRequest{
		InteractionId: "int-1", Justification: "normalizador corrigido",
	}, &resp)

	if err != nil || resp.GetReprocessed().GetNormalizationRevision() != 2 || resp.GetReprocessed().GetInteractionId() != "int-1" {
		t.Fatalf("ReprocessInteraction() = %v, %v; want revision 2", &resp, err)
	}
	var found servicev1.FindExternalInteractionResponse
	if err := h.invoke(withoutKey(t), "FindExternalInteraction", &servicev1.FindExternalInteractionRequest{InteractionId: "int-1"}, &found); err != nil ||
		found.GetInteraction().GetStatus() != servicev1.InteractionStatus_INTERACTION_STATUS_STORED ||
		found.GetInteraction().GetPreviousOutcome() != servicev1.InteractionStatus_INTERACTION_STATUS_SCHEMA_MISMATCH {
		t.Fatalf("FindExternalInteraction() = %v, %v; want stored again after a schema mismatch", found.GetInteraction(), err)
	}
}

func TestReprocessInteractionAnswersTheRefusalOfTheDomain(t *testing.T) {
	h := newHarness(t)
	h.registerConnector(t)
	h.concludedCapture(t, domain.InteractionNormalized)

	var resp servicev1.ReprocessInteractionResponse
	err := h.invoke(withKey(t), "ReprocessInteraction", &servicev1.ReprocessInteractionRequest{InteractionId: "int-1"}, &resp)

	if err != nil || resp.GetRejection().GetCode() != string(domain.CodeExternalInteractionJustificationMissing) {
		t.Fatalf("ReprocessInteraction() = %v, %v; want the justification-missing refusal", &resp, err)
	}
}

func TestTheInteractionMethodsAnswerAnUnknownInteractionAsNotFound(t *testing.T) {
	h := newHarness(t)

	err := h.invoke(withKey(t), "ReprocessInteraction", &servicev1.ReprocessInteractionRequest{InteractionId: "int-9", Justification: "x"},
		&servicev1.ReprocessInteractionResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("ReprocessInteraction(int-9) = %v, want NotFound", err)
	}
	err = h.invoke(withoutKey(t), "FindExternalInteractionByPartnerEvent", &servicev1.FindExternalInteractionByPartnerEventRequest{
		ConnectorId: "prt-1:production", PartnerEventId: "evt-p-9",
	}, &servicev1.FindExternalInteractionByPartnerEventResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("FindExternalInteractionByPartnerEvent(evt-p-9) = %v, want NotFound", err)
	}
}

func TestTheInteractionMethodsBoundTheirForm(t *testing.T) {
	h := newHarness(t)
	cases := map[string]struct {
		ctx    context.Context
		method string
		req    proto.Message
	}{
		"reprocess without id":       {withKey(t), "ReprocessInteraction", &servicev1.ReprocessInteractionRequest{Justification: "x"}},
		"find by command with colon": {withoutKey(t), "FindExternalInteractionsByCommand", &servicev1.FindExternalInteractionsByCommandRequest{CommandId: "a:b"}},
		"find reference without key": {withoutKey(t), "FindExternalReference", &servicev1.FindExternalReferenceRequest{}},
		"link without system": {withKey(t), "LinkExternalReference", &servicev1.LinkExternalReferenceRequest{
			ProviderId: "prt-1", ReferenceType: servicev1.ReferenceType_REFERENCE_TYPE_ASSET, Reference: "dup-1",
			SubjectType: "asset", SubjectId: "ast-1", AsOf: int64(occurred),
		}},
		"link without instant": {withKey(t), "LinkExternalReference", &servicev1.LinkExternalReferenceRequest{
			ProviderId: "prt-1", System: servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY, ReferenceType: servicev1.ReferenceType_REFERENCE_TYPE_ASSET,
			Reference: "dup-1", SubjectType: "asset", SubjectId: "ast-1",
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// The handler answers before any response is written, so an empty
			// message stands for the response of any method.
			if err := h.invoke(c.ctx, c.method, c.req, &emptypb.Empty{}); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("%s = %v, want InvalidArgument", c.method, err)
			}
		})
	}
}

func linkRequest(subject string) *servicev1.LinkExternalReferenceRequest {
	return &servicev1.LinkExternalReferenceRequest{
		ProviderId: "prt-1", System: servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY, ReferenceType: servicev1.ReferenceType_REFERENCE_TYPE_ASSET,
		Reference: " dup-0001 ", SubjectType: "asset", SubjectId: subject, AsOf: int64(occurred), ObservedVersion: "v1",
	}
}

// The edge bounds the form of provider_id only; the colon that makes the key
// ambiguous is the domain's refusal, with the code the spec declares.
func TestLinkExternalReferenceOfAProviderWithAColonIsTheDomainsRefusal(t *testing.T) {
	h := newHarness(t)
	first := linkRequest("ast-1")
	first.ProviderId, first.Reference = "p", "x:registry:asset:y"
	var linked servicev1.LinkExternalReferenceResponse
	if err := h.invoke(withKey(t), "LinkExternalReference", first, &linked); err != nil || linked.GetLinked().GetReferenceKey() != "p:registry:asset:x:registry:asset:y" {
		t.Fatalf("LinkExternalReference(reference with colons) = %v, %v", &linked, err)
	}

	second := linkRequest("ast-2")
	second.ProviderId, second.Reference = "p:registry:asset:x", "y"
	var refused servicev1.LinkExternalReferenceResponse
	err := h.invoke(withKey(t), "LinkExternalReference", second, &refused)
	if err != nil || refused.GetRejection().GetCode() != string(domain.CodeExternalReferenceProviderInvalid) {
		t.Fatalf("LinkExternalReference(provider with colons) = %v, %v; want the provider-invalid refusal", &refused, err)
	}
}

func TestLinkExternalReferenceAndItsQueriesOnTheWire(t *testing.T) {
	h := newHarness(t)

	var linked servicev1.LinkExternalReferenceResponse
	err := h.invoke(withKey(t), "LinkExternalReference", linkRequest("ast-1"), &linked)
	if err != nil || linked.GetLinked().GetReferenceKey() != "prt-1:registry:asset: dup-0001 " {
		t.Fatalf("LinkExternalReference() = %v, %v", &linked, err)
	}

	var found servicev1.FindExternalReferenceResponse
	err = h.invoke(withoutKey(t), "FindExternalReference", &servicev1.FindExternalReferenceRequest{ReferenceKey: linked.GetLinked().GetReferenceKey()}, &found)
	r := found.GetReference()
	if err != nil || r.GetSystem() != servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY || r.GetReferenceType() != servicev1.ReferenceType_REFERENCE_TYPE_ASSET ||
		r.GetReference() != " dup-0001 " || r.GetNormalizedReference() != "DUP-0001" || r.GetAsOf() != int64(occurred) {
		t.Fatalf("FindExternalReference() = %v, %v", r, err)
	}

	var bySubject servicev1.FindExternalReferencesBySubjectResponse
	err = h.invoke(withoutKey(t), "FindExternalReferencesBySubject", &servicev1.FindExternalReferencesBySubjectRequest{SubjectType: "asset", SubjectId: "ast-1"}, &bySubject)
	if err != nil || len(bySubject.GetReferences()) != 1 {
		t.Fatalf("FindExternalReferencesBySubject() = %v, %v", &bySubject, err)
	}

	var other servicev1.LinkExternalReferenceResponse
	err = h.invoke(withKey(t), "LinkExternalReference", linkRequest("ast-2"), &other)
	if err != nil || other.GetRejection().GetCode() != string(domain.CodeExternalReferenceAlreadyLinked) {
		t.Fatalf("LinkExternalReference(ast-2) = %v, %v; want the already-linked refusal", &other, err)
	}

	empty := linkRequest("ast-3")
	empty.Reference = ""
	var refused servicev1.LinkExternalReferenceResponse
	err = h.invoke(withKey(t), "LinkExternalReference", empty, &refused)
	if err != nil || refused.GetRejection().GetCode() != string(domain.CodeExternalReferenceReferenceEmpty) {
		t.Fatalf("LinkExternalReference(empty) = %v, %v; want the reference-empty refusal", &refused, err)
	}
}
