package api_test

import (
	"bufio"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func listening(t *testing.T, f fixture) string {
	t.Helper()
	srv := httptest.NewUnstartedServer(f.handler)
	srv.Config.ReadHeaderTimeout = 5 * time.Second
	srv.Config.ReadTimeout = 10 * time.Second
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

func TestABodyThatOutlivesTheRouteDeadlineIsRefusedWithinIt(t *testing.T) {
	cases := []struct {
		command string
		framing func(body string) string
		send    func(conn net.Conn, body string)
	}{
		{"registerOrganization", func(body string) string { return fmt.Sprintf("Content-Length: %d\r\n", len(body)) }, func(conn net.Conn, body string) {
			for i := range len(body) {
				if _, err := conn.Write([]byte{body[i]}); err != nil {
					return
				}
				time.Sleep(3 * routeBudget.Limit / time.Duration(len(body)))
			}
		}},
		{"approveBankAccountChange", func(string) string { return "Transfer-Encoding: chunked\r\n" }, func(net.Conn, string) {}},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			c := commandNamed(t, tc.command)
			f, fake := c.edge(t, c.reply, nil)
			conn, err := net.Dial("tcp", listening(t, f))
			if err != nil {
				t.Fatalf("Dial() = %v", err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			body := c.baseBody(t)
			start := time.Now()

			if _, err := fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: edge\r\nAuthorization: %s\r\nIdempotency-Key: k-1\r\nContent-Type: application/json\r\n%s\r\n",
				c.path, requesterCredential, tc.framing(body)); err != nil {
				t.Fatalf("writing the head = %v", err)
			}
			go tc.send(conn, body)
			_ = conn.SetReadDeadline(start.Add(4 * routeBudget.Limit))
			answer, err := http.ReadResponse(bufio.NewReader(conn), nil)

			t.Logf("answered after %v", time.Since(start).Round(time.Millisecond))
			if err != nil {
				t.Fatalf("ReadResponse() = %v after %v", err, time.Since(start))
			}
			defer func() { _ = answer.Body.Close() }()
			if elapsed := time.Since(start); elapsed > routeBudget.Limit+time.Second {
				t.Fatalf("answered after %v, want within the route deadline of %v", elapsed, routeBudget.Limit)
			}
			rec := httptest.NewRecorder()
			maps.Copy(rec.Header(), answer.Header)
			rec.Code = answer.StatusCode
			_, _ = rec.Body.ReadFrom(answer.Body)
			requireRejection(t, rec, http.StatusBadRequest, "invalid-request")
			if !strings.Contains(rec.Body.String(), "did not arrive within the deadline") {
				t.Fatalf("body = %s, want the deadline the body missed", rec.Body.String())
			}
			requireHops(t, fake, c.method, 0)
		})
	}
}

func TestSaturatedCommandsLeaveTheReadsOfAnotherTenantServed(t *testing.T) {
	c := commandNamed(t, "registerOrganization")
	fake := (&fakeIdentity{}).replying(c.method, func(req any) (any, error) {
		time.Sleep(700 * time.Millisecond)
		return c.reply(req)
	}).replying(methodOrganization, answer(&servicev1.FindOrganizationResponse{Organization: &servicev1.Organization{
		OrganizationId: "org-1", Jurisdiction: "BR", TaxIdType: servicev1.TaxIdType_TAX_ID_TYPE_CNPJ, TaxId: "1",
		Status: servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE, CreatedAt: createdAt,
	}}))
	f := identityEdge(t, fake, withAuthenticator(authn.DevAuthenticator{}), withLimit(admission.Limit{PerSecond: 1e6, Burst: 1e6, Concurrency: 64}))
	credential := func(tenant string) string {
		return fmt.Sprintf(`Bearer {"sub":"u-%s","tenant":%q,"permissions":["identity:register-organization","identity:find-organization"]}`, tenant, tenant)
	}
	var wg sync.WaitGroup
	var saturated atomic.Int32
	for i := range 40 {
		wg.Go(func() {
			rec := f.do(t, http.MethodPost, c.path, strings.NewReader(c.baseBody(t)),
				"Idempotency-Key", fmt.Sprintf("k-%d", i), "Content-Type", "application/json", "Authorization", credential("acme"))
			if rec.Code == http.StatusServiceUnavailable && rec.Header().Get("Retry-After") == "1" {
				saturated.Add(1)
			}
		})
	}
	time.Sleep(300 * time.Millisecond)

	rec := f.do(t, http.MethodGet, "/identity/organizations/org-1", nil, "Authorization", credential("globex"))

	wg.Wait()
	if rec.Code != http.StatusOK {
		t.Fatalf("read of another tenant while the commands saturate their pool = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if saturated.Load() == 0 {
		t.Fatal("no command was refused by its saturated bulkhead with Retry-After 1, want the pool full during the read")
	}
}

func commandsBudget(t *testing.T, method string) time.Duration {
	t.Helper()
	cfg := rpc.CommandsConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})
	return cfg.Methods[kernelgrpc.FullMethod(rpc.IdentityServiceName, method)].Budget.EstimatedDuration
}

func TestTheSecondAttemptOfACommandKeepsTheTimeItsDeadlineLeft(t *testing.T) {
	cases := []struct {
		name      string
		firstFail time.Duration
		secondRun time.Duration
	}{
		{"first refused at once, second running 600 ms", 0, 600 * time.Millisecond},
		{"first refused after 500 ms", 500 * time.Millisecond, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := commandNamed(t, "registerOrganization")
			var attempts atomic.Int32
			f, fake := c.edge(t, func(req any) (any, error) {
				if attempts.Add(1) == 1 {
					time.Sleep(tc.firstFail)
					return nil, status.Error(codes.Unavailable, "restarting")
				}
				time.Sleep(tc.secondRun)
				return c.reply(req)
			}, nil)

			rec := c.send(t, f, c.path, c.baseBody(t))

			hops := requireHops(t, fake, c.method, 2)
			t.Logf("attempt 1 had %v, attempt 2 had %v", hops[0].remaining.Round(time.Millisecond), hops[1].remaining.Round(time.Millisecond))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
			if estimated := commandsBudget(t, c.method); hops[1].remaining < estimated+tc.secondRun {
				t.Fatalf("attempt 2 had %v, want at least the %v the method estimates plus its %v of work", hops[1].remaining, estimated, tc.secondRun)
			}
		})
	}
}

func TestAnOverloadedOrUnavailableIdentityAsksTheCallerToRetryInASecond(t *testing.T) {
	c := commandNamed(t, "registerOrganization")
	for _, tc := range []struct {
		code   codes.Code
		status int
		name   string
	}{
		{codes.Unavailable, http.StatusServiceUnavailable, "unavailable"},
		{codes.ResourceExhausted, http.StatusTooManyRequests, "admission-refused"},
	} {
		failing := failing(status.Error(tc.code, "refused"))
		t.Run("command/"+tc.code.String(), func(t *testing.T) {
			f, _ := c.edge(t, failing, nil)

			rec := c.send(t, f, c.path, c.baseBody(t))

			requireRetryAfter(t, rec, tc.status, tc.name)
		})
		t.Run("read/"+tc.code.String(), func(t *testing.T) {
			fake := (&fakeIdentity{}).grant("identity:find-organization").replying(methodOrganization, failing)
			f := identityEdge(t, fake)

			rec := f.do(t, http.MethodGet, "/identity/organizations/org-1", nil, "Authorization", requesterCredential)

			requireRetryAfter(t, rec, tc.status, tc.name)
		})
	}
}

func requireRetryAfter(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	requireRejection(t, rec, status, code)
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
}
