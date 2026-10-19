package rpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func identityClient(t *testing.T, fake *fakeIdentity) rpc.Identity {
	t.Helper()
	dialer := fake.serveBuffered(t, healthpb.HealthCheckResponse_SERVING)
	opts := rpc.Options{Insecure: true, Clock: obsclock.System()}
	conn, err := rpc.Dial("passthrough:///identity", rpc.IdentityConfig(opts), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial(identity) = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return rpc.NewIdentity(conn)
}

func withDeadline(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func TestEveryMethodOfTheDescriptorDeclaresAPolicy(t *testing.T) {
	cfg := rpc.IdentityConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})

	names := kernelgrpc.MethodNames(identityDescriptor)
	for _, name := range names {
		if _, err := cfg.Policy(kernelgrpc.FullMethod(rpc.IdentityServiceName, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(cfg.Methods) != len(names) {
		t.Fatalf("declared %d policies for the %d methods of the descriptor", len(cfg.Methods), len(names))
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestThePermissionsConnectionDeclaresOnlyItsMethodUnderItsOwnDependency(t *testing.T) {
	routes := rpc.IdentityConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})
	cfg := rpc.PermissionsConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})
	method := kernelgrpc.FullMethod(rpc.IdentityServiceName, "FindEffectivePermissions")

	policy, err := cfg.Policy(method)
	if err != nil || len(cfg.Methods) != 1 {
		t.Fatalf("Policy(%s) = %v with %d methods, want this one method only (GRP-16)", method, err, len(cfg.Methods))
	}
	if cfg.Sheet.Dependency == routes.Sheet.Dependency || policy.Budget.Dependency != cfg.Sheet.Dependency || policy.Budget.Method != method {
		t.Fatalf("dependency %q, budget %+v, want a dependency of its own so breaker and bulkhead are not the routes' (RES-13)", cfg.Sheet.Dependency, policy.Budget)
	}
	if limit := routes.Methods[method].Budget.Limit; policy.Budget.Limit >= limit || policy.Budget.Slack <= 0 {
		t.Fatalf("budget %+v, want a slack and a limit below the %v of the route methods (GRP-17)", policy.Budget, limit)
	}
	if len(policy.RetryableCodes) != 1 || policy.RetryableCodes[0] != codes.Unavailable || !policy.Idempotent {
		t.Fatalf("policy %+v, want an idempotent method retried only on Unavailable (GRP-08)", policy)
	}
	if cfg.HealthServiceName != rpc.IdentityServiceName || cfg.InsecureForDevelopmentOnly != routes.InsecureForDevelopmentOnly || cfg.TLS != routes.TLS {
		t.Fatalf("HealthServiceName = %q, want the identity's and the transport of the routes", cfg.HealthServiceName)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestEveryMethodRetriesOnlyAnUnavailableIdentity(t *testing.T) {
	cfg := rpc.IdentityConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})

	for method, policy := range cfg.Methods {
		if len(policy.RetryableCodes) != 1 || policy.RetryableCodes[0] != codes.Unavailable {
			t.Errorf("%s: RetryableCodes = %v, want [Unavailable] (GRP-08)", method, policy.RetryableCodes)
		}
		if policy.Budget.Slack <= 0 || policy.Budget.Dependency != "identity" || policy.Budget.Method != method {
			t.Errorf("%s: budget %+v, want the identity dependency, the method and a slack (GRP-17)", method, policy.Budget)
		}
	}
}

func TestTheHealthCheckNamesTheIdentityService(t *testing.T) {
	cfg := rpc.IdentityConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})

	if cfg.HealthServiceName != string(identityDescriptor.FullName()) || rpc.IdentityServiceName != "company.identity.service.v1.IdentityService" {
		t.Fatalf("HealthServiceName = %q, IdentityServiceName = %q", cfg.HealthServiceName, rpc.IdentityServiceName)
	}
}

func TestEachQueryReachesItsMethod(t *testing.T) {
	fake := &fakeIdentity{}
	identity := identityClient(t, fake)
	cases := []struct {
		method string
		call   func(context.Context) error
		want   proto.Message
	}{
		{"FindOrganization", func(ctx context.Context) error {
			_, err := identity.FindOrganization(ctx, &servicev1.FindOrganizationRequest{OrganizationId: "org-1"})
			return err
		}, &servicev1.FindOrganizationRequest{OrganizationId: "org-1"}},
		{"FindOrganizationByTaxId", func(ctx context.Context) error {
			_, err := identity.FindOrganizationByTaxId(ctx, &servicev1.FindOrganizationByTaxIdRequest{TaxId: "11222333000181", Jurisdiction: "BR"})
			return err
		}, &servicev1.FindOrganizationByTaxIdRequest{TaxId: "11222333000181", Jurisdiction: "BR"}},
		{"FindEstablishmentsByOrganization", func(ctx context.Context) error {
			_, err := identity.FindEstablishmentsByOrganization(ctx, &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "org-1"})
			return err
		}, &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: "org-1"}},
		{"FindPerson", func(ctx context.Context) error {
			_, err := identity.FindPerson(ctx, &servicev1.FindPersonRequest{PersonId: "p-1"})
			return err
		}, &servicev1.FindPersonRequest{PersonId: "p-1"}},
		{"FindRepresentation", func(ctx context.Context) error {
			_, err := identity.FindRepresentation(ctx, &servicev1.FindRepresentationRequest{RepresentationId: "r-1"})
			return err
		}, &servicev1.FindRepresentationRequest{RepresentationId: "r-1"}},
		{"FindRepresentationsByOrganization", func(ctx context.Context) error {
			_, err := identity.FindRepresentationsByOrganization(ctx, &servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: "org-1", ValidAt: 7})
			return err
		}, &servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: "org-1", ValidAt: 7}},
		{"FindMembershipsByUser", func(ctx context.Context) error {
			_, err := identity.FindMembershipsByUser(ctx, &servicev1.FindMembershipsByUserRequest{UserId: "u-1"})
			return err
		}, &servicev1.FindMembershipsByUserRequest{UserId: "u-1"}},
		{"FindMembershipsByOrganization", func(ctx context.Context) error {
			_, err := identity.FindMembershipsByOrganization(ctx, &servicev1.FindMembershipsByOrganizationRequest{OrganizationId: "org-1"})
			return err
		}, &servicev1.FindMembershipsByOrganizationRequest{OrganizationId: "org-1"}},
		{"FindEffectivePermissions", func(ctx context.Context) error {
			_, err := identity.FindEffectivePermissions(ctx, &servicev1.FindEffectivePermissionsRequest{UserId: "u-1"})
			return err
		}, &servicev1.FindEffectivePermissionsRequest{UserId: "u-1"}},
		{"FindBankAccountChange", func(ctx context.Context) error {
			_, err := identity.FindBankAccountChange(ctx, &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "b-1"})
			return err
		}, &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: "b-1"}},
		{"FindBankAccountChangesByOrganization", func(ctx context.Context) error {
			_, err := identity.FindBankAccountChangesByOrganization(ctx, &servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: "org-1"})
			return err
		}, &servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: "org-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			if err := tc.call(withDeadline(t, 2*time.Second)); err != nil {
				t.Fatalf("%s() = %v", tc.method, err)
			}
			calls := fake.callsTo(tc.method)
			if len(calls) != 1 || !proto.Equal(calls[0].request, tc.want) {
				t.Fatalf("%s reached the identity with %v, want one call carrying %v", tc.method, calls, tc.want)
			}
		})
	}
}

func TestTheAnswerOfTheIdentityReachesTheCaller(t *testing.T) {
	fake := (&fakeIdentity{}).answer("FindEffectivePermissions", func(int) (proto.Message, error) {
		return &servicev1.FindEffectivePermissionsResponse{Permissions: []string{"identity:findOrganization"}}, nil
	})
	identity := identityClient(t, fake)

	resp, err := identity.FindEffectivePermissions(withDeadline(t, 2*time.Second), &servicev1.FindEffectivePermissionsRequest{UserId: "u-1"})

	if err != nil || len(resp.GetPermissions()) != 1 || resp.GetPermissions()[0] != "identity:findOrganization" {
		t.Fatalf("FindEffectivePermissions() = %v, %v", resp, err)
	}
}

func TestAnUnavailableIdentityIsRetried(t *testing.T) {
	fake := (&fakeIdentity{}).answer("FindOrganization", func(n int) (proto.Message, error) {
		if n == 1 {
			return nil, status.Error(codes.Unavailable, "restarting")
		}
		return &servicev1.FindOrganizationResponse{}, nil
	})
	identity := identityClient(t, fake)

	_, err := identity.FindOrganization(retry.WithBudget(withDeadline(t, 2*time.Second)), &servicev1.FindOrganizationRequest{OrganizationId: "org-1"})

	if err != nil {
		t.Fatalf("FindOrganization() = %v, want success after one retry", err)
	}
	if got := len(fake.callsTo("FindOrganization")); got != 2 {
		t.Fatalf("FindOrganization reached the identity %d times, want 2", got)
	}
}

func TestAnotherFailureOfTheIdentityIsNeverRetried(t *testing.T) {
	for _, code := range []codes.Code{codes.NotFound, codes.Internal, codes.DeadlineExceeded, codes.ResourceExhausted} {
		t.Run(code.String(), func(t *testing.T) {
			fake := (&fakeIdentity{}).answer("FindPerson", func(int) (proto.Message, error) {
				return nil, status.Error(code, "refused")
			})
			identity := identityClient(t, fake)

			_, err := identity.FindPerson(retry.WithBudget(withDeadline(t, 2*time.Second)), &servicev1.FindPersonRequest{PersonId: "p-1"})

			if status.Code(err) != code {
				t.Fatalf("FindPerson() = %v, want %s", err, code)
			}
			if got := len(fake.callsTo("FindPerson")); got != 1 {
				t.Fatalf("FindPerson reached the identity %d times, want 1", got)
			}
		})
	}
}

func TestTheIdentityReceivesADeadlineBelowTheCaller(t *testing.T) {
	fake := &fakeIdentity{}
	identity := identityClient(t, fake)
	ctx := withDeadline(t, 2*time.Second)
	callerDeadline, _ := ctx.Deadline()

	if _, err := identity.FindPerson(ctx, &servicev1.FindPersonRequest{PersonId: "p-1"}); err != nil {
		t.Fatalf("FindPerson() = %v", err)
	}

	calls := fake.callsTo("FindPerson")
	if len(calls) != 1 || !calls[0].hasDeadline {
		t.Fatalf("the identity received no deadline (GRP-04): %+v", calls)
	}
	if gap := callerDeadline.Sub(calls[0].deadline); gap < 50*time.Millisecond {
		t.Fatalf("the identity deadline is %v before the caller's, want the method slack kept (GRP-05, GRP-17)", gap)
	}
}

func TestACallWithoutDeadlineNeverReachesTheWire(t *testing.T) {
	fake := &fakeIdentity{}
	identity := identityClient(t, fake)

	_, err := identity.FindPerson(context.Background(), &servicev1.FindPersonRequest{PersonId: "p-1"})

	if !errors.Is(err, deadline.ErrNoDeadline) || len(fake.callsTo("FindPerson")) != 0 {
		t.Fatalf("FindPerson() = %v with %d calls, want ErrNoDeadline and none (GRP-04)", err, len(fake.callsTo("FindPerson")))
	}
}

func TestAnExhaustedDeadlineNeverReachesTheWire(t *testing.T) {
	fake := &fakeIdentity{}
	identity := identityClient(t, fake)

	_, err := identity.FindPerson(withDeadline(t, 20*time.Millisecond), &servicev1.FindPersonRequest{PersonId: "p-1"})

	if !errors.Is(err, deadline.ErrDeadlineExhausted) || len(fake.callsTo("FindPerson")) != 0 {
		t.Fatalf("FindPerson() = %v with %d calls, want ErrDeadlineExhausted and none (GRP-17)", err, len(fake.callsTo("FindPerson")))
	}
}

func TestAClientWithoutTransportPolicyIsRefused(t *testing.T) {
	_, err := rpc.Dial("127.0.0.1:1", rpc.IdentityConfig(rpc.Options{Clock: obsclock.System()}))

	if !errors.Is(err, kernelgrpc.ErrTLSRequired) {
		t.Fatalf("Dial() = %v, want ErrTLSRequired (GRP-15)", err)
	}
}
