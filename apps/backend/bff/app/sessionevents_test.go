package app_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
)

const (
	realmKeyID       = "realm-test"
	adminEventsPath  = "/admin/realms/dmpf/admin-events"
	logoutEventsPath = "/admin/realms/dmpf/events"
)

type fakeRealm struct {
	issuer string
	key    *rsa.PrivateKey

	hangToken  atomic.Bool
	tokenAsked chan struct{}
	release    chan struct{}

	mu     sync.Mutex
	polls  map[string][]string
	served map[string][]byte
	types  []string
}

func newFakeRealm(t *testing.T) *fakeRealm {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() = %v", err)
	}
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)
	realm := &fakeRealm{issuer: server.URL + "/realms/dmpf", key: key, polls: map[string][]string{}, served: map[string][]byte{},
		tokenAsked: make(chan struct{}, 1), release: make(chan struct{})}
	t.Cleanup(func() { close(realm.release) })
	encode := base64.RawURLEncoding.EncodeToString
	mux := http.NewServeMux()
	mux.HandleFunc("GET /realms/dmpf/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeRealmJSON(w, map[string]any{
			"issuer": realm.issuer, "jwks_uri": realm.issuer + "/protocol/openid-connect/certs",
			"authorization_endpoint": realm.issuer + "/auth", "token_endpoint": realm.issuer + "/protocol/openid-connect/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /realms/dmpf/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
		writeRealmJSON(w, map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": realmKeyID, "use": "sig", "alg": "RS256",
			"n": encode(key.N.Bytes()), "e": encode(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("POST /realms/dmpf/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		if realm.hangToken.Load() {
			select {
			case realm.tokenAsked <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
			case <-realm.release:
			}
			return
		}
		writeRealmJSON(w, map[string]any{"access_token": "polling", "token_type": "Bearer", "expires_in": 300})
	})
	events := func(w http.ResponseWriter, r *http.Request) {
		realm.mu.Lock()
		realm.polls[r.URL.Path] = append(realm.polls[r.URL.Path], r.URL.Query().Get("dateTo"))
		if r.URL.Path == logoutEventsPath {
			realm.types = append(realm.types, r.URL.Query()["type"]...)
		}
		body := realm.served[r.URL.Path]
		realm.mu.Unlock()
		if body == nil {
			body = []byte("[]")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
	mux.HandleFunc("GET "+adminEventsPath, events)
	mux.HandleFunc("GET "+logoutEventsPath, events)
	server.Config.Handler = mux
	return realm
}

func writeRealmJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func (r *fakeRealm) polled(path string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.polls[path])
}

// cyclesReadingBothSources counts the dateTo values both sources were asked
// for: SessionPolling.Poll computes one window per cycle (authn sessionpolling.go:155).
func (r *fakeRealm) cyclesReadingBothSources() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	cycles := 0
	for _, to := range slices.Compact(slices.Sorted(slices.Values(r.polls[adminEventsPath]))) {
		if slices.Contains(r.polls[logoutEventsPath], to) {
			cycles++
		}
	}
	return cycles
}

func (r *fakeRealm) logoutTypes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Compact(slices.Sorted(slices.Values(r.types)))
}

func startSessionEvents(t *testing.T, env ...string) edge {
	t.Helper()
	return startSessionEventsWith(t, func(*app.SessionEventsConfig) {}, env...)
}

func startSessionEventsWith(t *testing.T, adjust func(*app.SessionEventsConfig), env ...string) edge {
	t.Helper()
	base := append(slices.Clone(sessionEventsEnv), "SESSION_EVENTS_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0",
		"REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0")
	cfg, err := app.SessionEventsFromEnv(lookup(append(base, env...)...))
	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}
	cfg.Polling.Interval = 20 * time.Millisecond
	adjust(&cfg)
	e := edge{logs: &obstest.MemoryLogs{}, spans: tracetest.NewInMemoryExporter(), metrics: sdkmetric.NewManualReader(), done: make(chan error, 1)}
	config := otelboot.Config{
		Propagator:    propagation.TraceContext{},
		Resource:      otelboot.Resource{ServiceName: "bff", ServiceVersion: "test", ServiceInstanceID: "bff-session-events-1", Role: "session-events"},
		TraceExporter: e.spans,
		MetricReader:  e.metrics,
	}
	e.provider = otelboot.NewLoggerProvider(config, e.logs)
	config.LoggerProvider = e.provider
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	if e.runtime, err = otelboot.Start(ctx, config); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = e.runtime.Shutdown(context.Background())
	})

	go func() { e.done <- app.RunSessionEventsWith(ctx, cfg, e.runtime) }()
	e.addr = listeningAddr(t, e, "session events listening")
	e.admin = listeningAddr(t, e, "admin listening")
	return e
}

func quietRuntime(t *testing.T) *otelboot.Runtime {
	t.Helper()
	config := otelboot.Config{Propagator: propagation.TraceContext{}, TraceExporter: tracetest.NewInMemoryExporter(),
		Resource: otelboot.Resource{ServiceName: "bff", ServiceVersion: "test", ServiceInstanceID: "bff-session-events-1"}}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, &obstest.MemoryLogs{})
	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	return runtime
}

func postForm(t *testing.T, addr, path string, form url.Values) int {
	t.Helper()
	res, err := http.Post("http://"+addr+path, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST %s = %v", path, err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTheSessionEventsReceiverRefusesWhatItCannotVerify(t *testing.T) {
	realm := newFakeRealm(t)
	e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer)

	if status := postForm(t, e.addr, app.BackchannelLogoutPath, url.Values{"logout_token": {"not-a-jwt"}}); status != http.StatusBadRequest {
		t.Fatalf("POST %s with an unverifiable token = %d, want 400", app.BackchannelLogoutPath, status)
	}
	res, err := http.Get("http://" + e.addr + app.BackchannelLogoutPath)
	if err != nil {
		t.Fatalf("GET %s = %v", app.BackchannelLogoutPath, err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed || res.Header.Get("Allow") != http.MethodPost {
		t.Fatalf("GET %s = %d Allow %q, want 405 Allow POST", app.BackchannelLogoutPath, res.StatusCode, res.Header.Get("Allow"))
	}
	e.stop(t)
}

func TestTheSessionEventsHealthLivesOnlyOnTheAdministrationPort(t *testing.T) {
	realm := newFakeRealm(t)
	e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer)

	for _, path := range []string{"/livez", "/readyz"} {
		if status := e.probe(t, http.MethodGet, path); status/100 != 2 {
			t.Errorf("GET %s on the administration port = %d, want 2xx", path, status)
		}
		if status := e.request(t, http.MethodGet, path); status != http.StatusNotFound {
			t.Errorf("GET %s on the receiver port = %d, want 404", path, status)
		}
	}
	e.stop(t)
}

func TestTheSessionEventsPollingReadsBothSourcesOnEveryCycle(t *testing.T) {
	realm := newFakeRealm(t)
	e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer)

	waitFor(t, "two cycles reading the admin and the LOGOUT events", func() bool { return realm.cyclesReadingBothSources() >= 2 })
	e.stop(t)

	if admin, logout := realm.polled(adminEventsPath), realm.polled(logoutEventsPath); admin-logout != 0 && admin-logout != 1 {
		t.Fatalf("admin events read %d times and LOGOUT events %d times, want one of each per cycle", admin, logout)
	}
	if types := realm.logoutTypes(); !slices.Equal(types, []string{"LOGOUT"}) {
		t.Fatalf("user events filtered by type %v, want only LOGOUT", types)
	}
}

func TestTheSessionEventsRoleStopsPollingOnShutdown(t *testing.T) {
	realm := newFakeRealm(t)
	e := startSessionEvents(t, "OIDC_ISSUER", realm.issuer)
	waitFor(t, "a first cycle", func() bool { return realm.cyclesReadingBothSources() >= 1 })
	e.stop(t)

	settled := realm.polled(adminEventsPath) + realm.polled(logoutEventsPath)
	time.Sleep(100 * time.Millisecond)
	if got := realm.polled(adminEventsPath) + realm.polled(logoutEventsPath); got != settled {
		t.Fatalf("the realm was polled %d times after RunSessionEventsWith returned, want none", got-settled)
	}
	if res, err := http.Get("http://" + e.admin + "/livez"); err == nil {
		_ = res.Body.Close()
		t.Fatal("the administration port still answers after the shutdown")
	}
}

func TestTheSessionEventsRoleRefusesToStartWithoutDiscovery(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	issuer := gone.URL + "/realms/dmpf"
	gone.Close()
	cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), "OIDC_ISSUER", issuer,
		"SESSION_EVENTS_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0", "OIDC_DISCOVERY_TIMEOUT_SECONDS", "1")...))
	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}

	err = app.RunSessionEventsWith(context.Background(), cfg, quietRuntime(t))

	if err == nil || !strings.HasPrefix(err.Error(), "auth: discovering "+issuer) {
		t.Fatalf("RunSessionEventsWith() = %v, want the start refused by the discovery of %s", err, issuer)
	}
}

func TestTheSessionEventsRoleRefusesAnIssuerOutsideAKeycloakRealm(t *testing.T) {
	server := httptest.NewServer(nil)
	t.Cleanup(server.Close)
	issuer := server.URL + "/auth/dmpf"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/dmpf/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeRealmJSON(w, map[string]any{
			"issuer": issuer, "jwks_uri": issuer + "/certs", "authorization_endpoint": issuer + "/auth",
			"token_endpoint": issuer + "/token", "id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	server.Config.Handler = mux
	cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), "OIDC_ISSUER", issuer,
		"SESSION_EVENTS_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0")...))
	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}

	err = app.RunSessionEventsWith(context.Background(), cfg, quietRuntime(t))

	if !errors.Is(err, authn.ErrNotAKeycloakRealm) {
		t.Fatalf("RunSessionEventsWith() = %v, want %v: the polling reads the Admin REST of <base>/realms/<realm>", err, authn.ErrNotAKeycloakRealm)
	}
}

func TestTheSessionEventsRoleEndsCleanlyWhenStoppedDuringDiscovery(t *testing.T) {
	asked := make(chan struct{})
	var once sync.Once
	hanging := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(asked) })
		<-r.Context().Done()
	}))
	t.Cleanup(hanging.Close)
	cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), "OIDC_ISSUER", hanging.URL+"/realms/dmpf",
		"SESSION_EVENTS_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0")...))
	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}
	runtime := quietRuntime(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.RunSessionEventsWith(ctx, cfg, runtime) }()

	select {
	case <-asked:
	case err := <-done:
		t.Fatalf("RunSessionEventsWith() = %v before asking for the discovery", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the discovery was never asked for")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunSessionEventsWith() = %v, want nil: the signal arrived before the role served", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunSessionEventsWith() did not return after the cancellation")
	}
}

func stopWhileTheTokenEndpointHangs(t *testing.T, adjust func(*app.SessionEventsConfig), within time.Duration) {
	t.Helper()
	realm := newFakeRealm(t)
	realm.hangToken.Store(true)
	e := startSessionEventsWith(t, adjust, "OIDC_ISSUER", realm.issuer)
	select {
	case <-realm.tokenAsked:
	case <-time.After(5 * time.Second):
		t.Fatal("the polling never asked the token endpoint")
	}

	began := time.Now()
	e.cancel()
	select {
	case err := <-e.done:
		if err != nil {
			t.Fatalf("RunSessionEventsWith() = %v, want nil on shutdown", err)
		}
		if elapsed := time.Since(began); elapsed > within {
			t.Fatalf("RunSessionEventsWith() returned %v after the cancel, want within %v", elapsed, within)
		}
	case <-time.After(within + 5*time.Second):
		t.Fatalf("RunSessionEventsWith() still running %v after the cancel, want within %v", within+5*time.Second, within)
	}
}

func TestTheSessionEventsRoleStopsBeforeTheGraceWhileTheTokenEndpointHangs(t *testing.T) {
	timeout := app.SessionEventsDefaults().Polling.RequestTimeout
	if 2*timeout >= observability.ShutdownGrace {
		t.Fatalf("Polling.RequestTimeout = %v: the two token attempts must end before the %v grace", timeout, observability.ShutdownGrace)
	}

	stopWhileTheTokenEndpointHangs(t, func(*app.SessionEventsConfig) {}, 2*timeout+2*time.Second)
}

func TestTheSessionEventsRoleStopsWithinTheGraceWhateverTheTokenTimeout(t *testing.T) {
	stopWhileTheTokenEndpointHangs(t, func(cfg *app.SessionEventsConfig) { cfg.Polling.RequestTimeout = time.Minute },
		observability.ShutdownGrace+2*time.Second)
}
