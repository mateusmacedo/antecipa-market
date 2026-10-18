package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

// FindEligiblePartners reads each funder apart, an N+1 the single platform
// tenant of this version accepts until the cross-tenant projection exists.
func (s Service) FindEligiblePartners(ctx context.Context, q FindEligiblePartners) ([]domain.PartnerSnapshot, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindEligiblePartners)

	if err := s.Authorize(ctx, q); err != nil {
		end(port.AuthorizationResult(err))
		return nil, err
	}
	fail := func(err error) ([]domain.PartnerSnapshot, error) {
		failed := fmt.Errorf("application: find eligible partners for %s %s: %w", q.Operation, q.AssetType, err)
		end(port.Result{Outcome: port.OutcomeFailed, Err: failed})
		return nil, failed
	}

	funders, err := s.PartnersByRole.LoadByRole(ctx, domain.RoleFunder)
	var crossed []error
	if absent(err) {
		crossed, funders = append(crossed, err), nil
	} else if err != nil {
		return fail(err)
	}

	effective := domain.EffectiveQuery{Operation: q.Operation, AssetType: q.AssetType, Environment: q.Environment, TenantID: q.TenantID, AsOf: q.AsOf}
	request := domain.ProductRequest{AssetType: q.AssetType, Region: q.Region, AmountMinor: q.AmountMinor, TermDays: q.TermDays, AsOf: q.AsOf}
	eligible := []domain.PartnerSnapshot{}
	for _, funder := range funders {
		facts, crossedHere, err := s.effectiveFacts(ctx, funder.ID, funder, true, q.TenantID, true)
		if err != nil {
			return fail(err)
		}
		crossed = append(crossed, crossedHere...)
		if domain.ResolveEffectiveCapability(facts, effective).Allowed && domain.ServesRequest(facts.Products, request) {
			eligible = append(eligible, funder)
		}
	}
	end(readResult(crossed...))

	slices.SortFunc(eligible, func(a, b domain.PartnerSnapshot) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return eligible, nil
}
