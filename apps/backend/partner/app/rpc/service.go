package rpc

import (
	"context"
	"regexp"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const (
	maxCriteria     = 32
	maxProtocols    = 16
	maxBusinessDays = 7

	codeCeiling      = 64
	referenceCeiling = 256
	endpointCeiling  = 2048
	textCeiling      = 512
)

var descriptor = servicev1.File_company_partner_service_v1_partner_service_proto.Services().ByName("PartnerService")

var ServiceName = string(descriptor.FullName())

var (
	idFormat        = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	keyedIDFormat   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	criterionFormat = regexp.MustCompile(`^H[0-9]{1,2}$`)
	printableASCII  = regexp.MustCompile(`^[\x20-\x7E]*$`)
	controlFree     = regexp.MustCompile(`^[^\x00-\x1F\x7F-\x9F]*$`)
)

func Methods() []string {
	names := make([]string, 0, len(ServiceDesc.Methods))
	for _, method := range ServiceDesc.Methods {
		names = append(names, method.MethodName)
	}
	return names
}

// Commands lists the methods that require an idempotency key (IDM-01); the
// queries neither require nor read one.
func Commands() []string {
	return []string{
		"RegisterPartner", "ChangePartnerStage", "RegisterAgreement", "ActivateAgreement",
		"DeclareCapability", "RecordHomologation", "EnableCapability", "SuspendCapability", "ChangeRollout",
		"RegisterIntegrationProfile", "RaiseCertificateExpiryAlert", "DefineOperationalPolicy",
		"DefineProduct", "RetireProduct",
	}
}

func FullMethod(name string) string { return kernelgrpc.FullMethod(ServiceName, name) }

// PartnerServer is the handler type ServiceDesc registers.
type PartnerServer interface {
	RegisterPartner(context.Context, *servicev1.RegisterPartnerRequest) (*servicev1.RegisterPartnerResponse, error)
	ChangePartnerStage(context.Context, *servicev1.ChangePartnerStageRequest) (*servicev1.ChangePartnerStageResponse, error)
	RegisterAgreement(context.Context, *servicev1.RegisterAgreementRequest) (*servicev1.RegisterAgreementResponse, error)
	ActivateAgreement(context.Context, *servicev1.ActivateAgreementRequest) (*servicev1.ActivateAgreementResponse, error)
	DeclareCapability(context.Context, *servicev1.DeclareCapabilityRequest) (*servicev1.DeclareCapabilityResponse, error)
	RecordHomologation(context.Context, *servicev1.RecordHomologationRequest) (*servicev1.RecordHomologationResponse, error)
	EnableCapability(context.Context, *servicev1.EnableCapabilityRequest) (*servicev1.EnableCapabilityResponse, error)
	SuspendCapability(context.Context, *servicev1.SuspendCapabilityRequest) (*servicev1.SuspendCapabilityResponse, error)
	ChangeRollout(context.Context, *servicev1.ChangeRolloutRequest) (*servicev1.ChangeRolloutResponse, error)
	RegisterIntegrationProfile(context.Context, *servicev1.RegisterIntegrationProfileRequest) (*servicev1.RegisterIntegrationProfileResponse, error)
	RaiseCertificateExpiryAlert(context.Context, *servicev1.RaiseCertificateExpiryAlertRequest) (*servicev1.RaiseCertificateExpiryAlertResponse, error)
	DefineOperationalPolicy(context.Context, *servicev1.DefineOperationalPolicyRequest) (*servicev1.DefineOperationalPolicyResponse, error)
	DefineProduct(context.Context, *servicev1.DefineProductRequest) (*servicev1.DefineProductResponse, error)
	RetireProduct(context.Context, *servicev1.RetireProductRequest) (*servicev1.RetireProductResponse, error)
	FindPartner(context.Context, *servicev1.FindPartnerRequest) (*servicev1.FindPartnerResponse, error)
	FindPartnersByOrganization(context.Context, *servicev1.FindPartnersByOrganizationRequest) (*servicev1.FindPartnersByOrganizationResponse, error)
	FindAgreementsByPartner(context.Context, *servicev1.FindAgreementsByPartnerRequest) (*servicev1.FindAgreementsByPartnerResponse, error)
	FindCapabilitiesByPartner(context.Context, *servicev1.FindCapabilitiesByPartnerRequest) (*servicev1.FindCapabilitiesByPartnerResponse, error)
	FindEffectiveCapability(context.Context, *servicev1.FindEffectiveCapabilityRequest) (*servicev1.FindEffectiveCapabilityResponse, error)
	FindEligiblePartners(context.Context, *servicev1.FindEligiblePartnersRequest) (*servicev1.FindEligiblePartnersResponse, error)
	FindProductsByPartner(context.Context, *servicev1.FindProductsByPartnerRequest) (*servicev1.FindProductsByPartnerResponse, error)
	FindIntegrationProfile(context.Context, *servicev1.FindIntegrationProfileRequest) (*servicev1.FindIntegrationProfileResponse, error)
	FindOperationalPolicy(context.Context, *servicev1.FindOperationalPolicyRequest) (*servicev1.FindOperationalPolicyResponse, error)
	FindCertificatesExpiring(context.Context, *servicev1.FindCertificatesExpiringRequest) (*servicev1.FindCertificatesExpiringResponse, error)
}

// ServiceDesc binds the service in the app block: protoc-gen-go-grpc would put
// io.network into the contract block, which admits only wire.codec (ADR-015).
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*PartnerServer)(nil),
	Methods: []grpc.MethodDesc{
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterPartner"), PartnerServer.RegisterPartner),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "ChangePartnerStage"), PartnerServer.ChangePartnerStage),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterAgreement"), PartnerServer.RegisterAgreement),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "ActivateAgreement"), PartnerServer.ActivateAgreement),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "DeclareCapability"), PartnerServer.DeclareCapability),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RecordHomologation"), PartnerServer.RecordHomologation),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "EnableCapability"), PartnerServer.EnableCapability),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "SuspendCapability"), PartnerServer.SuspendCapability),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "ChangeRollout"), PartnerServer.ChangeRollout),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterIntegrationProfile"), PartnerServer.RegisterIntegrationProfile),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RaiseCertificateExpiryAlert"), PartnerServer.RaiseCertificateExpiryAlert),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "DefineOperationalPolicy"), PartnerServer.DefineOperationalPolicy),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "DefineProduct"), PartnerServer.DefineProduct),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RetireProduct"), PartnerServer.RetireProduct),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindPartner"), PartnerServer.FindPartner),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindPartnersByOrganization"), PartnerServer.FindPartnersByOrganization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindAgreementsByPartner"), PartnerServer.FindAgreementsByPartner),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindCapabilitiesByPartner"), PartnerServer.FindCapabilitiesByPartner),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindEffectiveCapability"), PartnerServer.FindEffectiveCapability),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindEligiblePartners"), PartnerServer.FindEligiblePartners),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindProductsByPartner"), PartnerServer.FindProductsByPartner),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindIntegrationProfile"), PartnerServer.FindIntegrationProfile),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindOperationalPolicy"), PartnerServer.FindOperationalPolicy),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindCertificatesExpiring"), PartnerServer.FindCertificatesExpiring),
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: descriptor.ParentFile().Path(),
}

// Server realizes PartnerServer over the use cases of the context.
type Server struct {
	Service application.Service
}

func (s Server) RegisterPartner(ctx context.Context, req *servicev1.RegisterPartnerRequest) (*servicev1.RegisterPartnerResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	organization, err := identifier("organization_id", req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	role, err := known(rolesOnTheWire, req.GetRole(), "role must be one of the six partner roles")
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RegisterPartnerResponse], error) {
			return s.Service.RegisterPartner(ctx, application.RegisterPartner{
				PartnerID:      domain.PartnerID(partner),
				OrganizationID: domain.OrganizationID(organization),
				Role:           role,
			})
		},
		func(r *kernel.Rejection) *servicev1.RegisterPartnerResponse {
			return &servicev1.RegisterPartnerResponse{Result: &servicev1.RegisterPartnerResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(registered domain.RegisterPartnerResponse) *servicev1.RegisterPartnerResponse {
			return &servicev1.RegisterPartnerResponse{Result: &servicev1.RegisterPartnerResponse_Registered{
				Registered: &servicev1.RegisteredPartner{PartnerId: string(registered.PartnerID)},
			}}
		})
}

func (s Server) RegisterAgreement(ctx context.Context, req *servicev1.RegisterAgreementRequest) (*servicev1.RegisterAgreementResponse, error) {
	agreement, err := identifier("agreement_id", req.GetAgreementId())
	if err != nil {
		return nil, err
	}
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	template, err := optionalIdentifier("instrument_template_id", req.GetInstrumentTemplateId())
	if err != nil {
		return nil, err
	}
	supersedes, err := optionalIdentifier("supersedes_id", req.GetSupersedesId())
	if err != nil {
		return nil, err
	}
	if req.GetValidFrom() < 0 || req.GetValidUntil() < 0 {
		return nil, status.Error(codes.InvalidArgument, "valid_from and valid_until must be instants in nanoseconds, never negative")
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RegisterAgreementResponse], error) {
			return s.Service.RegisterAgreement(ctx, application.RegisterAgreement{
				AgreementID:          domain.AgreementID(agreement),
				PartnerID:            domain.PartnerID(partner),
				ValidFrom:            kernel.Instant(req.GetValidFrom()),
				ValidUntil:           kernel.Instant(req.GetValidUntil()),
				InstrumentTemplateID: template,
				SupersedesID:         domain.AgreementID(supersedes),
			})
		},
		func(r *kernel.Rejection) *servicev1.RegisterAgreementResponse {
			return &servicev1.RegisterAgreementResponse{Result: &servicev1.RegisterAgreementResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(registered domain.RegisterAgreementResponse) *servicev1.RegisterAgreementResponse {
			return &servicev1.RegisterAgreementResponse{Result: &servicev1.RegisterAgreementResponse_Registered{
				Registered: &servicev1.RegisteredAgreement{AgreementId: string(registered.AgreementID)},
			}}
		})
}

func (s Server) ActivateAgreement(ctx context.Context, req *servicev1.ActivateAgreementRequest) (*servicev1.ActivateAgreementResponse, error) {
	agreement, err := identifier("agreement_id", req.GetAgreementId())
	if err != nil {
		return nil, err
	}
	evidence, err := optionalIdentifier("contract_evidence_id", req.GetContractEvidenceId())
	if err != nil {
		return nil, err
	}
	if err := criteriaWithinCeilings(req.GetSatisfiedCriteria()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ActivateAgreementResponse], error) {
			return s.Service.ActivateAgreement(ctx, application.ActivateAgreement{
				AgreementID:               domain.AgreementID(agreement),
				ContractEvidenceID:        evidence,
				SatisfiedCriteria:         req.GetSatisfiedCriteria(),
				RegulatoryCheckApplicable: req.GetRegulatoryCheckApplicable(),
			})
		},
		func(r *kernel.Rejection) *servicev1.ActivateAgreementResponse {
			return &servicev1.ActivateAgreementResponse{Result: &servicev1.ActivateAgreementResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(activated domain.ActivateAgreementResponse) *servicev1.ActivateAgreementResponse {
			return &servicev1.ActivateAgreementResponse{Result: &servicev1.ActivateAgreementResponse_Activated{
				Activated: &servicev1.ActivatedAgreement{AgreementId: string(activated.AgreementID)},
			}}
		})
}

func rejectionOf(r *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(r.Code()), Message: r.Message()}
}

func identifier(field, raw string) (string, error) {
	if !idFormat.MatchString(raw) {
		return "", status.Errorf(codes.InvalidArgument, "%s must have 1 to 128 characters of [A-Za-z0-9._:-]", field)
	}
	return raw, nil
}

func keyedIdentifier(field, raw string) (string, error) {
	if !keyedIDFormat.MatchString(raw) {
		return "", status.Errorf(codes.InvalidArgument, "%s must have 1 to 100 characters of [A-Za-z0-9._-]", field)
	}
	return raw, nil
}

func optionalIdentifier(field, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	return identifier(field, raw)
}

func criteriaWithinCeilings(criteria []string) error {
	if len(criteria) > maxCriteria {
		return status.Error(codes.InvalidArgument, "satisfied_criteria must hold at most 32 items")
	}
	for _, criterion := range criteria {
		if !criterionFormat.MatchString(criterion) {
			return status.Error(codes.InvalidArgument, "satisfied_criteria must hold criterion ids such as H14")
		}
	}
	return nil
}

var rolesOnTheWire = map[servicev1.PartnerRole]domain.PartnerRole{
	servicev1.PartnerRole_PARTNER_ROLE_FUNDER:                 domain.RoleFunder,
	servicev1.PartnerRole_PARTNER_ROLE_BOOKKEEPER:             domain.RoleBookkeeper,
	servicev1.PartnerRole_PARTNER_ROLE_REGISTRY:               domain.RoleRegistry,
	servicev1.PartnerRole_PARTNER_ROLE_DEPOSITORY:             domain.RoleDepository,
	servicev1.PartnerRole_PARTNER_ROLE_DATA_PROVIDER:          domain.RoleDataProvider,
	servicev1.PartnerRole_PARTNER_ROLE_SETTLEMENT_INSTITUTION: domain.RoleSettlementInstitution,
}

func known[W comparable, D any](catalogue map[W]D, raw W, refusal string) (D, error) {
	value, ok := catalogue[raw]
	if !ok {
		var none D
		return none, status.Error(codes.InvalidArgument, refusal)
	}
	return value, nil
}
