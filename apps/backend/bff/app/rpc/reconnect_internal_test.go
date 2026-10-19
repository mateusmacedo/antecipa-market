package rpc

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func TestTheChannelWaitsAtMostFiveSecondsBetweenReconnections(t *testing.T) {
	if got := reconnect.Backoff.MaxDelay; got <= 0 || got > 5*time.Second {
		t.Fatalf("MaxDelay = %v, want at most 5s instead of the 120s of grpc", got)
	}
	if got := reconnect.MinConnectTimeout; got <= 0 {
		t.Fatalf("MinConnectTimeout = %v, want a positive dial timeout: zero bounds each dial by the backoff alone", got)
	}
}

func TestTheChannelIsReadySoonAfterTheIdentityComesBack(t *testing.T) {
	const outage = 1500 * time.Millisecond
	ceiling := grpc.ConnectParams{
		Backoff:           backoff.Config{BaseDelay: 20 * time.Millisecond, Multiplier: 1.6, Jitter: 0.2, MaxDelay: 100 * time.Millisecond},
		MinConnectTimeout: time.Second,
	}
	listener := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	checks := health.NewServer()
	checks.SetServingStatus(IdentityServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, checks)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)

	var up atomic.Bool
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		if !up.Load() {
			return nil, errors.New("connection refused")
		}
		return listener.DialContext(ctx)
	}
	conn, err := dial("passthrough:///identity", IdentityConfig(Options{Insecure: true, Clock: obsclock.System()}), ceiling, grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatalf("dial() = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	conn.Connect()

	time.Sleep(outage)
	up.Store(true)
	back := time.Now()
	for !readyWithin(conn, 50*time.Millisecond) {
		if time.Since(back) > 3*time.Second {
			t.Fatal("the channel never became ready after the identity came back")
		}
	}

	if lag := time.Since(back); lag > 500*time.Millisecond {
		t.Fatalf("ready %v after the identity came back, want within the backoff ceiling of %v", lag, ceiling.Backoff.MaxDelay)
	}
}

func readyWithin(conn *grpc.ClientConn, wait time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if (Readiness{"identity": conn}).Check(ctx) == nil {
		return true
	}
	<-ctx.Done()
	return false
}
