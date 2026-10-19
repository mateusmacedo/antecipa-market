package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var internalBudget = deadline.Budget{Dependency: "identity", Method: "route", Limit: 2 * time.Second, Slack: 200 * time.Millisecond, EstimatedDuration: 100 * time.Millisecond}

func internalRoute(method string) kernelhttp.Route {
	route := kernelhttp.Route{
		Name:        "probe",
		Method:      method,
		Path:        "/probe",
		ContractRef: "apps/backend/identity/contract/openapi/v1/openapi.yaml#/paths/~1probe/" + method,
		Budget:      internalBudget,
		Requires:    kernelhttp.RequireSubjectAndTenant,
		Permission:  "identity:probe",
	}
	if method == http.MethodPost {
		route.IdempotencyKey = IdempotencyHeader
	}
	return route
}

func internalBinding(route kernelhttp.Route) binding {
	return binding{kind: routeQuery, route: route, build: func(handlers) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	}}
}

func internalController(t *testing.T, routes ...kernelhttp.Route) *admission.Controller {
	t.Helper()
	ctrl, err := admission.New(admission.Config{
		Limits:  limitsOf(routes, admission.Limit{PerSecond: 10, Burst: 10, Concurrency: 4}),
		MaxKeys: 8,
		Clock:   obsclock.System(),
	})
	if err != nil {
		t.Fatalf("admission.New() = %v", err)
	}
	return ctrl
}

func TestNewHandlerRefusesAnEdgeRouteWithoutPermission(t *testing.T) {
	route := internalRoute(http.MethodGet)
	route.Permission = ""

	_, err := newHandler([]binding{internalBinding(route)}, internalController(t, route), nil, Options{Authenticator: VerifyOnly(authn.DevAuthenticator{})})

	if !errors.Is(err, kernelhttp.ErrPermissionRequired) {
		t.Fatalf("newHandler() = %v, want ErrPermissionRequired", err)
	}
}

func TestNewHandlerRefusesAPostWithoutAnIdempotencyKey(t *testing.T) {
	route := internalRoute(http.MethodPost)
	route.IdempotencyKey = ""

	_, err := newHandler([]binding{internalBinding(route)}, internalController(t, route), nil, Options{Authenticator: VerifyOnly(authn.DevAuthenticator{})})

	if !errors.Is(err, kernelhttp.ErrIdempotencyKeyRequired) {
		t.Fatalf("newHandler() = %v, want ErrIdempotencyKeyRequired", err)
	}
}

func TestNewHandlerRefusesRoutesWithoutAnAdmissionController(t *testing.T) {
	route := internalRoute(http.MethodGet)

	_, err := newHandler([]binding{internalBinding(route)}, nil, nil, Options{Authenticator: VerifyOnly(authn.DevAuthenticator{})})

	if !errors.Is(err, errAdmissionRequired) {
		t.Fatalf("newHandler() = %v, want errAdmissionRequired", err)
	}
}

func TestNewHandlerServesWithoutRoutesOrController(t *testing.T) {
	if _, err := newHandler(nil, nil, nil, Options{Authenticator: VerifyOnly(authn.DevAuthenticator{})}); err != nil {
		t.Fatalf("newHandler() = %v, want the edge to start with no route", err)
	}
	if _, err := newHandler(nil, nil, nil, Options{}); !errors.Is(err, ErrAuthenticatorRequired) {
		t.Fatalf("newHandler() = %v, want ErrAuthenticatorRequired", err)
	}
}

func TestLimitsOfKeysEveryRouteByPattern(t *testing.T) {
	limit := admission.Limit{PerSecond: 1, Burst: 2, Concurrency: 3}

	limits := limitsOf([]kernelhttp.Route{internalRoute(http.MethodGet), internalRoute(http.MethodPost)}, limit)

	if len(limits) != 2 || limits["GET /probe"] != limit || limits["POST /probe"] != limit {
		t.Fatalf("limitsOf() = %v, want GET /probe and POST /probe at %v", limits, limit)
	}
}

func decodeOf(body string) error {
	var into struct {
		Name string `json:"name"`
	}
	r := httptest.NewRequest(http.MethodPost, "/probe", strings.NewReader(body))
	return decodeBody(httptest.NewRecorder(), r, &into)
}

func objectOf(size int) string {
	const frame = `{"name":""}`
	return `{"name":"` + strings.Repeat("a", size-len(frame)) + `"}`
}

func TestDecodeBodyAcceptsUpToSixtyFourKibibytes(t *testing.T) {
	if err := decodeOf(objectOf(1 << 16)); err != nil {
		t.Fatalf("decodeBody(64 KiB) = %v, want the limit itself accepted", err)
	}
	if err := decodeOf(objectOf(1<<16 + 1)); err == nil {
		t.Fatal("decodeBody(64 KiB + 1) = nil, want the body refused (TRP-49)")
	}
}

func TestDecodeBodyRefusesWhatTheContractDoesNotDeclare(t *testing.T) {
	for name, body := range map[string]string{
		"unknown field":         `{"name":"a","extra":true}`,
		"data after the object": `{"name":"a"} {"name":"b"}`,
		"not an object":         `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := decodeOf(body); err == nil {
				t.Fatalf("decodeBody(%s) = nil, want it refused (TRP-51)", body)
			}
		})
	}
	if err := decodeOf(`{"name":"a"}`); err != nil {
		t.Fatalf("decodeBody() = %v, want a single declared object accepted", err)
	}
}
