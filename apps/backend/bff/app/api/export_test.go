package api

import (
	"context"
	"net/http"
	"reflect"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
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
	return binding{kind: kind, route: route, bodyLimit: maxBodyBytes, build: func(handlers) http.HandlerFunc { return handler }}
}

const MaxBodyBytes = maxBodyBytes

func BodyLimitOf(b Binding) int64 { return b.bodyLimit }

func newHandler(bound []binding, ctrl *admission.Controller, instruments *metrics.Instruments, opts Options) (http.Handler, error) {
	return mount(bound, handlers{}, ctrl, instruments, opts)
}

type verifyOnly struct{ ports.Authenticator }

func (v verifyOnly) Verify(ctx context.Context, credential ports.Credential) (ports.Identity, error) {
	return v.Authenticate(ctx, credential)
}

func (verifyOnly) Resolve(_ context.Context, identity ports.Identity) (ports.Identity, error) {
	return identity, nil
}

func VerifyOnly(authenticator ports.Authenticator) Authenticator {
	if authenticator == nil {
		return nil
	}
	return verifyOnly{authenticator}
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

func WriteFailure(r *http.Request, w http.ResponseWriter, failure rpc.Failure) {
	writeFailure(r, w, failure)
}

func WriteJSON(r *http.Request, w http.ResponseWriter, status int, body any) {
	writeJSON(r, w, status, body)
}

func RequestBodies() map[string]reflect.Type {
	return map[string]reflect.Type{
		"registerOrganization":     reflect.TypeFor[registerOrganizationBody](),
		"findOrganizationByTaxId":  reflect.TypeFor[findOrganizationByTaxIDBody](),
		"addEstablishment":         reflect.TypeFor[addEstablishmentBody](),
		"registerPerson":           reflect.TypeFor[registerPersonBody](),
		"provisionUserAccount":     reflect.TypeFor[provisionUserAccountBody](),
		"requestBankAccountChange": reflect.TypeFor[requestBankAccountChangeBody](),
		"registerRepresentation":   reflect.TypeFor[registerRepresentationBody](),
		"inviteUser":               reflect.TypeFor[inviteUserBody](),
		"suspendUser":              reflect.TypeFor[reasonBody](),
		"revokeMembership":         reflect.TypeFor[reasonBody](),
		"approveBankAccountChange": nil,
	}
}
