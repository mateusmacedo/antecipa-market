package rpc

import (
	"context"
	"unicode/utf8"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Server) RegisterIntegrationProfile(ctx context.Context, req *servicev1.RegisterIntegrationProfileRequest) (*servicev1.RegisterIntegrationProfileResponse, error) {
	cmd, err := integrationProfileOf(req)
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RegisterIntegrationProfileResponse], error) {
			return s.Service.RegisterIntegrationProfile(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.RegisterIntegrationProfileResponse {
			return &servicev1.RegisterIntegrationProfileResponse{Result: &servicev1.RegisterIntegrationProfileResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(registered domain.RegisterIntegrationProfileResponse) *servicev1.RegisterIntegrationProfileResponse {
			return &servicev1.RegisterIntegrationProfileResponse{Result: &servicev1.RegisterIntegrationProfileResponse_Registered{
				Registered: &servicev1.RegisteredIntegrationProfile{ProfileId: string(registered.ProfileID)},
			}}
		})
}

func integrationProfileOf(req *servicev1.RegisterIntegrationProfileRequest) (application.RegisterIntegrationProfile, error) {
	var none application.RegisterIntegrationProfile
	profile, err := keyedIdentifier("profile_id", req.GetProfileId())
	if err != nil {
		return none, err
	}
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return none, err
	}
	supersedes, err := optionalIdentifier("supersedes_id", req.GetSupersedesId())
	if err != nil {
		return none, err
	}
	environment, err := known(environmentsOnTheWire, req.GetEnvironment(), "environment must be one of development, integration, homologation or production")
	if err != nil {
		return none, err
	}
	changeKind, err := known(changeKindsOnTheWire, req.GetChangeKind(), "change_kind must be one of initial, announced or unannounced")
	if err != nil {
		return none, err
	}
	if err := listWithin("protocols", req.GetProtocols(), maxProtocols); err != nil {
		return none, err
	}
	for _, field := range []struct {
		name, value string
		ceiling     int
	}{
		{"endpoint", req.GetEndpoint(), endpointCeiling},
		{"schema_version", req.GetSchemaVersion(), codeCeiling},
		{"secret_ref", req.GetSecretRef(), referenceCeiling},
		{"certificate_ref", req.GetCertificateRef(), referenceCeiling},
	} {
		if err := asciiWithin(field.name, field.value, field.ceiling); err != nil {
			return none, err
		}
	}
	if req.GetCertificateExpiresAt() < 0 || req.GetEffectiveFrom() < 0 {
		return none, status.Error(codes.InvalidArgument, "certificate_expires_at and effective_from must be instants in nanoseconds, never negative")
	}
	return application.RegisterIntegrationProfile{
		ProfileID:            domain.ProfileID(profile),
		PartnerID:            domain.PartnerID(partner),
		Environment:          environment,
		ChangeKind:           changeKind,
		Protocols:            req.GetProtocols(),
		Endpoint:             req.GetEndpoint(),
		SchemaVersion:        req.GetSchemaVersion(),
		SecretRef:            req.GetSecretRef(),
		CertificateRef:       req.GetCertificateRef(),
		CertificateExpiresAt: kernel.Instant(req.GetCertificateExpiresAt()),
		EffectiveFrom:        kernel.Instant(req.GetEffectiveFrom()),
		SupersedesID:         domain.ProfileID(supersedes),
	}, nil
}

func (s Server) RaiseCertificateExpiryAlert(ctx context.Context, req *servicev1.RaiseCertificateExpiryAlertRequest) (*servicev1.RaiseCertificateExpiryAlertResponse, error) {
	profile, err := keyedIdentifier("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RaiseCertificateExpiryAlertResponse], error) {
			return s.Service.RaiseCertificateExpiryAlert(ctx, application.RaiseCertificateExpiryAlert{
				ProfileID:     domain.ProfileID(profile),
				ThresholdDays: int(req.GetThresholdDays()),
			})
		},
		func(r *kernel.Rejection) *servicev1.RaiseCertificateExpiryAlertResponse {
			return &servicev1.RaiseCertificateExpiryAlertResponse{Result: &servicev1.RaiseCertificateExpiryAlertResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(raised domain.RaiseCertificateExpiryAlertResponse) *servicev1.RaiseCertificateExpiryAlertResponse {
			return &servicev1.RaiseCertificateExpiryAlertResponse{Result: &servicev1.RaiseCertificateExpiryAlertResponse_Raised{
				Raised: &servicev1.RaisedCertificateExpiryAlert{ProfileId: string(raised.ProfileID)},
			}}
		})
}

func (s Server) DefineOperationalPolicy(ctx context.Context, req *servicev1.DefineOperationalPolicyRequest) (*servicev1.DefineOperationalPolicyResponse, error) {
	cmd, err := operationalPolicyOf(req)
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.DefineOperationalPolicyResponse], error) {
			return s.Service.DefineOperationalPolicy(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.DefineOperationalPolicyResponse {
			return &servicev1.DefineOperationalPolicyResponse{Result: &servicev1.DefineOperationalPolicyResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(defined domain.DefineOperationalPolicyResponse) *servicev1.DefineOperationalPolicyResponse {
			return &servicev1.DefineOperationalPolicyResponse{Result: &servicev1.DefineOperationalPolicyResponse_Defined{
				Defined: &servicev1.DefinedOperationalPolicy{PolicyId: string(defined.PolicyID)},
			}}
		})
}

func operationalPolicyOf(req *servicev1.DefineOperationalPolicyRequest) (application.DefineOperationalPolicy, error) {
	var none application.DefineOperationalPolicy
	policy, err := identifier("policy_id", req.GetPolicyId())
	if err != nil {
		return none, err
	}
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return none, err
	}
	supersedes, err := optionalIdentifier("supersedes_id", req.GetSupersedesId())
	if err != nil {
		return none, err
	}
	if req.GetEffectiveFrom() < 0 {
		return none, status.Error(codes.InvalidArgument, "effective_from must be an instant in nanoseconds, never negative")
	}
	if err := listWithin("business_days", req.GetBusinessDays(), maxBusinessDays); err != nil {
		return none, err
	}
	for _, field := range []struct{ name, value string }{
		{"cutoff_time", req.GetCutoffTime()},
		{"processing_window", req.GetProcessingWindow()},
	} {
		if err := asciiWithin(field.name, field.value, codeCeiling); err != nil {
			return none, err
		}
	}
	for _, field := range []struct{ name, value string }{
		{"sla_ref", req.GetSlaRef()},
		{"fallback", req.GetFallback()},
		{"support_contact", req.GetSupportContact()},
		{"incident_contact", req.GetIncidentContact()},
	} {
		if err := textWithin(field.name, field.value); err != nil {
			return none, err
		}
	}
	return application.DefineOperationalPolicy{
		PolicyID:           domain.PolicyID(policy),
		PartnerID:          domain.PartnerID(partner),
		EffectiveFrom:      kernel.Instant(req.GetEffectiveFrom()),
		SupersedesID:       domain.PolicyID(supersedes),
		RateLimitPerMinute: int(req.GetRateLimitPerMinute()),
		BusinessDays:       req.GetBusinessDays(),
		CutoffTime:         req.GetCutoffTime(),
		ProcessingWindow:   req.GetProcessingWindow(),
		SlaRef:             req.GetSlaRef(),
		Fallback:           req.GetFallback(),
		SupportContact:     req.GetSupportContact(),
		IncidentContact:    req.GetIncidentContact(),
	}, nil
}

func listWithin(field string, items []string, ceiling int) error {
	if len(items) > ceiling {
		return status.Errorf(codes.InvalidArgument, "%s must hold at most %d items", field, ceiling)
	}
	for _, item := range items {
		if err := asciiWithin(field, item, codeCeiling); err != nil {
			return err
		}
	}
	return nil
}

func asciiWithin(field, value string, ceiling int) error {
	if len(value) > ceiling || !printableASCII.MatchString(value) {
		return status.Errorf(codes.InvalidArgument, "%s must have at most %d bytes of printable ASCII", field, ceiling)
	}
	return nil
}

func textWithin(field, value string) error {
	if utf8.RuneCountInString(value) > textCeiling || !controlFree.MatchString(value) {
		return status.Errorf(codes.InvalidArgument, "%s must have at most %d characters, none of them a control character", field, textCeiling)
	}
	return nil
}

var environmentsOnTheWire = map[servicev1.Environment]domain.Environment{
	servicev1.Environment_ENVIRONMENT_DEVELOPMENT:  domain.EnvironmentDevelopment,
	servicev1.Environment_ENVIRONMENT_INTEGRATION:  domain.EnvironmentIntegration,
	servicev1.Environment_ENVIRONMENT_HOMOLOGATION: domain.EnvironmentHomologation,
	servicev1.Environment_ENVIRONMENT_PRODUCTION:   domain.EnvironmentProduction,
}

var changeKindsOnTheWire = map[servicev1.IntegrationChangeKind]domain.IntegrationChangeKind{
	servicev1.IntegrationChangeKind_INTEGRATION_CHANGE_KIND_INITIAL:     domain.ChangeInitial,
	servicev1.IntegrationChangeKind_INTEGRATION_CHANGE_KIND_ANNOUNCED:   domain.ChangeAnnounced,
	servicev1.IntegrationChangeKind_INTEGRATION_CHANGE_KIND_UNANNOUNCED: domain.ChangeUnannounced,
}
