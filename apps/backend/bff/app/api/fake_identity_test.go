package api_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	methodPermissions                      = "FindEffectivePermissions"
	methodOrganization                     = "FindOrganization"
	methodPerson                           = "FindPerson"
	methodRepresentation                   = "FindRepresentation"
	methodBankAccountChange                = "FindBankAccountChange"
	methodEstablishmentsByOrganization     = "FindEstablishmentsByOrganization"
	methodRepresentationsByOrganization    = "FindRepresentationsByOrganization"
	methodMembershipsByOrganization        = "FindMembershipsByOrganization"
	methodBankAccountChangesByOrganization = "FindBankAccountChangesByOrganization"
	methodMembershipsByUser                = "FindMembershipsByUser"
	methodRequestBankAccountChange         = "RequestBankAccountChange"
	methodRegisterOrganization             = "RegisterOrganization"
	methodAddEstablishment                 = "AddEstablishment"
	methodRegisterPerson                   = "RegisterPerson"
	methodProvisionUserAccount             = "ProvisionUserAccount"
	methodRegisterRepresentation           = "RegisterRepresentation"
	methodInviteUser                       = "InviteUser"
	methodSuspendUser                      = "SuspendUser"
	methodRevokeMembership                 = "RevokeMembership"
	methodApproveBankAccountChange         = "ApproveBankAccountChange"
	methodOrganizationByTaxID              = "FindOrganizationByTaxId"
)

type hop struct {
	md        metadata.MD
	remaining time.Duration
	req       any
}

type reply func(req any) (any, error)

type permissionsAnswer func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error)

type fakeIdentity struct {
	mu      sync.Mutex
	answer  permissionsAnswer
	byUser  map[string][]string
	replies map[string]reply
	headers map[string]metadata.MD
	hops    map[string][]hop
}

func (f *fakeIdentity) grantTo(user string, permissions ...string) *fakeIdentity {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byUser == nil {
		f.byUser = map[string][]string{}
	}
	f.byUser[user] = permissions
	return f
}

func (f *fakeIdentity) replying(method string, answer reply) *fakeIdentity {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replies == nil {
		f.replies = map[string]reply{}
	}
	f.replies[method] = answer
	return f
}

func (f *fakeIdentity) sendingHeader(method string, header metadata.MD) *fakeIdentity {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.headers == nil {
		f.headers = map[string]metadata.MD{}
	}
	f.headers[method] = header
	return f
}

func (f *fakeIdentity) answering(answer permissionsAnswer) *fakeIdentity {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answer = answer
	return f
}

func (f *fakeIdentity) grant(permissions ...string) *fakeIdentity {
	return f.answering(func(context.Context) (*servicev1.FindEffectivePermissionsResponse, error) {
		return &servicev1.FindEffectivePermissionsResponse{Permissions: permissions}, nil
	})
}

func (f *fakeIdentity) record(ctx context.Context, method string, req any) permissionsAnswer {
	md, _ := metadata.FromIncomingContext(ctx)
	d, _ := ctx.Deadline()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hops == nil {
		f.hops = map[string][]hop{}
	}
	f.hops[method] = append(f.hops[method], hop{md: md.Copy(), remaining: time.Until(d), req: req})
	return f.answer
}

func (f *fakeIdentity) hopsTo(method string) []hop {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]hop(nil), f.hops[method]...)
}

func (f *fakeIdentity) findEffectivePermissions(_ any, ctx context.Context, req *servicev1.FindEffectivePermissionsRequest) (*servicev1.FindEffectivePermissionsResponse, error) {
	answer := f.record(ctx, methodPermissions, req)
	f.mu.Lock()
	granted, own := f.byUser[req.GetUserId()]
	f.mu.Unlock()
	if own {
		return &servicev1.FindEffectivePermissionsResponse{Permissions: granted}, nil
	}
	if answer != nil {
		return answer(ctx)
	}
	return &servicev1.FindEffectivePermissionsResponse{}, nil
}

func serving[Req, Resp any, PReq interface {
	*Req
	protoreflect.ProtoMessage
}, PResp interface {
	*Resp
	protoreflect.ProtoMessage
}](f *fakeIdentity, service protoreflect.ServiceDescriptor, method string) grpc.MethodDesc {
	return kernelgrpc.Unary[any, Req, PReq, PResp](rpc.IdentityServiceName, kernelgrpc.Method(service, protoreflect.Name(method)),
		func(_ any, ctx context.Context, req PReq) (PResp, error) {
			f.record(ctx, method, req)
			f.mu.Lock()
			answer, header := f.replies[method], f.headers[method]
			f.mu.Unlock()
			if header != nil {
				_ = grpc.SetHeader(ctx, header)
			}
			if answer == nil {
				return PResp(new(Resp)), nil
			}
			resp, err := answer(req)
			if err != nil {
				return nil, err
			}
			return resp.(PResp), nil
		})
}

func (f *fakeIdentity) dial(t *testing.T, clock obsclock.Clock) rpc.Identity {
	t.Helper()
	service := servicev1.File_company_identity_service_v1_identity_service_proto.Services().ByName("IdentityService")
	listener := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.IdentityServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			kernelgrpc.Unary[any, servicev1.FindEffectivePermissionsRequest, *servicev1.FindEffectivePermissionsRequest, *servicev1.FindEffectivePermissionsResponse](
				rpc.IdentityServiceName, kernelgrpc.Method(service, methodPermissions), f.findEffectivePermissions),
			serving[servicev1.FindOrganizationRequest, servicev1.FindOrganizationResponse](f, service, methodOrganization),
			serving[servicev1.FindPersonRequest, servicev1.FindPersonResponse](f, service, methodPerson),
			serving[servicev1.FindRepresentationRequest, servicev1.FindRepresentationResponse](f, service, methodRepresentation),
			serving[servicev1.FindBankAccountChangeRequest, servicev1.FindBankAccountChangeResponse](f, service, methodBankAccountChange),
			serving[servicev1.FindEstablishmentsByOrganizationRequest, servicev1.FindEstablishmentsByOrganizationResponse](f, service, methodEstablishmentsByOrganization),
			serving[servicev1.FindRepresentationsByOrganizationRequest, servicev1.FindRepresentationsByOrganizationResponse](f, service, methodRepresentationsByOrganization),
			serving[servicev1.FindMembershipsByOrganizationRequest, servicev1.FindMembershipsByOrganizationResponse](f, service, methodMembershipsByOrganization),
			serving[servicev1.FindBankAccountChangesByOrganizationRequest, servicev1.FindBankAccountChangesByOrganizationResponse](f, service, methodBankAccountChangesByOrganization),
			serving[servicev1.FindMembershipsByUserRequest, servicev1.FindMembershipsByUserResponse](f, service, methodMembershipsByUser),
			serving[servicev1.RequestBankAccountChangeRequest, servicev1.RequestBankAccountChangeResponse](f, service, methodRequestBankAccountChange),
			serving[servicev1.RegisterOrganizationRequest, servicev1.RegisterOrganizationResponse](f, service, methodRegisterOrganization),
			serving[servicev1.AddEstablishmentRequest, servicev1.AddEstablishmentResponse](f, service, methodAddEstablishment),
			serving[servicev1.RegisterPersonRequest, servicev1.RegisterPersonResponse](f, service, methodRegisterPerson),
			serving[servicev1.ProvisionUserAccountRequest, servicev1.ProvisionUserAccountResponse](f, service, methodProvisionUserAccount),
			serving[servicev1.RegisterRepresentationRequest, servicev1.RegisterRepresentationResponse](f, service, methodRegisterRepresentation),
			serving[servicev1.InviteUserRequest, servicev1.InviteUserResponse](f, service, methodInviteUser),
			serving[servicev1.SuspendUserRequest, servicev1.SuspendUserResponse](f, service, methodSuspendUser),
			serving[servicev1.RevokeMembershipRequest, servicev1.RevokeMembershipResponse](f, service, methodRevokeMembership),
			serving[servicev1.ApproveBankAccountChangeRequest, servicev1.ApproveBankAccountChangeResponse](f, service, methodApproveBankAccountChange),
			serving[servicev1.FindOrganizationByTaxIdRequest, servicev1.FindOrganizationByTaxIdResponse](f, service, methodOrganizationByTaxID),
		},
	}, f)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})
	dialer := func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }
	return dialIdentity(t, "passthrough:///identity", clock, grpc.WithContextDialer(dialer))
}

func identityDown(t *testing.T) rpc.Identity {
	t.Helper()
	return dialIdentity(t, obstest.UnusedLoopbackAddr(t), obsclock.System())
}

func dialIdentity(t *testing.T, target string, clock obsclock.Clock, extra ...grpc.DialOption) rpc.Identity {
	t.Helper()
	opts := rpc.Options{Insecure: true, Clock: clock}
	conns := make([]*grpc.ClientConn, 0, 2)
	for _, cfg := range []kernelgrpc.Config{rpc.IdentityConfig(opts), rpc.CommandsConfig(opts)} {
		conn, err := rpc.Dial(target, cfg, extra...)
		if err != nil {
			t.Fatalf("Dial(%s) = %v", target, err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		conns = append(conns, conn)
	}
	return rpc.NewIdentity(conns[0], conns[1])
}

// The deadline interceptor derives the outgoing deadline from Now alone
// (grpc@v1.0.0-rc.5 interceptor_deadline.go:55-59); timers stay real.
type driftingClock struct {
	obsclock.Clock
	ahead atomic.Int64
}

func newDriftingClock() *driftingClock { return &driftingClock{Clock: obsclock.System()} }

func (c *driftingClock) Now() time.Time { return c.Clock.Now().Add(time.Duration(c.ahead.Load())) }

func (c *driftingClock) advance(d time.Duration) { c.ahead.Add(int64(d)) }
