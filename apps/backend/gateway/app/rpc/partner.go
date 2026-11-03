package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	maxContentTypeLength = 128
	maxParametersBytes   = 64 << 10
)

var contentTypeFormat = regexp.MustCompile(`^[A-Za-z0-9!#$&^_.+-]+/[A-Za-z0-9!#$&^_.+-]+(;[ -~]*)?$`)

// partnerSourcesFromTheWire are the sources a partner sends on its own; the
// others are recorded by the context itself and never come in.
var partnerSourcesFromTheWire = map[servicev1.InteractionSource]domain.InteractionSource{
	servicev1.InteractionSource_INTERACTION_SOURCE_CALLBACK: domain.InteractionCallback,
	servicev1.InteractionSource_INTERACTION_SOURCE_FILE:     domain.InteractionFile,
}

// ReceivePartnerInteraction bounds the form only: the body is measured against
// the maxPayloadBytes of the operation by the use case, which refuses it with
// the code the spec declares.
func (s Server) ReceivePartnerInteraction(ctx context.Context, req *servicev1.ReceivePartnerInteractionRequest) (*servicev1.ReceivePartnerInteractionResponse, error) {
	cmd, err := receiptOf(req)
	if err != nil {
		return nil, err
	}
	return partnerCommand(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.InteractionResponse], error) {
			return s.Service.ReceivePartnerInteraction(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.ReceivePartnerInteractionResponse {
			return &servicev1.ReceivePartnerInteractionResponse{Result: &servicev1.ReceivePartnerInteractionResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(r domain.InteractionResponse) *servicev1.ReceivePartnerInteractionResponse {
			return &servicev1.ReceivePartnerInteractionResponse{Result: &servicev1.ReceivePartnerInteractionResponse_Received{
				Received: &servicev1.ReceivedPartnerInteraction{InteractionId: string(r.InteractionID), Status: interactionStatusesToTheWire[r.Status]},
			}}
		})
}

func receiptOf(req *servicev1.ReceivePartnerInteractionRequest) (application.ReceivePartnerInteraction, error) {
	var none application.ReceivePartnerInteraction
	source, err := enumFromTheWire("source", partnerSourcesFromTheWire, req.GetSource())
	if err != nil {
		return none, err
	}
	if err := requireID("connector_key", req.GetConnectorKey()); err != nil {
		return none, err
	}
	if err := requireOperation("operation", req.GetOperation(), false); err != nil {
		return none, err
	}
	if err := optionalID("partner_event_id", req.GetPartnerEventId()); err != nil {
		return none, err
	}
	if req.GetBusinessDate() != "" {
		if err := requireDate("business_date", req.GetBusinessDate()); err != nil {
			return none, err
		}
	}
	if len(req.GetContentType()) > maxContentTypeLength || !contentTypeFormat.MatchString(req.GetContentType()) {
		return none, invalid("content_type", "must be a media type of up to 128 characters")
	}
	if len(req.GetBody()) == 0 {
		return none, invalid("body", "must not be empty")
	}
	return application.ReceivePartnerInteraction{
		ConnectorKey: domain.ConnectorKey(req.GetConnectorKey()), Operation: req.GetOperation(), Source: source,
		PartnerEventID: req.GetPartnerEventId(), BusinessDate: req.GetBusinessDate(), ContentType: req.GetContentType(), Body: req.GetBody(),
	}, nil
}

// ReadExternal bounds the form and the vocabulary; the guard of the port is
// the use case's, which answers its refusals as rejections.
func (s Server) ReadExternal(ctx context.Context, req *servicev1.ReadExternalRequest) (*servicev1.ReadExternalResponse, error) {
	cmd, err := readOf(req)
	if err != nil {
		return nil, err
	}
	return partnerCommand(ctx,
		func(ctx context.Context) (usecase.Outcome[application.ExternalReading], error) {
			return s.Service.ReadExternal(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.ReadExternalResponse {
			return &servicev1.ReadExternalResponse{Result: &servicev1.ReadExternalResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(r application.ExternalReading) *servicev1.ReadExternalResponse {
			return &servicev1.ReadExternalResponse{Result: &servicev1.ReadExternalResponse_Reading{Reading: &servicev1.ExternalReading{
				InteractionId: string(r.InteractionID), AsOf: int64(r.AsOf), ProviderId: string(r.ProviderID),
				ExternalReference: r.ExternalReference, Reading: r.Reading, ReadingHash: r.ReadingHash,
			}}}
		})
}

func readOf(req *servicev1.ReadExternalRequest) (application.ReadExternal, error) {
	var none application.ReadExternal
	if !domain.IsReadOperation(req.GetOperation()) {
		return none, invalid("operation", "must be a read of the ports")
	}
	if err := requireID("connector_key", req.GetConnectorKey()); err != nil {
		return none, err
	}
	if err := requireAssetType(req.GetAssetType()); err != nil {
		return none, err
	}
	guarded := domain.RequiresQueryAuthorization(req.GetOperation())
	if guarded && req.GetAssetType() == "" {
		return none, invalid("asset_type", "must not be empty on a read of ReceivablesQueryPort")
	}
	organization := optionalID
	if guarded {
		organization = requireID
	}
	if err := organization("organization_id", req.GetOrganizationId()); err != nil {
		return none, err
	}
	if len(req.GetParameters()) > maxParametersBytes || !json.Valid([]byte(req.GetParameters())) {
		return none, invalid("parameters", "must be JSON of up to 64 KiB")
	}
	return application.ReadExternal{
		ConnectorKey: domain.ConnectorKey(req.GetConnectorKey()), Operation: req.GetOperation(),
		OrganizationID: domain.OrganizationID(req.GetOrganizationId()), AssetType: req.GetAssetType(), Parameters: []byte(req.GetParameters()),
	}, nil
}

// partnerCommand is kernelgrpc.Command with the categories the calls to a
// partner fail with mapped: an unavailable dependency is Unavailable, which the
// bff retries, and an invalid input InvalidArgument; StatusOf maps neither.
func partnerCommand[R, Resp any](ctx context.Context, run func(context.Context) (usecase.Outcome[R], error),
	refused func(*kernel.Rejection) Resp, accepted func(R) Resp) (Resp, error) {
	var none Resp
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return none, err
	}
	out, err := run(ctx)
	if err != nil {
		return none, partnerStatusOf(err)
	}
	if rejection, declined := out.Rejection(); declined {
		return refused(rejection), nil
	}
	return accepted(out.Response()), nil
}

func partnerStatusOf(err error) error {
	if failure, ok := errors.AsType[*usecase.Failure](err); ok {
		switch failure.Category() {
		case usecase.TransientDependency:
			return status.Error(codes.Unavailable, "a dependency is unavailable; retry the call")
		case usecase.Validation:
			return status.Error(codes.InvalidArgument, "invalid argument")
		}
	}
	return kernelgrpc.StatusOf(err)
}
