package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

var targets = []string{"IDENTITY_GRPC_TARGET", "identity:9090",
	"REDIS_URL", "redis://revocation:sentinel-redis-password@redis:6379/0"}

// The edge demands a subject on every route, so authn refuses a start that
// declares neither a verifier nor the mock: usable configs must name one.
const devMock = "AUTH_DEV_MOCK"

const (
	realmIssuer      = "http://keycloak:8080/realms/dmpf"
	realmAudience    = "dmpf-bff"
	realmTenantClaim = "organization.*.tenant_id"
)

func without(pairs []string, variable string) []string {
	kept := []string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i] != variable {
			kept = append(kept, pairs[i], pairs[i+1])
		}
	}
	return kept
}

func TestFromEnvRefusesAStartThatDeclaresNoIdentity(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true")...))

	if err == nil {
		t.Fatal("FromEnv() = nil: a start that could authenticate nobody must be refused")
	}
	if !strings.Contains(err.Error(), devMock) {
		t.Fatalf("FromEnv() = %v, want the error to name %s", err, devMock)
	}
}

func TestFromEnvAppliesTheDefaults(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.Service != "bff" || !cfg.GRPCInsecure || cfg.IdentityTarget != "identity:9090" {
		t.Fatalf("cfg = %+v, want :8080, bff, the opt-out and the identity target", cfg)
	}
	if cfg.RouteBudget.Validate() != nil || cfg.RouteBudget.Limit <= 0 || cfg.RouteBudget.Dependency != "identity" {
		t.Fatalf("RouteBudget = %+v, want a valid budget on the identity dependency", cfg.RouteBudget)
	}
}

func TestFromEnvNamesTheMissingIdentityTarget(t *testing.T) {
	_, err := app.FromEnv(lookup(append(without(targets, "IDENTITY_GRPC_TARGET"), "GRPC_INSECURE", "true", devMock, "true")...))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "IDENTITY_GRPC_TARGET") {
		t.Fatalf("FromEnv() = %v, want IDENTITY_GRPC_TARGET named", err)
	}
}

func TestFromEnvRequiresATransportPolicy(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, devMock, "true")...))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_INSECURE") || !strings.Contains(err.Error(), "GRPC_CA_FILE") {
		t.Fatalf("FromEnv() = %v, want both transport variables named (GRP-15)", err)
	}
}

func TestFromEnvAcceptsATrustAuthority(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_CA_FILE", "/etc/ca.pem", "GRPC_SERVER_NAME", "identity.internal",
		"GRPC_CLIENT_CERT_FILE", "/etc/bff.crt", "GRPC_CLIENT_KEY_FILE", "/etc/bff.key", "CORS_ORIGINS", "http://a, http://b", devMock, "true")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.GRPCInsecure || cfg.CAFile != "/etc/ca.pem" || cfg.ServerName != "identity.internal" || len(cfg.CORSOrigins) != 2 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

// IDN-03: the contexts only trust a verified workload, so an edge that trusts
// their authority also has to present its own certificate.
func TestFromEnvRefusesATrustAuthorityWithoutAClientPair(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, "GRPC_CA_FILE", "/etc/ca.pem", devMock, "true")...))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_CLIENT_CERT_FILE") {
		t.Fatalf("FromEnv() = %v, want the client pair named", err)
	}
}

func TestFromEnvRefusesAnInvalidBoolean(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "maybe")...))

	if !errors.Is(err, app.ErrInvalidVariable) {
		t.Fatalf("FromEnv() = %v, want ErrInvalidVariable", err)
	}
}

func TestFromEnvReadsTheDrainDelay(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")

	cfg, err := app.FromEnv(lookup(base...))
	if err != nil || cfg.DrainDelay != 0 {
		t.Fatalf("FromEnv() = %v, %v; want no drain by default", cfg.DrainDelay, err)
	}
	cfg, err = app.FromEnv(lookup(append(base, "DRAIN_DELAY", "5s")...))
	if err != nil || cfg.DrainDelay != 5*time.Second {
		t.Fatalf("FromEnv() = %v, %v; want 5s", cfg.DrainDelay, err)
	}
	for _, invalid := range []string{"soon", "-1s"} {
		if _, err := app.FromEnv(lookup(append(base, "DRAIN_DELAY", invalid)...)); err == nil || !strings.Contains(err.Error(), "DRAIN_DELAY") {
			t.Fatalf("FromEnv(DRAIN_DELAY=%s) = %v, want the variable named", invalid, err)
		}
	}
}

func TestFromEnvRefusesADrainThePodCannotFinish(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")

	if _, err := app.FromEnv(lookup(append(base, "DRAIN_DELAY", "10s")...)); err != nil {
		t.Fatalf("FromEnv(DRAIN_DELAY=10s) = %v, want it accepted", err)
	}
	if _, err := app.FromEnv(lookup(append(base, "DRAIN_DELAY", "11s")...)); !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "DRAIN_DELAY") {
		t.Fatalf("FromEnv(DRAIN_DELAY=11s) = %v, want ErrInvalidVariable naming DRAIN_DELAY", err)
	}
}

func TestFromEnvRefusesListenersThatCollide(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")
	const malformed, collides = "is not host:port", "share a listener"
	cases := []struct {
		http, admin string
		refusal     string
	}{
		{"8080", ":8090", malformed},
		{":9000", ":9000", collides},
		{":9000", "127.0.0.1:9000", collides},
		{"127.0.0.1:9000", "0.0.0.0:9000", collides},
		{"[::]:9000", "127.0.0.1:9000", collides},
		{"127.0.0.1:9000", "127.0.0.1:9000", collides},
		{"[::0]:9000", "127.0.0.1:9000", collides},
		{":09000", ":9000", collides},
		{":http", ":80", collides},
		{":http", ":8090", ""},
		{"127.0.0.1:9000", "127.0.0.2:9000", ""},
		{"127.0.0.1:0", "127.0.0.1:0", ""},
		{":8080", ":8090", ""},
	}
	for _, tc := range cases {
		_, err := app.FromEnv(lookup(append(base, "HTTP_ADDR", tc.http, "ADMIN_ADDR", tc.admin)...))
		switch {
		case tc.refusal != "" && (!errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "HTTP_ADDR") || !strings.Contains(err.Error(), tc.refusal)):
			t.Errorf("FromEnv(HTTP_ADDR=%s, ADMIN_ADDR=%s) = %v, want ErrInvalidVariable naming HTTP_ADDR and containing %q", tc.http, tc.admin, err, tc.refusal)
		case tc.refusal == "" && err != nil:
			t.Errorf("FromEnv(HTTP_ADDR=%s, ADMIN_ADDR=%s) = %v, want it accepted", tc.http, tc.admin, err)
		}
	}
}

func TestFromEnvReadsTheAdministrationAddress(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")

	cfg, err := app.FromEnv(lookup(base...))
	if err != nil || cfg.AdminAddr != ":8090" || cfg.AdminAddr == cfg.HTTPAddr {
		t.Fatalf("FromEnv() = %q, %v; want :8090, apart from the public %q", cfg.AdminAddr, err, cfg.HTTPAddr)
	}
	cfg, err = app.FromEnv(lookup(append(base, "ADMIN_ADDR", "127.0.0.1:9100")...))
	if err != nil || cfg.AdminAddr != "127.0.0.1:9100" {
		t.Fatalf("FromEnv() = %q, %v; want 127.0.0.1:9100", cfg.AdminAddr, err)
	}
	if _, err := app.FromEnv(lookup(append(base, "ADMIN_ADDR", "8090")...)); !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "ADMIN_ADDR") {
		t.Fatalf("FromEnv(ADMIN_ADDR=8090) = %v, want ErrInvalidVariable naming ADMIN_ADDR", err)
	}
}

func TestFromEnvReadsTheContractPath(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true",
		"OPENAPI_IDENTITY_PATH", "/contracts/identity.yaml")...))

	if err != nil || cfg.IdentityContractPath != "/contracts/identity.yaml" {
		t.Fatalf("FromEnv() = %q, %v; want the identity contract path", cfg.IdentityContractPath, err)
	}
}

func TestFromEnvRefusesAStartWithoutTheRevocationStore(t *testing.T) {
	_, err := app.FromEnv(lookup(append(without(targets, "REDIS_URL"), "GRPC_INSECURE", "true", devMock, "true")...))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("FromEnv() = %v, want REDIS_URL named: the edge never serves without the deny-list", err)
	}
}

func TestFromEnvRefusesAnUnusableRevocationStoreWithoutEchoingIt(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")
	cases := []struct {
		name    string
		env     []string
		wantErr error
	}{
		{"not a redis URL", []string{"REDIS_URL", "http://revocation:sentinel-redis-password@redis:6379/0"}, redis.ErrInvalidURL},
		{"database other than zero", []string{"REDIS_URL", "redis://revocation:sentinel-redis-password@redis:6379/1"}, redis.ErrDatabaseNotZero},
		{"clear text when TLS is required", []string{"REDIS_REQUIRE_TLS", "true"}, redis.ErrTLSRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := app.FromEnv(lookup(append(base, tc.env...)...))

			if !errors.Is(err, tc.wantErr) || strings.Contains(err.Error(), "sentinel-redis-password") {
				t.Fatalf("FromEnv() = %v, want %v with the URL left out", err, tc.wantErr)
			}
		})
	}
}

func TestFromEnvReadsTheRevocationStore(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.Redis.Options == nil || cfg.Redis.Options.Addr != "redis:6379" || cfg.Redis.Options.Username != "revocation" {
		t.Fatalf("cfg.Redis = %v, want redis:6379 as revocation", cfg.Redis)
	}
}

func TestFromEnvRefusesAnInvalidTimeout(t *testing.T) {
	base := append(targets, "GRPC_INSECURE", "true", devMock, "true")
	for _, variable := range []string{"REDIS_DIAL_TIMEOUT_MS", "REDIS_COMMAND_TIMEOUT_MS", "OIDC_REVOCATION_TIMEOUT_MS"} {
		for _, invalid := range []string{"soon", "0", "-1"} {
			t.Run(variable+"="+invalid, func(t *testing.T) {
				_, err := app.FromEnv(lookup(append(base, variable, invalid)...))

				if !errors.Is(err, envconfig.ErrInvalidVariable) || !strings.Contains(err.Error(), variable) {
					t.Fatalf("FromEnv() = %v, want ErrInvalidVariable naming %s", err, variable)
				}
			})
		}
	}
}

func TestFromEnvRefusesTheMockAlongsideAnIssuer(t *testing.T) {
	_, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true",
		"OIDC_ISSUER", realmIssuer, "OIDC_AUDIENCE", realmAudience, "OIDC_TENANT_CLAIM", realmTenantClaim)...))

	if !errors.Is(err, authn.ErrMockWithVerifier) {
		t.Fatalf("FromEnv() = %v, want authn.ErrMockWithVerifier", err)
	}
}

func TestFromEnvNamesWhatTheVerifierLacks(t *testing.T) {
	verifier := []string{"OIDC_ISSUER", realmIssuer, "OIDC_AUDIENCE", realmAudience, "OIDC_TENANT_CLAIM", realmTenantClaim}
	for _, variable := range []string{"OIDC_AUDIENCE", "OIDC_TENANT_CLAIM"} {
		t.Run(variable, func(t *testing.T) {
			_, err := app.FromEnv(lookup(append(append(targets, "GRPC_INSECURE", "true"), without(verifier, variable)...)...))

			if !errors.Is(err, authn.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestFromEnvReadsTheRealmVerifierLiterally(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true",
		"OIDC_ISSUER", realmIssuer, "OIDC_AUDIENCE", realmAudience, "OIDC_TENANT_CLAIM", realmTenantClaim)...))

	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	if cfg.Auth.Issuer != realmIssuer || cfg.Auth.Audience != realmAudience || cfg.Auth.TenantClaim != realmTenantClaim || cfg.Auth.DevMock {
		t.Fatalf("cfg.Auth = %+v, want the realm verifier with the nested tenant claim untouched", cfg.Auth)
	}
}

func TestFromEnvLeavesCORSClosedByDefault(t *testing.T) {
	for _, declared := range []string{"", " , "} {
		cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true", "CORS_ORIGINS", declared)...))

		if err != nil || len(cfg.CORSOrigins) != 0 {
			t.Fatalf("FromEnv(CORS_ORIGINS=%q) = %v, %v; want no origin allowed", declared, cfg.CORSOrigins, err)
		}
	}
}

func TestFromEnvReadsNoLegacyTelemetryVariable(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true",
		"SERVICE", "legacy", "SERVICE_VERSION", "9.9.9", "INSTANCE_ID", "legacy-1", "OTLP_ENDPOINT", "legacy:4317", "OTLP_INSECURE", "not-a-bool")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want the legacy telemetry variables ignored (RF-E1)", err)
	}
	telemetry := app.TelemetryOf(cfg)
	if telemetry.Service != "bff" || telemetry.Version == "9.9.9" || telemetry.Instance == "legacy-1" {
		t.Fatalf("TelemetryOf() = %+v, want the identity left to OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES (RF-E1, RF-E3)", telemetry)
	}
}

func TestTheIdentityOfTheProcessIsLeftToTheEnvironment(t *testing.T) {
	cfg, err := app.FromEnv(lookup(append(targets, "GRPC_INSECURE", "true", devMock, "true")...))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	if telemetry := app.TelemetryOf(cfg); telemetry.Version != "" || telemetry.Instance != "" {
		t.Fatalf("TelemetryOf() declares version %q and instance %q, want neither: both come from OTEL_RESOURCE_ATTRIBUTES (RF-E3)", telemetry.Version, telemetry.Instance)
	}
}
