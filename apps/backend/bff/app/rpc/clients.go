package rpc

import (
	"context"
	"crypto/tls"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protoreflect"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	identityDependency    = "identity"
	commandsDependency    = "identity-commands"
	permissionsDependency = "identity-permissions"

	methodLimit     = 1500 * time.Millisecond
	methodSlack     = 100 * time.Millisecond
	methodEstimated = 200 * time.Millisecond

	PermissionsLimit     = 500 * time.Millisecond
	permissionsEstimated = 100 * time.Millisecond
)

var (
	// grpc's default MaxDelay of 120s (grpc backoff/backoff.go DefaultConfig) keeps
	// the channel down long after the identity is back; WithConnectParams also
	// replaces the 20s dial timeout of clientconn.go:61, so it is restated.
	reconnect = grpc.ConnectParams{
		Backoff:           backoff.Config{BaseDelay: backoff.DefaultConfig.BaseDelay, Multiplier: backoff.DefaultConfig.Multiplier, Jitter: backoff.DefaultConfig.Jitter, MaxDelay: 5 * time.Second},
		MinConnectTimeout: 20 * time.Second,
	}

	identityService = servicev1.File_company_identity_service_v1_identity_service_proto.Services().ByName("IdentityService")

	IdentityServiceName = string(identityService.FullName())

	methodFindOrganization                     = fullMethod(identityService, "FindOrganization")
	methodFindOrganizationByTaxId              = fullMethod(identityService, "FindOrganizationByTaxId")
	methodFindEstablishmentsByOrganization     = fullMethod(identityService, "FindEstablishmentsByOrganization")
	methodFindPerson                           = fullMethod(identityService, "FindPerson")
	methodFindRepresentation                   = fullMethod(identityService, "FindRepresentation")
	methodFindRepresentationsByOrganization    = fullMethod(identityService, "FindRepresentationsByOrganization")
	methodFindMembershipsByUser                = fullMethod(identityService, "FindMembershipsByUser")
	methodFindMembershipsByOrganization        = fullMethod(identityService, "FindMembershipsByOrganization")
	methodFindEffectivePermissions             = fullMethod(identityService, "FindEffectivePermissions")
	methodFindBankAccountChange                = fullMethod(identityService, "FindBankAccountChange")
	methodFindBankAccountChangesByOrganization = fullMethod(identityService, "FindBankAccountChangesByOrganization")
	methodRegisterOrganization                 = fullMethod(identityService, "RegisterOrganization")
	methodAddEstablishment                     = fullMethod(identityService, "AddEstablishment")
	methodRegisterPerson                       = fullMethod(identityService, "RegisterPerson")
	methodRegisterRepresentation               = fullMethod(identityService, "RegisterRepresentation")
	methodProvisionUserAccount                 = fullMethod(identityService, "ProvisionUserAccount")
	methodSuspendUser                          = fullMethod(identityService, "SuspendUser")
	methodInviteUser                           = fullMethod(identityService, "InviteUser")
	methodRevokeMembership                     = fullMethod(identityService, "RevokeMembership")
	methodRequestBankAccountChange             = fullMethod(identityService, "RequestBankAccountChange")
	methodApproveBankAccountChange             = fullMethod(identityService, "ApproveBankAccountChange")

	queryMethods = []string{
		methodFindOrganization, methodFindOrganizationByTaxId, methodFindEstablishmentsByOrganization, methodFindPerson,
		methodFindRepresentation, methodFindRepresentationsByOrganization, methodFindMembershipsByUser,
		methodFindMembershipsByOrganization, methodFindEffectivePermissions, methodFindBankAccountChange,
		methodFindBankAccountChangesByOrganization,
	}
	commandMethods = []string{
		methodRegisterOrganization, methodAddEstablishment, methodRegisterPerson, methodRegisterRepresentation,
		methodProvisionUserAccount, methodSuspendUser, methodInviteUser, methodRevokeMembership,
		methodRequestBankAccountChange, methodApproveBankAccountChange,
	}
)

func fullMethod(service protoreflect.ServiceDescriptor, name protoreflect.Name) string {
	return kernelgrpc.FullMethod(string(service.FullName()), string(kernelgrpc.Method(service, name).Name()))
}

type Options struct {
	TLS            *tls.Config
	Insecure       bool
	Clock          clock.Clock
	Tracer         trace.Tracer
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	Propagator     propagation.TextMapPropagator
	Instruments    *metrics.Instruments
	LoggerProvider log.LoggerProvider
}

func IdentityConfig(opts Options) kernelgrpc.Config {
	return dependencyConfig(opts, identityDependency, policies(identityDependency, queryMethods))
}

// The kernel builds one breaker and one bulkhead per Config
// (grpc@v1.0.0-rc.5 interceptor_compose.go:45-46): commands that trip or fill
// their own leave the reads of every other tenant served.
func CommandsConfig(opts Options) kernelgrpc.Config {
	return dependencyConfig(opts, commandsDependency, policies(commandsDependency, commandMethods))
}

func policies(dependency string, methods []string) map[string]kernelgrpc.MethodPolicy {
	declared := make(map[string]kernelgrpc.MethodPolicy, len(methods))
	for _, method := range methods {
		declared[method] = policy(deadline.Budget{Dependency: dependency, Method: method, Limit: methodLimit, Slack: methodSlack, EstimatedDuration: methodEstimated})
	}
	return declared
}

// The kernel builds one breaker and one bulkhead per Config
// (grpc@v1.0.0-rc.5 interceptor_compose.go:45-46): a dependency of its own keeps
// a route that trips or fills the identity's from refusing every authentication.
func PermissionsConfig(opts Options) kernelgrpc.Config {
	return dependencyConfig(opts, permissionsDependency, map[string]kernelgrpc.MethodPolicy{
		methodFindEffectivePermissions: policy(deadline.Budget{
			Dependency:        permissionsDependency,
			Method:            methodFindEffectivePermissions,
			Limit:             PermissionsLimit,
			Slack:             methodSlack,
			EstimatedDuration: permissionsEstimated,
		}),
	})
}

func dependencyConfig(opts Options, dependency string, methods map[string]kernelgrpc.MethodPolicy) kernelgrpc.Config {
	return kernelgrpc.Config{
		TLS:                        opts.TLS,
		InsecureForDevelopmentOnly: opts.Insecure,
		Sheet:                      resilience.Defaults(dependency),
		Methods:                    methods,
		HealthServiceName:          IdentityServiceName,
		Clock:                      opts.Clock,
		Tracer:                     opts.Tracer,
		TracerProvider:             opts.TracerProvider,
		MeterProvider:              opts.MeterProvider,
		Propagator:                 opts.Propagator,
		Instruments:                opts.Instruments,
		LoggerProvider:             opts.LoggerProvider,
	}
}

// Retrying a command on Unavailable is safe because the identity refuses a
// command without its idempotency key and deduplicates by it (GRP-09, IDM-01).
func policy(budget deadline.Budget) kernelgrpc.MethodPolicy {
	return kernelgrpc.MethodPolicy{
		Budget:         budget,
		Idempotent:     true,
		RetryableCodes: []codes.Code{codes.Unavailable},
	}
}

func Dial(target string, cfg kernelgrpc.Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	return dial(target, cfg, reconnect, extra...)
}

func dial(target string, cfg kernelgrpc.Config, params grpc.ConnectParams, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := append([]grpc.DialOption{grpc.WithChainUnaryInterceptor(contextInterceptor), grpc.WithConnectParams(params)}, extra...)
	return kernelgrpc.Dial(target, cfg, options...)
}

type Unary[Req, Resp any] func(context.Context, *Req) (*Resp, error)

type Identity struct {
	FindOrganization                     Unary[servicev1.FindOrganizationRequest, servicev1.FindOrganizationResponse]
	FindOrganizationByTaxId              Unary[servicev1.FindOrganizationByTaxIdRequest, servicev1.FindOrganizationByTaxIdResponse]
	FindEstablishmentsByOrganization     Unary[servicev1.FindEstablishmentsByOrganizationRequest, servicev1.FindEstablishmentsByOrganizationResponse]
	FindPerson                           Unary[servicev1.FindPersonRequest, servicev1.FindPersonResponse]
	FindRepresentation                   Unary[servicev1.FindRepresentationRequest, servicev1.FindRepresentationResponse]
	FindRepresentationsByOrganization    Unary[servicev1.FindRepresentationsByOrganizationRequest, servicev1.FindRepresentationsByOrganizationResponse]
	FindMembershipsByUser                Unary[servicev1.FindMembershipsByUserRequest, servicev1.FindMembershipsByUserResponse]
	FindMembershipsByOrganization        Unary[servicev1.FindMembershipsByOrganizationRequest, servicev1.FindMembershipsByOrganizationResponse]
	FindEffectivePermissions             Unary[servicev1.FindEffectivePermissionsRequest, servicev1.FindEffectivePermissionsResponse]
	FindBankAccountChange                Unary[servicev1.FindBankAccountChangeRequest, servicev1.FindBankAccountChangeResponse]
	FindBankAccountChangesByOrganization Unary[servicev1.FindBankAccountChangesByOrganizationRequest, servicev1.FindBankAccountChangesByOrganizationResponse]
	RegisterOrganization                 Unary[servicev1.RegisterOrganizationRequest, servicev1.RegisterOrganizationResponse]
	AddEstablishment                     Unary[servicev1.AddEstablishmentRequest, servicev1.AddEstablishmentResponse]
	RegisterPerson                       Unary[servicev1.RegisterPersonRequest, servicev1.RegisterPersonResponse]
	RegisterRepresentation               Unary[servicev1.RegisterRepresentationRequest, servicev1.RegisterRepresentationResponse]
	ProvisionUserAccount                 Unary[servicev1.ProvisionUserAccountRequest, servicev1.ProvisionUserAccountResponse]
	SuspendUser                          Unary[servicev1.SuspendUserRequest, servicev1.SuspendUserResponse]
	InviteUser                           Unary[servicev1.InviteUserRequest, servicev1.InviteUserResponse]
	RevokeMembership                     Unary[servicev1.RevokeMembershipRequest, servicev1.RevokeMembershipResponse]
	RequestBankAccountChange             Unary[servicev1.RequestBankAccountChangeRequest, servicev1.RequestBankAccountChangeResponse]
	ApproveBankAccountChange             Unary[servicev1.ApproveBankAccountChangeRequest, servicev1.ApproveBankAccountChangeResponse]
}

func NewIdentity(conn, commands grpc.ClientConnInterface) Identity {
	return Identity{
		FindOrganization:                     unaryOf[servicev1.FindOrganizationRequest, servicev1.FindOrganizationResponse](conn, methodFindOrganization),
		FindOrganizationByTaxId:              unaryOf[servicev1.FindOrganizationByTaxIdRequest, servicev1.FindOrganizationByTaxIdResponse](conn, methodFindOrganizationByTaxId),
		FindEstablishmentsByOrganization:     unaryOf[servicev1.FindEstablishmentsByOrganizationRequest, servicev1.FindEstablishmentsByOrganizationResponse](conn, methodFindEstablishmentsByOrganization),
		FindPerson:                           unaryOf[servicev1.FindPersonRequest, servicev1.FindPersonResponse](conn, methodFindPerson),
		FindRepresentation:                   unaryOf[servicev1.FindRepresentationRequest, servicev1.FindRepresentationResponse](conn, methodFindRepresentation),
		FindRepresentationsByOrganization:    unaryOf[servicev1.FindRepresentationsByOrganizationRequest, servicev1.FindRepresentationsByOrganizationResponse](conn, methodFindRepresentationsByOrganization),
		FindMembershipsByUser:                unaryOf[servicev1.FindMembershipsByUserRequest, servicev1.FindMembershipsByUserResponse](conn, methodFindMembershipsByUser),
		FindMembershipsByOrganization:        unaryOf[servicev1.FindMembershipsByOrganizationRequest, servicev1.FindMembershipsByOrganizationResponse](conn, methodFindMembershipsByOrganization),
		FindEffectivePermissions:             unaryOf[servicev1.FindEffectivePermissionsRequest, servicev1.FindEffectivePermissionsResponse](conn, methodFindEffectivePermissions),
		FindBankAccountChange:                unaryOf[servicev1.FindBankAccountChangeRequest, servicev1.FindBankAccountChangeResponse](conn, methodFindBankAccountChange),
		FindBankAccountChangesByOrganization: unaryOf[servicev1.FindBankAccountChangesByOrganizationRequest, servicev1.FindBankAccountChangesByOrganizationResponse](conn, methodFindBankAccountChangesByOrganization),
		RegisterOrganization:                 commandOf[servicev1.RegisterOrganizationRequest, servicev1.RegisterOrganizationResponse](commands, methodRegisterOrganization),
		AddEstablishment:                     commandOf[servicev1.AddEstablishmentRequest, servicev1.AddEstablishmentResponse](commands, methodAddEstablishment),
		RegisterPerson:                       commandOf[servicev1.RegisterPersonRequest, servicev1.RegisterPersonResponse](commands, methodRegisterPerson),
		RegisterRepresentation:               commandOf[servicev1.RegisterRepresentationRequest, servicev1.RegisterRepresentationResponse](commands, methodRegisterRepresentation),
		ProvisionUserAccount:                 commandOf[servicev1.ProvisionUserAccountRequest, servicev1.ProvisionUserAccountResponse](commands, methodProvisionUserAccount),
		SuspendUser:                          commandOf[servicev1.SuspendUserRequest, servicev1.SuspendUserResponse](commands, methodSuspendUser),
		InviteUser:                           commandOf[servicev1.InviteUserRequest, servicev1.InviteUserResponse](commands, methodInviteUser),
		RevokeMembership:                     commandOf[servicev1.RevokeMembershipRequest, servicev1.RevokeMembershipResponse](commands, methodRevokeMembership),
		RequestBankAccountChange:             commandOf[servicev1.RequestBankAccountChangeRequest, servicev1.RequestBankAccountChangeResponse](commands, methodRequestBankAccountChange),
		ApproveBankAccountChange:             commandOf[servicev1.ApproveBankAccountChangeRequest, servicev1.ApproveBankAccountChangeResponse](commands, methodApproveBankAccountChange),
	}
}

// Armed on the first failure the budget is half of what that attempt left
// (observability@v1.0.0-rc.5 retry/budget.go Arm), and the second attempt got it
// less the estimate and the reserve (resilience/timeout.go:56-61): 0.35 s of 1.4 s.
func commandOf[Req, Resp any](conn grpc.ClientConnInterface, method string) Unary[Req, Resp] {
	call := unaryOf[Req, Resp](conn, method)
	return func(ctx context.Context, req *Req) (*Resp, error) {
		return call(retry.WithBudget(ctx, retry.WithTotal(methodLimit)), req)
	}
}

func unaryOf[Req, Resp any](conn grpc.ClientConnInterface, method string) Unary[Req, Resp] {
	return func(ctx context.Context, req *Req) (*Resp, error) {
		resp := new(Resp)
		var header metadata.MD
		if err := conn.Invoke(ctx, method, req, resp, grpc.Header(&header)); err != nil {
			return nil, err
		}
		markReplayed(ctx, header)
		return resp, nil
	}
}
