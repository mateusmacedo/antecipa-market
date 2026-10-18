package rpc

import (
	"context"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const environmentRefusal = "environment must be one of development, integration, homologation or production"

func (s Server) FindPartner(ctx context.Context, req *servicev1.FindPartnerRequest) (*servicev1.FindPartnerResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindPartner(ctx, domain.PartnerID(partner))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindPartnerResponse{Partner: partnerOnTheWire(found)}, nil
}

func (s Server) FindPartnersByOrganization(ctx context.Context, req *servicev1.FindPartnersByOrganizationRequest) (*servicev1.FindPartnersByOrganizationResponse, error) {
	organization, err := identifier("organization_id", req.GetOrganizationId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindPartnersByOrganization(ctx, domain.OrganizationID(organization))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindPartnersByOrganizationResponse{Partners: onTheWire(found, partnerOnTheWire)}, nil
}

func (s Server) FindAgreementsByPartner(ctx context.Context, req *servicev1.FindAgreementsByPartnerRequest) (*servicev1.FindAgreementsByPartnerResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindAgreementsByPartner(ctx, domain.PartnerID(partner))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindAgreementsByPartnerResponse{Agreements: onTheWire(found, agreementOnTheWire)}, nil
}

func (s Server) FindCapabilitiesByPartner(ctx context.Context, req *servicev1.FindCapabilitiesByPartnerRequest) (*servicev1.FindCapabilitiesByPartnerResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	var environment domain.Environment
	if req.GetEnvironment() != servicev1.Environment_ENVIRONMENT_UNSPECIFIED {
		if environment, err = known(environmentsOnTheWire, req.GetEnvironment(), environmentRefusal); err != nil {
			return nil, err
		}
	}
	var capabilityStatus domain.CapabilityStatus
	if req.GetStatus() != servicev1.CapabilityStatus_CAPABILITY_STATUS_UNSPECIFIED {
		capabilityStatus, err = known(capabilityStatusesFromTheWire, req.GetStatus(), "status must be one of declared, homologated, enabled or suspended")
		if err != nil {
			return nil, err
		}
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindCapabilitiesByPartner(ctx, application.FindCapabilitiesByPartner{
		Partner: domain.PartnerID(partner), Environment: environment, Status: capabilityStatus,
	})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindCapabilitiesByPartnerResponse{Capabilities: onTheWire(found, capabilityOnTheWire)}, nil
}

func (s Server) FindEffectiveCapability(ctx context.Context, req *servicev1.FindEffectiveCapabilityRequest) (*servicev1.FindEffectiveCapabilityResponse, error) {
	q, err := effectiveQueryOf(req)
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	resolved, err := s.Service.FindEffectiveCapability(ctx, q)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindEffectiveCapabilityResponse{Allowed: resolved.Allowed, FailedCondition: conditionsOnTheWire[resolved.FailedCondition]}, nil
}

func effectiveQueryOf(req *servicev1.FindEffectiveCapabilityRequest) (application.FindEffectiveCapability, error) {
	var none application.FindEffectiveCapability
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return none, err
	}
	product, err := optionalIdentifier("product_id", req.GetProductId())
	if err != nil {
		return none, err
	}
	series, err := seriesOf(req.GetOperation(), req.GetAssetType(), req.GetEnvironment(), req.GetTenantId(), req.GetAsOf())
	if err != nil {
		return none, err
	}
	if req.AmountMinor != nil && req.GetAmountMinor() < 0 || req.TermDays != nil && req.GetTermDays() < 0 {
		return none, status.Error(codes.InvalidArgument, "amount_minor and term_days must not be negative")
	}
	var termDays *int
	if req.TermDays != nil {
		days := int(req.GetTermDays())
		termDays = &days
	}
	return application.FindEffectiveCapability{
		Partner:     domain.PartnerID(partner),
		Operation:   series.operation,
		AssetType:   series.assetType,
		Environment: series.environment,
		TenantID:    series.tenantID,
		AsOf:        series.asOf,
		ProductID:   domain.ProductID(product),
		AmountMinor: req.AmountMinor,
		TermDays:    termDays,
	}, nil
}

type capabilitySeries struct {
	operation, assetType, tenantID string
	environment                    domain.Environment
	asOf                           kernel.Instant
}

func seriesOf(operation, assetType string, environment servicev1.Environment, tenantID string, asOf int64) (capabilitySeries, error) {
	var none capabilitySeries
	for _, field := range []struct{ name, value string }{{"operation", operation}, {"asset_type", assetType}} {
		if err := asciiWithin(field.name, field.value, codeCeiling); err != nil {
			return none, err
		}
	}
	env, err := known(environmentsOnTheWire, environment, environmentRefusal)
	if err != nil {
		return none, err
	}
	tenant, err := identifier("tenant_id", tenantID)
	if err != nil {
		return none, err
	}
	instant, err := positiveInstant("as_of", asOf)
	if err != nil {
		return none, err
	}
	return capabilitySeries{operation: operation, assetType: assetType, tenantID: tenant, environment: env, asOf: instant}, nil
}

func (s Server) FindEligiblePartners(ctx context.Context, req *servicev1.FindEligiblePartnersRequest) (*servicev1.FindEligiblePartnersResponse, error) {
	series, err := seriesOf(req.GetOperation(), req.GetAssetType(), req.GetEnvironment(), req.GetTenantId(), req.GetAsOf())
	if err != nil {
		return nil, err
	}
	if req.GetRegion() == "" {
		return nil, status.Error(codes.InvalidArgument, "region must not be empty")
	}
	if err := asciiWithin("region", req.GetRegion(), codeCeiling); err != nil {
		return nil, err
	}
	if req.GetAmountMinor() < 0 || req.GetTermDays() < 0 {
		return nil, status.Error(codes.InvalidArgument, "amount_minor and term_days must not be negative")
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindEligiblePartners(ctx, application.FindEligiblePartners{
		AssetType:   series.assetType,
		Operation:   series.operation,
		Environment: series.environment,
		TenantID:    series.tenantID,
		AmountMinor: req.GetAmountMinor(),
		TermDays:    int(req.GetTermDays()),
		Region:      req.GetRegion(),
		AsOf:        series.asOf,
	})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindEligiblePartnersResponse{Partners: onTheWire(found, partnerOnTheWire)}, nil
}

func (s Server) FindProductsByPartner(ctx context.Context, req *servicev1.FindProductsByPartnerRequest) (*servicev1.FindProductsByPartnerResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	asOf, err := positiveInstant("as_of", req.GetAsOf())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindProductsByPartner(ctx, application.FindProductsByPartner{Partner: domain.PartnerID(partner), AsOf: asOf})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindProductsByPartnerResponse{Products: onTheWire(found, productOnTheWire)}, nil
}

func (s Server) FindIntegrationProfile(ctx context.Context, req *servicev1.FindIntegrationProfileRequest) (*servicev1.FindIntegrationProfileResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	environment, err := known(environmentsOnTheWire, req.GetEnvironment(), environmentRefusal)
	if err != nil {
		return nil, err
	}
	asOf, err := positiveInstant("as_of", req.GetAsOf())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindIntegrationProfile(ctx, application.FindIntegrationProfile{
		Partner: domain.PartnerID(partner), Environment: environment, AsOf: asOf,
	})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindIntegrationProfileResponse{Profile: profileOnTheWire(found)}, nil
}

func (s Server) FindOperationalPolicy(ctx context.Context, req *servicev1.FindOperationalPolicyRequest) (*servicev1.FindOperationalPolicyResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	asOf, err := positiveInstant("as_of", req.GetAsOf())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindOperationalPolicy(ctx, application.FindOperationalPolicy{Partner: domain.PartnerID(partner), AsOf: asOf})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindOperationalPolicyResponse{Policy: policyOnTheWire(found)}, nil
}

func (s Server) FindCertificatesExpiring(ctx context.Context, req *servicev1.FindCertificatesExpiringRequest) (*servicev1.FindCertificatesExpiringResponse, error) {
	environment, err := known(environmentsOnTheWire, req.GetEnvironment(), environmentRefusal)
	if err != nil {
		return nil, err
	}
	expiresBefore, err := positiveInstant("expires_before", req.GetExpiresBefore())
	if err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindCertificatesExpiring(ctx, application.FindCertificatesExpiring{Environment: environment, ExpiresBefore: expiresBefore})
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindCertificatesExpiringResponse{Profiles: onTheWire(found, profileOnTheWire)}, nil
}

func positiveInstant(field string, raw int64) (kernel.Instant, error) {
	if raw <= 0 {
		return 0, status.Errorf(codes.InvalidArgument, "%s must be a positive instant in nanoseconds", field)
	}
	return kernel.Instant(raw), nil
}
