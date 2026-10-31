package rpc

import (
	"context"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func (s Server) RegisterConnector(ctx context.Context, req *servicev1.RegisterConnectorRequest) (*servicev1.RegisterConnectorResponse, error) {
	if err := requireID("partner_id", req.GetPartnerId()); err != nil {
		return nil, err
	}
	environment, err := enumFromTheWire("environment", environmentsFromTheWire, req.GetEnvironment())
	if err != nil {
		return nil, err
	}
	if !idFormat.MatchString(string(domain.ConnectorKeyOf(domain.PartnerID(req.GetPartnerId()), environment))) {
		return nil, invalid("partner_id", "must leave room for the environment within the 128 characters of the connector key")
	}
	if err := requireText("credential_ref", req.GetCredentialRef(), maxTextRunes); err != nil {
		return nil, err
	}
	if err := requireVersion("integration_version", req.GetIntegrationVersion()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ConnectorResponse], error) {
			return s.Service.RegisterConnector(ctx, application.RegisterConnector{
				PartnerID: domain.PartnerID(req.GetPartnerId()), Environment: environment,
				CredentialRef: req.GetCredentialRef(), IntegrationVersion: int(req.GetIntegrationVersion()),
			})
		},
		func(r *kernel.Rejection) *servicev1.RegisterConnectorResponse {
			return &servicev1.RegisterConnectorResponse{Result: &servicev1.RegisterConnectorResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.ConnectorResponse) *servicev1.RegisterConnectorResponse {
			return &servicev1.RegisterConnectorResponse{Result: &servicev1.RegisterConnectorResponse_Registered{
				Registered: &servicev1.RegisteredConnector{ConnectorKey: string(c.ConnectorKey)},
			}}
		})
}

func (s Server) SuspendOperation(ctx context.Context, req *servicev1.SuspendOperationRequest) (*servicev1.SuspendOperationResponse, error) {
	cmd, err := suspensionOf(req)
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ConnectorResponse], error) {
			return s.Service.SuspendOperation(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.SuspendOperationResponse {
			return &servicev1.SuspendOperationResponse{Result: &servicev1.SuspendOperationResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.ConnectorResponse) *servicev1.SuspendOperationResponse {
			return &servicev1.SuspendOperationResponse{Result: &servicev1.SuspendOperationResponse_Suspended{
				Suspended: &servicev1.SuspendedOperation{ConnectorKey: string(c.ConnectorKey)},
			}}
		})
}

func suspensionOf(req *servicev1.SuspendOperationRequest) (application.SuspendOperation, error) {
	var none application.SuspendOperation
	if err := requireID("connector_key", req.GetConnectorKey()); err != nil {
		return none, err
	}
	if err := requireOperation("operation", req.GetOperation(), true); err != nil {
		return none, err
	}
	if err := requireAssetType(req.GetAssetType()); err != nil {
		return none, err
	}
	cause, err := enumFromTheWire("cause", causesFromTheWire, req.GetCause())
	if err != nil {
		return none, err
	}
	if err := optionalID("capability_id", req.GetCapabilityId()); err != nil {
		return none, err
	}
	if err := requireVersion("version", req.GetVersion()); err != nil {
		return none, err
	}
	return application.SuspendOperation{
		ConnectorKey: domain.ConnectorKey(req.GetConnectorKey()), Operation: req.GetOperation(), AssetType: req.GetAssetType(),
		Cause: cause, CapabilityID: domain.CapabilityID(req.GetCapabilityId()), Version: int(req.GetVersion()),
	}, nil
}

func (s Server) EnableOperation(ctx context.Context, req *servicev1.EnableOperationRequest) (*servicev1.EnableOperationResponse, error) {
	if err := requireID("connector_key", req.GetConnectorKey()); err != nil {
		return nil, err
	}
	if err := requireID("capability_id", req.GetCapabilityId()); err != nil {
		return nil, err
	}
	if err := requireOperation("operation", req.GetOperation(), false); err != nil {
		return nil, err
	}
	if err := requireAssetType(req.GetAssetType()); err != nil {
		return nil, err
	}
	if err := requireVersion("version", req.GetVersion()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ConnectorResponse], error) {
			return s.Service.EnableOperation(ctx, application.EnableOperation{
				ConnectorKey: domain.ConnectorKey(req.GetConnectorKey()), CapabilityID: domain.CapabilityID(req.GetCapabilityId()),
				Operation: req.GetOperation(), AssetType: req.GetAssetType(), StateQuerySupported: req.GetStateQuerySupported(),
				Version: int(req.GetVersion()),
			})
		},
		func(r *kernel.Rejection) *servicev1.EnableOperationResponse {
			return &servicev1.EnableOperationResponse{Result: &servicev1.EnableOperationResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.ConnectorResponse) *servicev1.EnableOperationResponse {
			return &servicev1.EnableOperationResponse{Result: &servicev1.EnableOperationResponse_Enabled{
				Enabled: &servicev1.EnabledOperation{ConnectorKey: string(c.ConnectorKey)},
			}}
		})
}

func (s Server) DeclareCapabilityProfile(ctx context.Context, req *servicev1.DeclareCapabilityProfileRequest) (*servicev1.DeclareCapabilityProfileResponse, error) {
	if err := requireID("connector_id", req.GetConnectorId()); err != nil {
		return nil, err
	}
	if err := requireVersion("version", req.GetVersion()); err != nil {
		return nil, err
	}
	operations := make([]domain.OperationCapability, 0, len(req.GetOperations()))
	for _, o := range req.GetOperations() {
		operation, err := operationFromTheWire(o)
		if err != nil {
			return nil, err
		}
		operations = append(operations, operation)
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ProfileResponse], error) {
			return s.Service.DeclareCapabilityProfile(ctx, application.DeclareCapabilityProfile{
				ConnectorKey: domain.ConnectorKey(req.GetConnectorId()), Version: int(req.GetVersion()),
				EffectiveFrom: kernel.Instant(req.GetEffectiveFrom()), Operations: operations,
			})
		},
		func(r *kernel.Rejection) *servicev1.DeclareCapabilityProfileResponse {
			return &servicev1.DeclareCapabilityProfileResponse{Result: &servicev1.DeclareCapabilityProfileResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(p domain.ProfileResponse) *servicev1.DeclareCapabilityProfileResponse {
			return &servicev1.DeclareCapabilityProfileResponse{Result: &servicev1.DeclareCapabilityProfileResponse_Declared{
				Declared: &servicev1.DeclaredCapabilityProfile{ProfileKey: string(p.ProfileKey)},
			}}
		})
}

// operationFromTheWire bounds only the form: whether the declaration lets a
// command run is the kernel's ValidateExternalProfile, answered as a rejection.
func operationFromTheWire(o *servicev1.OperationCapability) (domain.OperationCapability, error) {
	var none domain.OperationCapability
	if err := requireOperation("operations.operation", o.GetOperation(), false); err != nil {
		return none, err
	}
	for _, assetType := range o.GetAssetTypes() {
		if err := requireAssetType(assetType); err != nil {
			return none, err
		}
	}
	for field, text := range map[string]string{
		"operations.business_calendar": o.GetBusinessCalendar(), "operations.cutoff_time": o.GetCutoffTime(),
		"operations.processing_window": o.GetProcessingWindow(), "operations.key_alphabet": o.GetKeyAlphabet(),
	} {
		if err := requireText(field, text, maxTextRunes); err != nil {
			return none, err
		}
	}
	protocol, err := enumFromTheWire("operations.protocol", protocolsFromTheWire, o.GetProtocol())
	if err != nil {
		return none, err
	}
	coverage, err := enumFromTheWire("operations.encumbrance_coverage", coveragesFromTheWire, o.GetEncumbranceCoverage())
	if err != nil {
		return none, err
	}
	return domain.OperationCapability{
		Operation: o.GetOperation(), Supported: o.GetSupported(), Protocol: protocol,
		StateQuerySupported: o.GetStateQuerySupported(), IdempotencySupport: idempotencySupportFromTheWire[o.GetIdempotencySupport()],
		PartialResultSupported: o.GetPartialResultSupported(), AssetTypes: append([]string(nil), o.GetAssetTypes()...),
		MaxItemsPerRequest: int(o.GetMaxItemsPerRequest()), MaxRequestsPerMinute: int(o.GetMaxRequestsPerMinute()),
		MaxPayloadBytes: int(o.GetMaxPayloadBytes()), BusinessCalendar: o.GetBusinessCalendar(), CutoffTime: o.GetCutoffTime(),
		ProcessingWindow: o.GetProcessingWindow(), EncumbranceCoverage: coverage,
		PositionFreshnessSLA: int(o.GetPositionFreshnessSla()), Efficacy: efficaciesFromTheWire[o.GetEfficacy()],
		Quiescence: o.GetQuiescence(), AuthoritativeAbsence: o.GetAuthoritativeAbsence(), MaxQueries: int(o.GetMaxQueries()),
		KeyAlphabet: o.GetKeyAlphabet(), KeyMaxLength: int(o.GetKeyMaxLength()),
		QueryInterval: int(o.GetQueryInterval()), AmbiguityDeadline: int(o.GetAmbiguityDeadline()),
	}, nil
}

func (s Server) GrantQueryAuthorization(ctx context.Context, req *servicev1.GrantQueryAuthorizationRequest) (*servicev1.GrantQueryAuthorizationResponse, error) {
	if err := requireID("consent_id", req.GetConsentId()); err != nil {
		return nil, err
	}
	if err := requireID("organization_id", req.GetOrganizationId()); err != nil {
		return nil, err
	}
	if err := requireVersion("consent_version", req.GetConsentVersion()); err != nil {
		return nil, err
	}
	if len(req.GetAssetTypes()) == 0 {
		return nil, invalid("asset_types", "must name at least one asset type, the scope of the consent")
	}
	for _, assetType := range req.GetAssetTypes() {
		if err := requireAssetType(assetType); err != nil {
			return nil, err
		}
	}
	if err := requireDate("period_from_date", req.GetPeriodFromDate()); err != nil {
		return nil, err
	}
	if err := requireDate("period_until_date", req.GetPeriodUntilDate()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.AuthorizationResponse], error) {
			return s.Service.GrantQueryAuthorization(ctx, application.GrantQueryAuthorization{
				ConsentID: domain.ConsentID(req.GetConsentId()), OrganizationID: domain.OrganizationID(req.GetOrganizationId()),
				ConsentVersion: int(req.GetConsentVersion()), AssetTypes: append([]string(nil), req.GetAssetTypes()...),
				PeriodFromDate: req.GetPeriodFromDate(), PeriodUntilDate: req.GetPeriodUntilDate(),
			})
		},
		func(r *kernel.Rejection) *servicev1.GrantQueryAuthorizationResponse {
			return &servicev1.GrantQueryAuthorizationResponse{Result: &servicev1.GrantQueryAuthorizationResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(a domain.AuthorizationResponse) *servicev1.GrantQueryAuthorizationResponse {
			return &servicev1.GrantQueryAuthorizationResponse{Result: &servicev1.GrantQueryAuthorizationResponse_Granted{
				Granted: &servicev1.GrantedQueryAuthorization{ConsentId: string(a.ConsentID)},
			}}
		})
}

func (s Server) RevokeQueryAuthorization(ctx context.Context, req *servicev1.RevokeQueryAuthorizationRequest) (*servicev1.RevokeQueryAuthorizationResponse, error) {
	if err := requireID("consent_id", req.GetConsentId()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.AuthorizationResponse], error) {
			return s.Service.RevokeQueryAuthorization(ctx, application.RevokeQueryAuthorization{
				ConsentID: domain.ConsentID(req.GetConsentId()), EffectiveAt: kernel.Instant(req.GetEffectiveAt()),
			})
		},
		func(r *kernel.Rejection) *servicev1.RevokeQueryAuthorizationResponse {
			return &servicev1.RevokeQueryAuthorizationResponse{Result: &servicev1.RevokeQueryAuthorizationResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(a domain.AuthorizationResponse) *servicev1.RevokeQueryAuthorizationResponse {
			return &servicev1.RevokeQueryAuthorizationResponse{Result: &servicev1.RevokeQueryAuthorizationResponse_Revoked{
				Revoked: &servicev1.RevokedQueryAuthorization{ConsentId: string(a.ConsentID)},
			}}
		})
}
