package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

var routeBudget = deadline.Budget{Dependency: "edge", Method: "route", Limit: 2 * time.Second, Slack: 200 * time.Millisecond, EstimatedDuration: 100 * time.Millisecond}

const identityContract = "apps/backend/identity/contract/openapi/v1/openapi.yaml#/paths/"

const (
	commandPath = "/probes/{id}/act"
	queryPath   = "/probes/{id}"

	commandPermission = "identity:act-on-probe"
	queryPermission   = "identity:find-probe"
)

func syntheticRoute(name, method, path string, permission ports.Permission) kernelhttp.Route {
	route := kernelhttp.Route{
		Name:        name,
		Method:      method,
		Path:        path,
		ContractRef: identityContract + strings.ReplaceAll(path, "/", "~1") + "/" + strings.ToLower(method),
		Budget:      routeBudget,
		Requires:    kernelhttp.RequireSubjectAndTenant,
		Permission:  permission,
	}
	if method == http.MethodPost {
		route.IdempotencyKey = api.IdempotencyHeader
	}
	return route
}

type seen struct {
	execution   ports.ExecutionContext
	hasContext  bool
	deadline    time.Time
	hasDeadline bool
}

type probe struct {
	mu      sync.Mutex
	calls   []seen
	respond http.HandlerFunc
}

func (p *probe) handle(w http.ResponseWriter, r *http.Request) {
	execution, hasContext := ports.ExecutionContextFrom(r.Context())
	d, hasDeadline := r.Context().Deadline()
	p.mu.Lock()
	p.calls = append(p.calls, seen{execution, hasContext, d, hasDeadline})
	respond := p.respond
	p.mu.Unlock()
	if respond != nil {
		respond(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": r.PathValue("id")})
}

func (p *probe) total() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

func (p *probe) last(t *testing.T) seen {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.calls) == 0 {
		t.Fatal("the synthetic handler was never called")
	}
	return p.calls[len(p.calls)-1]
}

func probeBindings(p *probe) []api.Binding {
	return []api.Binding{
		api.NewBinding(api.RouteCommand, syntheticRoute("actOnProbe", http.MethodPost, commandPath, commandPermission), p.handle),
		api.NewBinding(api.RouteQuery, syntheticRoute("findProbe", http.MethodGet, queryPath, queryPermission), p.handle),
	}
}

type fixture struct {
	probe       *probe
	handler     http.Handler
	admin       http.Handler
	spans       *tracetest.InMemoryExporter
	logs        *bytes.Buffer
	records     *logRecorder
	logProvider *sdklog.LoggerProvider
	metrics     *sdkmetric.ManualReader
}

type logRecorder struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (r *logRecorder) Export(_ context.Context, records []sdklog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range records {
		r.records = append(r.records, record.Clone())
	}
	return nil
}

func (*logRecorder) Shutdown(context.Context) error { return nil }

func (*logRecorder) ForceFlush(context.Context) error { return nil }

func (r *logRecorder) all() []sdklog.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.records)
}

type setup struct {
	budget           deadline.Budget
	limit            admission.Limit
	identityContract []byte
	cors             []string
	ready            func(context.Context) error
	authenticator    ports.Authenticator
	draining         func() bool
	clock            obsclock.Clock
	bindings         func(*probe) []api.Binding
	identity         *rpc.Identity
}

type option func(*setup)

func withBudget(b deadline.Budget) option { return func(s *setup) { s.budget = b } }

func withLimit(l admission.Limit) option { return func(s *setup) { s.limit = l } }

func withIdentityContract(document string) option {
	return func(s *setup) { s.identityContract = []byte(document) }
}

func withCORS(origins ...string) option { return func(s *setup) { s.cors = origins } }

func withReady(ready func(context.Context) error) option { return func(s *setup) { s.ready = ready } }

func withAuthenticator(a ports.Authenticator) option { return func(s *setup) { s.authenticator = a } }

func withDraining(draining func() bool) option { return func(s *setup) { s.draining = draining } }

func withClock(c obsclock.Clock) option { return func(s *setup) { s.clock = c } }

func withBindings(bindings func(*probe) []api.Binding) option {
	return func(s *setup) { s.bindings = bindings }
}

func withIdentity(identity rpc.Identity) option {
	return func(s *setup) { s.identity = &identity }
}

func stagesOf(authenticator ports.Authenticator) api.Authenticator {
	if staged, ok := authenticator.(api.Authenticator); ok {
		return staged
	}
	return api.VerifyOnly(authenticator)
}

func withoutRoutes() option { return withBindings(func(*probe) []api.Binding { return nil }) }

func routesOf(bound []api.Binding) []kernelhttp.Route {
	routes := make([]kernelhttp.Route, len(bound))
	for i, b := range bound {
		routes[i] = api.RouteOf(b)
	}
	return routes
}

func newFixture(t *testing.T, options ...option) fixture {
	t.Helper()
	cfg := &setup{
		budget:        routeBudget,
		limit:         admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 100},
		authenticator: authn.DevAuthenticator{},
		bindings:      probeBindings,
	}
	for _, apply := range options {
		apply(cfg)
	}

	logs := &bytes.Buffer{}
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(otelboot.NewPrivacyExporter(spans)))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	records := &logRecorder{}
	logProvider := otelboot.NewLoggerProvider(otelboot.Config{}, records)
	t.Cleanup(func() { _ = logProvider.Shutdown(context.Background()) })
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = meterProvider.Shutdown(context.Background()) })
	logger := slog.New(slog.NewMultiHandler(
		slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}),
		logging.NewLogger(logProvider, "github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api").Handler(),
	))

	p := &probe{}
	var bound []api.Binding
	if cfg.identity != nil {
		bound = api.Bindings(cfg.budget)
	} else {
		bound = cfg.bindings(p)
		for i, b := range bound {
			route := api.RouteOf(b)
			route.Budget = cfg.budget
			bound[i] = api.NewBinding(api.KindOf(b), route, p.handle)
		}
	}

	var ctrl *admission.Controller
	if len(bound) > 0 {
		tenants, err := metrics.DeclareTenants(testTenant)
		if err != nil {
			t.Fatalf("DeclareTenants() = %v", err)
		}
		ctrl, err = admission.New(admission.Config{Limits: api.LimitsOf(routesOf(bound), cfg.limit), Tenants: tenants, MaxKeys: 32, Clock: obsclock.System()})
		if err != nil {
			t.Fatalf("admission.New() = %v", err)
		}
	}

	handlerOptions := api.Options{
		Budget:           cfg.budget,
		Authenticator:    stagesOf(cfg.authenticator),
		IdentityContract: cfg.identityContract,
		CORSOrigins:      cfg.cors,
		Logger:           logger,
		TracerProvider:   provider,
		MeterProvider:    meterProvider,
		Ready:            cfg.ready,
		Draining:         cfg.draining,
		Clock:            cfg.clock,
	}
	var handler http.Handler
	var err error
	if cfg.identity != nil {
		handler, err = api.NewHandler(*cfg.identity, ctrl, nil, handlerOptions)
	} else {
		handler, err = api.NewHandlerOf(bound, ctrl, nil, handlerOptions)
	}
	if err != nil {
		t.Fatalf("NewHandler() = %v", err)
	}
	return fixture{probe: p, handler: handler, admin: api.NewAdminHandler(handlerOptions), spans: spans, logs: logs, records: records, logProvider: logProvider, metrics: reader}
}

// testCredential is what the development authenticator reads back as identity;
// a request without it is denied before any handler, which is what the 401
// cases assert by passing an empty Authorization explicitly.
const testCredential = `Bearer {"sub":"tester","tenant":"acme","permissions":["identity:act-on-probe","identity:find-probe"]}`

// testTenant is the tenant testCredential resolves, declared so its bucket and
// label are its own (MET-07).
const testTenant = "acme"

func (f fixture) do(t *testing.T, method, path string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(f.handler, method, path, body, headers...)
}

func (f fixture) doAdmin(t *testing.T, method, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(f.admin, method, path, nil, headers...)
}

func (f fixture) doOn(t *testing.T, admin bool, method, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	if admin {
		return f.doAdmin(t, method, path, headers...)
	}
	return f.do(t, method, path, nil, headers...)
}

func serve(handler http.Handler, method, path string, body io.Reader, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if _, declared := req.Header["Authorization"]; !declared {
		req.Header.Set("Authorization", testCredential)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func (f fixture) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	return f.do(t, http.MethodPost, path, reader, "Idempotency-Key", "k-1", "Content-Type", "application/json")
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json (body %s)", ct, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("body %s is not the expected JSON: %v", rec.Body.String(), err)
	}
}

type rejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func requireAdmissionRefused(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	requireRejection(t, rec, http.StatusTooManyRequests, "admission-refused")
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
}

func requireRejection(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	var body rejection
	decode(t, rec, &body)
	if body.Code != code || body.Message == "" {
		t.Fatalf("rejection = %+v, want code %q with a message", body, code)
	}
}
