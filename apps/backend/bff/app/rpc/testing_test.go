package rpc_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

var identityDescriptor = servicev1.File_company_identity_service_v1_identity_service_proto.Services().ByName("IdentityService")

type received struct {
	method      string
	request     proto.Message
	md          metadata.MD
	deadline    time.Time
	hasDeadline bool
}

type fakeIdentity struct {
	mu           sync.Mutex
	calls        []received
	answers      map[string]func(n int) (proto.Message, error)
	headers      map[string]func(n int) metadata.MD
	healthChecks atomic.Int32
}

type countingHealth struct {
	*health.Server
	checks *atomic.Int32
}

func (h countingHealth) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	h.checks.Add(1)
	return h.Server.Check(ctx, req)
}

func (f *fakeIdentity) answer(method string, respond func(n int) (proto.Message, error)) *fakeIdentity {
	if f.answers == nil {
		f.answers = map[string]func(int) (proto.Message, error){}
	}
	f.answers[method] = respond
	return f
}

func (f *fakeIdentity) header(method string, header func(n int) metadata.MD) *fakeIdentity {
	if f.headers == nil {
		f.headers = map[string]func(int) metadata.MD{}
	}
	f.headers[method] = header
	return f
}

func (f *fakeIdentity) record(ctx context.Context, method string, request proto.Message) (func(int) (proto.Message, error), func(int) metadata.MD, int) {
	md, _ := metadata.FromIncomingContext(ctx)
	d, ok := ctx.Deadline()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, received{method: method, request: request, md: md.Copy(), deadline: d, hasDeadline: ok})
	n := 0
	for _, c := range f.calls {
		if c.method == method {
			n++
		}
	}
	return f.answers[method], f.headers[method], n
}

func (f *fakeIdentity) callsTo(method string) []received {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []received
	for _, c := range f.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func unary[Req, Resp any, PReq interface {
	*Req
	proto.Message
}, PResp interface {
	*Resp
	proto.Message
}](f *fakeIdentity, name protoreflect.Name) grpc.MethodDesc {
	return kernelgrpc.Unary[any, Req, PReq, proto.Message](rpc.IdentityServiceName, kernelgrpc.Method(identityDescriptor, name),
		func(_ any, ctx context.Context, req PReq) (proto.Message, error) {
			respond, header, n := f.record(ctx, string(name), req)
			if header != nil {
				_ = grpc.SetHeader(ctx, header(n))
			}
			if respond == nil {
				return PResp(new(Resp)), nil
			}
			return respond(n)
		})
}

func (f *fakeIdentity) register(srv *grpc.Server, serving healthpb.HealthCheckResponse_ServingStatus) {
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: rpc.IdentityServiceName,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			unary[servicev1.FindOrganizationRequest, servicev1.FindOrganizationResponse](f, "FindOrganization"),
			unary[servicev1.FindOrganizationByTaxIdRequest, servicev1.FindOrganizationByTaxIdResponse](f, "FindOrganizationByTaxId"),
			unary[servicev1.FindEstablishmentsByOrganizationRequest, servicev1.FindEstablishmentsByOrganizationResponse](f, "FindEstablishmentsByOrganization"),
			unary[servicev1.FindPersonRequest, servicev1.FindPersonResponse](f, "FindPerson"),
			unary[servicev1.FindRepresentationRequest, servicev1.FindRepresentationResponse](f, "FindRepresentation"),
			unary[servicev1.FindRepresentationsByOrganizationRequest, servicev1.FindRepresentationsByOrganizationResponse](f, "FindRepresentationsByOrganization"),
			unary[servicev1.FindMembershipsByUserRequest, servicev1.FindMembershipsByUserResponse](f, "FindMembershipsByUser"),
			unary[servicev1.FindMembershipsByOrganizationRequest, servicev1.FindMembershipsByOrganizationResponse](f, "FindMembershipsByOrganization"),
			unary[servicev1.FindEffectivePermissionsRequest, servicev1.FindEffectivePermissionsResponse](f, "FindEffectivePermissions"),
			unary[servicev1.FindBankAccountChangeRequest, servicev1.FindBankAccountChangeResponse](f, "FindBankAccountChange"),
			unary[servicev1.FindBankAccountChangesByOrganizationRequest, servicev1.FindBankAccountChangesByOrganizationResponse](f, "FindBankAccountChangesByOrganization"),
			unary[servicev1.RegisterOrganizationRequest, servicev1.RegisterOrganizationResponse](f, "RegisterOrganization"),
			unary[servicev1.AddEstablishmentRequest, servicev1.AddEstablishmentResponse](f, "AddEstablishment"),
			unary[servicev1.RegisterPersonRequest, servicev1.RegisterPersonResponse](f, "RegisterPerson"),
			unary[servicev1.RegisterRepresentationRequest, servicev1.RegisterRepresentationResponse](f, "RegisterRepresentation"),
			unary[servicev1.ProvisionUserAccountRequest, servicev1.ProvisionUserAccountResponse](f, "ProvisionUserAccount"),
			unary[servicev1.SuspendUserRequest, servicev1.SuspendUserResponse](f, "SuspendUser"),
			unary[servicev1.InviteUserRequest, servicev1.InviteUserResponse](f, "InviteUser"),
			unary[servicev1.RevokeMembershipRequest, servicev1.RevokeMembershipResponse](f, "RevokeMembership"),
			unary[servicev1.RequestBankAccountChangeRequest, servicev1.RequestBankAccountChangeResponse](f, "RequestBankAccountChange"),
			unary[servicev1.ApproveBankAccountChangeRequest, servicev1.ApproveBankAccountChangeResponse](f, "ApproveBankAccountChange"),
		},
	}, f)
	h := health.NewServer()
	h.SetServingStatus(rpc.IdentityServiceName, serving)
	healthpb.RegisterHealthServer(srv, countingHealth{Server: h, checks: &f.healthChecks})
}

func (f *fakeIdentity) serveBuffered(t *testing.T, serving healthpb.HealthCheckResponse_ServingStatus) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	f.register(srv, serving)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = listener.Close()
	})
	return func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }
}
