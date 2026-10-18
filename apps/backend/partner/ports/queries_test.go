package ports_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

type partnersByRole map[domain.PartnerRole][]domain.PartnerSnapshot

func (r partnersByRole) LoadByRole(_ context.Context, role domain.PartnerRole) ([]domain.PartnerSnapshot, error) {
	return r[role], nil
}

type profilesByEnvironment map[domain.Environment][]domain.IntegrationProfileSnapshot

func (r profilesByEnvironment) LoadByEnvironment(_ context.Context, environment domain.Environment) ([]domain.IntegrationProfileSnapshot, error) {
	return r[environment], nil
}

func TestTheQueryRelationsAnswerEveryRowOfTheirKey(t *testing.T) {
	var (
		partners ports.PartnersByRoleReader = partnersByRole{domain.RoleFunder: {
			{ID: "prt-1", Role: domain.RoleFunder, Status: domain.PartnerActive},
			{ID: "prt-2", Role: domain.RoleFunder, Status: domain.PartnerSuspended},
		}}
		profiles ports.IntegrationProfilesByEnvironmentReader = profilesByEnvironment{domain.EnvironmentProduction: {
			{ID: "ipf-1", PartnerID: "prt-1", Environment: domain.EnvironmentProduction, Version: 1},
			{ID: "ipf-2", PartnerID: "prt-2", Environment: domain.EnvironmentProduction, Version: 1},
		}}
	)

	funders, errPartners := partners.LoadByRole(context.Background(), domain.RoleFunder)
	production, errProfiles := profiles.LoadByEnvironment(context.Background(), domain.EnvironmentProduction)

	if errPartners != nil || len(funders) != 2 || errProfiles != nil || len(production) != 2 {
		t.Fatalf("relations = %v, %v and %v, %v; want every funder in any status and every profile of the environment",
			funders, errPartners, production, errProfiles)
	}
}
