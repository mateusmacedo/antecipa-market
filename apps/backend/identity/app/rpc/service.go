package rpc

import (
	"context"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

// The ceilings bound the form only: a value within them still reaches the
// domain, which owns the refusals of the spec (jurisdiction-invalid and kin).
const (
	maxTaxIDBytes        = 32
	maxJurisdictionBytes = 8
	maxNameRunes         = 256
	maxReferenceRunes    = 256
	maxActorBytes        = 256
	maxCurrencyBytes     = 8
	maxActTypeBytes      = 64
	maxListItems         = 32
	maxEmailRunes        = 254
	maxReasonRunes       = 512
)

var descriptor = servicev1.File_company_identity_service_v1_identity_service_proto.Services().ByName("IdentityService")

var ServiceName = string(descriptor.FullName())

var (
	idFormat           = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	jurisdictionFormat = regexp.MustCompile(`^[A-Z]{2}$`)
	printableFormat    = regexp.MustCompile(`^[\x20-\x7E]*$`)
)

// controlled finds a control character in free text: the jsonb of the snapshot
// refuses U+0000 (SQLSTATE 22P05), which would answer Internal and leave part
// of the row in the database log.
func controlled(texts ...string) bool {
	for _, text := range texts {
		if strings.ContainsFunc(text, unicode.IsControl) {
			return true
		}
	}
	return false
}

func Methods() []string { return kernelgrpc.MethodNames(descriptor) }

// Commands lists the methods that require an idempotency key, since every
// command must carry one; the queries neither require nor read one.
func Commands() []string {
	return []string{
		"RegisterOrganization", "AddEstablishment", "RegisterPerson", "RegisterRepresentation",
		"ProvisionUserAccount", "SuspendUser", "InviteUser", "GrantMembership", "RevokeMembership",
		"RequestBankAccountChange", "ApproveBankAccountChange",
	}
}

func FullMethod(name string) string { return kernelgrpc.FullMethod(ServiceName, name) }

// IdentityServer is the handler type ServiceDesc registers.
type IdentityServer interface {
	RegisterOrganization(context.Context, *servicev1.RegisterOrganizationRequest) (*servicev1.RegisterOrganizationResponse, error)
	FindOrganization(context.Context, *servicev1.FindOrganizationRequest) (*servicev1.FindOrganizationResponse, error)
	FindOrganizationByTaxId(context.Context, *servicev1.FindOrganizationByTaxIdRequest) (*servicev1.FindOrganizationByTaxIdResponse, error)
	AddEstablishment(context.Context, *servicev1.AddEstablishmentRequest) (*servicev1.AddEstablishmentResponse, error)
	RegisterPerson(context.Context, *servicev1.RegisterPersonRequest) (*servicev1.RegisterPersonResponse, error)
	RegisterRepresentation(context.Context, *servicev1.RegisterRepresentationRequest) (*servicev1.RegisterRepresentationResponse, error)
	FindEstablishmentsByOrganization(context.Context, *servicev1.FindEstablishmentsByOrganizationRequest) (*servicev1.FindEstablishmentsByOrganizationResponse, error)
	FindPerson(context.Context, *servicev1.FindPersonRequest) (*servicev1.FindPersonResponse, error)
	FindRepresentation(context.Context, *servicev1.FindRepresentationRequest) (*servicev1.FindRepresentationResponse, error)
	FindRepresentationsByOrganization(context.Context, *servicev1.FindRepresentationsByOrganizationRequest) (*servicev1.FindRepresentationsByOrganizationResponse, error)
	ProvisionUserAccount(context.Context, *servicev1.ProvisionUserAccountRequest) (*servicev1.ProvisionUserAccountResponse, error)
	SuspendUser(context.Context, *servicev1.SuspendUserRequest) (*servicev1.SuspendUserResponse, error)
	InviteUser(context.Context, *servicev1.InviteUserRequest) (*servicev1.InviteUserResponse, error)
	GrantMembership(context.Context, *servicev1.GrantMembershipRequest) (*servicev1.GrantMembershipResponse, error)
	RevokeMembership(context.Context, *servicev1.RevokeMembershipRequest) (*servicev1.RevokeMembershipResponse, error)
	FindMembershipsByUser(context.Context, *servicev1.FindMembershipsByUserRequest) (*servicev1.FindMembershipsByUserResponse, error)
	FindMembershipsByOrganization(context.Context, *servicev1.FindMembershipsByOrganizationRequest) (*servicev1.FindMembershipsByOrganizationResponse, error)
	FindEffectivePermissions(context.Context, *servicev1.FindEffectivePermissionsRequest) (*servicev1.FindEffectivePermissionsResponse, error)
	RequestBankAccountChange(context.Context, *servicev1.RequestBankAccountChangeRequest) (*servicev1.RequestBankAccountChangeResponse, error)
	ApproveBankAccountChange(context.Context, *servicev1.ApproveBankAccountChangeRequest) (*servicev1.ApproveBankAccountChangeResponse, error)
	FindBankAccountChange(context.Context, *servicev1.FindBankAccountChangeRequest) (*servicev1.FindBankAccountChangeResponse, error)
	FindBankAccountChangesByOrganization(context.Context, *servicev1.FindBankAccountChangesByOrganizationRequest) (*servicev1.FindBankAccountChangesByOrganizationResponse, error)
}

// ServiceDesc binds IdentityService in the app block: protoc-gen-go-grpc would
// put network access into the contract block, whose declared external
// capabilities admit only the wire codec.
var ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*IdentityServer)(nil),
	Methods: []grpc.MethodDesc{
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterOrganization"), IdentityServer.RegisterOrganization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindOrganization"), IdentityServer.FindOrganization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindOrganizationByTaxId"), IdentityServer.FindOrganizationByTaxId),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "AddEstablishment"), IdentityServer.AddEstablishment),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterPerson"), IdentityServer.RegisterPerson),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RegisterRepresentation"), IdentityServer.RegisterRepresentation),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindEstablishmentsByOrganization"), IdentityServer.FindEstablishmentsByOrganization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindPerson"), IdentityServer.FindPerson),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindRepresentation"), IdentityServer.FindRepresentation),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindRepresentationsByOrganization"), IdentityServer.FindRepresentationsByOrganization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "ProvisionUserAccount"), IdentityServer.ProvisionUserAccount),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "SuspendUser"), IdentityServer.SuspendUser),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "InviteUser"), IdentityServer.InviteUser),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "GrantMembership"), IdentityServer.GrantMembership),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RevokeMembership"), IdentityServer.RevokeMembership),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindMembershipsByUser"), IdentityServer.FindMembershipsByUser),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindMembershipsByOrganization"), IdentityServer.FindMembershipsByOrganization),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindEffectivePermissions"), IdentityServer.FindEffectivePermissions),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "RequestBankAccountChange"), IdentityServer.RequestBankAccountChange),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "ApproveBankAccountChange"), IdentityServer.ApproveBankAccountChange),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindBankAccountChange"), IdentityServer.FindBankAccountChange),
		kernelgrpc.Unary(ServiceName, kernelgrpc.Method(descriptor, "FindBankAccountChangesByOrganization"), IdentityServer.FindBankAccountChangesByOrganization),
	},
	Metadata: descriptor.ParentFile().Path(),
}

// Server realizes IdentityServer over the use cases of the context.
type Server struct {
	Service application.Service
}

func (s Server) RegisterOrganization(ctx context.Context, req *servicev1.RegisterOrganizationRequest) (*servicev1.RegisterOrganizationResponse, error) {
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	kinds, err := kindsOf(req.GetKinds())
	if err != nil {
		return nil, err
	}
	taxIDType, err := taxIDTypeOf(req.GetTaxIdType())
	if err != nil {
		return nil, err
	}
	switch {
	case len(req.GetTaxId()) > maxTaxIDBytes:
		return nil, status.Error(codes.InvalidArgument, "tax_id must have at most 32 bytes")
	case !printableFormat.MatchString(req.GetTaxId()):
		return nil, status.Error(codes.InvalidArgument, "tax_id must hold printable ASCII only")
	case len(req.GetJurisdiction()) > maxJurisdictionBytes:
		return nil, status.Error(codes.InvalidArgument, "jurisdiction must have at most 8 bytes")
	case !printableFormat.MatchString(req.GetJurisdiction()):
		return nil, status.Error(codes.InvalidArgument, "jurisdiction must hold printable ASCII only")
	case utf8.RuneCountInString(req.GetLegalName()) > maxNameRunes || utf8.RuneCountInString(req.GetTradeName()) > maxNameRunes:
		return nil, status.Error(codes.InvalidArgument, "legal_name and trade_name must have at most 256 characters")
	case controlled(req.GetLegalName(), req.GetTradeName()):
		return nil, status.Error(codes.InvalidArgument, "legal_name and trade_name must not hold control characters")
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RegisterOrganizationResponse], error) {
			return s.Service.RegisterOrganization(ctx, application.RegisterOrganization{
				OrganizationID: organization,
				Kinds:          kinds,
				Jurisdiction:   req.GetJurisdiction(),
				TaxIDType:      taxIDType,
				TaxID:          req.GetTaxId(),
				LegalName:      req.GetLegalName(),
				TradeName:      req.GetTradeName(),
			})
		},
		registerOrganizationRefused,
		organizationRegisteredOf)
}

func registerOrganizationRefused(r *kernel.Rejection) *servicev1.RegisterOrganizationResponse {
	return &servicev1.RegisterOrganizationResponse{Result: &servicev1.RegisterOrganizationResponse_Rejection{
		Rejection: rejectionOf(r),
	}}
}

func organizationRegisteredOf(registered domain.RegisterOrganizationResponse) *servicev1.RegisterOrganizationResponse {
	return &servicev1.RegisterOrganizationResponse{Result: &servicev1.RegisterOrganizationResponse_Registered{
		Registered: &servicev1.RegisteredOrganization{OrganizationId: string(registered.OrganizationID)},
	}}
}

func rejectionOf(r *kernel.Rejection) *servicev1.Rejection {
	return &servicev1.Rejection{Code: string(r.Code()), Message: r.Message()}
}

func (s Server) FindOrganization(ctx context.Context, req *servicev1.FindOrganizationRequest) (*servicev1.FindOrganizationResponse, error) {
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindOrganization(ctx, organization)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindOrganizationResponse{Organization: organizationOf(snapshot)}, nil
}

func (s Server) FindOrganizationByTaxId(ctx context.Context, req *servicev1.FindOrganizationByTaxIdRequest) (*servicev1.FindOrganizationByTaxIdResponse, error) {
	taxIDType, err := taxIDTypeOf(req.GetTaxIdType())
	if err != nil {
		return nil, err
	}
	if n := len(req.GetTaxId()); n == 0 || n > maxTaxIDBytes {
		return nil, status.Error(codes.InvalidArgument, "tax_id must have 1 to 32 bytes")
	}
	if !printableFormat.MatchString(req.GetTaxId()) {
		return nil, status.Error(codes.InvalidArgument, "tax_id must hold printable ASCII only")
	}
	if !jurisdictionFormat.MatchString(req.GetJurisdiction()) {
		return nil, status.Error(codes.InvalidArgument, "jurisdiction must be an ISO 3166-1 alpha-2 code")
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindOrganizationByTaxID(ctx, application.FindOrganizationByTaxID{
		TaxIDType:    taxIDType,
		TaxID:        req.GetTaxId(),
		Jurisdiction: req.GetJurisdiction(),
	})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindOrganizationByTaxIdResponse{Organization: organizationOf(snapshot)}, nil
}

func (s Server) AddEstablishment(ctx context.Context, req *servicev1.AddEstablishmentRequest) (*servicev1.AddEstablishmentResponse, error) {
	establishment, err := identifier("establishment_id", req.GetEstablishmentId())
	if err != nil {
		return nil, err
	}
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	taxIDType, err := taxIDTypeOf(req.GetTaxIdType())
	if err != nil {
		return nil, err
	}
	switch {
	case len(req.GetTaxId()) > maxTaxIDBytes:
		return nil, status.Error(codes.InvalidArgument, "tax_id must have at most 32 bytes")
	case !printableFormat.MatchString(req.GetTaxId()):
		return nil, status.Error(codes.InvalidArgument, "tax_id must hold printable ASCII only")
	case utf8.RuneCountInString(req.GetTradeName()) > maxNameRunes:
		return nil, status.Error(codes.InvalidArgument, "trade_name must have at most 256 characters")
	case controlled(req.GetTradeName()):
		return nil, status.Error(codes.InvalidArgument, "trade_name must not hold control characters")
	}
	references, err := externalReferencesOf(req.GetExternalReferences())
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.AddEstablishmentResponse], error) {
			return s.Service.AddEstablishment(ctx, application.AddEstablishment{
				EstablishmentID:    domain.EstablishmentID(establishment),
				OrganizationID:     organization,
				TaxIDType:          taxIDType,
				TaxID:              req.GetTaxId(),
				TradeName:          req.GetTradeName(),
				Receiver:           req.GetReceiver(),
				ExternalReferences: references,
			})
		},
		func(r *kernel.Rejection) *servicev1.AddEstablishmentResponse {
			return &servicev1.AddEstablishmentResponse{Result: &servicev1.AddEstablishmentResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(added domain.AddEstablishmentResponse) *servicev1.AddEstablishmentResponse {
			return &servicev1.AddEstablishmentResponse{Result: &servicev1.AddEstablishmentResponse_Added{
				Added: &servicev1.AddedEstablishment{EstablishmentId: string(added.EstablishmentID)},
			}}
		})
}

func (s Server) RegisterPerson(ctx context.Context, req *servicev1.RegisterPersonRequest) (*servicev1.RegisterPersonResponse, error) {
	person, err := identifier("person_id", req.GetPersonId())
	if err != nil {
		return nil, err
	}
	switch {
	case utf8.RuneCountInString(req.GetFullName()) > maxNameRunes:
		return nil, status.Error(codes.InvalidArgument, "full_name must have at most 256 characters")
	case controlled(req.GetFullName()):
		return nil, status.Error(codes.InvalidArgument, "full_name must not hold control characters")
	case len(req.GetDocument()) > maxTaxIDBytes:
		return nil, status.Error(codes.InvalidArgument, "document must have at most 32 bytes")
	case !printableFormat.MatchString(req.GetDocument()):
		return nil, status.Error(codes.InvalidArgument, "document must hold printable ASCII only")
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RegisterPersonResponse], error) {
			return s.Service.RegisterPerson(ctx, application.RegisterPerson{
				PersonID: domain.PersonID(person),
				FullName: req.GetFullName(),
				Document: req.GetDocument(),
			})
		},
		func(r *kernel.Rejection) *servicev1.RegisterPersonResponse {
			return &servicev1.RegisterPersonResponse{Result: &servicev1.RegisterPersonResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(registered domain.RegisterPersonResponse) *servicev1.RegisterPersonResponse {
			return &servicev1.RegisterPersonResponse{Result: &servicev1.RegisterPersonResponse_Registered{
				Registered: &servicev1.RegisteredPerson{PersonId: string(registered.PersonID)},
			}}
		})
}

func (s Server) RegisterRepresentation(ctx context.Context, req *servicev1.RegisterRepresentationRequest) (*servicev1.RegisterRepresentationResponse, error) {
	representation, err := identifier("representation_id", req.GetRepresentationId())
	if err != nil {
		return nil, err
	}
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	person, err := identifier("person_id", req.GetPersonId())
	if err != nil {
		return nil, err
	}
	powerType, err := known(powerTypesOnTheWire, req.GetPowerType(), "power_type must be legal_representative, attorney_in_fact or operator")
	if err != nil {
		return nil, err
	}
	sourceType, err := known(sourceTypesOnTheWire, req.GetSourceType(),
		"source_type must be articles_of_association, power_of_attorney, board_resolution or internal_delegation")
	if err != nil {
		return nil, err
	}
	switch {
	case utf8.RuneCountInString(req.GetSourceEvidenceId()) > maxReferenceRunes:
		return nil, status.Error(codes.InvalidArgument, "source_evidence_id must have at most 256 characters")
	case controlled(req.GetSourceEvidenceId()):
		return nil, status.Error(codes.InvalidArgument, "source_evidence_id must not hold control characters")
	case len(req.GetScopeCurrency()) > maxCurrencyBytes:
		return nil, status.Error(codes.InvalidArgument, "scope_currency must have at most 8 bytes")
	case !printableFormat.MatchString(req.GetScopeCurrency()):
		return nil, status.Error(codes.InvalidArgument, "scope_currency must hold printable ASCII only")
	case len(req.GetProposedByUserId()) > maxActorBytes:
		return nil, status.Error(codes.InvalidArgument, "proposed_by_user_id must have at most 256 bytes")
	case controlled(req.GetProposedByUserId()):
		return nil, status.Error(codes.InvalidArgument, "proposed_by_user_id must not hold control characters")
	}
	if err := actTypesWithinCeilings(req.GetScopeActTypes()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RegisterRepresentationResponse], error) {
			return s.Service.RegisterRepresentation(ctx, application.RegisterRepresentation{
				RepresentationID:    domain.RepresentationID(representation),
				OrganizationID:      organization,
				PersonID:            domain.PersonID(person),
				PowerType:           powerType,
				SourceType:          sourceType,
				SourceEvidenceID:    req.GetSourceEvidenceId(),
				ScopeCurrency:       req.GetScopeCurrency(),
				ScopeMaxAmountMinor: req.GetScopeMaxAmountMinor(),
				ScopeActTypes:       req.GetScopeActTypes(),
				ValidFrom:           kernel.Instant(req.GetValidFrom()),
				ValidUntil:          kernel.Instant(req.GetValidUntil()),
				ProposedByUserID:    req.GetProposedByUserId(),
			})
		},
		func(r *kernel.Rejection) *servicev1.RegisterRepresentationResponse {
			return &servicev1.RegisterRepresentationResponse{Result: &servicev1.RegisterRepresentationResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(registered domain.RegisterRepresentationResponse) *servicev1.RegisterRepresentationResponse {
			return &servicev1.RegisterRepresentationResponse{Result: &servicev1.RegisterRepresentationResponse_Registered{
				Registered: &servicev1.RegisteredRepresentation{RepresentationId: string(registered.RepresentationID)},
			}}
		})
}

func (s Server) FindEstablishmentsByOrganization(ctx context.Context, req *servicev1.FindEstablishmentsByOrganizationRequest) (*servicev1.FindEstablishmentsByOrganizationResponse, error) {
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindEstablishmentsByOrganization(ctx, organization)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	establishments := make([]*servicev1.Establishment, 0, len(found))
	for _, e := range found {
		establishments = append(establishments, establishmentOf(e))
	}
	return &servicev1.FindEstablishmentsByOrganizationResponse{Establishments: establishments}, nil
}

func (s Server) FindPerson(ctx context.Context, req *servicev1.FindPersonRequest) (*servicev1.FindPersonResponse, error) {
	person, err := identifier("person_id", req.GetPersonId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindPerson(ctx, domain.PersonID(person))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindPersonResponse{Person: personOf(snapshot)}, nil
}

func (s Server) FindRepresentation(ctx context.Context, req *servicev1.FindRepresentationRequest) (*servicev1.FindRepresentationResponse, error) {
	representation, err := identifier("representation_id", req.GetRepresentationId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindRepresentation(ctx, domain.RepresentationID(representation))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindRepresentationResponse{Representation: representationOf(snapshot)}, nil
}

func (s Server) FindRepresentationsByOrganization(ctx context.Context, req *servicev1.FindRepresentationsByOrganizationRequest) (*servicev1.FindRepresentationsByOrganizationResponse, error) {
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	if req.GetValidAt() < 0 {
		return nil, status.Error(codes.InvalidArgument, "valid_at must not be negative")
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindRepresentationsByOrganization(ctx, application.FindRepresentationsByOrganization{
		Organization: organization,
		ValidAt:      kernel.Instant(req.GetValidAt()),
	})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	representations := make([]*servicev1.Representation, 0, len(found))
	for _, r := range found {
		representations = append(representations, representationOf(r))
	}
	return &servicev1.FindRepresentationsByOrganizationResponse{Representations: representations}, nil
}

func (s Server) ProvisionUserAccount(ctx context.Context, req *servicev1.ProvisionUserAccountRequest) (*servicev1.ProvisionUserAccountResponse, error) {
	subject, err := subjectOf(req.GetSubject())
	if err != nil {
		return nil, err
	}
	person, err := optionalIdentifier("person_id", req.GetPersonId())
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ProvisionUserAccountResponse], error) {
			return s.Service.ProvisionUserAccount(ctx, application.ProvisionUserAccount{Subject: subject, PersonID: domain.PersonID(person)})
		},
		func(r *kernel.Rejection) *servicev1.ProvisionUserAccountResponse {
			return &servicev1.ProvisionUserAccountResponse{Result: &servicev1.ProvisionUserAccountResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(provisioned domain.ProvisionUserAccountResponse) *servicev1.ProvisionUserAccountResponse {
			return &servicev1.ProvisionUserAccountResponse{Result: &servicev1.ProvisionUserAccountResponse_Provisioned{
				Provisioned: &servicev1.ProvisionedUserAccount{UserId: string(provisioned.UserID)},
			}}
		})
}

func (s Server) SuspendUser(ctx context.Context, req *servicev1.SuspendUserRequest) (*servicev1.SuspendUserResponse, error) {
	user, err := userID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	switch {
	case utf8.RuneCountInString(req.GetReason()) > maxReasonRunes:
		return nil, status.Error(codes.InvalidArgument, "reason must have at most 512 characters")
	case controlled(req.GetReason()):
		return nil, status.Error(codes.InvalidArgument, "reason must not hold control characters")
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.SuspendUserResponse], error) {
			return s.Service.SuspendUser(ctx, application.SuspendUser{UserID: user, Reason: req.GetReason()})
		},
		func(r *kernel.Rejection) *servicev1.SuspendUserResponse {
			return &servicev1.SuspendUserResponse{Result: &servicev1.SuspendUserResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(suspended domain.SuspendUserResponse) *servicev1.SuspendUserResponse {
			return &servicev1.SuspendUserResponse{Result: &servicev1.SuspendUserResponse_Suspended{
				Suspended: &servicev1.SuspendedUser{UserId: string(suspended.UserID)},
			}}
		})
}

func (s Server) InviteUser(ctx context.Context, req *servicev1.InviteUserRequest) (*servicev1.InviteUserResponse, error) {
	membership, err := membershipID(req.GetMembershipId())
	if err != nil {
		return nil, err
	}
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	role, err := known(rolesOnTheWire, req.GetRole(), "role must be operator or auditor")
	if err != nil {
		return nil, err
	}
	switch {
	case utf8.RuneCountInString(req.GetInviteeEmail()) > maxEmailRunes:
		return nil, status.Error(codes.InvalidArgument, "invitee_email must have at most 254 characters")
	case controlled(req.GetInviteeEmail()):
		return nil, status.Error(codes.InvalidArgument, "invitee_email must not hold control characters")
	case len(req.GetInvitedByUserId()) > maxActorBytes:
		return nil, status.Error(codes.InvalidArgument, "invited_by_user_id must have at most 256 bytes")
	case controlled(req.GetInvitedByUserId()):
		return nil, status.Error(codes.InvalidArgument, "invited_by_user_id must not hold control characters")
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.InviteUserResponse], error) {
			return s.Service.InviteUser(ctx, application.InviteUser{
				MembershipID:        membership,
				OrganizationID:      organization,
				InviteeEmail:        req.GetInviteeEmail(),
				Role:                role,
				InvitedByUserID:     req.GetInvitedByUserId(),
				InvitationExpiresAt: kernel.Instant(req.GetInvitationExpiresAt()),
			})
		},
		func(r *kernel.Rejection) *servicev1.InviteUserResponse {
			return &servicev1.InviteUserResponse{Result: &servicev1.InviteUserResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(invited domain.InviteUserResponse) *servicev1.InviteUserResponse {
			return &servicev1.InviteUserResponse{Result: &servicev1.InviteUserResponse_Invited{
				Invited: &servicev1.InvitedUser{MembershipId: string(invited.MembershipID)},
			}}
		})
}

func (s Server) GrantMembership(ctx context.Context, req *servicev1.GrantMembershipRequest) (*servicev1.GrantMembershipResponse, error) {
	membership, err := membershipID(req.GetMembershipId())
	if err != nil {
		return nil, err
	}
	user, err := userID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	person, err := optionalIdentifier("person_id", req.GetPersonId())
	if err != nil {
		return nil, err
	}
	level, err := known(verificationLevelsOnTheWire, req.GetVerificationLevel(), "verification_level must be channel_ownership or personal_data_checked")
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.GrantMembershipResponse], error) {
			return s.Service.GrantMembership(ctx, application.GrantMembership{
				MembershipID:      membership,
				UserID:            user,
				PersonID:          domain.PersonID(person),
				ChannelConfirmed:  req.GetChannelConfirmed(),
				VerificationLevel: level,
			})
		},
		func(r *kernel.Rejection) *servicev1.GrantMembershipResponse {
			return &servicev1.GrantMembershipResponse{Result: &servicev1.GrantMembershipResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(granted domain.GrantMembershipResponse) *servicev1.GrantMembershipResponse {
			return &servicev1.GrantMembershipResponse{Result: &servicev1.GrantMembershipResponse_Granted{
				Granted: &servicev1.GrantedMembership{MembershipId: string(granted.MembershipID)},
			}}
		})
}

func (s Server) RevokeMembership(ctx context.Context, req *servicev1.RevokeMembershipRequest) (*servicev1.RevokeMembershipResponse, error) {
	membership, err := membershipID(req.GetMembershipId())
	if err != nil {
		return nil, err
	}
	switch {
	case utf8.RuneCountInString(req.GetReason()) > maxReasonRunes:
		return nil, status.Error(codes.InvalidArgument, "reason must have at most 512 characters")
	case controlled(req.GetReason()):
		return nil, status.Error(codes.InvalidArgument, "reason must not hold control characters")
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RevokeMembershipResponse], error) {
			return s.Service.RevokeMembership(ctx, application.RevokeMembership{MembershipID: membership, Reason: req.GetReason()})
		},
		func(r *kernel.Rejection) *servicev1.RevokeMembershipResponse {
			return &servicev1.RevokeMembershipResponse{Result: &servicev1.RevokeMembershipResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(revoked domain.RevokeMembershipResponse) *servicev1.RevokeMembershipResponse {
			return &servicev1.RevokeMembershipResponse{Result: &servicev1.RevokeMembershipResponse_Revoked{
				Revoked: &servicev1.RevokedMembership{MembershipId: string(revoked.MembershipID)},
			}}
		})
}

func (s Server) FindMembershipsByUser(ctx context.Context, req *servicev1.FindMembershipsByUserRequest) (*servicev1.FindMembershipsByUserResponse, error) {
	user, err := userID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindMembershipsByUser(ctx, user)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindMembershipsByUserResponse{Memberships: membershipsOf(found)}, nil
}

func (s Server) FindMembershipsByOrganization(ctx context.Context, req *servicev1.FindMembershipsByOrganizationRequest) (*servicev1.FindMembershipsByOrganizationResponse, error) {
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindMembershipsByOrganization(ctx, organization)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindMembershipsByOrganizationResponse{Memberships: membershipsOf(found)}, nil
}

func (s Server) FindEffectivePermissions(ctx context.Context, req *servicev1.FindEffectivePermissionsRequest) (*servicev1.FindEffectivePermissionsResponse, error) {
	user, err := userID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindEffectivePermissions(ctx, user)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	permissions := make([]string, 0, len(found))
	for _, p := range found {
		permissions = append(permissions, string(p))
	}
	return &servicev1.FindEffectivePermissionsResponse{Permissions: permissions}, nil
}

func (s Server) RequestBankAccountChange(ctx context.Context, req *servicev1.RequestBankAccountChangeRequest) (*servicev1.RequestBankAccountChangeResponse, error) {
	change, err := bankAccountChangeID(req.GetBankAccountChangeId())
	if err != nil {
		return nil, err
	}
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	switch {
	case utf8.RuneCountInString(req.GetDestinationEvidenceId()) > maxReferenceRunes:
		return nil, status.Error(codes.InvalidArgument, "destination_evidence_id must have at most 256 characters")
	case controlled(req.GetDestinationEvidenceId()):
		return nil, status.Error(codes.InvalidArgument, "destination_evidence_id must not hold control characters")
	}
	requester, err := actor("requested_by_user_id", req.GetRequestedByUserId())
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RequestBankAccountChangeResponse], error) {
			return s.Service.RequestBankAccountChange(ctx, application.RequestBankAccountChange{
				BankAccountChangeID:   change,
				OrganizationID:        organization,
				DestinationEvidenceID: req.GetDestinationEvidenceId(),
				RequestedByUserID:     requester,
			})
		},
		func(r *kernel.Rejection) *servicev1.RequestBankAccountChangeResponse {
			return &servicev1.RequestBankAccountChangeResponse{Result: &servicev1.RequestBankAccountChangeResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(requested domain.RequestBankAccountChangeResponse) *servicev1.RequestBankAccountChangeResponse {
			return &servicev1.RequestBankAccountChangeResponse{Result: &servicev1.RequestBankAccountChangeResponse_Requested{
				Requested: &servicev1.RequestedBankAccountChange{BankAccountChangeId: string(requested.BankAccountChangeID)},
			}}
		})
}

func (s Server) ApproveBankAccountChange(ctx context.Context, req *servicev1.ApproveBankAccountChangeRequest) (*servicev1.ApproveBankAccountChangeResponse, error) {
	change, err := bankAccountChangeID(req.GetBankAccountChangeId())
	if err != nil {
		return nil, err
	}
	approver, err := actor("approved_by_user_id", req.GetApprovedByUserId())
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ApproveBankAccountChangeResponse], error) {
			return s.Service.ApproveBankAccountChange(ctx, application.ApproveBankAccountChange{BankAccountChangeID: change, ApprovedByUserID: approver})
		},
		func(r *kernel.Rejection) *servicev1.ApproveBankAccountChangeResponse {
			return &servicev1.ApproveBankAccountChangeResponse{Result: &servicev1.ApproveBankAccountChangeResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(approved domain.ApproveBankAccountChangeResponse) *servicev1.ApproveBankAccountChangeResponse {
			return &servicev1.ApproveBankAccountChangeResponse{Result: &servicev1.ApproveBankAccountChangeResponse_Approved{
				Approved: &servicev1.ApprovedBankAccountChange{BankAccountChangeId: string(approved.BankAccountChangeID)},
			}}
		})
}

func (s Server) FindBankAccountChange(ctx context.Context, req *servicev1.FindBankAccountChangeRequest) (*servicev1.FindBankAccountChangeResponse, error) {
	change, err := bankAccountChangeID(req.GetBankAccountChangeId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	snapshot, err := s.Service.FindBankAccountChange(ctx, change)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindBankAccountChangeResponse{BankAccountChange: bankAccountChangeOf(snapshot)}, nil
}

func (s Server) FindBankAccountChangesByOrganization(ctx context.Context, req *servicev1.FindBankAccountChangesByOrganizationRequest) (*servicev1.FindBankAccountChangesByOrganizationResponse, error) {
	organization, err := organizationID(req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindBankAccountChangesByOrganization(ctx, organization)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	changes := make([]*servicev1.BankAccountChange, 0, len(found))
	for _, c := range found {
		changes = append(changes, bankAccountChangeOf(c))
	}
	return &servicev1.FindBankAccountChangesByOrganizationResponse{BankAccountChanges: changes}, nil
}

func identifier(field, raw string) (string, error) {
	if !idFormat.MatchString(raw) {
		return "", status.Errorf(codes.InvalidArgument, "%s must have 1 to 128 characters of [A-Za-z0-9._:-]", field)
	}
	return raw, nil
}

func organizationID(raw string) (domain.OrganizationID, error) {
	id, err := identifier("organization_id", raw)
	return domain.OrganizationID(id), err
}

func membershipID(raw string) (domain.MembershipID, error) {
	id, err := identifier("membership_id", raw)
	return domain.MembershipID(id), err
}

func bankAccountChangeID(raw string) (domain.BankAccountChangeID, error) {
	id, err := identifier("bank_account_change_id", raw)
	return domain.BankAccountChangeID(id), err
}

// actor refuses a blank subject before the domain: an empty requester makes
// every approver distinct from it, so the segregation of duties would pass
// vacuously (the spec states that actor fields carry the authenticated user's
// subject, taken from the verified token and never from the request body).
func actor(field, raw string) (string, error) {
	switch {
	case strings.TrimSpace(raw) == "":
		return "", status.Errorf(codes.InvalidArgument, "%s must hold the subject of the authenticated user", field)
	case len(raw) > maxActorBytes:
		return "", status.Errorf(codes.InvalidArgument, "%s must have at most 256 bytes", field)
	case controlled(raw):
		return "", status.Errorf(codes.InvalidArgument, "%s must not hold control characters", field)
	}
	return raw, nil
}

func userID(raw string) (domain.UserID, error) {
	id, err := identifier("user_id", raw)
	return domain.UserID(id), err
}

// subjectOf lets an empty subject through: the domain owns the subject-empty
// refusal of the spec, which a form check here would answer as a status.
func subjectOf(raw string) (domain.UserID, error) {
	id, err := optionalIdentifier("subject", raw)
	return domain.UserID(id), err
}

func optionalIdentifier(field, raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	return identifier(field, raw)
}

var kindsOnTheWire = map[servicev1.OrganizationKind]domain.OrganizationKind{
	servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT:   domain.KindClient,
	servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER:   domain.KindFunder,
	servicev1.OrganizationKind_ORGANIZATION_KIND_PARTNER:  domain.KindPartner,
	servicev1.OrganizationKind_ORGANIZATION_KIND_PLATFORM: domain.KindPlatform,
}

var externalSystemsOnTheWire = map[servicev1.ExternalSystem]domain.ExternalSystem{
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY:   domain.ExternalSystemRegistry,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_BOOKKEEPER: domain.ExternalSystemBookkeeper,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_ACQUIRER:   domain.ExternalSystemAcquirer,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_FUNDER:     domain.ExternalSystemFunder,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_DEPOSITORY: domain.ExternalSystemDepository,
}

var referenceTypesOnTheWire = map[servicev1.ExternalReferenceType]domain.ExternalReferenceType{
	servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_ASSET:         domain.ExternalReferenceAsset,
	servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_EFFECT:        domain.ExternalReferenceEffect,
	servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_AUTHORIZATION: domain.ExternalReferenceAuthorization,
	servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_COMMAND:       domain.ExternalReferenceCommand,
	servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_SETTLEMENT:    domain.ExternalReferenceSettlement,
}

var powerTypesOnTheWire = map[servicev1.PowerType]domain.PowerType{
	servicev1.PowerType_POWER_TYPE_LEGAL_REPRESENTATIVE: domain.PowerLegalRepresentative,
	servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT:     domain.PowerAttorneyInFact,
	servicev1.PowerType_POWER_TYPE_OPERATOR:             domain.PowerOperator,
}

var sourceTypesOnTheWire = map[servicev1.SourceType]domain.SourceType{
	servicev1.SourceType_SOURCE_TYPE_ARTICLES_OF_ASSOCIATION: domain.SourceArticlesOfAssociation,
	servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY:       domain.SourcePowerOfAttorney,
	servicev1.SourceType_SOURCE_TYPE_BOARD_RESOLUTION:        domain.SourceBoardResolution,
	servicev1.SourceType_SOURCE_TYPE_INTERNAL_DELEGATION:     domain.SourceInternalDelegation,
}

var rolesOnTheWire = map[servicev1.MembershipRole]domain.Role{
	servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR: domain.RoleOperator,
	servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR:  domain.RoleAuditor,
}

var verificationLevelsOnTheWire = map[servicev1.VerificationLevel]domain.VerificationLevel{
	servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP:     domain.VerificationChannelOwnership,
	servicev1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED: domain.VerificationPersonalDataChecked,
}

var membershipStatusesOnTheWire = map[servicev1.MembershipStatus]domain.MembershipStatus{
	servicev1.MembershipStatus_MEMBERSHIP_STATUS_INVITED: domain.MembershipStatusInvited,
	servicev1.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE:  domain.MembershipStatusActive,
	servicev1.MembershipStatus_MEMBERSHIP_STATUS_REVOKED: domain.MembershipStatusRevoked,
}

var bankAccountChangeStatusesOnTheWire = map[servicev1.BankAccountChangeStatus]domain.BankAccountChangeStatus{
	servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_REQUESTED: domain.BankAccountChangeStatusRequested,
	servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED:  domain.BankAccountChangeStatusApproved,
}

// known refuses a value outside the catalogue, the unspecified zero included,
// instead of letting an empty domain value reach the use case.
func known[W comparable, D any](catalogue map[W]D, raw W, refusal string) (D, error) {
	value, ok := catalogue[raw]
	if !ok {
		var none D
		return none, status.Error(codes.InvalidArgument, refusal)
	}
	return value, nil
}

func onTheWire[W, D comparable](catalogue map[W]D, value D) W {
	for wire, candidate := range catalogue {
		if candidate == value {
			return wire
		}
	}
	var unspecified W
	return unspecified
}

// kindsOf refuses a kind outside the catalogue instead of dropping it, which
// KindsOf would do silently; an empty list still reaches the domain refusal.
func kindsOf(raw []servicev1.OrganizationKind) ([]domain.OrganizationKind, error) {
	kinds := make([]domain.OrganizationKind, 0, len(raw))
	for _, kind := range raw {
		known, ok := kindsOnTheWire[kind]
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "kinds must hold only client, funder, partner or platform")
		}
		kinds = append(kinds, known)
	}
	return kinds, nil
}

func taxIDTypeOf(raw servicev1.TaxIdType) (domain.TaxIDType, error) {
	switch raw {
	case servicev1.TaxIdType_TAX_ID_TYPE_CNPJ:
		return domain.TaxIDTypeCNPJ, nil
	case servicev1.TaxIdType_TAX_ID_TYPE_CPF:
		return domain.TaxIDTypeCPF, nil
	default:
		return "", status.Error(codes.InvalidArgument, "tax_id_type must be cnpj or cpf")
	}
}

func externalReferencesOf(raw []*servicev1.ExternalReference) ([]domain.ExternalReference, error) {
	if len(raw) > maxListItems {
		return nil, status.Error(codes.InvalidArgument, "external_references must hold at most 32 items")
	}
	references := make([]domain.ExternalReference, 0, len(raw))
	for _, ref := range raw {
		system, err := known(externalSystemsOnTheWire, ref.GetSystem(), "external_references.system must be a known external system")
		if err != nil {
			return nil, err
		}
		referenceType, err := known(referenceTypesOnTheWire, ref.GetReferenceType(), "external_references.reference_type must be a known reference type")
		if err != nil {
			return nil, err
		}
		if utf8.RuneCountInString(ref.GetProviderId()) > maxReferenceRunes ||
			utf8.RuneCountInString(ref.GetReference()) > maxReferenceRunes ||
			utf8.RuneCountInString(ref.GetObservedVersion()) > maxReferenceRunes {
			return nil, status.Error(codes.InvalidArgument, "external_references provider_id, reference and observed_version must have at most 256 characters")
		}
		if controlled(ref.GetProviderId(), ref.GetReference(), ref.GetObservedVersion()) {
			return nil, status.Error(codes.InvalidArgument, "external_references provider_id, reference and observed_version must not hold control characters")
		}
		references = append(references, domain.ExternalReference{
			ProviderID:      ref.GetProviderId(),
			System:          system,
			ReferenceType:   referenceType,
			Reference:       ref.GetReference(),
			AsOf:            kernel.Instant(ref.GetAsOf()),
			ObservedVersion: ref.GetObservedVersion(),
		})
	}
	return references, nil
}

func actTypesWithinCeilings(acts []string) error {
	if len(acts) > maxListItems {
		return status.Error(codes.InvalidArgument, "scope_act_types must hold at most 32 items")
	}
	for _, act := range acts {
		switch {
		case len(act) > maxActTypeBytes:
			return status.Error(codes.InvalidArgument, "scope_act_types items must have at most 64 bytes")
		case !printableFormat.MatchString(act):
			return status.Error(codes.InvalidArgument, "scope_act_types items must hold printable ASCII only")
		}
	}
	return nil
}

func organizationOf(s domain.OrganizationSnapshot) *servicev1.Organization {
	listed := s.Kinds.List()
	kinds := make([]servicev1.OrganizationKind, 0, len(listed))
	for _, kind := range listed {
		kinds = append(kinds, onTheWire(kindsOnTheWire, kind))
	}
	return &servicev1.Organization{
		OrganizationId:      string(s.ID),
		Kinds:               kinds,
		Jurisdiction:        s.Jurisdiction,
		TaxIdType:           taxIDTypeOnTheWire(s.TaxIDType),
		TaxId:               s.TaxID,
		TaxIdHash:           s.TaxIDHash,
		TaxIdHashKeyVersion: int32(s.TaxIDHashKeyVersion),
		TaxIdMasked:         s.TaxIDMasked,
		LegalName:           s.LegalName,
		TradeName:           s.TradeName,
		Status:              organizationStatus(s.Status),
		CreatedAt:           int64(s.CreatedAt),
	}
}

func establishmentOf(s domain.EstablishmentSnapshot) *servicev1.Establishment {
	references := make([]*servicev1.ExternalReference, 0, len(s.ExternalReferences))
	for _, ref := range s.ExternalReferences {
		references = append(references, &servicev1.ExternalReference{
			ProviderId:      ref.ProviderID,
			System:          onTheWire(externalSystemsOnTheWire, ref.System),
			ReferenceType:   onTheWire(referenceTypesOnTheWire, ref.ReferenceType),
			Reference:       ref.Reference,
			AsOf:            int64(ref.AsOf),
			ObservedVersion: ref.ObservedVersion,
		})
	}
	return &servicev1.Establishment{
		EstablishmentId:     string(s.ID),
		OrganizationId:      string(s.OrganizationID),
		TaxIdType:           taxIDTypeOnTheWire(s.TaxIDType),
		TaxId:               s.TaxID,
		TaxIdHash:           s.TaxIDHash,
		TaxIdHashKeyVersion: int32(s.TaxIDHashKeyVersion),
		TaxIdMasked:         s.TaxIDMasked,
		TradeName:           s.TradeName,
		Receiver:            s.Receiver,
		ExternalReferences:  references,
		CreatedAt:           int64(s.CreatedAt),
	}
}

func personOf(s domain.PersonSnapshot) *servicev1.Person {
	return &servicev1.Person{
		PersonId:               string(s.ID),
		FullName:               s.FullName,
		DocumentHash:           s.DocumentHash,
		DocumentHashKeyVersion: int32(s.DocumentHashKeyVersion),
		DocumentMasked:         s.DocumentMasked,
		CreatedAt:              int64(s.CreatedAt),
	}
}

func representationOf(s domain.RepresentationSnapshot) *servicev1.Representation {
	verification := servicev1.RepresentationVerificationStatus_REPRESENTATION_VERIFICATION_STATUS_UNSPECIFIED
	if s.VerificationStatus == domain.RepresentationDeclared {
		verification = servicev1.RepresentationVerificationStatus_REPRESENTATION_VERIFICATION_STATUS_DECLARED
	}
	return &servicev1.Representation{
		RepresentationId:    string(s.ID),
		OrganizationId:      string(s.OrganizationID),
		PersonId:            string(s.PersonID),
		PowerType:           onTheWire(powerTypesOnTheWire, s.PowerType),
		SourceType:          onTheWire(sourceTypesOnTheWire, s.SourceType),
		SourceEvidenceId:    s.SourceEvidenceID,
		ScopeCurrency:       s.ScopeCurrency,
		ScopeMaxAmountMinor: s.ScopeMaxAmountMinor,
		ScopeActTypes:       s.ScopeActTypes,
		ValidFrom:           int64(s.ValidFrom),
		ValidUntil:          int64(s.ValidUntil),
		VerificationStatus:  verification,
		ProposedByUserId:    s.ProposedByUserID,
	}
}

func membershipsOf(found []domain.MembershipSnapshot) []*servicev1.Membership {
	memberships := make([]*servicev1.Membership, 0, len(found))
	for _, m := range found {
		memberships = append(memberships, &servicev1.Membership{
			MembershipId:        string(m.ID),
			OrganizationId:      string(m.OrganizationID),
			InviteeEmail:        m.InviteeEmail,
			Role:                onTheWire(rolesOnTheWire, m.Role),
			Status:              onTheWire(membershipStatusesOnTheWire, m.Status),
			InvitedByUserId:     m.InvitedByUserID,
			InvitedAt:           int64(m.InvitedAt),
			InvitationExpiresAt: int64(m.InvitationExpiresAt),
			UserId:              string(m.UserID),
			PersonId:            string(m.PersonID),
			VerificationLevel:   onTheWire(verificationLevelsOnTheWire, m.VerificationLevel),
			GrantedAt:           int64(m.GrantedAt),
			RevokedAt:           int64(m.RevokedAt),
			RevocationReason:    m.RevocationReason,
		})
	}
	return memberships
}

func bankAccountChangeOf(s domain.BankAccountChangeSnapshot) *servicev1.BankAccountChange {
	return &servicev1.BankAccountChange{
		BankAccountChangeId:   string(s.ID),
		OrganizationId:        string(s.OrganizationID),
		DestinationEvidenceId: s.DestinationEvidenceID,
		RequestedByUserId:     s.RequestedByUserID,
		ApprovedByUserId:      s.ApprovedByUserID,
		Status:                onTheWire(bankAccountChangeStatusesOnTheWire, s.Status),
		RequestedAt:           int64(s.RequestedAt),
		ApprovedAt:            int64(s.ApprovedAt),
	}
}

func taxIDTypeOnTheWire(kind domain.TaxIDType) servicev1.TaxIdType {
	switch kind {
	case domain.TaxIDTypeCNPJ:
		return servicev1.TaxIdType_TAX_ID_TYPE_CNPJ
	case domain.TaxIDTypeCPF:
		return servicev1.TaxIdType_TAX_ID_TYPE_CPF
	default:
		return servicev1.TaxIdType_TAX_ID_TYPE_UNSPECIFIED
	}
}

func organizationStatus(s domain.OrganizationStatus) servicev1.OrganizationStatus {
	switch s {
	case domain.OrganizationActive:
		return servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE
	case domain.OrganizationSuspended:
		return servicev1.OrganizationStatus_ORGANIZATION_STATUS_SUSPENDED
	case domain.OrganizationOffboarded:
		return servicev1.OrganizationStatus_ORGANIZATION_STATUS_OFFBOARDED
	default:
		return servicev1.OrganizationStatus_ORGANIZATION_STATUS_UNSPECIFIED
	}
}
