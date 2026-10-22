package application_test

import (
	"context"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

// The relational preconditions (†) read through the Resources of the unit of
// work; the read pool of the service may fail without reaching the decision.
func TestTheRelationalPreconditionsReadInsideTheUnitOfWork(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *harness, context.Context)
		run     func(context.Context, *harness) error
	}{
		{"ChangePartnerStage", func(t *testing.T, h *harness, ctx context.Context) {
			seed(t, h, ctx, partnerTable, testPartnerID, partnerAt(domain.PartnerHomologating))
			h.agreements.hold(testPartnerID, activeAgreement(testAgreementID, kernel.Instant(testOccurred)+365*day))
			h.caps.hold(testPartnerID, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))
		}, func(ctx context.Context, h *harness) error {
			_, err := h.service.ChangePartnerStage(ctx, application.ChangePartnerStage{PartnerID: testPartnerID, Target: domain.PartnerActive})
			return err
		}},
		{"EnableCapability", func(t *testing.T, h *harness, ctx context.Context) {
			holdAgreementAndProfile(h)
			seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""))
		}, func(ctx context.Context, h *harness) error {
			_, err := h.service.EnableCapability(ctx, enableCapability())
			return err
		}},
		{"DefineProduct", func(t *testing.T, h *harness, ctx context.Context) {
			seedSustainingAgreement(t, h, ctx)
		}, func(ctx context.Context, h *harness) error {
			_, err := h.service.DefineProduct(ctx, defineProduct())
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			tc.prepare(t, h, ctx)
			h.service.AgreementsByPartner = failingAgreements{}
			h.service.ProfilesByPartner = failingProfiles{}
			h.service.CapabilitiesByPartner = failingCapabilities{}

			if err := tc.run(ctx, h); err != nil {
				t.Fatalf("%s() error = %v, want the series read inside the unit of work", tc.name, err)
			}
		})
	}
}

type failingAgreements struct{}

func (failingAgreements) LoadByPartner(context.Context, domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	return nil, errBoom
}

type failingProfiles struct{}

func (failingProfiles) LoadByPartner(context.Context, domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error) {
	return nil, errBoom
}

type failingCapabilities struct{}

func (failingCapabilities) LoadByPartner(context.Context, domain.PartnerID) ([]domain.CapabilitySnapshot, error) {
	return nil, errBoom
}
