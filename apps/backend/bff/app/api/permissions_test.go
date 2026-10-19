package api_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	otelcodes "go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const methodSlack = 100 * time.Millisecond

func edgeOver(t *testing.T, identity rpc.Identity, options ...option) fixture {
	t.Helper()
	f := newFixture(t, append([]option{withAuthenticator(app.NewPermissions(authn.DevAuthenticator{}, identity))}, options...)...)
	f.probe.respond = func(w http.ResponseWriter, r *http.Request) {
		if _, err := identity.FindOrganization(r.Context(), &servicev1.FindOrganizationRequest{OrganizationId: r.PathValue("id")}); err != nil {
			api.WriteFailure(r, w, rpc.Classify(err))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	return f
}

func TestTheEdgeAnswers503WhenTheIdentityIsDown(t *testing.T) {
	f := edgeOver(t, identityDown(t))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, http.StatusServiceUnavailable, "unavailable")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestTheEdgeAnswers504WhenThePermissionsMissTheDeadline(t *testing.T) {
	identity := (&fakeIdentity{}).answering(func(ctx context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	tight := routeBudget
	tight.Limit, tight.Slack, tight.EstimatedDuration = 300*time.Millisecond, 50*time.Millisecond, 50*time.Millisecond
	f := edgeOver(t, identity.dial(t, obsclock.System()), withBudget(tight))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, http.StatusGatewayTimeout, "deadline-exceeded")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
}

func TestARefusingVerifierStays401WhileTheIdentityIsDown(t *testing.T) {
	f := edgeOver(t, identityDown(t))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Authorization", forgedCredential)

	requireRejection(t, rec, http.StatusUnauthorized, "unauthenticated")
}

func TestAnEmptyEffectiveListIs403WithoutCallingTheRoute(t *testing.T) {
	identity := (&fakeIdentity{}).grant()
	f := edgeOver(t, identity.dial(t, obsclock.System()))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, http.StatusForbidden, "permission-denied")
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
	if n := len(identity.hopsTo(methodOrganization)); n != 0 {
		t.Fatalf("%s calls = %d, want 0", methodOrganization, n)
	}
}

func TestBothHopsCarryTheCallOfTheRequest(t *testing.T) {
	identity := (&fakeIdentity{}).grant(queryPermission)
	f := edgeOver(t, identity.dial(t, obsclock.System()))
	correlation := kernelgrpc.NewID("test")

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, api.CorrelationHeader, correlation, "Accept-Language", "pt-BR")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	permissions, route := identity.hopsTo(methodPermissions), identity.hopsTo(methodOrganization)
	if len(permissions) != 1 || len(route) != 1 {
		t.Fatalf("hops = %d to %s and %d to %s, want one each", len(permissions), methodPermissions, len(route), methodOrganization)
	}
	causation := permissions[0].md.Get(kernelgrpc.CausationKey)
	if len(causation) != 1 || causation[0] == "" {
		t.Fatalf("%s on the permissions hop = %v, want the request id", kernelgrpc.CausationKey, causation)
	}
	for name, md := range map[string]map[string][]string{methodPermissions: permissions[0].md, methodOrganization: route[0].md} {
		for key, want := range map[string]string{
			kernelgrpc.CorrelationKey: correlation,
			kernelgrpc.CausationKey:   causation[0],
			kernelgrpc.LocaleKey:      "pt-BR",
			kernelgrpc.TenantKey:      testTenant,
		} {
			if got := md[key]; len(got) != 1 || got[0] != want {
				t.Errorf("%s: metadata %s = %v, want %q", name, key, got, want)
			}
		}
	}
}

func TestTheRouteCallReceivesOnlyWhatThePermissionsHopLeft(t *testing.T) {
	const spent = 250 * time.Millisecond
	identity := (&fakeIdentity{}).answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		time.Sleep(spent)
		return &servicev1.FindEffectivePermissionsResponse{Permissions: []string{queryPermission}}, nil
	})
	short := routeBudget
	short.Limit = 600 * time.Millisecond
	f := edgeOver(t, identity.dial(t, obsclock.System()), withBudget(short))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	route := identity.hopsTo(methodOrganization)
	if len(route) != 1 {
		t.Fatalf("%s calls = %d, want 1", methodOrganization, len(route))
	}
	left := short.Limit - spent - methodSlack
	if got := route[0].remaining; got <= 0 || got > left {
		t.Fatalf("the route call received %v, want at most the %v the permissions hop left (GRP-17)", got, left)
	}
}

func TestWithoutSlackLeftTheRouteCallIs504AndNeverSent(t *testing.T) {
	clock := newDriftingClock()
	identity := (&fakeIdentity{}).answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		clock.advance(routeBudget.Limit - 50*time.Millisecond)
		return &servicev1.FindEffectivePermissionsResponse{Permissions: []string{queryPermission}}, nil
	})
	f := edgeOver(t, identity.dial(t, clock))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, http.StatusGatewayTimeout, "deadline-exceeded")
	if n := len(identity.hopsTo(methodOrganization)); n != 0 {
		t.Fatalf("%s calls = %d, want 0: no slack left for the hop (GRP-17)", methodOrganization, n)
	}
}

var exhaustible = admission.Limit{PerSecond: 0.001, Burst: 1, Concurrency: 10}

type countingVerifier struct {
	inner ports.Authenticator
	calls atomic.Int32
}

func (c *countingVerifier) Authenticate(ctx context.Context, credential ports.Credential) (ports.Identity, error) {
	c.calls.Add(1)
	return c.inner.Authenticate(ctx, credential)
}

func TestAnExhaustedTenantIsRefusedBeforeThePermissionsHop(t *testing.T) {
	identity := (&fakeIdentity{}).grant(queryPermission)
	f := edgeOver(t, identity.dial(t, obsclock.System()), withLimit(exhaustible))

	if rec := f.do(t, http.MethodGet, "/probes/p-1", nil); rec.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	requireAdmissionRefused(t, f.do(t, http.MethodGet, "/probes/p-1", nil))

	if n := len(identity.hopsTo(methodPermissions)); n != 1 {
		t.Fatalf("%s calls = %d, want 1: the refused request never reaches the identity (RES-16)", methodPermissions, n)
	}
}

func TestARequestTheEdgeCannotAttributeToATenantConsumesNoAdmission(t *testing.T) {
	cases := []struct {
		name       string
		credential string
		status     int
		code       string
	}{
		{name: "without a credential", credential: "", status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "with a refused credential", credential: forgedCredential, status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "without a tenant", credential: tenantlessSubject, status: http.StatusForbidden, code: "tenant-unresolved"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := (&fakeIdentity{}).grant(queryPermission)
			f := edgeOver(t, identity.dial(t, obsclock.System()), withLimit(exhaustible))

			for range 2 {
				requireRejection(t, f.do(t, http.MethodGet, "/probes/p-1", nil, "Authorization", tc.credential), tc.status, tc.code)
			}
			if n := len(identity.hopsTo(methodPermissions)); n != 0 {
				t.Fatalf("%s calls = %d, want 0", methodPermissions, n)
			}
		})
	}
}

func TestTheTokenIsVerifiedOncePerRequest(t *testing.T) {
	identity := (&fakeIdentity{}).grant(queryPermission)
	verifier := &countingVerifier{inner: authn.DevAuthenticator{}}
	f := newFixture(t, withAuthenticator(app.NewPermissions(verifier, identity.dial(t, obsclock.System()))), withLimit(exhaustible))

	if rec := f.do(t, http.MethodGet, "/probes/p-1", nil); rec.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	requireAdmissionRefused(t, f.do(t, http.MethodGet, "/probes/p-1", nil))

	if n := verifier.calls.Load(); n != 2 {
		t.Fatalf("verifications = %d, want 2: one per request", n)
	}
}

func TestACancelledPermissionsHopIs499WithoutMarkingTheSpan(t *testing.T) {
	identity := (&fakeIdentity{}).answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		return nil, status.Error(codes.Canceled, "the caller went away")
	})
	f := edgeOver(t, identity.dial(t, obsclock.System()))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, kernelgrpc.StatusClientClosedRequest, "client-closed-request")
	span := serverSpan(t, f, "GET /probes/{id}")
	if _, typed := attributesOf(span)["error.type"]; typed || span.Status.Code == otelcodes.Error {
		t.Fatalf("SERVER status = %v, attributes %v; want no error for a caller that went away", span.Status, span.Attributes)
	}
}

func TestAPermissionsHopTheIdentityRefusesIs500(t *testing.T) {
	for name, code := range map[string]codes.Code{
		"not found": codes.NotFound, "invalid": codes.InvalidArgument,
		"internal": codes.Internal, "unknown": codes.Unknown, "aborted": codes.Aborted,
	} {
		t.Run(name, func(t *testing.T) {
			identity := (&fakeIdentity{}).answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
				return nil, status.Error(code, "refused")
			})
			f := edgeOver(t, identity.dial(t, obsclock.System()))

			rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

			requireRejection(t, rec, http.StatusInternalServerError, "internal-failure")
			if n := f.probe.total(); n != 0 {
				t.Fatalf("handler calls = %d, want 0", n)
			}
			if span := serverSpan(t, f, "GET /probes/{id}"); span.Status.Code != otelcodes.Error {
				t.Fatalf("SERVER status = %v, want Error", span.Status)
			}
		})
	}
}

func TestAPermissionsHopTheIdentityRefusesByAdmissionIs503ToRetryInASecond(t *testing.T) {
	identity := (&fakeIdentity{}).answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		return nil, status.Error(codes.ResourceExhausted, "per-tenant limit")
	})
	f := edgeOver(t, identity.dial(t, obsclock.System()))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, http.StatusServiceUnavailable, "unavailable")
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
	if n := f.probe.total(); n != 0 {
		t.Fatalf("handler calls = %d, want 0", n)
	}
	if span := serverSpan(t, f, "GET /probes/{id}"); span.Status.Code != otelcodes.Error {
		t.Fatalf("SERVER status = %v, want Error like every other 503 of the edge", span.Status)
	}
}

func TestAnUnavailablePermissionsHopCarriesNoRetryAfter(t *testing.T) {
	f := edgeOver(t, identityDown(t))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil)

	requireRejection(t, rec, http.StatusServiceUnavailable, "unavailable")
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Fatalf("Retry-After = %q, want none for an identity nobody knows when returns", got)
	}
}
