package rpc_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// rawStore keeps every version of a key in memory.
type rawStore struct {
	mu      sync.Mutex
	objects map[string][]ports.RawBody
}

func (r *rawStore) Put(_ context.Context, key string, body ports.RawBody) (domain.RawPayload, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.objects == nil {
		r.objects = map[string][]ports.RawBody{}
	}
	r.objects[key] = append(r.objects[key], body)
	sum := sha256.Sum256(body.Body)
	return domain.RawPayload{Ref: key, VersionID: "v" + strconv.Itoa(len(r.objects[key])), Hash: fmt.Sprintf("%x", sum)}, nil
}

func (r *rawStore) Get(_ context.Context, payload domain.RawPayload) (ports.RawBody, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, err := strconv.Atoi(strings.TrimPrefix(payload.VersionID, "v"))
	if versions := r.objects[payload.Ref]; err != nil || n < 1 || n > len(versions) {
		return ports.RawBody{}, fmt.Errorf("%w: %s", ports.ErrRawPayloadDefect, payload.Ref)
	}
	return r.objects[payload.Ref][n-1], nil
}

// withPartner serves the harness with a raw store and the partner simulator.
func withPartner(t *testing.T) (*rawStore, func(*application.Service)) {
	t.Helper()
	raw := &rawStore{}
	sim, err := partnersim.New(memory.FixedClock{At: occurred}, &memory.SequenceIDs{Prefix: "int-"}, raw)
	if err != nil {
		t.Fatalf("partnersim.New() = %v", err)
	}
	return raw, func(s *application.Service) { s.Raw, s.Partners = raw, sim }
}

func (h *harness) command(t *testing.T, method string, req, resp proto.Message) {
	t.Helper()
	if err := h.invoke(withKey(t), method, req, resp); err != nil {
		t.Fatalf("%s() = %v", method, err)
	}
}

// bounded is the connector ready for submitEffect and queryAsset, each
// bounded at limit bytes, with the authorization of query of org-1.
func (h *harness) bounded(t *testing.T, limit int32) {
	t.Helper()
	h.ready(t)
	h.command(t, "EnableOperation", &servicev1.EnableOperationRequest{
		ConnectorKey: "prt-1:production", CapabilityId: "cap-2", Operation: "queryAsset", Version: 1,
	}, &servicev1.EnableOperationResponse{})
	submit, read := submitEffect(), submitEffect()
	submit.MaxPayloadBytes = limit
	read.Operation, read.MaxPayloadBytes = "queryAsset", limit
	var declared servicev1.DeclareCapabilityProfileResponse
	h.command(t, "DeclareCapabilityProfile", &servicev1.DeclareCapabilityProfileRequest{
		ConnectorId: "prt-1:production", Version: 2, EffectiveFrom: int64(occurred), Operations: []*servicev1.OperationCapability{submit, read},
	}, &declared)
	if declared.GetDeclared() == nil {
		t.Fatalf("DeclareCapabilityProfile() = %v", &declared)
	}
	h.command(t, "GrantQueryAuthorization", grantRequest(), &servicev1.GrantQueryAuthorizationResponse{})
}

func receiveRequest(event string, body []byte) *servicev1.ReceivePartnerInteractionRequest {
	return &servicev1.ReceivePartnerInteractionRequest{
		ConnectorKey: "prt-1:production", Operation: "submitEffect", Source: servicev1.InteractionSource_INTERACTION_SOURCE_CALLBACK,
		PartnerEventId: event, BusinessDate: "2026-09-02", ContentType: "application/json; charset=utf-8", Body: body,
	}
}

func TestReceivePartnerInteractionAnswersTheReceiptOnTheWire(t *testing.T) {
	raw, configure := withPartner(t)
	h := newHarness(t, configure)
	h.bounded(t, 64)

	var first, again, large servicev1.ReceivePartnerInteractionResponse
	h.command(t, "ReceivePartnerInteraction", receiveRequest("evt-p-1", []byte(`{"format":"partnersim/v1"}`)), &first)
	h.command(t, "ReceivePartnerInteraction", receiveRequest("evt-p-1", []byte(`{"format":"partnersim/v1"}`)), &again)
	h.command(t, "ReceivePartnerInteraction", receiveRequest("evt-p-2", make([]byte, 65)), &large)

	received := first.GetReceived()
	if received.GetStatus() != servicev1.InteractionStatus_INTERACTION_STATUS_STORED || received.GetInteractionId() == "" {
		t.Fatalf("ReceivePartnerInteraction() = %v, want stored", &first)
	}
	if again.GetReceived().GetStatus() != servicev1.InteractionStatus_INTERACTION_STATUS_DUPLICATE {
		t.Fatalf("ReceivePartnerInteraction(again) = %v, want a duplicate", &again)
	}
	if large.GetRejection().GetCode() != string(domain.CodeExternalInteractionPayloadTooLarge) {
		t.Fatalf("ReceivePartnerInteraction(65 bytes) = %v, want payload-too-large", &large)
	}
	if n := len(raw.objects); n != 1 {
		t.Fatalf("raw keys = %d, want only the partner event received", n)
	}
	var found servicev1.FindExternalInteractionResponse
	if err := h.invoke(withoutKey(t), "FindExternalInteraction", &servicev1.FindExternalInteractionRequest{InteractionId: received.GetInteractionId()}, &found); err != nil ||
		found.GetInteraction().GetContentType() != "application/json; charset=utf-8" || found.GetInteraction().GetBusinessDate() != "2026-09-02" {
		t.Fatalf("FindExternalInteraction() = %v, %v", found.GetInteraction(), err)
	}
}

// The ceiling a profile may declare is what a message of the edge carries
// beside its envelope; a message above 4 MiB never reaches the handler.
func TestABodyAtTheCeilingCrossesTheEdgeAndALargerMessageDoesNot(t *testing.T) {
	_, configure := withPartner(t)
	h := newHarness(t, configure)
	h.bounded(t, application.MaxPayloadBytes)

	var atTheCeiling servicev1.ReceivePartnerInteractionResponse
	h.command(t, "ReceivePartnerInteraction", receiveRequest("evt-p-1", make([]byte, application.MaxPayloadBytes)), &atTheCeiling)
	err := h.invoke(withKey(t), "ReceivePartnerInteraction", receiveRequest("evt-p-2", make([]byte, 4<<20)), &servicev1.ReceivePartnerInteractionResponse{})

	if atTheCeiling.GetReceived().GetStatus() != servicev1.InteractionStatus_INTERACTION_STATUS_STORED {
		t.Fatalf("ReceivePartnerInteraction(ceiling) = %v, want stored", &atTheCeiling)
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("ReceivePartnerInteraction(4 MiB) = %v, want ResourceExhausted", err)
	}
}

func TestTheMethodsOfAPartnerFailAsUnavailableWithoutAdapterOrStore(t *testing.T) {
	h := newHarness(t)
	h.bounded(t, 64)

	receiveErr := h.invoke(withKey(t), "ReceivePartnerInteraction", receiveRequest("evt-p-1", []byte(`{}`)), &servicev1.ReceivePartnerInteractionResponse{})
	readErr := h.invoke(withKey(t), "ReadExternal", readRequest(), &servicev1.ReadExternalResponse{})

	if status.Code(receiveErr) != codes.Unavailable || status.Code(readErr) != codes.Unavailable {
		t.Fatalf("ReceivePartnerInteraction() = %v, ReadExternal() = %v; want Unavailable", receiveErr, readErr)
	}
}

func readRequest() *servicev1.ReadExternalRequest {
	return &servicev1.ReadExternalRequest{
		Operation: "queryAsset", ConnectorKey: "prt-1:production", OrganizationId: "org-1", AssetType: "duplicata",
		Parameters: string(partnersim.Parameters(partnersim.Read{ExternalRef: "ast-ext-1", Reading: []byte(`{"position":100}`)})),
	}
}

func TestReadExternalAnswersTheReadingOnTheWireAndFailsClosedOnceRevoked(t *testing.T) {
	raw, configure := withPartner(t)
	h := newHarness(t, configure)
	h.bounded(t, 1<<12)

	var read servicev1.ReadExternalResponse
	h.command(t, "ReadExternal", readRequest(), &read)
	h.command(t, "RevokeQueryAuthorization", &servicev1.RevokeQueryAuthorizationRequest{ConsentId: "cns-1", EffectiveAt: int64(occurred)}, &servicev1.RevokeQueryAuthorizationResponse{})
	keys := len(raw.objects)
	var revoked servicev1.ReadExternalResponse
	h.command(t, "ReadExternal", readRequest(), &revoked)

	r := read.GetReading()
	if r.GetInteractionId() == "" || r.GetAsOf() != int64(occurred) || r.GetProviderId() != "prt-1" || r.GetExternalReference() != "ast-ext-1" ||
		!strings.Contains(r.GetReading(), `"position":100`) || len(r.GetReadingHash()) != 64 {
		t.Fatalf("ReadExternal() = %v, want the reading with its provenance", &read)
	}
	if revoked.GetRejection().GetCode() != string(domain.CodeQueryAuthorizationConsentNotActive) || len(raw.objects) != keys {
		t.Fatalf("ReadExternal(revoked) = %v with %d raw keys, want consent-not-active and nothing kept", &revoked, len(raw.objects))
	}
}

func TestThePartnerMethodsBoundTheirForm(t *testing.T) {
	h := newHarness(t)
	receive := func(change func(*servicev1.ReceivePartnerInteractionRequest)) proto.Message {
		req := receiveRequest("evt-p-1", []byte(`{}`))
		change(req)
		return req
	}
	read := func(change func(*servicev1.ReadExternalRequest)) proto.Message {
		req := readRequest()
		change(req)
		return req
	}
	tests := map[string]struct {
		method string
		req    proto.Message
	}{
		"a source the partner does not send": {"ReceivePartnerInteraction", receive(func(r *servicev1.ReceivePartnerInteractionRequest) {
			r.Source = servicev1.InteractionSource_INTERACTION_SOURCE_STATE_QUERY
		})},
		"an empty body":           {"ReceivePartnerInteraction", receive(func(r *servicev1.ReceivePartnerInteractionRequest) { r.Body = nil })},
		"no content type":         {"ReceivePartnerInteraction", receive(func(r *servicev1.ReceivePartnerInteractionRequest) { r.ContentType = "" })},
		"a business date not ISO": {"ReceivePartnerInteraction", receive(func(r *servicev1.ReceivePartnerInteractionRequest) { r.BusinessDate = "02/09/2026" })},
		"a malformed partner event": {"ReceivePartnerInteraction", receive(func(r *servicev1.ReceivePartnerInteractionRequest) {
			r.PartnerEventId = "evt p 1"
		})},
		"a write named as a read": {"ReadExternal", read(func(r *servicev1.ReadExternalRequest) { r.Operation = "submitEffect" })},
		"a guarded read without organization": {"ReadExternal", read(func(r *servicev1.ReadExternalRequest) {
			r.OrganizationId = ""
		})},
		"a guarded read without asset type": {"ReadExternal", read(func(r *servicev1.ReadExternalRequest) { r.AssetType = "" })},
		"parameters that are not JSON":      {"ReadExternal", read(func(r *servicev1.ReadExternalRequest) { r.Parameters = "{" })},
		"parameters above 64 KiB": {"ReadExternal", read(func(r *servicev1.ReadExternalRequest) {
			r.Parameters = `"` + strings.Repeat("a", 64<<10) + `"`
		})},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var resp proto.Message = &servicev1.ReceivePartnerInteractionResponse{}
			if tc.method == "ReadExternal" {
				resp = &servicev1.ReadExternalResponse{}
			}
			if err := h.invoke(withKey(t), tc.method, tc.req, resp); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("%s() = %v, want InvalidArgument", tc.method, err)
			}
		})
	}
}
