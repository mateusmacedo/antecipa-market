package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func TestRefusedIdempotencyKeyDecidesByTheRoute(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	keyed := kernelhttp.Route{Method: http.MethodPost, IdempotencyKey: IdempotencyHeader}
	unkeyed := kernelhttp.Route{Method: http.MethodPost}
	other := kernelhttp.Route{Method: http.MethodPost, IdempotencyKey: "X-Command-Key"}
	cases := map[string]struct {
		route  kernelhttp.Route
		header string
		key    string
		want   int
		code   string
	}{
		"keyed route without a key":            {keyed, IdempotencyHeader, "", http.StatusBadRequest, "missing-idempotency-key"},
		"keyed route with a valid key":         {keyed, IdempotencyHeader, "k-1", http.StatusNoContent, ""},
		"keyed route with an invalid key":      {keyed, IdempotencyHeader, " ", http.StatusBadRequest, "invalid-idempotency-key"},
		"unkeyed route without a key":          {unkeyed, IdempotencyHeader, "", http.StatusNoContent, ""},
		"unkeyed route with an invalid key":    {unkeyed, IdempotencyHeader, "abcé", http.StatusBadRequest, "invalid-idempotency-key"},
		"route header carries the key":         {other, "X-Command-Key", "k-1", http.StatusNoContent, ""},
		"route header absent, default present": {other, IdempotencyHeader, "k-1", http.StatusBadRequest, "X-Command-Key header"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.route.Method, "/probe", nil)
			if tc.key != "" {
				req.Header.Set(tc.header, tc.key)
			}
			rec := httptest.NewRecorder()

			if !refusedIdempotencyKey(req, rec, tc.route, req.Header.Get(idempotencyHeaderOf(tc.route))) {
				next.ServeHTTP(rec, req)
			}

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.code != "" && !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("body = %s, want it to mention %s", rec.Body.String(), tc.code)
			}
		})
	}
}

type actedRequest struct {
	id    string
	actor string
}

type probeBody struct {
	Name *string `json:"name"`
}

func requestTo(body io.Reader) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/probes/p-1/act", body)
	r.SetPathValue("id", "p-1")
	return r
}

func rejectionOf(t *testing.T, rec *httptest.ResponseRecorder) rejection {
	t.Helper()
	var got rejection
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %s is not a Rejection: %v", rec.Body.String(), err)
	}
	return got
}

func TestFromPathWithoutBodyRefusesAnyContent(t *testing.T) {
	decode := fromPathWithoutBody("id", func(id string) string { return id })
	cases := map[string]struct {
		id   string
		body io.Reader
		want int
	}{
		"no body":         {"p-1", nil, http.StatusOK},
		"empty body":      {"p-1", strings.NewReader(""), http.StatusOK},
		"empty object":    {"p-1", strings.NewReader("{}"), http.StatusBadRequest},
		"asserted actor":  {"p-1", strings.NewReader(`{"approvedByUserId":"u-1"}`), http.StatusBadRequest},
		"whitespace":      {"p-1", strings.NewReader(" "), http.StatusBadRequest},
		"null":            {"p-1", strings.NewReader("null"), http.StatusBadRequest},
		"invalid path id": {"p 1", nil, http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := requestTo(tc.body)
			r.SetPathValue("id", tc.id)
			rec := httptest.NewRecorder()

			id, ok := decode(rec, r)

			if tc.want == http.StatusOK {
				if !ok || id != "p-1" || rec.Body.Len() != 0 {
					t.Fatalf("decode() = %q, %v with body %s, want the path id and nothing written", id, ok, rec.Body.String())
				}
				return
			}
			if ok || rec.Code != tc.want || rejectionOf(t, rec).Code != "invalid-request" {
				t.Fatalf("decode() = %v, status %d, body %s, want 400 invalid-request", ok, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFromPathAndBodyRefusesWhatTheSchemaDoesNotAdmit(t *testing.T) {
	decode := fromPathAndBody("id", "ProbeRequest", "name is required",
		func(b probeBody) bool { return b.Name != nil },
		func(id string, b probeBody) actedRequest { return actedRequest{id: id + "/" + *b.Name} })
	cases := map[string]struct {
		body string
		want string
	}{
		"unknown field":   {`{"name":"a","requestedByUserId":"u-1"}`, "the body is not the ProbeRequest of the contract"},
		"not json":        {`name=a`, "the body is not the ProbeRequest of the contract"},
		"missing field":   {`{}`, "name is required"},
		"empty but given": {`{"name":""}`, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			req, ok := decode(rec, requestTo(strings.NewReader(tc.body)))

			if tc.want == "" {
				if !ok || req.id != "p-1/" {
					t.Fatalf("decode() = %+v, %v, want the empty value passed on to the context", req, ok)
				}
				return
			}
			got := rejectionOf(t, rec)
			if ok || rec.Code != http.StatusBadRequest || got.Code != "invalid-request" || got.Message != tc.want {
				t.Fatalf("decode() = %v, status %d, rejection %+v, want 400 invalid-request %q", ok, rec.Code, got, tc.want)
			}
		})
	}
}

func executionWith(t *testing.T, subject *ports.SubjectID) context.Context {
	t.Helper()
	spec := ports.ExecutionContextSpec{
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		TraceContext:  "trace-1",
		Subject:       subject,
		Deadline:      ports.Instant(time.Now().Add(time.Second).UnixNano()),
		Locale:        "pt-BR",
	}
	if subject != nil {
		spec.Permissions = []ports.Permission{}
	}
	execution, err := ports.NewExecutionContext(spec)
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return kernelhttp.WithExecutionContext(context.Background(), execution)
}

func TestACommandWithoutAnAuthenticatedSubjectNeverReachesTheContext(t *testing.T) {
	subject := ports.SubjectID("tester")
	cases := map[string]struct {
		ctx   context.Context
		want  int
		actor string
	}{
		"no execution context":  {context.Background(), http.StatusInternalServerError, ""},
		"no subject in context": {executionWith(t, nil), http.StatusInternalServerError, ""},
		"authenticated subject": {executionWith(t, &subject), http.StatusCreated, "tester"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var reached []actedRequest
			handler := endpoint(
				acting(fromPath("id", func(id string) *actedRequest { return &actedRequest{id: id} }),
					func(req *actedRequest, actor string) { req.actor = actor }),
				func(_ context.Context, req *actedRequest) (*actedRequest, error) {
					reached = append(reached, *req)
					return req, nil
				},
				outcome(http.StatusCreated, func(req *actedRequest) (reference, refusal) { return reference{"id", req.id}, nil }))
			rec := httptest.NewRecorder()

			handler(rec, requestTo(nil).WithContext(tc.ctx))

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			if tc.actor == "" {
				if len(reached) != 0 || rejectionOf(t, rec).Code != "internal-failure" {
					t.Fatalf("calls = %v, body %s, want no call and internal-failure", reached, rec.Body.String())
				}
				return
			}
			if len(reached) != 1 || reached[0].actor != tc.actor {
				t.Fatalf("calls = %+v, want one carrying the actor %q from the subject", reached, tc.actor)
			}
		})
	}
}

func TestOutcomeAnswersWhatTheContextDecided(t *testing.T) {
	cases := map[string]struct {
		body    reference
		refused refusal
		status  int
		code    string
		message string
	}{
		"accepted":                  {body: reference{"id", "o-1"}, status: http.StatusCreated},
		"rejected":                  {refused: &servicev1.Rejection{Code: "duplicate-tax-id", Message: "already registered"}, status: http.StatusUnprocessableEntity, code: "duplicate-tax-id", message: "already registered"},
		"unrepresentable":           {status: http.StatusInternalServerError, code: "internal-failure", message: "the request could not be completed"},
		"accepted with an empty id": {body: reference{"id", ""}, status: http.StatusInternalServerError, code: "internal-failure", message: "the request could not be completed"},
		"accepted with an id outside the contract": {body: reference{"id", "o 1"}, status: http.StatusInternalServerError, code: "internal-failure", message: "the request could not be completed"},
		"rejected without a code":                  {refused: &servicev1.Rejection{Message: "refused"}, status: http.StatusInternalServerError, code: "internal-failure", message: "the request could not be completed"},
		"rejected by a nil rejection":              {refused: (*servicev1.Rejection)(nil), status: http.StatusInternalServerError, code: "internal-failure", message: "the request could not be completed"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			present := outcome(http.StatusCreated, func(struct{}) (reference, refusal) { return tc.body, tc.refused })
			rec := httptest.NewRecorder()

			present(rec, requestTo(nil), struct{}{})

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.status, rec.Body.String())
			}
			if tc.code == "" {
				return
			}
			if got := rejectionOf(t, rec); got.Code != tc.code || got.Message != tc.message {
				t.Fatalf("rejection = %+v, want %s: %s", got, tc.code, tc.message)
			}
		})
	}
}
