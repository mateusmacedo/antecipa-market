package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

const allowedOrigin = "https://app.example"

type unreadable struct{}

func (unreadable) Read([]byte) (int, error) { panic("the body was read before admission") }

func TestCORSWithoutAListEmitsNoAllowOrigin(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Origin", allowedOrigin)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want none when no origin is declared", got)
	}
}

func TestCORSPreflightOfAnAllowedOriginIs204WithItsHeaders(t *testing.T) {
	f := newFixture(t, withCORS(allowedOrigin))

	rec := f.do(t, http.MethodOptions, "/probes/p-1/act", nil, "Origin", allowedOrigin, "Access-Control-Request-Method", "POST")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	for header, want := range map[string]string{
		"Access-Control-Allow-Origin":  allowedOrigin,
		"Access-Control-Allow-Methods": "POST",
		"Access-Control-Allow-Headers": "Idempotency-Key",
		"Access-Control-Max-Age":       "600",
		"Vary":                         "Origin",
	} {
		if got := rec.Header().Get(header); !strings.Contains(got, want) {
			t.Fatalf("%s = %q, want it to contain %q", header, got, want)
		}
	}
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0: the preflight is answered by the edge", n)
	}
}

func TestAPostWithoutTokenAndKeyIs401NotBadRequest(t *testing.T) {
	f := newFixture(t)

	rec := f.do(t, http.MethodPost, "/probes/p-1/act", nil, "Authorization", "")

	requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
	requireRejection(t, f.do(t, http.MethodPost, "/probes/p-1/act", nil), http.StatusBadRequest, "missing-idempotency-key")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestAdmissionRefusesWith429BeforeReadingTheBody(t *testing.T) {
	f := newFixture(t, withLimit(admission.Limit{PerSecond: 0.001, Burst: 1, Concurrency: 10}))

	if rec := f.do(t, http.MethodGet, "/probes/p-1", nil); rec.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", rec.Code)
	}
	requireAdmissionRefused(t, f.do(t, http.MethodGet, "/probes/p-1", unreadable{}))
	if n := f.probe.total(); n != 1 {
		t.Fatalf("handler calls = %d, want 1: the refused request never reached it", n)
	}
}

func TestAdmissionRefusesWith429WhileTheConcurrencySlotIsHeld(t *testing.T) {
	f := newFixture(t, withLimit(admission.Limit{PerSecond: 1000, Burst: 1000, Concurrency: 1}))
	entered, release := make(chan struct{}), make(chan struct{})
	f.probe.respond = func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}
	held := make(chan int)
	go func() { held <- f.do(t, http.MethodGet, "/probes/p-1", nil).Code }()
	select {
	case <-entered:
	case code := <-held:
		t.Fatalf("held request = %d before reaching the handler", code)
	case <-time.After(5 * time.Second):
		t.Fatal("held request never reached the handler")
	}

	refused := f.do(t, http.MethodGet, "/probes/p-1", unreadable{})
	close(release)

	requireAdmissionRefused(t, refused)
	if got := <-held; got != http.StatusNoContent {
		t.Fatalf("held request = %d, want 204", got)
	}
}

func TestAPanicInTheHandlerIs500InternalFailure(t *testing.T) {
	f := newFixture(t)
	f.probe.respond = func(http.ResponseWriter, *http.Request) { panic("handler bug") }

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, http.StatusInternalServerError, "internal-failure")
}

func TestTheHandlerSeesADeadlineWithinTheRouteBudget(t *testing.T) {
	tight := routeBudget
	tight.Limit, tight.Slack, tight.EstimatedDuration = 300*time.Millisecond, 50*time.Millisecond, 50*time.Millisecond
	f := newFixture(t, withBudget(tight))

	f.do(t, http.MethodGet, "/probes/p-1", nil)

	seen := f.probe.last(t)
	if !seen.hasDeadline {
		t.Fatal("the handler saw no deadline, want one governed by the route budget (GRP-05)")
	}
	if limit := time.Until(seen.deadline); limit > tight.Limit {
		t.Fatalf("remaining deadline = %v, want at most the route budget %v", limit, tight.Limit)
	}
}

func TestTheEdgeWithoutRoutesServesOnlyTheContract(t *testing.T) {
	f := newFixture(t, withoutRoutes(), withIdentityContract("identity: yes"))

	if rec := f.do(t, http.MethodGet, api.IdentityContractPath, nil); rec.Code != http.StatusOK || rec.Body.String() != "identity: yes" {
		t.Fatalf("GET %s = %d %q, want 200 with the contract", api.IdentityContractPath, rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodGet, "/probes/p-1", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /probes/p-1 = %d, want 404 with no route mounted", rec.Code)
	}
	if f.probe.total() != 0 {
		t.Fatalf("handler calls = %d, want none", f.probe.total())
	}
}
