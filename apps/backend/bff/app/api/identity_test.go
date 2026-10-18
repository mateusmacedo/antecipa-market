package api_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
)

const (
	noCredential      = ""
	forgedCredential  = "Bearer not-a-declaration"
	tenantlessSubject = `Bearer {"sub":"tester","permissions":[]}`
)

func TestARequestWithoutACredentialNeverReachesTheHandler(t *testing.T) {
	f := newFixture(t)

	cases := []struct {
		name, method, path string
	}{
		{name: "actOnProbe", method: http.MethodPost, path: "/probes/p-1/act"},
		{name: "findProbe", method: http.MethodGet, path: "/probes/p-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.do(t, tc.method, tc.path, nil, "Authorization", noCredential, "Idempotency-Key", "k-1")

			requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
		})
	}

	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0: the edge denies before the handler", n)
	}
}

func TestAForgedCredentialIsRefusedAsUnauthenticated(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Authorization", forgedCredential)

	requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestAnAuthenticatedSubjectWithoutTenantIsForbiddenNotUnauthenticated(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Authorization", tenantlessSubject)

	requireRejection(t, rec, http.StatusForbidden, "tenant-unresolved")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestAHeaderAssertingAnotherTenantIsRefused(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "X-Tenant-ID", "globex")

	requireRejection(t, rec, http.StatusForbidden, "identity-mismatch")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestAHeaderRepeatingTheResolvedTenantIsRedundantNotRefused(t *testing.T) {
	f := newFixture(t)

	if rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "X-Tenant-ID", testTenant); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: an equal value is redundant, and still never the source", rec.Code)
	}
}

func TestASubjectWithoutTheRoutesPermissionIsRefusedAtTheEdge(t *testing.T) {
	f := newFixture(t)
	readOnly := `Bearer {"sub":"tester","tenant":"acme","permissions":["identity:read-only"]}`

	rec := f.do(t, http.MethodPost, "/probes/p-1/act", strings.NewReader(`{}`),
		"Idempotency-Key", "k-1", "Content-Type", "application/json", "Authorization", readOnly)

	requireRejection(t, rec, http.StatusForbidden, "permission-denied")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func newIssuer(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() = %v", err)
	}
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)
	encode := base64.RawURLEncoding.EncodeToString
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": server.URL, "jwks_uri": server.URL + "/jwks",
			"authorization_endpoint": server.URL + "/auth", "token_endpoint": server.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "bff-test", "use": "sig", "alg": "RS256",
			"n": encode(key.N.Bytes()), "e": encode(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	server.Config.Handler = mux
	return server.URL
}

func TestATokenTheRealVerifierRejectsIsRefusedAsUnauthenticated(t *testing.T) {
	verifier, err := authn.NewVerifier(context.Background(), authn.Config{
		Issuer:           newIssuer(t),
		Audience:         "dmpf-bff",
		TenantClaim:      "organization.*.tenant_id",
		DiscoveryTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewVerifier() = %v", err)
	}
	f := newFixture(t, withAuthenticator(verifier))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Authorization", "Bearer not-a-jwt")

	requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}
