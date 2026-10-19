package app_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

type permissionsAnswer func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error)

type permissionsAnswerTo func(ctx context.Context, userID string) (*servicev1.FindEffectivePermissionsResponse, error)

type permissionsAsked struct {
	userID   string
	md       metadata.MD
	deadline time.Time
}

type fakeIdentity struct {
	addr   string
	server *grpc.Server
	open   atomic.Int32

	mu       sync.Mutex
	answer   permissionsAnswer
	answerTo permissionsAnswerTo
	asked    []permissionsAsked
}

func (f *fakeIdentity) answering(answer permissionsAnswer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answer = answer
}

func (f *fakeIdentity) answeringTo(answer permissionsAnswerTo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answerTo = answer
}

func (f *fakeIdentity) askedFor(userID string) int {
	n := 0
	for _, asked := range f.questions() {
		if asked.userID == userID {
			n++
		}
	}
	return n
}

func (f *fakeIdentity) grant(permissions ...string) {
	f.answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		return &servicev1.FindEffectivePermissionsResponse{Permissions: permissions}, nil
	})
}

func (f *fakeIdentity) questions() []permissionsAsked {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]permissionsAsked(nil), f.asked...)
}

func (f *fakeIdentity) findEffectivePermissions(_ any, ctx context.Context, req *servicev1.FindEffectivePermissionsRequest) (*servicev1.FindEffectivePermissionsResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	deadline, _ := ctx.Deadline()
	f.mu.Lock()
	f.asked = append(f.asked, permissionsAsked{userID: req.GetUserId(), md: md.Copy(), deadline: deadline})
	answer, answerTo := f.answer, f.answerTo
	f.mu.Unlock()
	if answerTo != nil {
		return answerTo(ctx, req.GetUserId())
	}
	if answer == nil {
		return &servicev1.FindEffectivePermissionsResponse{}, nil
	}
	return answer(ctx)
}

func serveIdentity(t *testing.T, options ...grpc.ServerOption) *fakeIdentity {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	f := &fakeIdentity{addr: listener.Addr().String(), server: grpc.NewServer(options...)}
	checks := health.NewServer()
	checks.SetServingStatus(rpc.IdentityServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(f.server, checks)
	service := servicev1.File_company_identity_service_v1_identity_service_proto.Services().ByName("IdentityService")
	f.server.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.IdentityServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			kernelgrpc.Unary[any, servicev1.FindEffectivePermissionsRequest, *servicev1.FindEffectivePermissionsRequest, *servicev1.FindEffectivePermissionsResponse](
				rpc.IdentityServiceName, kernelgrpc.Method(service, "FindEffectivePermissions"), f.findEffectivePermissions),
		},
	}, f)
	go func() { _ = f.server.Serve(trackingListener{Listener: listener, open: &f.open}) }()
	t.Cleanup(f.server.Stop)
	return f
}

func serveIdentityOverTLS(t *testing.T, caFile, certFile, keyFile string) *fakeIdentity {
	t.Helper()
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("LoadX509KeyPair() = %v", err)
	}
	authority, err := os.ReadFile(caFile)
	if err != nil {
		t.Fatalf("ReadFile(%s) = %v", caFile, err)
	}
	clients := x509.NewCertPool()
	if !clients.AppendCertsFromPEM(authority) {
		t.Fatalf("%s holds no PEM certificate", caFile)
	}
	return serveIdentity(t, grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{pair},
		ClientCAs:    clients,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	})))
}

type trackingListener struct {
	net.Listener
	open *atomic.Int32
}

func (l trackingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	l.open.Add(1)
	return &trackedConn{Conn: conn, open: l.open}, nil
}

type trackedConn struct {
	net.Conn
	open   *atomic.Int32
	closed sync.Once
}

func (c *trackedConn) Close() error {
	c.closed.Do(func() { c.open.Add(-1) })
	return c.Conn.Close()
}
