// Package api is the public REST edge: each route references a published
// OpenAPI operation (RST-04) and forwards to one gRPC call of a bounded context.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	IdempotencyHeader = "Idempotency-Key"
	ReplayedHeader    = "Idempotent-Replayed"
	CorrelationHeader = "X-Correlation-ID"

	IdentityContractPath = "/openapi/identity/v1/openapi.yaml"
)

type Options struct {
	Budget           deadline.Budget
	Authenticator    ports.Authenticator
	IdentityContract []byte
	CORSOrigins      []string
	Logger           *slog.Logger
	Ready            func(context.Context) error
	Draining         func() bool
	TracerProvider   trace.TracerProvider
	MeterProvider    metric.MeterProvider
	Clock            obsclock.Clock
}

type routeKind string

const (
	routeCommand routeKind = "command"
	routeLookup  routeKind = "lookup"
	routeQuery   routeKind = "query"
	routeListing routeKind = "listing"
)

type binding struct {
	kind  routeKind
	route kernelhttp.Route
	build func(handlers) http.HandlerFunc
}

func bindings(deadline.Budget) []binding { return nil }

func Routes(budget deadline.Budget) []kernelhttp.Route {
	bound := bindings(budget)
	routes := make([]kernelhttp.Route, len(bound))
	for i, b := range bound {
		routes[i] = b.route
	}
	return routes
}

func Limits(limit admission.Limit) map[string]admission.Limit {
	return limitsOf(Routes(deadline.Budget{}), limit)
}

func limitsOf(routes []kernelhttp.Route, limit admission.Limit) map[string]admission.Limit {
	limits := make(map[string]admission.Limit, len(routes))
	for _, route := range routes {
		limits[pattern(route)] = limit
	}
	return limits
}

func pattern(route kernelhttp.Route) string { return route.Method + " " + route.Path }

var (
	// ErrAuthenticatorRequired refuses a handler that could not authenticate:
	// every route demands a subject, and a nil verifier would deny all of them
	// at the first request instead of at the start.
	ErrAuthenticatorRequired = errors.New("api: no authenticator provided")

	errAdmissionRequired = errors.New("api: routes declared without an admission controller")
)

func NewHandler(ctrl *admission.Controller, instruments *metrics.Instruments, opts Options) (http.Handler, error) {
	return newHandler(bindings(opts.Budget), ctrl, instruments, opts)
}

// WHY: the deadline wraps the execution context, not the reverse, because
// CTX-01 makes deadline mandatory and the context cannot resolve what the
// timeout has not yet set. Admission stays inside, still ahead of the body.
func newHandler(bound []binding, ctrl *admission.Controller, instruments *metrics.Instruments, opts Options) (http.Handler, error) {
	if opts.Authenticator == nil {
		return nil, ErrAuthenticatorRequired
	}
	if len(bound) > 0 && ctrl == nil {
		return nil, errAdmissionRequired
	}

	logger := loggerOf(opts)

	var h handlers

	admit := kernelhttp.Admission(ctrl, routeOf, tenantOf, instruments, refuseAsRejection)
	mux := http.NewServeMux()
	for _, b := range bound {
		route := b.route
		if err := route.ValidateEdge(); err != nil {
			return nil, err
		}
		handler := requireIdempotencyKey(route, b.build(h))
		mounted := withExecutionContext(logger, opts.Authenticator, route, admit(handler))
		mux.Handle(pattern(route), withRecover(withRouteDeadline(route.Budget, mounted)))
	}
	if len(opts.IdentityContract) > 0 {
		mux.Handle("GET "+IdentityContractPath, serveContract(opts.IdentityContract))
	}
	return otelhttp.NewHandler(withRoute(withAccessLog(logger, withCORS(opts.CORSOrigins, mux))), "bff", instrumentation(opts)...), nil
}

func instrumentation(opts Options) []otelhttp.Option {
	return []otelhttp.Option{
		otelhttp.WithTracerProvider(opts.TracerProvider),
		otelhttp.WithMeterProvider(opts.MeterProvider),
		otelhttp.WithPropagators(traceparentOnly{}),
	}
}

func loggerOf(opts Options) *slog.Logger {
	if opts.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return opts.Logger
}

func clockOf(opts Options) obsclock.Clock {
	if opts.Clock == nil {
		return obsclock.System()
	}
	return opts.Clock
}

func withRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if _, route, routed := strings.Cut(r.Pattern, " "); routed {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
		}
	})
}

func routeOf(r *http.Request) string { return r.Pattern }

// tenantOf keys the admission bucket by the tenant the edge authenticated
// (RES-16); admission runs inside withExecutionContext, so the context is there.
func tenantOf(r *http.Request) string {
	execution, ok := ports.ExecutionContextFrom(r.Context())
	if !ok {
		return ""
	}
	tenant, _ := execution.Tenant()
	return string(tenant)
}
