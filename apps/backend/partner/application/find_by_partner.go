package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) FindAgreementsByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	return findRelated(ctx, s, OperationFindAgreementsByPartner, FindAgreementsByPartner{Partner: partnerID},
		fmt.Sprintf("find agreements by partner %s", partnerID),
		func(ctx context.Context) ([]domain.PartnerAgreementSnapshot, error) {
			return s.AgreementsByPartner.LoadByPartner(ctx, partnerID)
		})
}

func (s Service) FindCapabilitiesByPartner(ctx context.Context, q FindCapabilitiesByPartner) ([]domain.CapabilitySnapshot, error) {
	found, err := findRelated(ctx, s, OperationFindCapabilitiesByPartner, q,
		fmt.Sprintf("find capabilities by partner %s", q.Partner),
		func(ctx context.Context) ([]domain.CapabilitySnapshot, error) {
			return s.CapabilitiesByPartner.LoadByPartner(ctx, q.Partner)
		})
	if err != nil {
		return nil, err
	}
	matching := make([]domain.CapabilitySnapshot, 0, len(found))
	for _, c := range found {
		if (q.Environment == "" || c.Environment == q.Environment) && (q.Status == "" || c.Status == q.Status) {
			matching = append(matching, c)
		}
	}
	return matching, nil
}

func (s Service) FindProductsByPartner(ctx context.Context, q FindProductsByPartner) ([]domain.FinancialProductSnapshot, error) {
	found, err := findRelated(ctx, s, OperationFindProductsByPartner, q,
		fmt.Sprintf("find products by partner %s", q.Partner),
		func(ctx context.Context) ([]domain.FinancialProductSnapshot, error) {
			return s.ProductsByPartner.LoadByPartner(ctx, q.Partner)
		})
	if err != nil {
		return nil, err
	}
	return domain.CurrentProducts(found, q.AsOf), nil
}

func (s Service) FindIntegrationProfile(ctx context.Context, q FindIntegrationProfile) (domain.IntegrationProfileSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindIntegrationProfile, q,
		func(ctx context.Context) (domain.IntegrationProfileSnapshot, error) {
			found, err := s.ProfilesByPartner.LoadByPartner(ctx, q.Partner)
			if err != nil {
				return domain.IntegrationProfileSnapshot{}, fmt.Errorf("application: find integration profile %s %s: %w", q.Partner, q.Environment, err)
			}
			current, ok := domain.CurrentProfile(found, q.Environment, q.AsOf)
			if !ok {
				return domain.IntegrationProfileSnapshot{}, fmt.Errorf("application: find integration profile %s %s: %w", q.Partner, q.Environment, port.ErrNotFound)
			}
			return current, nil
		})
}

func (s Service) FindOperationalPolicy(ctx context.Context, q FindOperationalPolicy) (domain.OperationalPolicySnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindOperationalPolicy, q,
		func(ctx context.Context) (domain.OperationalPolicySnapshot, error) {
			found, err := s.PoliciesByPartner.LoadByPartner(ctx, q.Partner)
			if err != nil {
				return domain.OperationalPolicySnapshot{}, fmt.Errorf("application: find operational policy %s: %w", q.Partner, err)
			}
			current, ok := domain.CurrentPolicy(found, q.AsOf)
			if !ok {
				return domain.OperationalPolicySnapshot{}, fmt.Errorf("application: find operational policy %s: %w", q.Partner, port.ErrNotFound)
			}
			return current, nil
		})
}

// FindCertificatesExpiring refines the expiry in memory: the kernel Relation
// filters by one column, the environment, and the linear cost is accepted in
// the single platform tenant.
func (s Service) FindCertificatesExpiring(ctx context.Context, q FindCertificatesExpiring) ([]domain.IntegrationProfileSnapshot, error) {
	found, err := findRelated(ctx, s, OperationFindCertificatesExpiring, q,
		fmt.Sprintf("find certificates expiring in %s", q.Environment),
		func(ctx context.Context) ([]domain.IntegrationProfileSnapshot, error) {
			return s.ProfilesByEnvironment.LoadByEnvironment(ctx, q.Environment)
		})
	if err != nil {
		return nil, err
	}
	at := kernel.Instant(s.Clock.Now())
	byPartner := map[domain.PartnerID][]domain.IntegrationProfileSnapshot{}
	var partners []domain.PartnerID
	for _, p := range found {
		if _, seen := byPartner[p.PartnerID]; !seen {
			partners = append(partners, p.PartnerID)
		}
		byPartner[p.PartnerID] = append(byPartner[p.PartnerID], p)
	}
	expiring := make([]domain.IntegrationProfileSnapshot, 0, len(partners))
	for _, partnerID := range partners {
		current, ok := domain.CurrentProfile(byPartner[partnerID], q.Environment, at)
		if ok && current.CertificateExpiresAt < q.ExpiresBefore {
			expiring = append(expiring, current)
		}
	}
	return expiring, nil
}
