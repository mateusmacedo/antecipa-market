package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRegisterAcceptsAFunderOfAFunderOrganization(t *testing.T) {
	p := domain.NewPartner(partnerID)

	acc, rej := p.Register(registerPartnerCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RegisterPartnerResponse{PartnerID: partnerID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	want := domain.PartnerRegistered{PartnerID: partnerID, OrganizationID: organizationID, Role: domain.RoleFunder, At: at}
	if len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want [%+v]", events, want)
	}
}

func TestRegisterLeavesThePartnerProspect(t *testing.T) {
	p := domain.NewPartner(partnerID)

	_, rej := p.Register(registerPartnerCommand())

	tb.RequireAccepted(t, rej)
	want := domain.PartnerSnapshot{
		ID:             partnerID,
		OrganizationID: organizationID,
		Role:           domain.RoleFunder,
		Status:         domain.PartnerProspect,
		RegisteredAt:   at,
		StageChangedAt: at,
	}
	if got := p.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestRegisterAdmitsEveryOtherRoleOfAPartnerOrganization(t *testing.T) {
	for _, role := range []domain.PartnerRole{
		domain.RoleBookkeeper, domain.RoleRegistry, domain.RoleDepository, domain.RoleDataProvider, domain.RoleSettlementInstitution,
	} {
		t.Run(string(role), func(t *testing.T) {
			cmd := registerPartnerCommand()
			cmd.Role = role
			cmd.OrganizationKinds = []domain.OrganizationKind{"client", domain.KindPartner}

			_, rej := domain.NewPartner(partnerID).Register(cmd)

			tb.RequireAccepted(t, rej)
		})
	}
}

func TestRegisterRejectsEachPrecondition(t *testing.T) {
	organization := kernel.Detail{Key: "organization", Value: string(organizationID)}
	funder := kernel.Detail{Key: "role", Value: string(domain.RoleFunder)}
	tests := []struct {
		name    string
		mutate  func(*domain.RegisterPartner)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"organization not found", func(c *domain.RegisterPartner) { c.OrganizationFound, c.OrganizationKinds = false, nil },
			domain.CodePartnerOrganizationNotFound, []kernel.Detail{organization}},
		{"funder of a partner organization", func(c *domain.RegisterPartner) { c.OrganizationKinds = []domain.OrganizationKind{domain.KindPartner} },
			domain.CodePartnerOrganizationKindMismatch, []kernel.Detail{organization, funder}},
		{"registry of a funder organization", func(c *domain.RegisterPartner) { c.Role = domain.RoleRegistry },
			domain.CodePartnerOrganizationKindMismatch, []kernel.Detail{organization, {Key: "role", Value: string(domain.RoleRegistry)}}},
		{"organization without kinds", func(c *domain.RegisterPartner) { c.OrganizationKinds = nil },
			domain.CodePartnerOrganizationKindMismatch, []kernel.Detail{organization, funder}},
		{"role outside the catalogue", func(c *domain.RegisterPartner) { c.Role = "acquirer" },
			domain.CodePartnerOrganizationKindMismatch, []kernel.Detail{organization, {Key: "role", Value: "acquirer"}}},
		{"organization and role already registered", func(c *domain.RegisterPartner) { c.AlreadyRegistered = true },
			domain.CodePartnerAlreadyRegistered, []kernel.Detail{organization, funder}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registerPartnerCommand()
			tt.mutate(&cmd)
			p := domain.NewPartner(partnerID)
			before := p.Snapshot()

			acc, rej := p.Register(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requirePartnerUnchanged(t, before, p.Snapshot())
		})
	}
}

func TestRegisterReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.RegisterPartner{OrganizationID: organizationID, Role: domain.RoleFunder, AlreadyRegistered: true, At: at}
	want := []kernel.Code{
		domain.CodePartnerOrganizationNotFound,
		domain.CodePartnerOrganizationKindMismatch,
		domain.CodePartnerAlreadyRegistered,
	}
	fixes := []func(*domain.RegisterPartner){
		func(c *domain.RegisterPartner) { c.OrganizationFound = true },
		func(c *domain.RegisterPartner) { c.OrganizationKinds = []domain.OrganizationKind{domain.KindFunder} },
		func(*domain.RegisterPartner) {},
	}
	for i, code := range want {
		acc, rej := domain.NewPartner(partnerID).Register(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}
