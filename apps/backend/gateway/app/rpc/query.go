package rpc

import (
	"context"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func (s Server) FindConnector(ctx context.Context, req *servicev1.FindConnectorRequest) (*servicev1.FindConnectorResponse, error) {
	if err := requireID("connector_key", req.GetConnectorKey()); err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	connector, err := s.Service.FindConnector(ctx, domain.ConnectorKey(req.GetConnectorKey()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindConnectorResponse{Connector: connectorOnTheWire(connector)}, nil
}

func (s Server) FindEffectiveCapabilityProfile(ctx context.Context, req *servicev1.FindEffectiveCapabilityProfileRequest) (*servicev1.FindEffectiveCapabilityProfileResponse, error) {
	if err := requireID("connector_id", req.GetConnectorId()); err != nil {
		return nil, err
	}
	if req.GetEffectiveAt() <= 0 {
		return nil, invalid("effective_at", "must be a positive instant in nanoseconds")
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	profile, err := s.Service.FindEffectiveCapabilityProfile(ctx, domain.ConnectorKey(req.GetConnectorId()), kernel.Instant(req.GetEffectiveAt()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindEffectiveCapabilityProfileResponse{Profile: profileOnTheWire(profile)}, nil
}

func (s Server) FindCapabilityProfile(ctx context.Context, req *servicev1.FindCapabilityProfileRequest) (*servicev1.FindCapabilityProfileResponse, error) {
	if err := requireID("connector_id", req.GetConnectorId()); err != nil {
		return nil, err
	}
	if err := requireVersion("version", req.GetVersion()); err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	profile, err := s.Service.FindCapabilityProfile(ctx, domain.ConnectorKey(req.GetConnectorId()), int(req.GetVersion()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindCapabilityProfileResponse{Profile: profileOnTheWire(profile)}, nil
}

func (s Server) FindQueryAuthorizationByOrganization(ctx context.Context, req *servicev1.FindQueryAuthorizationByOrganizationRequest) (*servicev1.FindQueryAuthorizationByOrganizationResponse, error) {
	if err := requireID("organization_id", req.GetOrganizationId()); err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindQueryAuthorizationByOrganization(ctx, domain.OrganizationID(req.GetOrganizationId()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindQueryAuthorizationByOrganizationResponse{Authorizations: onTheWire(found, authorizationOnTheWire)}, nil
}

func (s Server) FindExternalCommand(ctx context.Context, req *servicev1.FindExternalCommandRequest) (*servicev1.FindExternalCommandResponse, error) {
	if err := requireIdempotencyKey(req.GetIdempotencyKey()); err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	command, err := s.Service.FindExternalCommand(ctx, domain.IdempotencyKey(req.GetIdempotencyKey()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalCommandResponse{Command: commandOnTheWire(command)}, nil
}

func (s Server) FindExternalCommandsByBusinessKey(ctx context.Context, req *servicev1.FindExternalCommandsByBusinessKeyRequest) (*servicev1.FindExternalCommandsByBusinessKeyResponse, error) {
	if err := requireID("connector_id", req.GetConnectorId()); err != nil {
		return nil, err
	}
	if req.GetBusinessKey() == "" || !keyFormat.MatchString(req.GetBusinessKey()) {
		return nil, invalid("business_key", "must have 1 to 128 characters of [A-Za-z0-9._:/-]")
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindExternalCommandsByBusinessKey(ctx, domain.ConnectorKey(req.GetConnectorId()), req.GetBusinessKey())
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalCommandsByBusinessKeyResponse{Commands: onTheWire(found, commandOnTheWire)}, nil
}

func (s Server) FindExternalCommandsByStatus(ctx context.Context, req *servicev1.FindExternalCommandsByStatusRequest) (*servicev1.FindExternalCommandsByStatusResponse, error) {
	status, err := enumFromTheWire("status", statusesFromTheWire, req.GetStatus())
	if err != nil {
		return nil, err
	}
	if err := optionalID("connector_id", req.GetConnectorId()); err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindExternalCommandsByStatus(ctx, status, domain.ConnectorKey(req.GetConnectorId()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalCommandsByStatusResponse{Commands: onTheWire(found, commandOnTheWire)}, nil
}
