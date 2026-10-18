package application

import (
	"context"
	"errors"
	"fmt"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

// FindEffectiveCapability fails closed: a read that fails answers the error,
// never an allowance, and the consumer denies what it cannot confirm. Data of
// another tenant reads as absent, so it denies like a partner nobody holds.
func (s Service) FindEffectiveCapability(ctx context.Context, q FindEffectiveCapability) (domain.EffectiveCapability, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, OperationFindEffectiveCapability)

	if err := s.Authorize(ctx, q); err != nil {
		end(port.AuthorizationResult(err))
		return domain.EffectiveCapability{}, err
	}
	fail := func(err error) (domain.EffectiveCapability, error) {
		failed := fmt.Errorf("application: find effective capability %s: %w", q.Partner, err)
		end(port.Result{Outcome: port.OutcomeFailed, Err: failed})
		return domain.EffectiveCapability{}, failed
	}

	partner, _, partnerErr := s.PartnerReader.Load(ctx, q.Partner)
	if partnerErr != nil && !errors.Is(partnerErr, port.ErrNotFound) {
		return fail(partnerErr)
	}
	facts, crossed, err := s.effectiveFacts(ctx, q.Partner, partner, partnerErr == nil, q.TenantID, q.ProductID != "")
	if err != nil {
		return fail(err)
	}
	end(readResult(append(crossed, partnerErr)...))

	return domain.ResolveEffectiveCapability(facts, domain.EffectiveQuery{
		Operation:   q.Operation,
		AssetType:   q.AssetType,
		Environment: q.Environment,
		TenantID:    q.TenantID,
		AsOf:        q.AsOf,
		ProductID:   q.ProductID,
		AmountMinor: q.AmountMinor,
		TermDays:    q.TermDays,
	}), nil
}

func (s Service) effectiveFacts(ctx context.Context, partnerID domain.PartnerID, partner domain.PartnerSnapshot, found bool, tenantID string, withProducts bool) (domain.EffectiveFacts, []error, error) {
	facts := domain.EffectiveFacts{
		Partner:      partner,
		PartnerFound: found,
		Bucket:       func(id domain.CapabilityID) int { return s.Rollout.Bucket(id, port.TenantID(tenantID)) },
	}
	var crossed []error
	keep := func(err error) error {
		if absent(err) {
			crossed = append(crossed, err)
			return nil
		}
		return err
	}

	agreements, err := s.AgreementsByPartner.LoadByPartner(ctx, partnerID)
	if err = keep(err); err != nil {
		return facts, nil, err
	}
	facts.Agreements = agreements
	if withProducts {
		products, err := s.ProductsByPartner.LoadByPartner(ctx, partnerID)
		if err = keep(err); err != nil {
			return facts, nil, err
		}
		facts.Products = products
	}
	capabilities, err := s.CapabilitiesByPartner.LoadByPartner(ctx, partnerID)
	if err = keep(err); err != nil {
		return facts, nil, err
	}
	facts.Capabilities = capabilities
	return facts, crossed, nil
}
