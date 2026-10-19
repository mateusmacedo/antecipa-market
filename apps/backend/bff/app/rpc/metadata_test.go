package rpc_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

type traced struct {
	identity rpc.Identity
	spans    *tracetest.InMemoryExporter
	edge     trace.Tracer
}

func tracedClient(t *testing.T, fake *fakeIdentity) traced {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	dialer := fake.serveBuffered(t, healthpb.HealthCheckResponse_SERVING)
	opts := rpc.Options{Insecure: true, Clock: obsclock.System(), Tracer: provider.Tracer("rpc_test"), TracerProvider: provider, Propagator: propagation.TraceContext{}}
	conn, err := rpc.Dial("passthrough:///identity", rpc.IdentityConfig(opts), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial(identity) = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return traced{identity: rpc.NewIdentity(conn), spans: spans, edge: provider.Tracer("edge")}
}

func authoredKeys(md metadata.MD) []string {
	var keys []string
	for key := range md {
		if strings.HasPrefix(key, "x-") {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func TestTheCallTravelsAsMetadata(t *testing.T) {
	fake := &fakeIdentity{}
	identity := identityClient(t, fake)
	ctx := rpc.WithCall(withDeadline(t, 2*time.Second), rpc.Call{CorrelationID: "corr-1", RequestID: "req-1", TenantID: "tenant-1", Locale: "pt-BR"})

	if _, err := identity.FindEffectivePermissions(ctx, &servicev1.FindEffectivePermissionsRequest{UserId: "u-1"}); err != nil {
		t.Fatalf("FindEffectivePermissions() = %v", err)
	}

	md := fake.callsTo("FindEffectivePermissions")[0].md
	for key, want := range map[string]string{
		kernelgrpc.CorrelationKey: "corr-1",
		kernelgrpc.CausationKey:   "req-1",
		kernelgrpc.TenantKey:      "tenant-1",
		kernelgrpc.LocaleKey:      "pt-BR",
	} {
		if got := md.Get(key); len(got) != 1 || got[0] != want {
			t.Errorf("metadata %s = %v, want %q", key, got, want)
		}
	}
	want := []string{kernelgrpc.CausationKey, kernelgrpc.CorrelationKey, kernelgrpc.LocaleKey, kernelgrpc.TenantKey}
	if got := authoredKeys(md); !slices.Equal(got, want) {
		t.Errorf("authored metadata %v, want only %v: subject and permissions never cross the edge (CTX-12)", got, want)
	}
	if got := md.Get("authorization"); len(got) != 0 {
		t.Errorf("metadata authorization = %v, want the token kept at the edge", got)
	}
}

func TestAnAbsentValueIsNeverSent(t *testing.T) {
	fake := &fakeIdentity{}
	identity := identityClient(t, fake)
	ctx := rpc.WithCall(withDeadline(t, 2*time.Second), rpc.Call{CorrelationID: "corr-1"})

	if _, err := identity.FindPerson(ctx, &servicev1.FindPersonRequest{PersonId: "p-1"}); err != nil {
		t.Fatalf("FindPerson() = %v", err)
	}

	if got := authoredKeys(fake.callsTo("FindPerson")[0].md); !slices.Equal(got, []string{kernelgrpc.CorrelationKey}) {
		t.Fatalf("authored metadata %v, want only the correlation; a tenant is never invented (CTX-13)", got)
	}
}

func TestACallOutsideARequestCarriesNoAuthoredMetadata(t *testing.T) {
	fake := &fakeIdentity{}
	identity := identityClient(t, fake)

	if _, err := identity.FindPerson(withDeadline(t, 2*time.Second), &servicev1.FindPersonRequest{PersonId: "p-1"}); err != nil {
		t.Fatalf("FindPerson() = %v", err)
	}

	if got := authoredKeys(fake.callsTo("FindPerson")[0].md); len(got) != 0 {
		t.Fatalf("authored metadata %v, want none", got)
	}
}

func TestCallFromReturnsWhatWithCallStored(t *testing.T) {
	call := rpc.Call{CorrelationID: "corr-1", RequestID: "req-1", TenantID: "tenant-1", Locale: "pt-BR"}

	got, ok := rpc.CallFrom(rpc.WithCall(context.Background(), call))
	_, absent := rpc.CallFrom(context.Background())

	if !ok || got != call || absent {
		t.Fatalf("CallFrom() = %+v, %v; on a bare context %v", got, ok, absent)
	}
}

func TestTheTraceparentNamesTheClientSpanOfTheCall(t *testing.T) {
	fake := &fakeIdentity{}
	h := tracedClient(t, fake)
	ctx, root := h.edge.Start(withDeadline(t, 2*time.Second), "HTTP GET /identity/persons/{personId}", trace.WithSpanKind(trace.SpanKindServer))

	if _, err := h.identity.FindPerson(ctx, &servicev1.FindPersonRequest{PersonId: "p-1"}); err != nil {
		t.Fatalf("FindPerson() = %v", err)
	}
	root.End()

	traceparent := fake.callsTo("FindPerson")[0].md.Get("traceparent")
	if len(traceparent) != 1 {
		t.Fatalf("metadata traceparent = %v, want one W3C value", traceparent)
	}
	parts := strings.Split(traceparent[0], "-")
	if len(parts) != 4 || parts[1] != root.SpanContext().TraceID().String() {
		t.Fatalf("traceparent %q does not continue the edge trace %s", traceparent[0], root.SpanContext().TraceID())
	}
	var sent *tracetest.SpanStub
	for _, s := range h.spans.GetSpans() {
		if s.SpanContext.SpanID().String() == parts[2] {
			sent = &s
		}
	}
	if sent == nil || sent.SpanKind != trace.SpanKindClient || sent.Name != strings.TrimPrefix(kernelgrpc.FullMethod(rpc.IdentityServiceName, "FindPerson"), "/") {
		t.Fatalf("traceparent names %+v, want the client span of the call, not the edge span", sent)
	}
}
