package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestFindPartnerAnswersTheStoredStateOutsideAnyUnitOfWork(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedPartner(t, h, ctx, domain.PartnerActive)

	got, err := h.service.FindPartner(ctx, testPartnerID)

	if err != nil || !got.Equal(partnerAt(domain.PartnerActive)) {
		t.Fatalf("FindPartner() = %+v, %v; want the active partner", got, err)
	}
	if want := []string{"authorize", "partners.Read"}; !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("observed = %v, want %v", h.rec.Observed(), want)
	}
}

func TestFindPartnerOfAnUnknownIDOrOfAnotherTenantIsNotFound(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withTenant(t, context.Background(), "globex"), partnerTable, testPartnerID, partnerAt(domain.PartnerActive))

	for _, id := range []domain.PartnerID{"prt-none", testPartnerID} {
		if _, err := h.service.FindPartner(withExecution(t, context.Background()), id); !errors.Is(err, port.ErrNotFound) {
			t.Fatalf("FindPartner(%s) = %v, want ErrNotFound", id, err)
		}
	}
}

func TestFindPartnersByOrganizationAnswersEveryRoleOfIt(t *testing.T) {
	h := newHarness(t)
	registry := partnerAt(domain.PartnerProspect)
	registry.ID, registry.Role = "prt-101", domain.RoleRegistry
	h.partners.hold(testOrganizationID, partnerAt(domain.PartnerActive), registry)

	got, err := h.service.FindPartnersByOrganization(withExecution(t, context.Background()), testOrganizationID)

	if err != nil || len(got) != 2 || !slices.Equal(h.partners.asked, []domain.OrganizationID{testOrganizationID}) {
		t.Fatalf("FindPartnersByOrganization() = %v, %v after asking %v; want both roles", got, err, h.partners.asked)
	}
}

func TestTheRelationsOfAnotherTenantAnswerAnEmptyCollection(t *testing.T) {
	access := port.CrossTenantAccess{Object: "partner_agreements?partner_id=prt-100", ContextTenant: "acme", DataTenant: "globex"}
	h := newHarness(t)
	h.partners.err, h.agreements.err, h.caps.err, h.products.err, h.byEnv.err = access, access, access, access, access
	ctx := withExecution(t, context.Background())

	partners, errPartners := h.service.FindPartnersByOrganization(ctx, testOrganizationID)
	agreements, errAgreements := h.service.FindAgreementsByPartner(ctx, testPartnerID)
	capabilities, errCapabilities := h.service.FindCapabilitiesByPartner(ctx, application.FindCapabilitiesByPartner{Partner: testPartnerID})
	products, errProducts := h.service.FindProductsByPartner(ctx, application.FindProductsByPartner{Partner: testPartnerID, AsOf: kernel.Instant(testOccurred)})
	expiring, errExpiring := h.service.FindCertificatesExpiring(ctx, application.FindCertificatesExpiring{
		Environment: domain.EnvironmentProduction, ExpiresBefore: kernel.Instant(testOccurred),
	})

	if err := errors.Join(errPartners, errAgreements, errCapabilities, errProducts, errExpiring); err != nil {
		t.Fatalf("relations of another tenant failed with %v, want empty answers", err)
	}
	if partners == nil || agreements == nil || capabilities == nil || products == nil || expiring == nil ||
		len(partners)+len(agreements)+len(capabilities)+len(products)+len(expiring) != 0 {
		t.Fatalf("relations of another tenant = %v %v %v %v %v, want empty collections", partners, agreements, capabilities, products, expiring)
	}
}

func TestARelationFailureIsTheFailureOfTheQuery(t *testing.T) {
	h := newHarness(t)
	boom := errors.New("read pool unavailable")
	h.agreements.err = boom

	if _, err := h.service.FindAgreementsByPartner(withExecution(t, context.Background()), testPartnerID); !errors.Is(err, boom) {
		t.Fatalf("FindAgreementsByPartner() = %v, want the failure of the read", err)
	}
}

func TestFindAgreementsByPartnerAnswersEveryVersion(t *testing.T) {
	h := newHarness(t)
	h.agreements.hold(testPartnerID, activeAgreement(testAgreementID, 0), draftAgreement("agr-101", 2, testAgreementID))

	got, err := h.service.FindAgreementsByPartner(withExecution(t, context.Background()), testPartnerID)

	if err != nil || len(got) != 2 {
		t.Fatalf("FindAgreementsByPartner() = %v, %v; want both versions", got, err)
	}
}

func TestFindCapabilitiesByPartnerRefinesByTheOptionalFilters(t *testing.T) {
	h := newHarness(t)
	homologation := capabilityVersion("cap-101", domain.CapabilityStatusEnabled, 1, "")
	homologation.Environment = domain.EnvironmentHomologation
	h.caps.hold(testPartnerID,
		capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""),
		capabilityVersion("cap-102", domain.CapabilityStatusSuspended, 2, testCapabilityID),
		homologation,
	)
	ctx := withExecution(t, context.Background())
	ids := func(q application.FindCapabilitiesByPartner) []domain.CapabilityID {
		t.Helper()
		found, err := h.service.FindCapabilitiesByPartner(ctx, q)
		if err != nil {
			t.Fatalf("FindCapabilitiesByPartner(%+v) = %v", q, err)
		}
		out := make([]domain.CapabilityID, 0, len(found))
		for _, c := range found {
			out = append(out, c.ID)
		}
		slices.Sort(out)
		return out
	}

	for _, tc := range []struct {
		name  string
		query application.FindCapabilitiesByPartner
		want  []domain.CapabilityID
	}{
		{"no filter", application.FindCapabilitiesByPartner{Partner: testPartnerID}, []domain.CapabilityID{testCapabilityID, "cap-101", "cap-102"}},
		{"by environment", application.FindCapabilitiesByPartner{Partner: testPartnerID, Environment: domain.EnvironmentProduction},
			[]domain.CapabilityID{testCapabilityID, "cap-102"}},
		{"by status", application.FindCapabilitiesByPartner{Partner: testPartnerID, Status: domain.CapabilityStatusEnabled},
			[]domain.CapabilityID{testCapabilityID, "cap-101"}},
		{"by both", application.FindCapabilitiesByPartner{
			Partner: testPartnerID, Environment: domain.EnvironmentProduction, Status: domain.CapabilityStatusSuspended,
		}, []domain.CapabilityID{"cap-102"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(tc.query); !slices.Equal(got, tc.want) {
				t.Fatalf("FindCapabilitiesByPartner() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFindProductsByPartnerAnswersTheCurrentVersionOfEachCode(t *testing.T) {
	h := newHarness(t)
	future := productVersion("fpr-102", "receivables-advance", 2, testProductID)
	future.ValidFrom = kernel.Instant(testOccurred) + day
	h.products.hold(testPartnerID,
		productVersion(testProductID, "receivables-advance", 1, ""),
		future,
		productVersion("fpr-103", "invoice-advance", 1, ""),
	)

	got, err := h.service.FindProductsByPartner(withExecution(t, context.Background()),
		application.FindProductsByPartner{Partner: testPartnerID, AsOf: kernel.Instant(testOccurred)})

	if err != nil || len(got) != 2 || got[0].ID != "fpr-103" || got[1].ID != testProductID {
		t.Fatalf("FindProductsByPartner() = %+v, %v; want fpr-103 and fpr-100, the version 2 not yet valid", got, err)
	}
}

func TestFindIntegrationProfileAnswersTheCurrentVersionOfTheEnvironment(t *testing.T) {
	h := newHarness(t)
	next := profileVersion("ipf-101", domain.EnvironmentProduction, 2, testProfileID, 90*day)
	next.EffectiveFrom = kernel.Instant(testOccurred) + day
	h.profiles.hold(testPartnerID, profileVersion(testProfileID, domain.EnvironmentProduction, 1, "", 90*day), next)
	ctx := withExecution(t, context.Background())

	got, err := h.service.FindIntegrationProfile(ctx, application.FindIntegrationProfile{
		Partner: testPartnerID, Environment: domain.EnvironmentProduction, AsOf: kernel.Instant(testOccurred),
	})
	if err != nil || got.ID != testProfileID {
		t.Fatalf("FindIntegrationProfile() = %+v, %v; want ipf-100, the version 2 not yet effective", got, err)
	}

	_, err = h.service.FindIntegrationProfile(ctx, application.FindIntegrationProfile{
		Partner: testPartnerID, Environment: domain.EnvironmentHomologation, AsOf: kernel.Instant(testOccurred),
	})
	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindIntegrationProfile(homologation) = %v, want ErrNotFound", err)
	}
}

func TestFindOperationalPolicyAnswersTheCurrentVersion(t *testing.T) {
	h := newHarness(t)
	h.policies.hold(testPartnerID, policyVersion(testPolicyID, 1, ""), policyVersion("pol-101", 2, testPolicyID))
	ctx := withExecution(t, context.Background())

	got, err := h.service.FindOperationalPolicy(ctx, application.FindOperationalPolicy{Partner: testPartnerID, AsOf: kernel.Instant(testOccurred)})
	if err != nil || got.ID != "pol-101" {
		t.Fatalf("FindOperationalPolicy() = %+v, %v; want pol-101", got, err)
	}

	_, err = h.service.FindOperationalPolicy(ctx, application.FindOperationalPolicy{Partner: testPartnerID, AsOf: kernel.Instant(testOccurred) - day})
	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("FindOperationalPolicy() before every version = %v, want ErrNotFound", err)
	}
}

func TestFindCertificatesExpiringRefinesTheEnvironmentByTheExpiry(t *testing.T) {
	h := newHarness(t)
	h.byEnv.hold(domain.EnvironmentProduction,
		profileOf(testPartnerID, testProfileID, 1, "", 25*day, 0),
		profileOf("prt-101", "ipf-101", 1, "", 30*day, 0),
		profileOf("prt-102", "ipf-102", 1, "", 90*day, 0),
	)

	got, err := h.service.FindCertificatesExpiring(withExecution(t, context.Background()), application.FindCertificatesExpiring{
		Environment: domain.EnvironmentProduction, ExpiresBefore: kernel.Instant(testOccurred) + 30*day,
	})

	if err != nil || len(got) != 1 || got[0].ID != testProfileID {
		t.Fatalf("FindCertificatesExpiring() = %+v, %v; want only the certificate expiring before the bound", got, err)
	}
	if !slices.Equal(h.byEnv.asked, []domain.Environment{domain.EnvironmentProduction}) {
		t.Fatalf("profiles asked by %v, want the environment", h.byEnv.asked)
	}
}

func profileOf(partnerID domain.PartnerID, id domain.ProfileID, version int, supersedes domain.ProfileID, expiresIn, effectiveIn kernel.Instant) domain.IntegrationProfileSnapshot {
	p := profileVersion(id, domain.EnvironmentProduction, version, supersedes, expiresIn)
	p.PartnerID, p.EffectiveFrom = partnerID, kernel.Instant(testOccurred)+effectiveIn
	return p
}

func TestFindCertificatesExpiringAnswersOnlyTheCurrentVersionOfEachSeries(t *testing.T) {
	h := newHarness(t)
	h.byEnv.hold(domain.EnvironmentProduction,
		profileOf(testPartnerID, testProfileID, 1, "", 10*day, -2*day),
		profileOf(testPartnerID, "ipf-101", 2, testProfileID, 300*day, -day),
		profileOf("prt-102", "ipf-102", 1, "", 20*day, -2*day),
		profileOf("prt-102", "ipf-103", 2, "ipf-102", 300*day, day),
	)

	got, err := h.service.FindCertificatesExpiring(withExecution(t, context.Background()), application.FindCertificatesExpiring{
		Environment: domain.EnvironmentProduction, ExpiresBefore: kernel.Instant(testOccurred) + 30*day,
	})

	if err != nil || len(got) != 1 || got[0].ID != "ipf-102" {
		t.Fatalf("FindCertificatesExpiring() = %+v, %v; want only ipf-102, the superseded ipf-100 left out", got, err)
	}
}
