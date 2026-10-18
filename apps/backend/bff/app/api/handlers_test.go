package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

const incomingTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

const incomingTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"

func TestAnIncomingTraceparentIsContinued(t *testing.T) {
	f := newFixture(t)

	f.do(t, http.MethodGet, "/probes/p-1", nil, "traceparent", incomingTraceparent)

	for _, s := range f.spans.GetSpans() {
		if s.Name == "GET /probes/{id}" {
			if s.SpanContext.TraceID().String() != incomingTraceID {
				t.Fatalf("edge trace = %s, want the incoming trace", s.SpanContext.TraceID())
			}
			return
		}
	}
	t.Fatal("no edge span recorded")
}

func TestATracestateFromThePublicClientReachesNoSpan(t *testing.T) {
	f := newFixture(t)
	members := make([]string, 32)
	for i := range members {
		members[i] = fmt.Sprintf("probe%d=%s", i, strings.Repeat("x", 200))
	}

	f.do(t, http.MethodGet, "/probes/p-1", nil, "traceparent", incomingTraceparent, "tracestate", strings.Join(members, ","))

	spans := f.spans.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no span recorded")
	}
	for _, s := range spans {
		if s.SpanContext.TraceID().String() != incomingTraceID {
			t.Fatalf("%s trace = %s, want the incoming traceparent continued", s.Name, s.SpanContext.TraceID())
		}
		if s.SpanContext.TraceState().Len() != 0 || s.Parent.TraceState().Len() != 0 {
			t.Fatalf("%s tracestate = %q (parent %q), want none from the public client", s.Name, s.SpanContext.TraceState(), s.Parent.TraceState())
		}
	}
}

func TestAMalformedCorrelationIsReplaced(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, api.CorrelationHeader, "bad correlation;")

	minted := rec.Header().Get(api.CorrelationHeader)
	if minted == "" || minted == "bad correlation;" {
		t.Fatalf("%s = %q, want a minted correlation", api.CorrelationHeader, minted)
	}
	if got := f.probe.last(t).execution.CorrelationID(); got != minted {
		t.Fatalf("execution correlation = %q, want the minted %q", got, minted)
	}
}

func TestAValidCorrelationIsEchoed(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, api.CorrelationHeader, "corr-1")

	if got := rec.Header().Get(api.CorrelationHeader); got != "corr-1" {
		t.Fatalf("%s = %q, want corr-1 echoed", api.CorrelationHeader, got)
	}
	if got := f.probe.last(t).execution.CorrelationID(); got != "corr-1" {
		t.Fatalf("execution correlation = %q, want corr-1", got)
	}
}

func TestTheContractIsServedOnlyWhenProvided(t *testing.T) {
	silent := newFixture(t)
	if rec := silent.do(t, http.MethodGet, api.IdentityContractPath, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("%s without document = %d, want 404", api.IdentityContractPath, rec.Code)
	}

	f := newFixture(t, withIdentityContract("identity: yes"))
	rec := f.do(t, http.MethodGet, api.IdentityContractPath, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "identity: yes" {
		t.Fatalf("%s = %d %q, want 200 %q", api.IdentityContractPath, rec.Code, rec.Body.String(), "identity: yes")
	}
}

func TestCORSAnswersThePreflightOfDeclaredOrigins(t *testing.T) {
	f := newFixture(t, withCORS("http://localhost:8082"))

	rec := f.do(t, http.MethodOptions, "/probes/p-1/act", nil, "Origin", "http://localhost:8082", "Access-Control-Request-Method", "POST")

	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8082" {
		t.Fatalf("preflight = %d %v, want 204 with the origin allowed", rec.Code, rec.Header())
	}
	for _, header := range []string{api.IdempotencyHeader, "Authorization"} {
		if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), header) {
			t.Fatalf("Access-Control-Allow-Headers = %q, want %s", rec.Header().Get("Access-Control-Allow-Headers"), header)
		}
	}

	other := f.do(t, http.MethodOptions, "/probes/p-1/act", nil, "Origin", "http://evil.example")
	if other.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("an undeclared origin was allowed")
	}
}

func TestAnIdempotencyKeyTheWireRejectsNeverReachesTheHandler(t *testing.T) {
	f := newFixture(t)
	for _, key := range []string{"abcé", "abc\tdef", strings.Repeat("k", 200), " "} {
		t.Run(fmt.Sprintf("%q", key), func(t *testing.T) {
			rec := f.do(t, http.MethodPost, "/probes/p-1/act", nil,
				"Idempotency-Key", key, "Content-Type", "application/json")

			requireRejection(t, rec, http.StatusBadRequest, "invalid-idempotency-key")
		})
	}
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestAGetWithAnInvalidIdempotencyKeyNeverReachesTheHandler(t *testing.T) {
	f := newFixture(t)
	for _, key := range []string{"abcé", strings.Repeat("k", 200), " "} {
		t.Run(fmt.Sprintf("%q", key), func(t *testing.T) {
			rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Idempotency-Key", key)

			requireRejection(t, rec, http.StatusBadRequest, "invalid-idempotency-key")
		})
	}
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestOneTenantExhaustingItsBucketDoesNotRefuseAnother(t *testing.T) {
	f := newFixture(t, withLimit(admission.Limit{PerSecond: 0.001, Burst: 1, Concurrency: 10}))
	other := `Bearer {"sub":"tester","tenant":"globex","permissions":["identity:act-on-probe","identity:find-probe"]}`

	if rec := f.do(t, http.MethodGet, "/probes/p-1", nil); rec.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", rec.Code)
	}
	requireAdmissionRefused(t, f.do(t, http.MethodGet, "/probes/p-1", nil))
	if rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Authorization", other); rec.Code != http.StatusOK {
		t.Fatalf("another tenant's request = %d, want 200: the buckets are per tenant (RES-16)", rec.Code)
	}
}
