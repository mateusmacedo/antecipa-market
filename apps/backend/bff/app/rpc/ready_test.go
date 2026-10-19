package rpc_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func identityChannel(t *testing.T, dialer func(context.Context, string) (net.Conn, error)) *grpc.ClientConn {
	t.Helper()
	opts := rpc.Options{Insecure: true, Clock: obsclock.System()}
	conn, err := rpc.Dial("passthrough:///identity", rpc.IdentityConfig(opts), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("Dial(identity) = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func refused(context.Context, string) (net.Conn, error) { return nil, errors.New("connection refused") }

func TestReadinessPassesOnceTheIdentityAnswersServing(t *testing.T) {
	fake := &fakeIdentity{}
	conn := identityChannel(t, fake.serveBuffered(t, healthpb.HealthCheckResponse_SERVING))

	if err := (rpc.Readiness{"identity": conn}).Check(withDeadline(t, 5*time.Second)); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
	if _, err := rpc.NewIdentity(conn).FindPerson(withDeadline(t, 2*time.Second), &servicev1.FindPersonRequest{PersonId: "p-1"}); err != nil {
		t.Fatalf("FindPerson() over the checked channel = %v", err)
	}
	if got := fake.healthChecks.Load(); got != 0 {
		t.Fatalf("Health/Check called %d times, want the service config health check only (GRP-13, GRP-16)", got)
	}
}

func TestReadinessFailsBeforeTheProbeEnds(t *testing.T) {
	cases := map[string]func(t *testing.T) func(context.Context, string) (net.Conn, error){
		"not serving": func(t *testing.T) func(context.Context, string) (net.Conn, error) {
			return (&fakeIdentity{}).serveBuffered(t, healthpb.HealthCheckResponse_NOT_SERVING)
		},
		"no server": func(*testing.T) func(context.Context, string) (net.Conn, error) { return refused },
	}
	for name, dialer := range cases {
		t.Run(name, func(t *testing.T) {
			conn := identityChannel(t, dialer(t))
			ctx := withDeadline(t, 5*time.Second)

			err := (rpc.Readiness{"identity": conn}).Check(ctx)

			if !errors.Is(err, rpc.ErrNotReady) || !strings.HasSuffix(err.Error(), ": identity") {
				t.Fatalf("Check() = %v, want ErrNotReady naming the identity", err)
			}
			if ctx.Err() != nil {
				t.Fatal("Check() answered only when the probe ended, want the failure as soon as the channel reports it")
			}
		})
	}
}

func TestReadinessFailsOnAClosedChannel(t *testing.T) {
	conn := identityChannel(t, (&fakeIdentity{}).serveBuffered(t, healthpb.HealthCheckResponse_SERVING))
	_ = conn.Close()
	ctx := withDeadline(t, 5*time.Second)

	if err := (rpc.Readiness{"identity": conn}).Check(ctx); !errors.Is(err, rpc.ErrNotReady) || ctx.Err() != nil {
		t.Fatalf("Check() = %v, want ErrNotReady before the probe ends", err)
	}
}
