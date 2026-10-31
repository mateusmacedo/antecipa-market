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

func (s Server) CreateExternalCommand(ctx context.Context, req *servicev1.CreateExternalCommandRequest) (*servicev1.CreateExternalCommandResponse, error) {
	cmd, err := creationOf(req)
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.CommandStatusResponse], error) {
			return s.Service.CreateExternalCommand(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.CreateExternalCommandResponse {
			return &servicev1.CreateExternalCommandResponse{Result: &servicev1.CreateExternalCommandResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.CommandStatusResponse) *servicev1.CreateExternalCommandResponse {
			return &servicev1.CreateExternalCommandResponse{Result: &servicev1.CreateExternalCommandResponse_Created{
				Created: &servicev1.CreatedExternalCommand{IdempotencyKey: string(c.IdempotencyKey), Status: statusesToTheWire[c.Status]},
			}}
		})
}

// creationOf bounds the form only: an empty business key or payload is the
// domain's refusal, with the code the spec declares.
func creationOf(req *servicev1.CreateExternalCommandRequest) (application.CreateExternalCommand, error) {
	var none application.CreateExternalCommand
	if err := requireIdempotencyKey(req.GetIdempotencyKey()); err != nil {
		return none, err
	}
	for field, id := range map[string]string{"tenant_id": req.GetTenantId(), "connector_id": req.GetConnectorId()} {
		if err := requireID(field, id); err != nil {
			return none, err
		}
	}
	if err := requireOperation("operation", req.GetOperation(), false); err != nil {
		return none, err
	}
	if err := requireAssetType(req.GetAssetType()); err != nil {
		return none, err
	}
	if !keyFormat.MatchString(req.GetBusinessKey()) {
		return none, invalid("business_key", "must have up to 128 characters of [A-Za-z0-9._:/-]")
	}
	if err := requireVersion("payload_version", req.GetPayloadVersion()); err != nil {
		return none, err
	}
	if err := requireText("payload", req.GetPayload(), maxPayloadBytes); err != nil {
		return none, err
	}
	if len(req.GetPayload()) > maxPayloadBytes {
		return none, invalid("payload", "must not exceed 1 MiB")
	}
	// WHY: a repeat of a taken key is told apart from a new payload by the hash
	// alone, so an empty hash would answer two payloads as the same command.
	if err := requireNonEmptyText("payload_hash", req.GetPayloadHash()); err != nil {
		return none, err
	}
	return application.CreateExternalCommand{
		IdempotencyKey: domain.IdempotencyKey(req.GetIdempotencyKey()), TenantID: domain.TenantID(req.GetTenantId()),
		ConnectorKey: domain.ConnectorKey(req.GetConnectorId()), Operation: req.GetOperation(), AssetType: req.GetAssetType(),
		BusinessKey: req.GetBusinessKey(), PayloadVersion: int(req.GetPayloadVersion()), Payload: req.GetPayload(),
		PayloadHash: req.GetPayloadHash(),
	}, nil
}

func (s Server) CancelExternalCommand(ctx context.Context, req *servicev1.CancelExternalCommandRequest) (*servicev1.CancelExternalCommandResponse, error) {
	if err := requireIdempotencyKey(req.GetIdempotencyKey()); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.CommandResponse], error) {
			return s.Service.CancelExternalCommand(ctx, application.CancelExternalCommand{IdempotencyKey: domain.IdempotencyKey(req.GetIdempotencyKey())})
		},
		func(r *kernel.Rejection) *servicev1.CancelExternalCommandResponse {
			return &servicev1.CancelExternalCommandResponse{Result: &servicev1.CancelExternalCommandResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.CommandResponse) *servicev1.CancelExternalCommandResponse {
			return &servicev1.CancelExternalCommandResponse{Result: &servicev1.CancelExternalCommandResponse_Cancelled{
				Cancelled: &servicev1.CancelledExternalCommand{IdempotencyKey: string(c.IdempotencyKey)},
			}}
		})
}

func (s Server) ResolveUnknownOutcome(ctx context.Context, req *servicev1.ResolveUnknownOutcomeRequest) (*servicev1.ResolveUnknownOutcomeResponse, error) {
	if err := requireIdempotencyKey(req.GetIdempotencyKey()); err != nil {
		return nil, err
	}
	resolution, err := enumFromTheWire("resolution", resolutionsFromTheWire, req.GetResolution())
	if err != nil {
		return nil, err
	}
	for field, id := range map[string]string{"query_interaction_id": req.GetQueryInteractionId(), "external_reference_id": req.GetExternalReferenceId()} {
		if err := optionalID(field, id); err != nil {
			return nil, err
		}
	}
	for field, text := range map[string]string{
		"partner_code": req.GetPartnerCode(), "partner_message": req.GetPartnerMessage(), "justification": req.GetJustification(),
	} {
		if err := requireText(field, text, maxTextRunes); err != nil {
			return nil, err
		}
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.CommandStatusResponse], error) {
			return s.Service.ResolveUnknownOutcome(ctx, application.ResolveUnknownOutcome{
				IdempotencyKey: domain.IdempotencyKey(req.GetIdempotencyKey()), Resolution: resolution,
				QueryInteractionID: domain.InteractionID(req.GetQueryInteractionId()), ExternalReferenceID: domain.ExternalReferenceID(req.GetExternalReferenceId()),
				PartnerCode: req.GetPartnerCode(), PartnerMessage: req.GetPartnerMessage(), Justification: req.GetJustification(), Partial: req.GetPartial(),
			})
		},
		func(r *kernel.Rejection) *servicev1.ResolveUnknownOutcomeResponse {
			return &servicev1.ResolveUnknownOutcomeResponse{Result: &servicev1.ResolveUnknownOutcomeResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.CommandStatusResponse) *servicev1.ResolveUnknownOutcomeResponse {
			return &servicev1.ResolveUnknownOutcomeResponse{Result: &servicev1.ResolveUnknownOutcomeResponse_Resolved{
				Resolved: &servicev1.ResolvedUnknownOutcome{IdempotencyKey: string(c.IdempotencyKey), Status: statusesToTheWire[c.Status]},
			}}
		})
}

// SweepExternalCommands answers counts, never a rejection: each command the
// sweep acts on is a command of its own, refused or not.
func (s Server) SweepExternalCommands(ctx context.Context, _ *servicev1.SweepExternalCommandsRequest) (*servicev1.SweepExternalCommandsResponse, error) {
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	report, err := s.Service.SweepExternalCommands(ctx)
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.SweepExternalCommandsResponse{
		ResendRequested: int32(report.ResendRequested), AmbiguityExpired: int32(report.AmbiguityExpired),
		Skipped: int32(report.Skipped), Failed: int32(report.Failed),
	}, nil
}
