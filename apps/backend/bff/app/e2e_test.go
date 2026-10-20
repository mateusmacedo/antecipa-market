//go:build integration

package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

func devCredential(t *testing.T, subject, tenant string) string {
	t.Helper()
	declared, err := json.Marshal(map[string]string{"sub": subject, "tenant": tenant})
	if err != nil {
		t.Fatalf("Marshal() = %v", err)
	}
	return "Bearer " + string(declared)
}

func (top *topology) call(t *testing.T, req edgeRequest, authorization string) (int, []byte) {
	t.Helper()
	var body io.Reader
	if req.body != "" {
		body = strings.NewReader(req.body)
	}
	r, err := http.NewRequest(req.method, "http://"+top.bffAddr+req.path, body)
	if err != nil {
		t.Fatalf("NewRequest() = %v", err)
	}
	for name, value := range req.headers {
		r.Header.Set(name, value)
	}
	if authorization != "" {
		r.Header.Set("Authorization", authorization)
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("%s %s = %v", req.method, req.path, err)
	}
	defer func() { _ = res.Body.Close() }()
	answered, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll() = %v", err)
	}
	return res.StatusCode, answered
}

// The identity refuses with the same code but its own message, so only the
// message tells the edge's refusal apart (rejectionMessage, app/api/middleware.go).
const (
	edgeForbidden    = "the resolved identity does not carry what this operation requires"
	edgeUnauthorized = "the request carries no verifiable credential"
)

func requireRejection(t *testing.T, req edgeRequest, status int, body []byte, wantStatus int, wantCode, wantMessage string) {
	t.Helper()
	var rejection struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &rejection); err != nil || status != wantStatus || rejection.Code != wantCode || rejection.Message != wantMessage {
		t.Errorf("%s %s (%s) = %d %s, want %d %s from the edge", req.method, req.path, req.name, status, body, wantStatus, wantCode)
	}
}

func TestTheEdgeEndToEnd(t *testing.T) {
	bin := buildBinaries(t)
	top := &topology{}
	top.boot(t, bin)

	routes := api.Routes(app.Defaults().RouteBudget)
	if len(routes) == 0 {
		t.Fatal("api.Routes() is empty: the e2e would prove nothing")
	}

	t.Run("every route refuses a user without a membership, and a missing key before the permissions", func(t *testing.T) {
		unlinked := devCredential(t, newUUID(), "e2e-tenant-"+newUUID())
		for _, route := range routes {
			for _, req := range requestsFor(route) {
				status, body := top.call(t, req, unlinked)
				if req.omittedKey != "" {
					requireRejection(t, req, status, body, http.StatusBadRequest, "missing-idempotency-key", req.method+" requires the "+req.omittedKey+" header")
					continue
				}
				requireRejection(t, req, status, body, http.StatusForbidden, "permission-denied", edgeForbidden)
			}
		}
	})

	t.Run("every route refuses an anonymous caller", func(t *testing.T) {
		for _, route := range routes {
			for _, req := range requestsFor(route) {
				status, body := top.call(t, req, "")
				requireRejection(t, req, status, body, http.StatusUnauthorized, "unauthenticated", edgeUnauthorized)
			}
		}
	})

	t.Run("the identity contract is served", func(t *testing.T) {
		want, err := os.ReadFile(filepath.Join(top.root, identityContractFile))
		if err != nil {
			t.Fatalf("ReadFile() = %v", err)
		}
		status, body := top.call(t, edgeRequest{name: "contract", method: http.MethodGet, path: api.IdentityContractPath}, "")
		if status != http.StatusOK || !bytes.Equal(body, want) {
			t.Fatalf("GET %s = %d with %d bytes, want 200 with the %d bytes of %s", api.IdentityContractPath, status, len(body), len(want), identityContractFile)
		}
	})

	t.Run("the administration port reports ready", func(t *testing.T) {
		res, err := http.Get("http://" + top.adminAddr + "/readyz")
		if err != nil {
			t.Fatalf("GET /readyz = %v", err)
		}
		_ = res.Body.Close()
		if res.StatusCode/100 != 2 {
			t.Fatalf("GET /readyz = %d, want 2xx", res.StatusCode)
		}
	})
}
