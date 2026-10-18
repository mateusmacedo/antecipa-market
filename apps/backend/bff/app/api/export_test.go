package api

import (
	"net/http"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

type (
	Binding   = binding
	RouteKind = routeKind
)

const (
	RouteCommand = routeCommand
	RouteLookup  = routeLookup
	RouteQuery   = routeQuery
	RouteListing = routeListing
)

func NewBinding(kind RouteKind, route kernelhttp.Route, handler http.HandlerFunc) Binding {
	return binding{kind: kind, route: route, build: func(handlers) http.HandlerFunc { return handler }}
}

func NewHandlerOf(bound []Binding, ctrl *admission.Controller, instruments *metrics.Instruments, opts Options) (http.Handler, error) {
	return newHandler(bound, ctrl, instruments, opts)
}

func LimitsOf(routes []kernelhttp.Route, limit admission.Limit) map[string]admission.Limit {
	return limitsOf(routes, limit)
}

func Bindings(budget deadline.Budget) []Binding { return bindings(budget) }

func KindOf(b Binding) RouteKind { return b.kind }

func RouteOf(b Binding) kernelhttp.Route { return b.route }
