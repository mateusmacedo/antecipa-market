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

const (
	maxAssetTypes     = 32
	maxRegions        = 64
	maxRequiredChecks = 64

	currencyCeiling = 8
	nameCeiling     = 256
)

func (s Server) DefineProduct(ctx context.Context, req *servicev1.DefineProductRequest) (*servicev1.DefineProductResponse, error) {
	cmd, err := productOf(req)
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.DefineProductResponse], error) {
			return s.Service.DefineProduct(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.DefineProductResponse {
			return &servicev1.DefineProductResponse{Result: &servicev1.DefineProductResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(defined domain.DefineProductResponse) *servicev1.DefineProductResponse {
			return &servicev1.DefineProductResponse{Result: &servicev1.DefineProductResponse_Defined{
				Defined: &servicev1.DefinedProduct{ProductId: string(defined.ProductID)},
			}}
		})
}

func productOf(req *servicev1.DefineProductRequest) (application.DefineProduct, error) {
	var none application.DefineProduct
	product, err := keyedIdentifier("product_id", req.GetProductId())
	if err != nil {
		return none, err
	}
	partner, err := identifier("partner_id", req.GetPartnerId())
	if err != nil {
		return none, err
	}
	agreement, err := identifier("agreement_id", req.GetAgreementId())
	if err != nil {
		return none, err
	}
	supersedes, err := optionalIdentifier("supersedes_id", req.GetSupersedesId())
	if err != nil {
		return none, err
	}
	if err := asciiWithin("code", req.GetCode(), codeCeiling); err != nil {
		return none, err
	}
	if err := asciiWithin("currency", req.GetCurrency(), currencyCeiling); err != nil {
		return none, err
	}
	if utf8.RuneCountInString(req.GetName()) > nameCeiling || !controlFree.MatchString(req.GetName()) {
		return none, status.Errorf(codes.InvalidArgument, "name must have at most %d characters, none of them a control character", nameCeiling)
	}
	for _, list := range []struct {
		name    string
		items   []string
		ceiling int
	}{
		{"asset_types", req.GetAssetTypes(), maxAssetTypes},
		{"regions", req.GetRegions(), maxRegions},
		{"required_check_ids", req.GetRequiredCheckIds(), maxRequiredChecks},
	} {
		if err := listWithin(list.name, list.items, list.ceiling); err != nil {
			return none, err
		}
	}
	if req.GetValidFrom() < 0 || req.GetValidUntil() < 0 {
		return none, status.Error(codes.InvalidArgument, "valid_from and valid_until must be instants in nanoseconds, never negative")
	}
	return application.DefineProduct{
		ProductID:        domain.ProductID(product),
		PartnerID:        domain.PartnerID(partner),
		AgreementID:      domain.AgreementID(agreement),
		Code:             req.GetCode(),
		Name:             req.GetName(),
		AssetTypes:       req.GetAssetTypes(),
		Regions:          req.GetRegions(),
		Currency:         req.GetCurrency(),
		MinAmountMinor:   req.GetMinAmountMinor(),
		MaxAmountMinor:   req.GetMaxAmountMinor(),
		MinTermDays:      int(req.GetMinTermDays()),
		MaxTermDays:      int(req.GetMaxTermDays()),
		RequiredCheckIDs: req.GetRequiredCheckIds(),
		ValidFrom:        kernel.Instant(req.GetValidFrom()),
		ValidUntil:       kernel.Instant(req.GetValidUntil()),
		SupersedesID:     domain.ProductID(supersedes),
	}, nil
}

func (s Server) RetireProduct(ctx context.Context, req *servicev1.RetireProductRequest) (*servicev1.RetireProductResponse, error) {
	product, err := keyedIdentifier("product_id", req.GetProductId())
	if err != nil {
		return nil, err
	}
	reason, err := known(retireReasonsOnTheWire, req.GetReason(), "reason must be one of expired or withdrawn")
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.RetireProductResponse], error) {
			return s.Service.RetireProduct(ctx, application.RetireProduct{ProductID: domain.ProductID(product), Reason: reason})
		},
		func(r *kernel.Rejection) *servicev1.RetireProductResponse {
			return &servicev1.RetireProductResponse{Result: &servicev1.RetireProductResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(retired domain.RetireProductResponse) *servicev1.RetireProductResponse {
			return &servicev1.RetireProductResponse{Result: &servicev1.RetireProductResponse_Retired{
				Retired: &servicev1.RetiredProduct{ProductId: string(retired.ProductID)},
			}}
		})
}

var retireReasonsOnTheWire = map[servicev1.ProductRetireReason]domain.ProductRetireReason{
	servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_EXPIRED:   domain.RetireExpired,
	servicev1.ProductRetireReason_PRODUCT_RETIRE_REASON_WITHDRAWN: domain.RetireWithdrawn,
}
