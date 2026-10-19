//go:build integration

package app_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	signingKeyID     = "bff-test"
	organizationPath = "/identity/organizations/org-1"
	tokenRejected    = "auth: token rejected"
	refusalReasonKey = "dmpf.auth.refusal_reason"
)

type signingIssuer struct {
	url string
	key *rsa.PrivateKey
}

func newSigningIssuer(t *testing.T) signingIssuer {
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
			"kty": "RSA", "kid": signingKeyID, "use": "sig", "alg": "RS256",
			"n": encode(key.N.Bytes()), "e": encode(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	server.Config.Handler = mux
	return signingIssuer{url: server.URL, key: key}
}

func (i signingIssuer) token(t *testing.T, subject, session string, extra map[string]any) string {
	t.Helper()
	now := time.Now()
	claims := map[string]any{
		"iss": i.url, "aud": realmAudience, "sub": subject, "sid": session,
		"iat": now.Add(-10 * time.Second).Unix(), "exp": now.Add(5 * time.Minute).Unix(),
	}
	for name, value := range extra {
		claims[name] = value
	}
	segment := func(value any) string {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal() = %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	signed := segment(map[string]string{"alg": "RS256", "kid": signingKeyID, "typ": "JWT"}) + "." + segment(claims)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("SignPKCS1v15() = %v", err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func organizations(tenants map[string]any) map[string]any {
	return map[string]any{"organization": tenants}
}

type organizationLookups struct{ asked atomic.Int32 }

// The fake identity registers FindEffectivePermissions alone; grpc-go hands any
// other method of a known service to this handler (grpc server.go:1591).
func (o *organizationLookups) serve(_ any, stream grpc.ServerStream) error {
	method, _ := grpc.MethodFromServerStream(stream)
	if method != "/"+rpc.IdentityServiceName+"/FindOrganization" {
		return status.Errorf(codes.Unimplemented, "%s is not faked", method)
	}
	var req servicev1.FindOrganizationRequest
	if err := stream.RecvMsg(&req); err != nil {
		return err
	}
	o.asked.Add(1)
	return stream.SendMsg(&servicev1.FindOrganizationResponse{Organization: &servicev1.Organization{
		OrganizationId: req.GetOrganizationId(),
		Kinds:          []servicev1.OrganizationKind{servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT},
		Jurisdiction:   "BR",
		TaxIdType:      servicev1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxId:          "12345678000195",
		LegalName:      "Acme Ltda",
		Status:         servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE,
	}})
}

type realmEdge struct {
	edge
	issuer        signingIssuer
	identity      *fakeIdentity
	organizations *organizationLookups
}

func startRealmEdge(t *testing.T) realmEdge {
	t.Helper()
	lookups := &organizationLookups{}
	identity := serveIdentity(t, grpc.UnknownServiceHandler(lookups.serve))
	identity.grant(effectivePermission)
	issuer := newSigningIssuer(t)
	e := startEdge(t, "IDENTITY_GRPC_TARGET", identity.addr, "REDIS_URL", tb.Env(t, "REDIS_URL"),
		"OIDC_ISSUER", issuer.url, "OIDC_AUDIENCE", realmAudience, "OIDC_TENANT_CLAIM", realmTenantClaim)
	return realmEdge{edge: e, issuer: issuer, identity: identity, organizations: lookups}
}

func (e realmEdge) hops() (permissions, organizations int) {
	return len(e.identity.questions()), int(e.organizations.asked.Load())
}

func (e realmEdge) refusals(t *testing.T) []string {
	t.Helper()
	var reasons []string
	for _, record := range e.records(t) {
		if record.Body().AsString() == tokenRejected {
			reasons = append(reasons, attributesOf(record)[refusalReasonKey].AsString())
		}
	}
	return reasons
}

func getWithToken(t *testing.T, addr, path, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		t.Fatalf("NewRequest() = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s = %v", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll() = %v", err)
	}
	return res.StatusCode, string(body)
}

func requireRefusal(t *testing.T, answered int, body string, wantStatus int, wantCode string) {
	t.Helper()
	var refusal struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(body), &refusal); err != nil || answered != wantStatus || refusal.Code != wantCode {
		t.Fatalf("GET %s = %d %s, want %d %s", organizationPath, answered, body, wantStatus, wantCode)
	}
}

func unique(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func TestARevokedTokenAcceptedBeforeIsRefusedWithoutReachingTheIdentity(t *testing.T) {
	cases := []struct {
		name   string
		revoke func(subject, session string) authn.Revocation
	}{
		{name: "the session", revoke: func(_, session string) authn.Revocation {
			return authn.Revocation{Kind: authn.RevocationSession, SessionID: session}
		}},
		{name: "the subject", revoke: func(subject, _ string) authn.Revocation {
			return authn.Revocation{Kind: authn.RevocationSubject, Subject: ports.SubjectID(subject)}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := startRealmEdge(t)
			subject, session := unique("user"), unique("sid")
			token := e.issuer.token(t, subject, session, organizations(map[string]any{"acme": map[string]any{"tenant_id": []string{"T"}}}))

			answered, body := getWithToken(t, e.addr, organizationPath, token)
			if answered != http.StatusOK || !strings.Contains(body, `"organizationId":"org-1"`) {
				t.Fatalf("GET %s before the revocation = %d %s, want 200 with the organization", organizationPath, answered, body)
			}
			asked := e.identity.questions()
			if len(asked) != 1 || asked[0].userID != subject || strings.Join(asked[0].md.Get(kernelgrpc.TenantKey), ",") != "T" {
				t.Fatalf("FindEffectivePermissions calls = %+v, want one for the subject in the tenant of the organization claim", asked)
			}
			if _, lookups := e.hops(); lookups != 1 {
				t.Fatalf("FindOrganization calls = %d, want 1", lookups)
			}
			if reasons := e.refusals(t); len(reasons) != 0 {
				t.Fatalf("%q records before the revocation = %v, want none", tokenRejected, reasons)
			}

			revokeIn(t, tc.revoke(subject, session))

			answered, body = getWithToken(t, e.addr, organizationPath, token)
			requireRefusal(t, answered, body, http.StatusUnauthorized, "unauthenticated")
			if permissions, lookups := e.hops(); permissions != 1 || lookups != 1 {
				t.Fatalf("identity calls after the revocation = %d permissions and %d lookups, want no new hop", permissions-1, lookups-1)
			}
			if reasons := e.refusals(t); len(reasons) != 1 || reasons[0] != "session_revoked" {
				t.Fatalf("%q reasons = %v, want exactly session_revoked", tokenRejected, reasons)
			}
		})
	}
}

func revokeIn(t *testing.T, revocation authn.Revocation) {
	t.Helper()
	url := tb.Env(t, "REDIS_URL")
	cfg, err := redis.FromEnv(lookup("REDIS_URL", url))
	if err != nil {
		t.Fatalf("redis.FromEnv() = %v", err)
	}
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Fatalf("redis.NewClient() = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	revocation.EventID = unique("ev")
	revocation.OccurredAt, revocation.ExpiresAt = time.Now(), time.Now().Add(time.Minute)
	changed, err := redis.NewRevocationStore(client, clock.System()).Revoke(context.Background(), revocation)
	if err != nil || !changed {
		t.Fatalf("Revoke() = %t, %v, want the entry written", changed, err)
	}
}

func TestATokenWithoutOneActiveOrganizationIsRefusedBeforeThePermissions(t *testing.T) {
	cases := []struct {
		name   string
		claims map[string]any
	}{
		{name: "without the organization claim"},
		{name: "with two organizations", claims: organizations(map[string]any{
			"acme":   map[string]any{"tenant_id": []string{"T"}},
			"globex": map[string]any{"tenant_id": []string{"U"}},
		})},
		{name: "without tenant_id", claims: organizations(map[string]any{"acme": map[string]any{}})},
		{name: "with a multivalued tenant_id", claims: organizations(map[string]any{"acme": map[string]any{"tenant_id": []string{"T", "U"}}})},
	}
	e := startRealmEdge(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := e.issuer.token(t, unique("user"), unique("sid"), tc.claims)

			answered, body := getWithToken(t, e.addr, organizationPath, token)

			requireRefusal(t, answered, body, http.StatusForbidden, "tenant-unresolved")
			if permissions, lookups := e.hops(); permissions != 0 || lookups != 0 {
				t.Fatalf("identity calls = %d permissions and %d lookups, want none", permissions, lookups)
			}
		})
	}
	if reasons := e.refusals(t); len(reasons) != 0 {
		t.Fatalf("%q records = %v, want none: every token was verified", tokenRejected, reasons)
	}
}
