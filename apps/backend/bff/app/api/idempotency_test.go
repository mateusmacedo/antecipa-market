package api_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"google.golang.org/grpc/metadata"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	lookupPath       = "/probe-lookups"
	lookupPermission = "identity:look-up-probe"
)

func keyedBindings(p *probe) []api.Binding {
	return append(probeBindings(p), api.NewBinding(api.RouteLookup, syntheticRoute("lookUpProbe", http.MethodPost, lookupPath, lookupPermission), p.handle))
}

type keyedRequest struct {
	method string
	path   string
}

var (
	commandRequest = keyedRequest{http.MethodPost, "/probes/p-1/act"}
	lookupRequest  = keyedRequest{http.MethodPost, lookupPath}
	queryRequest   = keyedRequest{http.MethodGet, "/probes/p-1"}
)

func (k keyedRequest) send(t *testing.T, f fixture) *httptest.ResponseRecorder {
	t.Helper()
	return f.do(t, k.method, k.path, strings.NewReader(`{}`), "Idempotency-Key", "k-1", "Content-Type", "application/json")
}

func keyedEdge(t *testing.T, identity *fakeIdentity) (fixture, rpc.Identity) {
	t.Helper()
	client := identity.grant(commandPermission, queryPermission, lookupPermission).dial(t, obsclock.System())
	return edgeOver(t, client, withBindings(keyedBindings)), client
}

func derived(subject, key string) string {
	digest := sha256.Sum256([]byte(subject + "\x00" + key))
	return hex.EncodeToString(digest[:])
}

func TestTheAccessLogCarriesTheClientAndTheDerivedKey(t *testing.T) {
	f := newFixture(t)

	f.post(t, "/probes/p-1/act", `{}`)

	record := onlyAccessLog(t, f)
	if record["dmpf.idempotency_key"] != "k-1" || record["dmpf.idempotency_key.derived"] != derived("tester", "k-1") {
		t.Fatalf("access log = %v, want the client key and the key derived from the subject", record)
	}
}

func TestTheAccessLogFlagsAnInvalidKeyWithoutRecordingIt(t *testing.T) {
	f := newFixture(t)

	f.do(t, http.MethodPost, "/probes/p-1/act", strings.NewReader(`{}`),
		"Idempotency-Key", "k 1\nforged=true", "Content-Type", "application/json", "Authorization", testCredential)

	record := onlyAccessLog(t, f)
	if _, logged := record["dmpf.idempotency_key"]; logged || record["dmpf.idempotency_key.invalid"] != true {
		t.Fatalf("access log = %v, want dmpf.idempotency_key.invalid and no dmpf.idempotency_key: the header is client input", record)
	}
}

func TestAKeyRefusedAtTheEdgeDerivesNoKeyInTheAccessLog(t *testing.T) {
	for name, key := range map[string]string{"invalid": "k 1", "missing": ""} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)

			rec := f.do(t, http.MethodPost, "/probes/p-1/act", strings.NewReader(`{}`),
				"Idempotency-Key", key, "Content-Type", "application/json", "Authorization", testCredential)

			requireRejection(t, rec, http.StatusBadRequest, name+"-idempotency-key")
			if derived, logged := onlyAccessLog(t, f)["dmpf.idempotency_key.derived"]; logged {
				t.Fatalf("dmpf.idempotency_key.derived = %v, want none for a key the edge refused", derived)
			}
		})
	}
}

func TestCORSExposesTheReplayHeader(t *testing.T) {
	f := newFixture(t, withCORS("https://app.example"))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Origin", "https://app.example")

	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposed, "Idempotent-Replayed") || !strings.Contains(exposed, "X-Correlation-ID") {
		t.Fatalf("Access-Control-Expose-Headers = %q, want X-Correlation-ID and Idempotent-Replayed", exposed)
	}
}

func TestOnlyACommandForwardsTheDerivedKeyAndNeverToThePermissionsHop(t *testing.T) {
	cases := map[string]struct {
		request keyedRequest
		want    []string
	}{
		"command": {commandRequest, []string{derived("tester", "k-1")}},
		"lookup":  {lookupRequest, nil},
		"query":   {queryRequest, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			identity := &fakeIdentity{}
			f, _ := keyedEdge(t, identity)

			rec := tc.request.send(t, f)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
			}
			permissions, business := identity.hopsTo(methodPermissions), identity.hopsTo(methodOrganization)
			if len(permissions) != 1 || len(business) != 1 {
				t.Fatalf("hops = %d to the permissions and %d to the route, want one each", len(permissions), len(business))
			}
			if got := permissions[0].md.Get(kernelgrpc.IdempotencyKey); len(got) != 0 {
				t.Fatalf("permissions hop carried %s = %v, want none: the key belongs to the command alone", kernelgrpc.IdempotencyKey, got)
			}
			if got := business[0].md.Get(kernelgrpc.IdempotencyKey); !slices.Equal(got, tc.want) {
				t.Fatalf("route hop carried %s = %v, want %v", kernelgrpc.IdempotencyKey, got, tc.want)
			}
		})
	}
}

func TestTheReplayHeaderReachesTheClientOnlyOnACommandBelow500(t *testing.T) {
	replayed := metadata.Pairs(kernelgrpc.ReplayedHeader, "true")
	cases := map[string]struct {
		request keyedRequest
		header  metadata.MD
		status  int
		want    string
	}{
		"command answered":            {commandRequest, replayed, http.StatusCreated, "true"},
		"command refused":             {commandRequest, replayed, http.StatusUnprocessableEntity, "true"},
		"command failing at the edge": {commandRequest, replayed, http.StatusInternalServerError, ""},
		"command unavailable":         {commandRequest, replayed, http.StatusServiceUnavailable, ""},
		"command answered afresh":     {commandRequest, nil, http.StatusCreated, ""},
		"lookup":                      {lookupRequest, replayed, http.StatusOK, ""},
		"query":                       {queryRequest, replayed, http.StatusOK, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			identity := &fakeIdentity{}
			if tc.header != nil {
				identity.sendingHeader(methodOrganization, tc.header)
			}
			f, client := keyedEdge(t, identity)
			f.probe.respond = func(w http.ResponseWriter, r *http.Request) {
				if _, err := client.FindOrganization(r.Context(), &servicev1.FindOrganizationRequest{OrganizationId: "org-1"}); err != nil {
					api.WriteFailure(r, w, rpc.Classify(err))
					return
				}
				api.WriteJSON(r, w, tc.status, map[string]string{})
			}

			rec := tc.request.send(t, f)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.status, rec.Body.String())
			}
			if got := rec.Header().Get(api.ReplayedHeader); got != tc.want {
				t.Fatalf("%s = %q, want %q", api.ReplayedHeader, got, tc.want)
			}
		})
	}
}
