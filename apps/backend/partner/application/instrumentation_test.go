package application_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

type instrumentationRecorder struct {
	steps   *serviceskit.Steps
	results []port.Result
	audits  []port.AuditEvent
}

func (r *instrumentationRecorder) BeginOperation(ctx context.Context, _ string) (context.Context, port.EndOperation) {
	r.steps.Record("begin")
	return ctx, func(result port.Result) {
		r.steps.Record("end")
		r.results = append(r.results, result)
	}
}

func (r *instrumentationRecorder) Audit(_ context.Context, event port.AuditEvent) {
	r.steps.Record("audit")
	r.audits = append(r.audits, event)
}

func newInstrumentedHarness(t *testing.T) (*harness, *instrumentationRecorder) {
	t.Helper()
	h := newHarness(t)
	instr := &instrumentationRecorder{steps: h.rec}
	h.service.Instrumentation = instr
	return h, instr
}

func TestEachAcceptedCommandAuditsOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		run    func(*testing.T, *harness, context.Context) error
		object string
		action string
	}{
		{"register partner", func(_ *testing.T, h *harness, ctx context.Context) error {
			_, err := h.service.RegisterPartner(ctx, registerPartner())
			return err
		}, string(testPartnerID), application.OperationRegisterPartner},
		{"register agreement", func(_ *testing.T, h *harness, ctx context.Context) error {
			_, err := h.service.RegisterAgreement(ctx, registerAgreement())
			return err
		}, string(testAgreementID), application.OperationRegisterAgreement},
		{"register integration profile", func(_ *testing.T, h *harness, ctx context.Context) error {
			_, err := h.service.RegisterIntegrationProfile(ctx, registerProfile())
			return err
		}, string(testProfileID), application.OperationRegisterIntegrationProfile},
		{"raise certificate expiry alert", func(t *testing.T, h *harness, ctx context.Context) error {
			seedProfile(t, h, ctx, expiringProfile(25*day, 0))
			_, err := h.service.RaiseCertificateExpiryAlert(ctx, raiseAlert(30))
			return err
		}, string(testProfileID), application.OperationRaiseCertificateExpiryAlert},
		{"define operational policy", func(_ *testing.T, h *harness, ctx context.Context) error {
			_, err := h.service.DefineOperationalPolicy(ctx, definePolicy())
			return err
		}, string(testPolicyID), application.OperationDefineOperationalPolicy},
		{"change partner stage", func(t *testing.T, h *harness, ctx context.Context) error {
			return runOffboardPartner(t, h, ctx)
		}, string(testPartnerID), application.OperationChangePartnerStage},
		{"declare capability", func(t *testing.T, h *harness, ctx context.Context) error {
			return runDeclareCapability(t, h, ctx)
		}, string(testCapabilityID), application.OperationDeclareCapability},
		{"record homologation", func(t *testing.T, h *harness, ctx context.Context) error {
			return runRecordHomologation(t, h, ctx)
		}, string(testCapabilityID), application.OperationRecordHomologation},
		{"enable capability", func(t *testing.T, h *harness, ctx context.Context) error {
			return runEnableCapability(t, h, ctx)
		}, string(testCapabilityID), application.OperationEnableCapability},
		{"suspend capability", func(t *testing.T, h *harness, ctx context.Context) error {
			return runSuspendCapability(t, h, ctx)
		}, string(testCapabilityID), application.OperationSuspendCapability},
		{"change rollout", func(t *testing.T, h *harness, ctx context.Context) error {
			return runChangeRollout(t, h, ctx)
		}, string(testCapabilityID), application.OperationChangeRollout},
		{"define product", func(t *testing.T, h *harness, ctx context.Context) error {
			seedSustainingAgreement(t, h, ctx)
			_, err := h.service.DefineProduct(ctx, defineProduct())
			return err
		}, string(testProductID), application.OperationDefineProduct},
		{"retire product", func(t *testing.T, h *harness, ctx context.Context) error {
			seedProduct(t, h, ctx, productVersion(testProductID, "receivables-advance", 1, ""))
			_, err := h.service.RetireProduct(ctx, retireProduct(domain.RetireWithdrawn))
			return err
		}, string(testProductID), application.OperationRetireProduct},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, instr := newInstrumentedHarness(t)

			if err := tc.run(t, h, withExecution(t, context.Background())); err != nil {
				t.Fatalf("command error = %v, want nil", err)
			}

			want := port.AuditEvent{Object: tc.object, Action: tc.action, Outcome: port.OutcomeAccepted, At: testOccurred}
			if len(instr.audits) != 1 || instr.audits[0] != want {
				t.Fatalf("audits = %+v, want exactly %+v", instr.audits, want)
			}
		})
	}
}

func TestADeniedRegisterPartnerReachesNoDirectory(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	h.service.Authorize = func(context.Context, application.Operation) error {
		return errors.Join(errors.New("tenant missing"), port.ErrDenied)
	}

	_, err := h.service.RegisterPartner(withExecution(t, context.Background()), registerPartner())

	if !errors.Is(err, port.ErrDenied) {
		t.Fatalf("RegisterPartner() error = %v, want a denial", err)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != port.OutcomeDenied || instr.results[0].Err != nil {
		t.Fatalf("results = %+v, want one denied result without error", instr.results)
	}
	if len(h.directory.calls) != 0 {
		t.Fatalf("FindOrganization called %d times, want none: a denied command reads nothing", len(h.directory.calls))
	}
}
