package app_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

const contractBody = "openapi: 3.1.0\n"

type edge struct {
	addr     string
	admin    string
	logs     *obstest.MemoryLogs
	provider *sdklog.LoggerProvider
	spans    *tracetest.InMemoryExporter
	metrics  *sdkmetric.ManualReader
	runtime  *otelboot.Runtime
	cancel   context.CancelFunc
	done     chan error
}

func edgeConfig(t *testing.T, env ...string) app.Config {
	t.Helper()
	base := append(slices.Clone(targets), "GRPC_INSECURE", "true", "HTTP_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0")
	cfg, err := app.FromEnv(lookup(append(base, env...)...))
	if err != nil {
		t.Fatalf("FromEnv() = %v", err)
	}
	return cfg
}

func bootEdge(t *testing.T, cfg app.Config) edge {
	t.Helper()
	telemetry := app.TelemetryOf(cfg)
	e := edge{logs: &obstest.MemoryLogs{}, spans: tracetest.NewInMemoryExporter(), metrics: sdkmetric.NewManualReader(), done: make(chan error, 1)}
	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: telemetry.Service, ServiceVersion: "test",
			ServiceInstanceID: "bff-1", Role: telemetry.Role},
		TraceExporter: e.spans,
		MetricReader:  e.metrics,
	}
	e.provider = otelboot.NewLoggerProvider(config, e.logs)
	config.LoggerProvider = e.provider
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	var err error
	if e.runtime, err = otelboot.Start(ctx, config); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = e.runtime.Shutdown(context.Background())
	})

	go func() { e.done <- app.RunWith(ctx, cfg, e.runtime) }()
	return e
}

func startEdge(t *testing.T, env ...string) edge {
	t.Helper()
	e := bootEdge(t, edgeConfig(t, env...))
	e.addr = listeningAddr(t, e, "http listening")
	e.admin = listeningAddr(t, e, "admin listening")
	return e
}

func (e edge) stop(t *testing.T) {
	t.Helper()
	e.cancel()
	if err := <-e.done; err != nil {
		t.Fatalf("RunWith() = %v, want nil on shutdown", err)
	}
	if err := e.provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
}

func (e edge) records(t *testing.T) []sdklog.Record {
	t.Helper()
	if err := e.provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	return e.logs.Records()
}

func (e edge) request(t *testing.T, method, path string, headers ...string) int {
	t.Helper()
	return requestAt(t, e.addr, method, path, headers...)
}

func (e edge) probe(t *testing.T, method, path string) int {
	t.Helper()
	return requestAt(t, e.admin, method, path)
}

func requestAt(t *testing.T, addr, method, path string, headers ...string) int {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+addr+path, nil)
	if err != nil {
		t.Fatalf("NewRequest() = %v", err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		if http.CanonicalHeaderKey(headers[i]) == "Host" {
			req.Host = headers[i+1]
			continue
		}
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s = %v", method, path, err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func attributesOf(record sdklog.Record) map[string]attribute.Value {
	attributes := map[string]attribute.Value{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		attributes[string(kv.Key)] = kv.Value
		return true
	})
	return attributes
}

func recordNamed(t *testing.T, records []sdklog.Record, body string) sdklog.Record {
	t.Helper()
	var bodies []string
	for _, record := range records {
		if record.Body().AsString() == body {
			return record
		}
		bodies = append(bodies, record.Body().AsString())
	}
	t.Fatalf("no %q record in %v", body, bodies)
	return sdklog.Record{}
}

func requireNoExecution(t *testing.T, body string, attributes map[string]attribute.Value) {
	t.Helper()
	for _, key := range []string{tracing.KeyCorrelationID, tracing.KeyRequestID, tracing.KeyTenantID} {
		if value, present := attributes[key]; present {
			t.Errorf("%q carries %s = %s, want no key of an execution on a start or shutdown record (RF-A4)", body, key, value.String())
		}
	}
}

func listeningAddr(t *testing.T, e edge, body string) string {
	t.Helper()
	var bodies []string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		bodies = bodies[:0]
		for _, record := range e.records(t) {
			bodies = append(bodies, record.Body().AsString())
			if record.Body().AsString() != body {
				continue
			}
			if scope := record.InstrumentationScope().Name; scope != reflect.TypeFor[app.Config]().PkgPath() {
				t.Fatalf("readiness line scope = %q, want the import path of the app package that emits it (RF-A1)", scope)
			}
			attributes := attributesOf(record)
			host, port := attributes[string(semconv.ServerAddressKey)], attributes[string(semconv.ServerPortKey)]
			if host.AsString() == "" || port.Type() != attribute.INT64 {
				t.Fatalf("readiness line = %v, want server.address and server.port surviving the processor (RF-A3)", attributes)
			}
			requireNoExecution(t, body, attributes)
			return net.JoinHostPort(host.AsString(), strconv.FormatInt(port.AsInt64(), 10))
		}
		select {
		case err := <-e.done:
			t.Fatalf("RunWith() = %v before %q", err, body)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatalf("no %q line in %v", body, bodies)
	return ""
}

func writeContract(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(path, []byte(contractBody), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	return path
}

func discoveryOnlyIssuer(t *testing.T) string {
	t.Helper()
	issuer := httptest.NewServer(nil)
	t.Cleanup(issuer.Close)
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": issuer.URL, "jwks_uri": issuer.URL + "/jwks",
			"authorization_endpoint": issuer.URL + "/auth", "token_endpoint": issuer.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"keys":[]}`)
	})
	issuer.Config.Handler = mux
	return issuer.URL
}

func clientMaterial(t *testing.T) (caFile, certFile, keyFile string) {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "bff"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() = %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey() = %v", err)
	}
	write := func(name, kind string, block []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: block}), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) = %v", name, err)
		}
		return path
	}
	return write("ca.pem", "CERTIFICATE", der), write("bff.crt", "CERTIFICATE", der), write("bff.key", "EC PRIVATE KEY", keyDER)
}

func TestTheReadinessLineCarriesThePortTheKernelChose(t *testing.T) {
	e := startEdge(t, devMock, "true")

	if _, port, err := net.SplitHostPort(e.addr); err != nil || port == "0" || port == "" {
		t.Fatalf("addr = %q, want the address the listener resolved, not the configured one", e.addr)
	}
	conn, err := net.DialTimeout("tcp", e.addr, time.Second)
	if err != nil {
		t.Fatalf("dial %s = %v, want the edge accepting on the address it logged", e.addr, err)
	}
	_ = conn.Close()
	e.stop(t)
}

func TestTheShutdownLineCarriesTheDrainDelayUnderAPlatformKey(t *testing.T) {
	e := startEdge(t, devMock, "true", "DRAIN_DELAY", "10ms")

	e.stop(t)

	attributes := attributesOf(recordNamed(t, e.records(t), "http draining"))
	if got := attributes["dmpf.drain.delay"].AsString(); got != "10ms" {
		t.Fatalf("\"http draining\" = %v, want dmpf.drain.delay=10ms surviving the processor (RF-A3)", attributes)
	}
	requireNoExecution(t, "http draining", attributes)
}

func TestTheAdministrationPortRefusesReadinessWhileThePublicOneDrains(t *testing.T) {
	e := startEdge(t, devMock, "true", "DRAIN_DELAY", "1s", "OPENAPI_IDENTITY_PATH", writeContract(t))
	readiness := func(want int) bool {
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if e.probe(t, http.MethodGet, api.ReadinessPath) == want {
				return true
			}
		}
		return false
	}

	if !readiness(http.StatusNoContent) {
		t.Fatalf("%s on the administration port never answered 204", api.ReadinessPath)
	}
	e.cancel()
	if !readiness(http.StatusServiceUnavailable) {
		t.Fatalf("%s on the administration port never answered 503 during the drain", api.ReadinessPath)
	}
	if status := e.request(t, http.MethodGet, api.IdentityContractPath); status != http.StatusOK {
		t.Fatalf("public GET %s during the drain = %d, want it still served (200)", api.IdentityContractPath, status)
	}
	e.stop(t)
}

func TestTheAdministrationPortClosesOnlyAfterThePublicOneHasDrained(t *testing.T) {
	e := startEdge(t, devMock, "true", "OPENAPI_IDENTITY_PATH", writeContract(t))
	pending, err := net.Dial("tcp", e.addr)
	if err != nil {
		t.Fatalf("Dial() = %v", err)
	}
	t.Cleanup(func() { _ = pending.Close() })
	if _, err := pending.Write([]byte("GET " + api.IdentityContractPath + " HTTP/1.1\r\n")); err != nil {
		t.Fatalf("Write() = %v", err)
	}
	if status := e.request(t, http.MethodGet, api.IdentityContractPath); status != http.StatusOK {
		t.Fatalf("public GET %s = %d, want 200", api.IdentityContractPath, status)
	}

	e.cancel()
	draining := false
	for deadline := time.Now().Add(5 * time.Second); !draining && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		draining = e.probe(t, http.MethodGet, api.ReadinessPath) == http.StatusServiceUnavailable
	}
	if !draining {
		t.Fatalf("%s on the administration port never answered 503 during the drain", api.ReadinessPath)
	}
	time.Sleep(200 * time.Millisecond)
	if status := e.probe(t, http.MethodGet, api.ReadinessPath); status != http.StatusServiceUnavailable {
		t.Fatalf("%s while the public port still drains = %d, want 503", api.ReadinessPath, status)
	}
	_ = pending.Close()
	e.stop(t)
}

func TestTheEdgeServesTheContractItWasGiven(t *testing.T) {
	e := startEdge(t, devMock, "true", "OPENAPI_IDENTITY_PATH", writeContract(t), "CORS_ORIGINS", "http://app.example")

	res, err := http.Get("http://" + e.addr + api.IdentityContractPath)
	if err != nil {
		t.Fatalf("GET %s = %v", api.IdentityContractPath, err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != contractBody {
		t.Fatalf("GET %s = %d %q, want 200 with the file of OPENAPI_IDENTITY_PATH", api.IdentityContractPath, res.StatusCode, body)
	}
	for _, c := range []struct {
		method, path string
		headers      []string
		status       int
	}{
		{http.MethodGet, "/nowhere", nil, http.StatusNotFound},
		{http.MethodDelete, api.IdentityContractPath, nil, http.StatusMethodNotAllowed},
		{http.MethodOptions, api.IdentityContractPath, []string{"Origin", "http://app.example", "Access-Control-Request-Method", "GET"}, http.StatusNoContent},
	} {
		if status := e.request(t, c.method, c.path, c.headers...); status != c.status {
			t.Fatalf("%s %s = %d, want %d", c.method, c.path, status, c.status)
		}
	}
	e.stop(t)
}

func TestTheEdgeRefusesToStartWithAContractItCannotRead(t *testing.T) {
	e := bootEdge(t, edgeConfig(t, devMock, "true", "OPENAPI_IDENTITY_PATH", filepath.Join(t.TempDir(), "absent.yaml")))

	select {
	case err := <-e.done:
		if err == nil || !strings.Contains(err.Error(), "openapi:") {
			t.Fatalf("RunWith() = %v, want the unreadable contract refused at the start", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWith() kept running with an unreadable contract")
	}
}

func TestTheEdgeRefusesToStartWithAnEmptyContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	e := bootEdge(t, edgeConfig(t, devMock, "true", "OPENAPI_IDENTITY_PATH", path))

	select {
	case err := <-e.done:
		if err == nil || !strings.Contains(err.Error(), "openapi:") || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("RunWith() = %v, want the empty contract refused at the start", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWith() kept running with an empty contract")
	}
}

func TestASignalDuringTheIssuerDiscoveryEndsTheStartCleanly(t *testing.T) {
	asked, release := make(chan struct{}, 1), make(chan struct{})
	issuer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case asked <- struct{}{}:
		default:
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(issuer.Close)
	t.Cleanup(func() { close(release) })
	e := bootEdge(t, edgeConfig(t, "OIDC_ISSUER", issuer.URL, "OIDC_AUDIENCE", realmAudience, "OIDC_TENANT_CLAIM", realmTenantClaim))

	select {
	case <-asked:
	case err := <-e.done:
		t.Fatalf("RunWith() = %v before asking the issuer", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the issuer was never asked for its discovery document")
	}
	e.cancel()
	select {
	case err := <-e.done:
		if err != nil {
			t.Fatalf("RunWith() = %v, want nil when the signal interrupts the discovery", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWith() did not return after the signal")
	}
}

func TestReadinessDoesNotDependOnTheRevocationStore(t *testing.T) {
	e := startEdge(t, devMock, "true", "REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0")

	if status := e.probe(t, http.MethodGet, api.ReadinessPath); status != http.StatusNoContent {
		t.Fatalf("%s with the deny-list unreachable = %d, want 204: readiness never waits on Redis", api.ReadinessPath, status)
	}
	e.stop(t)
}

func TestTheEdgeStartsWithTheVerifierOfARealm(t *testing.T) {
	e := startEdge(t, "OIDC_ISSUER", discoveryOnlyIssuer(t), "OIDC_AUDIENCE", realmAudience, "OIDC_TENANT_CLAIM", realmTenantClaim)

	if status := e.probe(t, http.MethodGet, api.ReadinessPath); status != http.StatusNoContent {
		t.Fatalf("%s after the discovery = %d, want 204", api.ReadinessPath, status)
	}
	e.stop(t)
}

func TestTheEdgeLoadsTheClientCertificateAtTheStart(t *testing.T) {
	caFile, certFile, keyFile := clientMaterial(t)
	e := startEdge(t, devMock, "true", "GRPC_CA_FILE", caFile, "GRPC_SERVER_NAME", "identity.internal",
		"GRPC_CLIENT_CERT_FILE", certFile, "GRPC_CLIENT_KEY_FILE", keyFile)

	if status := e.probe(t, http.MethodGet, api.ReadinessPath); status != http.StatusNoContent {
		t.Fatalf("%s with readable client material = %d, want 204", api.ReadinessPath, status)
	}
	e.stop(t)
}

func TestTheEdgeRefusesToStartWithAnUnreadableAuthority(t *testing.T) {
	_, certFile, keyFile := clientMaterial(t)
	unreadable := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(unreadable, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}
	e := bootEdge(t, edgeConfig(t, devMock, "true", "GRPC_CA_FILE", unreadable,
		"GRPC_CLIENT_CERT_FILE", certFile, "GRPC_CLIENT_KEY_FILE", keyFile))

	select {
	case err := <-e.done:
		if err == nil || !strings.Contains(err.Error(), "PEM") {
			t.Fatalf("RunWith() = %v, want the unreadable authority refused at the start", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWith() kept running with an unreadable authority")
	}
}

func TestEveryRequestOfTheEdgeIsOneServerSpanAndOnePointOfItsDuration(t *testing.T) {
	e := startEdge(t, devMock, "true", "CORS_ORIGINS", "http://app.example", "OPENAPI_IDENTITY_PATH", writeContract(t))
	requests := []struct {
		method, path string
		headers      []string
		status       int
		span         string
		admin        bool
	}{
		{http.MethodGet, api.IdentityContractPath, nil, http.StatusOK, "GET " + api.IdentityContractPath, false},
		{http.MethodGet, api.LivenessPath, nil, http.StatusNotFound, "GET", false},
		{http.MethodGet, api.ReadinessPath, nil, http.StatusNotFound, "GET", false},
		{http.MethodGet, api.LivenessPath, nil, http.StatusNoContent, "", true},
		{http.MethodGet, api.ReadinessPath, nil, http.StatusNoContent, "", true},
		{http.MethodGet, "/nowhere", nil, http.StatusNotFound, "GET", true},
		{http.MethodOptions, api.IdentityContractPath, []string{"Origin", "http://app.example", "Access-Control-Request-Method", "GET"}, http.StatusNoContent, "OPTIONS", false},
		{http.MethodGet, "/nowhere", nil, http.StatusNotFound, "GET", false},
		{http.MethodDelete, api.IdentityContractPath, nil, http.StatusMethodNotAllowed, "DELETE", false},
	}
	var want []string
	for _, r := range requests {
		addr := map[bool]string{false: e.addr, true: e.admin}[r.admin]
		if status := requestAt(t, addr, r.method, r.path, r.headers...); status != r.status {
			t.Fatalf("%s %s on %s = %d, want %d", r.method, r.path, addr, status, r.status)
		}
		if r.span != "" {
			want = append(want, r.span)
		}
	}
	if err := e.runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	var got []string
	for _, span := range e.spans.GetSpans() {
		if span.SpanKind == trace.SpanKindServer {
			got = append(got, span.Name)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("SERVER spans = %v, want exactly one per request outside the health routes %v (RF-B2)", got, want)
	}
	if points := durationPoints(t, e.metrics); points != uint64(len(want)) {
		t.Fatalf("http.server.request.duration counted %d requests, want %d, none of the health routes (RF-B2, RF-D2)", points, len(want))
	}
	e.stop(t)
}

func TestAnEdgeSeriesDoesNotSplitByTheHostTheClientSends(t *testing.T) {
	e := startEdge(t, devMock, "true", "OPENAPI_IDENTITY_PATH", writeContract(t))
	for _, host := range []string{"a.example", "b.example", "c.example:1234", "c.example:4321"} {
		if status := e.request(t, http.MethodGet, api.IdentityContractPath, "Host", host); status != http.StatusOK {
			t.Fatalf("GET %s with Host %s = %d, want 200", api.IdentityContractPath, host, status)
		}
	}
	if err := e.runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	for _, name := range []string{
		httpconv.ServerRequestDuration{}.Name(),
		httpconv.ServerRequestBodySize{}.Name(),
		httpconv.ServerResponseBodySize{}.Name(),
	} {
		series := routeSeries(t, e.metrics, name, api.IdentityContractPath)
		if len(series) != 1 || series[0].HasValue(semconv.ServerAddressKey) || series[0].HasValue(semconv.ServerPortKey) {
			encoded := make([]string, len(series))
			for i, set := range series {
				encoded[i] = set.Encoded(attribute.DefaultEncoder())
			}
			t.Errorf("%s for %s = %q, want one series without server.address or server.port, "+
				"which otelhttp takes from the Host the client sends (RF-D3)", name, api.IdentityContractPath, encoded)
		}
	}
	e.stop(t)
}

func routeSeries(t *testing.T, reader *sdkmetric.ManualReader, name, route string) []attribute.Set {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	var sets []attribute.Set
	keep := func(set attribute.Set) {
		if value, ok := set.Value(semconv.HTTPRouteKey); ok && value.AsString() == route {
			sets = append(sets, set)
		}
	}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			switch data := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, point := range data.DataPoints {
					keep(point.Attributes)
				}
			case metricdata.Histogram[int64]:
				for _, point := range data.DataPoints {
					keep(point.Attributes)
				}
			default:
				t.Fatalf("%s is %T, want a histogram", name, m.Data)
			}
		}
	}
	return sets
}

func durationPoints(t *testing.T, reader *sdkmetric.ManualReader) uint64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	var count uint64
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if histogram, ok := m.Data.(metricdata.Histogram[float64]); ok && m.Name == "http.server.request.duration" {
				for _, point := range histogram.DataPoints {
					count += point.Count
				}
			}
		}
	}
	return count
}
