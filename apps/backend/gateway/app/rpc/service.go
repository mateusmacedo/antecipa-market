package rpc

import (
	"context"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
)

var descriptor = servicev1.File_company_gateway_service_v1_gateway_service_proto.Services().ByName("GatewayService")

// ServiceName is the full name the generated descriptor gives the service.
var ServiceName = string(descriptor.FullName())

func Methods() []string { return kernelgrpc.MethodNames(descriptor) }

// Commands lists the methods that require an idempotency key (IDM-01); the
// queries neither require nor read one.
func Commands() []string {
	return []string{
		"RegisterConnector", "SuspendOperation", "EnableOperation", "DeclareCapabilityProfile",
		"GrantQueryAuthorization", "RevokeQueryAuthorization",
		"CreateExternalCommand", "CancelExternalCommand", "ResolveUnknownOutcome", "SweepExternalCommands",
	}
}

// FullMethod is the wire name of a method of the service: /<service>/<method>.
func FullMethod(name string) string { return kernelgrpc.FullMethod(ServiceName, name) }

// GatewayServer is the handler type ServiceDesc registers.
type GatewayServer interface {
	RegisterConnector(context.Context, *servicev1.RegisterConnectorRequest) (*servicev1.RegisterConnectorResponse, error)
	SuspendOperation(context.Context, *servicev1.SuspendOperationRequest) (*servicev1.SuspendOperationResponse, error)
	EnableOperation(context.Context, *servicev1.EnableOperationRequest) (*servicev1.EnableOperationResponse, error)
	DeclareCapabilityProfile(context.Context, *servicev1.DeclareCapabilityProfileRequest) (*servicev1.DeclareCapabilityProfileResponse, error)
	GrantQueryAuthorization(context.Context, *servicev1.GrantQueryAuthorizationRequest) (*servicev1.GrantQueryAuthorizationResponse, error)
	RevokeQueryAuthorization(context.Context, *servicev1.RevokeQueryAuthorizationRequest) (*servicev1.RevokeQueryAuthorizationResponse, error)
	CreateExternalCommand(context.Context, *servicev1.CreateExternalCommandRequest) (*servicev1.CreateExternalCommandResponse, error)
	CancelExternalCommand(context.Context, *servicev1.CancelExternalCommandRequest) (*servicev1.CancelExternalCommandResponse, error)
	ResolveUnknownOutcome(context.Context, *servicev1.ResolveUnknownOutcomeRequest) (*servicev1.ResolveUnknownOutcomeResponse, error)
	SweepExternalCommands(context.Context, *servicev1.SweepExternalCommandsRequest) (*servicev1.SweepExternalCommandsResponse, error)
	FindConnector(context.Context, *servicev1.FindConnectorRequest) (*servicev1.FindConnectorResponse, error)
	FindEffectiveCapabilityProfile(context.Context, *servicev1.FindEffectiveCapabilityProfileRequest) (*servicev1.FindEffectiveCapabilityProfileResponse, error)
	FindCapabilityProfile(context.Context, *servicev1.FindCapabilityProfileRequest) (*servicev1.FindCapabilityProfileResponse, error)
	FindQueryAuthorizationByOrganization(context.Context, *servicev1.FindQueryAuthorizationByOrganizationRequest) (*servicev1.FindQueryAuthorizationByOrganizationResponse, error)
	FindExternalCommand(context.Context, *servicev1.FindExternalCommandRequest) (*servicev1.FindExternalCommandResponse, error)
	FindExternalCommandsByBusinessKey(context.Context, *servicev1.FindExternalCommandsByBusinessKeyRequest) (*servicev1.FindExternalCommandsByBusinessKeyResponse, error)
	FindExternalCommandsByStatus(context.Context, *servicev1.FindExternalCommandsByStatusRequest) (*servicev1.FindExternalCommandsByStatusResponse, error)
}

// ServiceDesc binds GatewayService in the app block: protoc-gen-go-grpc would
// put io.network into the contract block, which admits only wire.codec (ADR-015).
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*GatewayServer)(nil),
	Methods: []grpc.MethodDesc{
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterConnector"), GatewayServer.RegisterConnector),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "SuspendOperation"), GatewayServer.SuspendOperation),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "EnableOperation"), GatewayServer.EnableOperation),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "DeclareCapabilityProfile"), GatewayServer.DeclareCapabilityProfile),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "GrantQueryAuthorization"), GatewayServer.GrantQueryAuthorization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RevokeQueryAuthorization"), GatewayServer.RevokeQueryAuthorization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "CreateExternalCommand"), GatewayServer.CreateExternalCommand),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "CancelExternalCommand"), GatewayServer.CancelExternalCommand),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "ResolveUnknownOutcome"), GatewayServer.ResolveUnknownOutcome),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "SweepExternalCommands"), GatewayServer.SweepExternalCommands),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindConnector"), GatewayServer.FindConnector),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindEffectiveCapabilityProfile"), GatewayServer.FindEffectiveCapabilityProfile),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindCapabilityProfile"), GatewayServer.FindCapabilityProfile),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindQueryAuthorizationByOrganization"), GatewayServer.FindQueryAuthorizationByOrganization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindExternalCommand"), GatewayServer.FindExternalCommand),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindExternalCommandsByBusinessKey"), GatewayServer.FindExternalCommandsByBusinessKey),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindExternalCommandsByStatus"), GatewayServer.FindExternalCommandsByStatus),
	},
	Metadata: descriptor.ParentFile().Path(),
}

// Server realizes GatewayServer over the use cases of the context.
type Server struct {
	Service application.Service
}

var _ GatewayServer = Server{}
