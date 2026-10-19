package app_test

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func dialPermissions(t *testing.T, identity *fakeIdentity) rpc.Identity {
	t.Helper()
	conn, err := rpc.Dial(identity.addr, rpc.PermissionsConfig(rpc.Options{Insecure: true, Clock: obsclock.System()}))
	if err != nil {
		t.Fatalf("Dial(%s) = %v", identity.addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return rpc.NewIdentity(conn)
}

func TestATransientFailureOfThePermissionsHopLeavesTheRouteCallItsWholeDeadline(t *testing.T) {
	const routeUser = "route-user"
	identity := serveIdentity(t)
	var hops atomic.Int32
	identity.answeringTo(func(_ context.Context, user string) (*servicev1.FindEffectivePermissionsResponse, error) {
		if user != routeUser && hops.Add(1) == 1 {
			return nil, status.Error(codes.Unavailable, "restarting")
		}
		return &servicev1.FindEffectivePermissionsResponse{}, nil
	})
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialPermissions(t, identity))
	routes := dialIdentity(t, identity)
	ctx := retry.WithBudget(requestContext(t, 2*time.Second))

	if _, err := decorated.Authenticate(ctx, bearer(claimingCredential)); err != nil || hops.Load() != 2 {
		t.Fatalf("Authenticate() = %v after %d hops, want success on the retry", err, hops.Load())
	}
	sent := time.Now()
	if _, err := routes.FindEffectivePermissions(ctx, &servicev1.FindEffectivePermissionsRequest{UserId: routeUser}); err != nil {
		t.Fatalf("FindEffectivePermissions() = %v", err)
	}

	asked := identity.questions()
	route := asked[slices.IndexFunc(asked, func(q permissionsAsked) bool { return q.userID == routeUser })]
	budget := rpc.IdentityConfig(rpc.Options{Insecure: true, Clock: obsclock.System()}).
		Methods[kernelgrpc.FullMethod(rpc.IdentityServiceName, "FindEffectivePermissions")].Budget
	if got, want := route.deadline.Sub(sent), budget.Limit-budget.Slack; got < want-20*time.Millisecond {
		t.Fatalf("the route call received %v, want the %v its method declares: the retry of the hop spent another budget (RES-06)", got, want)
	}
}
