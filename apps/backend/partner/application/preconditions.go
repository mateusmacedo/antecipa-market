package application

import (
	"context"
	"errors"
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

// ErrTenantUnresolved refuses to ask the directory without a tenant: the
// organization is read in the tenant of the request, and no placeholder tenant
// is invented to fill its absence.
var ErrTenantUnresolved = errors.New("application: the execution context resolved no tenant")

func (s Service) organizationKinds(ctx context.Context, id domain.OrganizationID) ([]domain.OrganizationKind, bool, error) {
	execution, err := port.RequireExecutionContext(ctx)
	if err != nil {
		return nil, false, err
	}
	tenant, ok := execution.Tenant()
	if !ok {
		return nil, false, ErrTenantUnresolved
	}
	kinds, err := s.Directory.FindOrganization(ctx, tenant, id)
	switch {
	case errors.Is(err, port.ErrNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return kinds, true, nil
}

func (s Service) agreementHead(ctx context.Context, partnerID domain.PartnerID) (domain.AgreementHead, error) {
	found, err := s.AgreementsByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return domain.AgreementHead{}, err
	}
	var head domain.AgreementHead
	for _, agreement := range found {
		if agreement.Version > head.Version {
			head = domain.AgreementHead{ID: agreement.ID, Version: agreement.Version}
		}
	}
	return head, nil
}

func (s Service) profileHead(ctx context.Context, partnerID domain.PartnerID, environment domain.Environment) (domain.ProfileHead, error) {
	found, err := s.ProfilesByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return domain.ProfileHead{}, err
	}
	var head domain.ProfileHead
	for _, profile := range found {
		if profile.Environment == environment && profile.Version > head.Version {
			head = domain.ProfileHead{ID: profile.ID, Version: profile.Version}
		}
	}
	return head, nil
}

func (s Service) policyHead(ctx context.Context, partnerID domain.PartnerID) (domain.PolicyHead, error) {
	found, err := s.PoliciesByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return domain.PolicyHead{}, err
	}
	var head domain.PolicyHead
	for _, policy := range found {
		if policy.Version > head.Version {
			head = domain.PolicyHead{ID: policy.ID, Version: policy.Version}
		}
	}
	return head, nil
}

// absent tells a read that met only another tenant's rows: nothing of them is
// visible in this tenant, so the pre-reads find the key free.
func absent(err error) bool {
	var access port.CrossTenantAccess
	return errors.As(err, &access)
}

// The relational preconditions (†) read the series through the Resources of
// the unit of work, under the guard on the partner (ADR-067).
func agreementInEffect(ctx context.Context, res Resources, partnerID domain.PartnerID, at kernel.Instant) (bool, error) {
	found, err := res.AgreementsByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return false, err
	}
	return domain.AgreementInEffect(found, at), nil
}

func profileInEffect(ctx context.Context, res Resources, partnerID domain.PartnerID, environment domain.Environment, at kernel.Instant) (bool, error) {
	found, err := res.ProfilesByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return false, err
	}
	return domain.ProfileInEffect(found, environment, at), nil
}

func capabilityEnabled(ctx context.Context, res Resources, partnerID domain.PartnerID) (bool, error) {
	found, err := res.CapabilitiesByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return false, err
	}
	return domain.HasEnabledCapability(found), nil
}

func (s Service) capabilityHead(ctx context.Context, cmd DeclareCapability) (domain.CapabilityHead, error) {
	found, err := s.CapabilitiesByPartner.LoadByPartner(ctx, cmd.PartnerID)
	if err != nil && !absent(err) {
		return domain.CapabilityHead{}, err
	}
	var head domain.CapabilityHead
	for _, c := range found {
		sameSeries := c.Operation == cmd.Operation && c.AssetType == cmd.AssetType && c.Environment == cmd.Environment
		if sameSeries && c.Version > head.Version {
			head = domain.CapabilityHead{ID: c.ID, Version: c.Version}
		}
	}
	return head, nil
}

func (s Service) productHead(ctx context.Context, partnerID domain.PartnerID, code string) (domain.ProductHead, error) {
	found, err := s.ProductsByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return domain.ProductHead{}, err
	}
	var head domain.ProductHead
	for _, product := range found {
		if product.Code == code && product.Version > head.Version {
			head = domain.ProductHead{ID: product.ID, Version: product.Version}
		}
	}
	return head, nil
}

// agreementSustains takes the agreement from the series it reads under the
// guard: a Load of its own before the guard could see the agreement of before
// a concurrent ActivateAgreement and the series of after it.
func agreementSustains(ctx context.Context, res Resources, id domain.AgreementID, partnerID domain.PartnerID, at kernel.Instant) (bool, error) {
	series, err := res.AgreementsByPartner.LoadByPartner(ctx, partnerID)
	if err != nil && !absent(err) {
		return false, err
	}
	i := slices.IndexFunc(series, func(a domain.PartnerAgreementSnapshot) bool { return a.ID == id })
	if i < 0 {
		return false, nil
	}
	return domain.AgreementSustains(series[i], series, partnerID, at), nil
}

func loaded[ID comparable, S any](ctx context.Context, repository port.Repository[ID, S], id ID) (S, bool, error) {
	snapshot, _, err := repository.Load(ctx, id)
	switch {
	case errors.Is(err, port.ErrNotFound):
		return snapshot, false, nil
	case err != nil:
		return snapshot, false, err
	}
	return snapshot, true, nil
}
