package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

var errBoom = errors.New("boom")

func TestEachFailureOfRegisterPartnerCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		wantMsg string
	}{
		{"directory", nil, func(h *harness) { h.directory.err = errBoom }, "application: register partner prt-100: boom"},
		{"uniqueness read", nil, func(h *harness) { h.partners.err = errBoom }, "application: register partner prt-100: boom"},
		{"load", []option{withTableFaults("partners", serviceskit.Faults{Load: errBoom})}, nil, "application: register partner prt-100: boom"},
		{"save", []option{withTableFaults("partners", serviceskit.Faults{Save: errBoom})}, nil, "application: register partner prt-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, nil, "application: register partner prt-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.prepare != nil {
				tc.prepare(h)
			}

			_, err := h.service.RegisterPartner(withExecution(t, context.Background()), registerPartner())

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a failure", h.outboxLen())
			}
		})
	}
}

func TestEachFailureOfTheAgreementCommandsCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		run     func(*testing.T, *harness, context.Context) error
		wantMsg string
	}{
		{"register series read", nil, func(h *harness) { h.agreements.err = errBoom }, runRegisterAgreement,
			"application: register agreement agr-100: boom"},
		{"register save", []option{withTableFaults("partner_agreements", serviceskit.Faults{Save: errBoom})}, nil, runRegisterAgreement,
			"application: register agreement agr-100: boom"},
		{"activate load", []option{withTableFaults("partner_agreements", serviceskit.Faults{Load: errBoom})}, nil, runActivateAgreement,
			"application: activate agreement agr-100: boom"},
		{"activate enqueue", []option{withEnqueueError(errBoom)}, nil, runActivateAgreement,
			"application: activate agreement agr-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.prepare != nil {
				tc.prepare(h)
			}

			err := tc.run(t, h, withExecution(t, context.Background()))

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
		})
	}
}

func runRegisterAgreement(_ *testing.T, h *harness, ctx context.Context) error {
	_, err := h.service.RegisterAgreement(ctx, registerAgreement())
	return err
}

func runActivateAgreement(t *testing.T, h *harness, ctx context.Context) error {
	seedDraft(t, h, ctx)
	_, err := h.service.ActivateAgreement(ctx, activateAgreement())
	return err
}

func TestEachFailureOfTheProfileAndPolicyCommandsCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		run     func(*testing.T, *harness, context.Context) error
		wantMsg string
	}{
		{"register series read", nil, func(h *harness) { h.profiles.err = errBoom }, runRegisterProfile,
			"application: register integration profile ipf-100: boom"},
		{"register save", []option{withTableFaults("integration_profiles", serviceskit.Faults{Save: errBoom})}, nil, runRegisterProfile,
			"application: register integration profile ipf-100: boom"},
		{"alert load", []option{withTableFaults("integration_profiles", serviceskit.Faults{Load: errBoom})}, nil, runRaiseAlert,
			"application: raise certificate expiry alert ipf-100: boom"},
		{"alert enqueue", []option{withEnqueueError(errBoom)}, nil, runRaiseAlert,
			"application: raise certificate expiry alert ipf-100: enqueue: boom"},
		{"define series read", nil, func(h *harness) { h.policies.err = errBoom }, runDefinePolicy,
			"application: define operational policy pol-100: boom"},
		{"define save", []option{withTableFaults("operational_policies", serviceskit.Faults{Save: errBoom})}, nil, runDefinePolicy,
			"application: define operational policy pol-100: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.prepare != nil {
				tc.prepare(h)
			}

			err := tc.run(t, h, withExecution(t, context.Background()))

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a failure", h.outboxLen())
			}
		})
	}
}

func runRegisterProfile(_ *testing.T, h *harness, ctx context.Context) error {
	_, err := h.service.RegisterIntegrationProfile(ctx, registerProfile())
	return err
}

func runRaiseAlert(t *testing.T, h *harness, ctx context.Context) error {
	seedProfile(t, h, ctx, expiringProfile(25*day, 0))
	_, err := h.service.RaiseCertificateExpiryAlert(ctx, raiseAlert(30))
	return err
}

func runDefinePolicy(_ *testing.T, h *harness, ctx context.Context) error {
	_, err := h.service.DefineOperationalPolicy(ctx, definePolicy())
	return err
}

func TestEachFailureOfTheCapabilityCommandsCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		run     func(*testing.T, *harness, context.Context) error
		wantMsg string
	}{
		{"stage partner read", []option{withTableFaults("partners", serviceskit.Faults{Load: errBoom})}, nil, runActivatePartner,
			"application: change partner stage prt-100: boom"},
		{"stage agreement read", nil, func(h *harness) { h.agreements.err = errBoom }, runActivatePartner,
			"application: change partner stage prt-100: boom"},
		{"stage capability read", nil, func(h *harness) { h.caps.err = errBoom }, runActivatePartner,
			"application: change partner stage prt-100: boom"},
		{"stage enqueue", []option{withEnqueueError(errBoom)}, nil, runOffboardPartner,
			"application: change partner stage prt-100: enqueue: boom"},
		{"declare series read", nil, func(h *harness) { h.caps.err = errBoom }, runDeclareCapability,
			"application: declare capability cap-100: boom"},
		{"declare save", []option{withTableFaults("capabilities", serviceskit.Faults{Save: errBoom})}, nil, runDeclareCapability,
			"application: declare capability cap-100: boom"},
		{"homologation load", []option{withTableFaults("capabilities", serviceskit.Faults{Load: errBoom})}, nil, runRecordHomologation,
			"application: record homologation cap-100: boom"},
		{"enable capability read", []option{withTableFaults("capabilities", serviceskit.Faults{Load: errBoom})}, nil, runEnableCapability,
			"application: enable capability cap-100: boom"},
		{"enable agreement read", nil, func(h *harness) { h.agreements.err = errBoom }, runEnableCapability,
			"application: enable capability cap-100: boom"},
		{"enable profile read", nil, func(h *harness) { h.profiles.err = errBoom }, runEnableCapability,
			"application: enable capability cap-100: boom"},
		{"enable enqueue", []option{withEnqueueError(errBoom)}, nil, runEnableCapability,
			"application: enable capability cap-100: enqueue: boom"},
		{"suspend enqueue", []option{withEnqueueError(errBoom)}, nil, runSuspendCapability,
			"application: suspend capability cap-100: enqueue: boom"},
		{"rollout save", []option{withTableFaults("capabilities", serviceskit.Faults{Save: errBoom})}, nil, runChangeRollout,
			"application: change rollout cap-100: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.prepare != nil {
				tc.prepare(h)
			}

			err := tc.run(t, h, withExecution(t, context.Background()))

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a failure", h.outboxLen())
			}
		})
	}
}

func runActivatePartner(t *testing.T, h *harness, ctx context.Context) error {
	seedPartner(t, h, ctx, domain.PartnerHomologating)
	holdEffectiveAgreement(h)
	_, err := h.service.ChangePartnerStage(ctx, changeStage(domain.PartnerActive, ""))
	return err
}

func runOffboardPartner(t *testing.T, h *harness, ctx context.Context) error {
	seedPartner(t, h, ctx, domain.PartnerActive)
	_, err := h.service.ChangePartnerStage(ctx, changeStage(domain.PartnerOffboarding, "contract terminated"))
	return err
}

func runDeclareCapability(_ *testing.T, h *harness, ctx context.Context) error {
	_, err := h.service.DeclareCapability(ctx, declareCapability())
	return err
}

func runRecordHomologation(t *testing.T, h *harness, ctx context.Context) error {
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusDeclared, 1, ""))
	_, err := h.service.RecordHomologation(ctx, recordHomologation(homologationOf(true, "H1")))
	return err
}

func runEnableCapability(t *testing.T, h *harness, ctx context.Context) error {
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusHomologated, 1, ""))
	holdAgreementAndProfile(h)
	_, err := h.service.EnableCapability(ctx, enableCapability())
	return err
}

func runSuspendCapability(t *testing.T, h *harness, ctx context.Context) error {
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))
	_, err := h.service.SuspendCapability(ctx, suspendCapability("partner incident"))
	return err
}

func runChangeRollout(t *testing.T, h *harness, ctx context.Context) error {
	seedCapability(t, h, ctx, capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))
	_, err := h.service.ChangeRollout(ctx, changeRollout())
	return err
}
