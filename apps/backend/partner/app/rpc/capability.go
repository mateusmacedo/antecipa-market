package rpc

import (
	"context"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const (
	maxHomologationResults = 12
	maxRolloutTenants      = 1000
)

func (s Server) ChangePartnerStage(ctx context.Context, req *servicev1.ChangePartnerStageRequest) (*servicev1.ChangePartnerStageResponse, error) {
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return nil, err
	}
	target, err := known(targetsOnTheWire, req.GetTarget(), "target must be one of contracting, integrating, homologating, active, suspended or offboarding")
	if err != nil {
		return nil, err
	}
	if err := textWithin("reason", req.GetReason()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ChangePartnerStageResponse], error) {
			return s.Service.ChangePartnerStage(ctx, application.ChangePartnerStage{PartnerID: domain.PartnerID(partner), Target: target, Reason: req.GetReason()})
		},
		func(r *kernel.Rejection) *servicev1.ChangePartnerStageResponse {
			return &servicev1.ChangePartnerStageResponse{Result: &servicev1.ChangePartnerStageResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(changed domain.ChangePartnerStageResponse) *servicev1.ChangePartnerStageResponse {
			return &servicev1.ChangePartnerStageResponse{Result: &servicev1.ChangePartnerStageResponse_Changed{
				Changed: &servicev1.ChangedPartnerStage{PartnerId: string(changed.PartnerID), Status: partnerStatusesOnTheWire[changed.Status]},
			}}
		})
}

func (s Server) DeclareCapability(ctx context.Context, req *servicev1.DeclareCapabilityRequest) (*servicev1.DeclareCapabilityResponse, error) {
	cmd, err := declarationOf(req)
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.DeclareCapabilityResponse], error) {
			return s.Service.DeclareCapability(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.DeclareCapabilityResponse {
			return &servicev1.DeclareCapabilityResponse{Result: &servicev1.DeclareCapabilityResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(declared domain.DeclareCapabilityResponse) *servicev1.DeclareCapabilityResponse {
			return &servicev1.DeclareCapabilityResponse{Result: &servicev1.DeclareCapabilityResponse_Declared{
				Declared: &servicev1.DeclaredCapability{CapabilityId: string(declared.CapabilityID)},
			}}
		})
}

func declarationOf(req *servicev1.DeclareCapabilityRequest) (application.DeclareCapability, error) {
	var none application.DeclareCapability
	capability, err := identifier("capability_id", req.GetCapabilityId())
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
	for _, field := range []struct{ name, value string }{
		{"operation", req.GetOperation()},
		{"asset_type", req.GetAssetType()},
	} {
		if err := asciiWithin(field.name, field.value, codeCeiling); err != nil {
			return none, err
		}
	}
	if err := textWithin("restriction_note", req.GetRestrictionNote()); err != nil {
		return none, err
	}
	return application.DeclareCapability{
		CapabilityID:        domain.CapabilityID(capability),
		PartnerID:           domain.PartnerID(partner),
		Operation:           req.GetOperation(),
		AssetType:           req.GetAssetType(),
		Environment:         environment,
		IsWrite:             req.GetIsWrite(),
		StateQuerySupported: req.GetStateQuerySupported(),
		RestrictionNote:     req.GetRestrictionNote(),
		CallbackApplicable:  req.GetCallbackApplicable(),
		SupersedesID:        domain.CapabilityID(supersedes),
	}, nil
}

func (s Server) RecordHomologation(ctx context.Context, req *servicev1.RecordHomologationRequest) (*servicev1.RecordHomologationResponse, error) {
	capability, err := identifier("capability_id", req.GetCapabilityId())
	if err != nil {
		return nil, err
	}
	results, err := homologationResultsOf(req.GetResults())
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RecordHomologationResponse], error) {
			return s.Service.RecordHomologation(ctx, application.RecordHomologation{CapabilityID: domain.CapabilityID(capability), Results: results})
		},
		func(r *kernel.Rejection) *servicev1.RecordHomologationResponse {
			return &servicev1.RecordHomologationResponse{Result: &servicev1.RecordHomologationResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(recorded domain.RecordHomologationResponse) *servicev1.RecordHomologationResponse {
			return &servicev1.RecordHomologationResponse{Result: &servicev1.RecordHomologationResponse_Recorded{
				Recorded: &servicev1.RecordedHomologation{CapabilityId: string(recorded.CapabilityID), Status: capabilityStatusesOnTheWire[recorded.Status]},
			}}
		})
}

func homologationResultsOf(given []*servicev1.HomologationResult) ([]domain.HomologationResult, error) {
	if len(given) > maxHomologationResults {
		return nil, status.Errorf(codes.InvalidArgument, "results must hold at most %d items", maxHomologationResults)
	}
	results := make([]domain.HomologationResult, 0, len(given))
	for _, result := range given {
		if err := asciiWithin("criterion_id", result.GetCriterionId(), codeCeiling); err != nil {
			return nil, err
		}
		if err := asciiWithin("evidence_id", result.GetEvidenceId(), referenceCeiling); err != nil {
			return nil, err
		}
		results = append(results, domain.HomologationResult{CriterionID: result.GetCriterionId(), Passed: result.GetPassed(), EvidenceID: result.GetEvidenceId()})
	}
	return results, nil
}

func (s Server) EnableCapability(ctx context.Context, req *servicev1.EnableCapabilityRequest) (*servicev1.EnableCapabilityResponse, error) {
	capability, err := identifier("capability_id", req.GetCapabilityId())
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.EnableCapabilityResponse], error) {
			return s.Service.EnableCapability(ctx, application.EnableCapability{CapabilityID: domain.CapabilityID(capability)})
		},
		func(r *kernel.Rejection) *servicev1.EnableCapabilityResponse {
			return &servicev1.EnableCapabilityResponse{Result: &servicev1.EnableCapabilityResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(enabled domain.EnableCapabilityResponse) *servicev1.EnableCapabilityResponse {
			return &servicev1.EnableCapabilityResponse{Result: &servicev1.EnableCapabilityResponse_Enabled{
				Enabled: &servicev1.EnabledCapability{CapabilityId: string(enabled.CapabilityID)},
			}}
		})
}

// SuspendCapability takes no trigger from the caller: the service suspends by
// hand, and schema_mismatch and offboarding are left to internal flows.
func (s Server) SuspendCapability(ctx context.Context, req *servicev1.SuspendCapabilityRequest) (*servicev1.SuspendCapabilityResponse, error) {
	capability, err := identifier("capability_id", req.GetCapabilityId())
	if err != nil {
		return nil, err
	}
	if err := textWithin("reason", req.GetReason()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.SuspendCapabilityResponse], error) {
			return s.Service.SuspendCapability(ctx, application.SuspendCapability{
				CapabilityID: domain.CapabilityID(capability),
				Reason:       req.GetReason(),
				Trigger:      domain.TriggerManual,
			})
		},
		func(r *kernel.Rejection) *servicev1.SuspendCapabilityResponse {
			return &servicev1.SuspendCapabilityResponse{Result: &servicev1.SuspendCapabilityResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(suspended domain.SuspendCapabilityResponse) *servicev1.SuspendCapabilityResponse {
			return &servicev1.SuspendCapabilityResponse{Result: &servicev1.SuspendCapabilityResponse_Suspended{
				Suspended: &servicev1.SuspendedCapability{CapabilityId: string(suspended.CapabilityID)},
			}}
		})
}

func (s Server) ChangeRollout(ctx context.Context, req *servicev1.ChangeRolloutRequest) (*servicev1.ChangeRolloutResponse, error) {
	capability, err := identifier("capability_id", req.GetCapabilityId())
	if err != nil {
		return nil, err
	}
	mode, err := known(rolloutModesOnTheWire, req.GetMode(), "mode must be one of disabled, allowlist, percentage or enabled")
	if err != nil {
		return nil, err
	}
	for field, tenants := range map[string][]string{
		"allowlist_tenant_ids": req.GetAllowlistTenantIds(),
		"excluded_tenant_ids":  req.GetExcludedTenantIds(),
	} {
		if err := tenantsWithin(field, tenants); err != nil {
			return nil, err
		}
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ChangeRolloutResponse], error) {
			return s.Service.ChangeRollout(ctx, application.ChangeRollout{
				CapabilityID:       domain.CapabilityID(capability),
				Mode:               mode,
				AllowlistTenantIDs: req.GetAllowlistTenantIds(),
				Percentage:         int(req.GetPercentage()),
				ExcludedTenantIDs:  req.GetExcludedTenantIds(),
			})
		},
		func(r *kernel.Rejection) *servicev1.ChangeRolloutResponse {
			return &servicev1.ChangeRolloutResponse{Result: &servicev1.ChangeRolloutResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(changed domain.ChangeRolloutResponse) *servicev1.ChangeRolloutResponse {
			return &servicev1.ChangeRolloutResponse{Result: &servicev1.ChangeRolloutResponse_Changed{
				Changed: &servicev1.ChangedRollout{CapabilityId: string(changed.CapabilityID)},
			}}
		})
}

func tenantsWithin(field string, tenants []string) error {
	if len(tenants) > maxRolloutTenants {
		return status.Errorf(codes.InvalidArgument, "%s must hold at most %d items", field, maxRolloutTenants)
	}
	seen := make(map[string]struct{}, len(tenants))
	for _, tenant := range tenants {
		if _, err := identifier(field, tenant); err != nil {
			return err
		}
		if _, repeated := seen[tenant]; repeated {
			return status.Errorf(codes.InvalidArgument, "%s must not repeat a tenant", field)
		}
		seen[tenant] = struct{}{}
	}
	return nil
}

var targetsOnTheWire = map[servicev1.PartnerStatus]domain.PartnerStatus{
	servicev1.PartnerStatus_PARTNER_STATUS_CONTRACTING:  domain.PartnerContracting,
	servicev1.PartnerStatus_PARTNER_STATUS_INTEGRATING:  domain.PartnerIntegrating,
	servicev1.PartnerStatus_PARTNER_STATUS_HOMOLOGATING: domain.PartnerHomologating,
	servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE:       domain.PartnerActive,
	servicev1.PartnerStatus_PARTNER_STATUS_SUSPENDED:    domain.PartnerSuspended,
	servicev1.PartnerStatus_PARTNER_STATUS_OFFBOARDING:  domain.PartnerOffboarding,
}

var partnerStatusesOnTheWire = map[domain.PartnerStatus]servicev1.PartnerStatus{
	domain.PartnerProspect:     servicev1.PartnerStatus_PARTNER_STATUS_PROSPECT,
	domain.PartnerContracting:  servicev1.PartnerStatus_PARTNER_STATUS_CONTRACTING,
	domain.PartnerIntegrating:  servicev1.PartnerStatus_PARTNER_STATUS_INTEGRATING,
	domain.PartnerHomologating: servicev1.PartnerStatus_PARTNER_STATUS_HOMOLOGATING,
	domain.PartnerActive:       servicev1.PartnerStatus_PARTNER_STATUS_ACTIVE,
	domain.PartnerSuspended:    servicev1.PartnerStatus_PARTNER_STATUS_SUSPENDED,
	domain.PartnerOffboarding:  servicev1.PartnerStatus_PARTNER_STATUS_OFFBOARDING,
}

var capabilityStatusesOnTheWire = map[domain.CapabilityStatus]servicev1.CapabilityStatus{
	domain.CapabilityStatusDeclared:    servicev1.CapabilityStatus_CAPABILITY_STATUS_DECLARED,
	domain.CapabilityStatusHomologated: servicev1.CapabilityStatus_CAPABILITY_STATUS_HOMOLOGATED,
	domain.CapabilityStatusEnabled:     servicev1.CapabilityStatus_CAPABILITY_STATUS_ENABLED,
	domain.CapabilityStatusSuspended:   servicev1.CapabilityStatus_CAPABILITY_STATUS_SUSPENDED,
}

var rolloutModesOnTheWire = map[servicev1.RolloutMode]domain.RolloutMode{
	servicev1.RolloutMode_ROLLOUT_MODE_DISABLED:   domain.RolloutDisabled,
	servicev1.RolloutMode_ROLLOUT_MODE_ALLOWLIST:  domain.RolloutAllowlist,
	servicev1.RolloutMode_ROLLOUT_MODE_PERCENTAGE: domain.RolloutPercentage,
	servicev1.RolloutMode_ROLLOUT_MODE_ENABLED:    domain.RolloutEnabled,
}
