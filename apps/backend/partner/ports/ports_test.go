package ports_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

var (
	_ port.Repository[domain.PartnerID, domain.PartnerSnapshot]            = ports.PartnerRepository(nil)
	_ ports.PartnerReader                                                  = port.Reader[domain.PartnerID, domain.PartnerSnapshot](nil)
	_ port.Repository[domain.AgreementID, domain.PartnerAgreementSnapshot] = ports.AgreementRepository(nil)
	_ ports.AgreementReader                                                = port.Reader[domain.AgreementID, domain.PartnerAgreementSnapshot](nil)

	_ port.Repository[domain.ProfileID, domain.IntegrationProfileSnapshot] = ports.IntegrationProfileRepository(nil)
	_ ports.IntegrationProfileReader                                       = port.Reader[domain.ProfileID, domain.IntegrationProfileSnapshot](nil)
	_ port.Repository[domain.PolicyID, domain.OperationalPolicySnapshot]   = ports.OperationalPolicyRepository(nil)
	_ ports.OperationalPolicyReader                                        = port.Reader[domain.PolicyID, domain.OperationalPolicySnapshot](nil)

	_ port.Repository[domain.CapabilityID, domain.CapabilitySnapshot] = ports.CapabilityRepository(nil)
	_ ports.CapabilityReader                                          = port.Reader[domain.CapabilityID, domain.CapabilitySnapshot](nil)
)

type directoryFunc func(context.Context, port.TenantID, domain.OrganizationID) ([]domain.OrganizationKind, error)

func (f directoryFunc) FindOrganization(ctx context.Context, tenant port.TenantID, id domain.OrganizationID) ([]domain.OrganizationKind, error) {
	return f(ctx, tenant, id)
}

func TestFindOrganizationReceivesTheTenantAndTheOrganizationAndAnswersTheKinds(t *testing.T) {
	type call struct {
		tenant port.TenantID
		id     domain.OrganizationID
	}
	var got call
	var directory ports.OrganizationDirectory = directoryFunc(
		func(_ context.Context, tenant port.TenantID, id domain.OrganizationID) ([]domain.OrganizationKind, error) {
			got = call{tenant, id}
			return []domain.OrganizationKind{domain.KindFunder}, nil
		})

	kinds, err := directory.FindOrganization(context.Background(), "acme", "org-1")

	if err != nil || !slices.Equal(kinds, []domain.OrganizationKind{domain.KindFunder}) {
		t.Fatalf("FindOrganization() = %v, %v; want the funder kind", kinds, err)
	}
	if want := (call{"acme", "org-1"}); got != want {
		t.Fatalf("FindOrganization received %+v, want %+v", got, want)
	}
}

func TestFindOrganizationAnswersAnUnknownOrganizationAsNotFound(t *testing.T) {
	var directory ports.OrganizationDirectory = directoryFunc(
		func(context.Context, port.TenantID, domain.OrganizationID) ([]domain.OrganizationKind, error) {
			return nil, port.ErrNotFound
		})

	if _, err := directory.FindOrganization(context.Background(), "acme", "org-none"); !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindOrganization() = %v, want ErrNotFound", err)
	}
}

type partnersByOrganization map[domain.OrganizationID][]domain.PartnerSnapshot

func (r partnersByOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.PartnerSnapshot, error) {
	return r[id], nil
}

type agreementsByPartner map[domain.PartnerID][]domain.PartnerAgreementSnapshot

func (r agreementsByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	return r[id], nil
}

func TestTheRelationsAnswerEveryRowOfTheirKey(t *testing.T) {
	var (
		partners   ports.PartnersByOrganizationReader = partnersByOrganization{"org-1": {{ID: "prt-1", Role: domain.RoleFunder}, {ID: "prt-2", Role: domain.RoleRegistry}}}
		agreements ports.AgreementsByPartnerReader    = agreementsByPartner{"prt-1": {{ID: "agr-1", Version: 1}, {ID: "agr-2", Version: 2}}}
	)

	byOrganization, errPartners := partners.LoadByOrganization(context.Background(), "org-1")
	byPartner, errAgreements := agreements.LoadByPartner(context.Background(), "prt-1")

	if errPartners != nil || len(byOrganization) != 2 || errAgreements != nil || len(byPartner) != 2 {
		t.Fatalf("relations = %v, %v and %v, %v; want every role of the organization and every version of the series",
			byOrganization, errPartners, byPartner, errAgreements)
	}
}

type profilesByPartner map[domain.PartnerID][]domain.IntegrationProfileSnapshot

func (r profilesByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error) {
	return r[id], nil
}

type policiesByPartner map[domain.PartnerID][]domain.OperationalPolicySnapshot

func (r policiesByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.OperationalPolicySnapshot, error) {
	return r[id], nil
}

func TestTheSeriesOfProfilesAndPoliciesAnswerEveryVersionOfThePartner(t *testing.T) {
	var (
		profiles ports.IntegrationProfilesByPartnerReader = profilesByPartner{"prt-1": {
			{ID: "ipf-1", Environment: domain.EnvironmentProduction, Version: 1},
			{ID: "ipf-2", Environment: domain.EnvironmentHomologation, Version: 1},
		}}
		policies ports.OperationalPoliciesByPartnerReader = policiesByPartner{"prt-1": {{ID: "pol-1", Version: 1}, {ID: "pol-2", Version: 2}}}
	)

	byProfile, errProfiles := profiles.LoadByPartner(context.Background(), "prt-1")
	byPolicy, errPolicies := policies.LoadByPartner(context.Background(), "prt-1")

	if errProfiles != nil || len(byProfile) != 2 || errPolicies != nil || len(byPolicy) != 2 {
		t.Fatalf("relations = %v, %v and %v, %v; want every environment of the profiles and every version of the policies",
			byProfile, errProfiles, byPolicy, errPolicies)
	}
}

type capabilitiesByPartner map[domain.PartnerID][]domain.CapabilitySnapshot

func (r capabilitiesByPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.CapabilitySnapshot, error) {
	return r[id], nil
}

func TestTheCapabilitiesOfAPartnerAnswerEverySeriesAndVersion(t *testing.T) {
	var capabilities ports.CapabilitiesByPartnerReader = capabilitiesByPartner{"prt-1": {
		{ID: "cap-1", Operation: "register-receivable", Version: 1},
		{ID: "cap-2", Operation: "register-receivable", Version: 2, SupersedesID: "cap-1"},
		{ID: "cap-3", Operation: "settle", Version: 1},
	}}

	found, err := capabilities.LoadByPartner(context.Background(), "prt-1")

	if err != nil || len(found) != 3 {
		t.Fatalf("LoadByPartner() = %v, %v; want every version of every series of the partner", found, err)
	}
}

type bucketFunc func(domain.CapabilityID, port.TenantID) int

func (f bucketFunc) Bucket(id domain.CapabilityID, tenant port.TenantID) int { return f(id, tenant) }

func TestTheRolloutBucketReceivesTheCapabilityAndTheTenant(t *testing.T) {
	type call struct {
		id     domain.CapabilityID
		tenant port.TenantID
	}
	var got call
	var bucket ports.RolloutBucket = bucketFunc(func(id domain.CapabilityID, tenant port.TenantID) int {
		got = call{id, tenant}
		return 42
	})

	if b := bucket.Bucket("cap-1", "tenant-a"); b != 42 || got != (call{"cap-1", "tenant-a"}) {
		t.Fatalf("Bucket() = %d with %+v, want 42 for cap-1 and tenant-a", b, got)
	}
}
