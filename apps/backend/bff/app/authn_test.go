package app_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	effectivePermission = "identity:find-organization"
	claimedPermission   = "identity:act-on-probe"

	claimingCredential   = `{"sub":"tester","tenant":"acme","permissions":["identity:act-on-probe"]}`
	tenantlessCredential = `{"sub":"tester","permissions":["identity:act-on-probe"]}`
	forgedCredential     = "not-a-declaration"
)

func bearer(value string) ports.Credential { return ports.Credential{Scheme: "Bearer", Value: value} }

func dialIdentity(t *testing.T, identity *fakeIdentity) rpc.Identity {
	t.Helper()
	return dialIdentityAt(t, identity.addr)
}

func dialIdentityAt(t *testing.T, target string) rpc.Identity {
	t.Helper()
	conn, err := rpc.Dial(target, rpc.IdentityConfig(rpc.Options{Insecure: true, Clock: obsclock.System()}))
	if err != nil {
		t.Fatalf("Dial(%s) = %v", target, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return rpc.NewIdentity(conn)
}

func requestContext(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return rpc.WithCall(ctx, rpc.Call{CorrelationID: "corr-1", RequestID: "req-1", Locale: "pt-BR"})
}

func TestTheDecoratorAsksTheIdentityOnceWithTheCallOfTheRequestAndTheTenantOfTheToken(t *testing.T) {
	identity := serveIdentity(t)
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))

	if _, err := decorated.Authenticate(requestContext(t, 2*time.Second), bearer(claimingCredential)); err != nil {
		t.Fatalf("Authenticate() = %v", err)
	}

	asked := identity.questions()
	if len(asked) != 1 {
		t.Fatalf("FindEffectivePermissions calls = %d, want 1", len(asked))
	}
	if asked[0].userID != "tester" {
		t.Errorf("user_id = %q, want the subject of the token", asked[0].userID)
	}
	for key, want := range map[string]string{
		kernelgrpc.CorrelationKey: "corr-1",
		kernelgrpc.CausationKey:   "req-1",
		kernelgrpc.LocaleKey:      "pt-BR",
		kernelgrpc.TenantKey:      "acme",
	} {
		if got := asked[0].md.Get(key); len(got) != 1 || got[0] != want {
			t.Errorf("metadata %s = %v, want %q", key, got, want)
		}
	}
}

func TestTheEffectivePermissionsReplaceTheClaimsOfTheToken(t *testing.T) {
	cases := []struct {
		name      string
		effective []string
	}{
		{name: "a granted permission", effective: []string{effectivePermission}},
		{name: "an empty list", effective: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := serveIdentity(t)
			identity.grant(tc.effective...)
			decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))

			resolved, err := decorated.Authenticate(requestContext(t, 2*time.Second), bearer(claimingCredential))
			if err != nil {
				t.Fatalf("Authenticate() = %v", err)
			}

			want := make([]ports.Permission, len(tc.effective))
			for i, p := range tc.effective {
				want[i] = ports.Permission(p)
			}
			if !slices.Equal(resolved.Permissions, want) {
				t.Fatalf("permissions = %v, want %v: the claims never authorize", resolved.Permissions, want)
			}
		})
	}
}

func TestATenantlessIdentityNeverReachesTheIdentity(t *testing.T) {
	identity := serveIdentity(t)
	identity.grant(effectivePermission)
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))

	resolved, err := decorated.Authenticate(requestContext(t, 2*time.Second), bearer(tenantlessCredential))
	if err != nil {
		t.Fatalf("Authenticate() = %v", err)
	}

	if resolved.Subject != "tester" || resolved.Tenant != nil || len(resolved.Permissions) != 0 {
		t.Fatalf("resolved = %+v, want the subject without tenant nor permissions", resolved)
	}
	if n := len(identity.questions()); n != 0 {
		t.Fatalf("FindEffectivePermissions calls = %d, want 0", n)
	}
}

func TestARefusedCredentialNeverReachesTheIdentity(t *testing.T) {
	identity := serveIdentity(t)
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))

	_, err := decorated.Authenticate(requestContext(t, 2*time.Second), bearer(forgedCredential))

	var unavailable *rpc.PermissionsUnavailable
	if !errors.Is(err, ports.ErrCredentialRejected) || errors.As(err, &unavailable) {
		t.Fatalf("Authenticate() = %v, want the refusal of the verifier as it is", err)
	}
	if n := len(identity.questions()); n != 0 {
		t.Fatalf("FindEffectivePermissions calls = %d, want 0", n)
	}
}

func TestAFailingIdentityLeavesThePermissionsUnavailable(t *testing.T) {
	cases := []struct {
		name   string
		answer permissionsAnswer
		status int
		code   string
	}{
		{
			name: "refused as unavailable",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.Unavailable, "identity down")
			},
			status: http.StatusServiceUnavailable,
			code:   "unavailable",
		},
		{
			name: "past the deadline",
			answer: func(ctx context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
			status: http.StatusGatewayTimeout,
			code:   "deadline-exceeded",
		},
		{
			name: "refused as cancelled",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.Canceled, "gone")
			},
			status: kernelgrpc.StatusClientClosedRequest,
			code:   "client-closed-request",
		},
		{
			name: "refused as not found",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.NotFound, "no membership")
			},
			status: http.StatusInternalServerError,
			code:   "internal-failure",
		},
		{
			name: "refused by admission",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.ResourceExhausted, "per-tenant limit")
			},
			status: http.StatusServiceUnavailable,
			code:   "unavailable",
		},
		{
			name: "refused as internal",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.Internal, "poison row")
			},
			status: http.StatusInternalServerError,
			code:   "internal-failure",
		},
		{
			name: "refused as unknown",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.Unknown, "panic")
			},
			status: http.StatusInternalServerError,
			code:   "internal-failure",
		},
		{
			name: "refused as aborted",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.Aborted, "conflict")
			},
			status: http.StatusInternalServerError,
			code:   "internal-failure",
		},
		{
			name: "refused as invalid",
			answer: func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(codes.InvalidArgument, "user_id")
			},
			status: http.StatusInternalServerError,
			code:   "internal-failure",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := serveIdentity(t)
			identity.answering(tc.answer)
			decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))

			_, err := decorated.Authenticate(requestContext(t, 400*time.Millisecond), bearer(claimingCredential))

			var unavailable *rpc.PermissionsUnavailable
			if !errors.As(err, &unavailable) {
				t.Fatalf("Authenticate() = %v, want *rpc.PermissionsUnavailable", err)
			}
			if unavailable.Failure.Status != tc.status || unavailable.Failure.Code != tc.code {
				t.Fatalf("failure = %+v, want %d %s", unavailable.Failure, tc.status, tc.code)
			}
		})
	}
}

func TestAnIdentityRefusingByAdmissionAsksTheCallerToRetryInASecond(t *testing.T) {
	identity := serveIdentity(t)
	identity.answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		return nil, status.Error(codes.ResourceExhausted, "per-tenant limit")
	})
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))

	_, err := decorated.Authenticate(requestContext(t, 400*time.Millisecond), bearer(claimingCredential))

	var unavailable *rpc.PermissionsUnavailable
	if !errors.As(err, &unavailable) || unavailable.Failure.RetryAfter != time.Second {
		t.Fatalf("Authenticate() = %v, want *rpc.PermissionsUnavailable asking a retry after 1s", err)
	}
}

func TestACallerThatWentAwayLeavesThePermissionsAsClientClosed(t *testing.T) {
	identity := serveIdentity(t)
	started := make(chan struct{})
	var once sync.Once
	identity.answering(func(ctx context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return nil, ctx.Err()
	})
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))
	ctx, cancel := context.WithCancel(requestContext(t, 2*time.Second))
	go func() {
		<-started
		cancel()
	}()

	_, err := decorated.Authenticate(ctx, bearer(claimingCredential))

	var unavailable *rpc.PermissionsUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("Authenticate() = %v, want *rpc.PermissionsUnavailable", err)
	}
	if unavailable.Failure.Status != kernelgrpc.StatusClientClosedRequest || unavailable.Failure.Code != "client-closed-request" {
		t.Fatalf("failure = %+v, want 499 client-closed-request", unavailable.Failure)
	}
}

func TestAnAlreadyCancelledRequestLeavesThePermissionsAsClientClosed(t *testing.T) {
	identity := serveIdentity(t)
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))
	ctx, cancel := context.WithCancel(requestContext(t, 2*time.Second))
	cancel()

	_, err := decorated.Authenticate(ctx, bearer(claimingCredential))

	var unavailable *rpc.PermissionsUnavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("Authenticate() = %v, want *rpc.PermissionsUnavailable", err)
	}
	if unavailable.Failure.Status != kernelgrpc.StatusClientClosedRequest || unavailable.Failure.Code != "client-closed-request" {
		t.Fatalf("failure = %+v, want 499 client-closed-request", unavailable.Failure)
	}
}

func TestVerifyNeverReachesTheIdentity(t *testing.T) {
	identity := serveIdentity(t)
	identity.grant(effectivePermission)
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))

	verified, err := decorated.Verify(requestContext(t, 2*time.Second), bearer(claimingCredential))
	if err != nil {
		t.Fatalf("Verify() = %v", err)
	}

	if verified.Subject != "tester" || verified.Tenant == nil || *verified.Tenant != "acme" {
		t.Fatalf("verified = %+v, want the subject and tenant of the token", verified)
	}
	if n := len(identity.questions()); n != 0 {
		t.Fatalf("FindEffectivePermissions calls = %d, want 0: only Resolve asks", n)
	}
}

func TestResolveReplacesTheClaimsOfAVerifiedIdentity(t *testing.T) {
	identity := serveIdentity(t)
	identity.grant(effectivePermission)
	decorated := app.NewPermissions(authn.DevAuthenticator{}, dialIdentity(t, identity))
	tenant := ports.TenantID("acme")

	resolved, err := decorated.Resolve(requestContext(t, 2*time.Second), ports.Identity{Subject: "tester", Tenant: &tenant, Permissions: []ports.Permission{claimedPermission}})
	if err != nil {
		t.Fatalf("Resolve() = %v", err)
	}

	if !slices.Equal(resolved.Permissions, []ports.Permission{effectivePermission}) {
		t.Fatalf("permissions = %v, want only the effective one", resolved.Permissions)
	}
	asked := identity.questions()
	if len(asked) != 1 || asked[0].md.Get(kernelgrpc.TenantKey)[0] != "acme" {
		t.Fatalf("questions = %+v, want one carrying the tenant acme", asked)
	}
}
