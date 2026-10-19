package app

import (
	"context"
	"net/http"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

type Permissions struct {
	inner ports.Authenticator
	find  rpc.Unary[servicev1.FindEffectivePermissionsRequest, servicev1.FindEffectivePermissionsResponse]
}

func NewPermissions(inner ports.Authenticator, identity rpc.Identity) Permissions {
	return Permissions{inner: inner, find: identity.FindEffectivePermissions}
}

func (p Permissions) Authenticate(ctx context.Context, credential ports.Credential) (ports.Identity, error) {
	identity, err := p.Verify(ctx, credential)
	if err != nil {
		return identity, err
	}
	return p.Resolve(ctx, identity)
}

func (p Permissions) Verify(ctx context.Context, credential ports.Credential) (ports.Identity, error) {
	return p.inner.Authenticate(ctx, credential)
}

func (p Permissions) Resolve(ctx context.Context, identity ports.Identity) (ports.Identity, error) {
	if identity.Tenant == nil {
		identity.Permissions = nil
		return identity, nil
	}

	call, _ := rpc.CallFrom(ctx)
	call.TenantID = string(*identity.Tenant)
	effective, err := p.find(hopBudget(rpc.WithCall(ctx, call)), &servicev1.FindEffectivePermissionsRequest{UserId: string(identity.Subject)})
	if err != nil {
		return ports.Identity{}, &rpc.PermissionsUnavailable{Failure: permissionsFailure(err), Cause: err}
	}

	identity.Permissions = make([]ports.Permission, 0, len(effective.GetPermissions()))
	for _, permission := range effective.GetPermissions() {
		identity.Permissions = append(identity.Permissions, ports.Permission(permission))
	}
	return identity, nil
}

// The route's budget would cap the route call by what the hop's retry left of it
// (observability@v1.0.0-rc.5 resilience/timeout.go:56-61); armed on the first
// failure instead, half of the hop's deadline leaves no time past the reserve.
func hopBudget(ctx context.Context) context.Context {
	return retry.WithBudget(ctx, retry.WithTotal(rpc.PermissionsLimit))
}

// A 4xx of the identity answers the edge's own question, not the caller's, so
// echoing it would blame the request for a fault of the platform.
func permissionsFailure(err error) rpc.Failure {
	failure := rpc.Classify(err)
	switch failure.Status {
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout, rpc.CanceledFailure.Status:
		return failure
	case http.StatusTooManyRequests:
		return rpc.OverloadedFailure
	}
	return rpc.InternalFailure
}
