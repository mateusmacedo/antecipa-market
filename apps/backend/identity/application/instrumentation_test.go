package application_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
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

func TestRegisterAcceptedAuditsOnce(t *testing.T) {
	h, instr := newInstrumentedHarness(t)

	if _, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register()); err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}

	want := port.AuditEvent{
		Object:  string(testOrganizationID),
		Action:  application.OperationRegisterOrganization,
		Outcome: port.OutcomeAccepted,
		At:      testOccurred,
	}
	if len(instr.audits) != 1 || instr.audits[0] != want {
		t.Fatalf("audits = %+v, want exactly %+v", instr.audits, want)
	}
}

func TestRegisterDeniedReportsDeniedBeforeAnyProtection(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	h.service.Authorize = func(context.Context, application.Operation) error {
		return errors.Join(errors.New("tenant missing"), port.ErrDenied)
	}

	_, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register())

	if !errors.Is(err, port.ErrDenied) {
		t.Fatalf("RegisterOrganization() error = %v, want a denial", err)
	}
	if len(instr.results) != 1 || instr.results[0].Outcome != port.OutcomeDenied || instr.results[0].Err != nil {
		t.Fatalf("results = %+v, want one denied result without error", instr.results)
	}
	if len(h.protector.calls) != 0 {
		t.Fatalf("Protect called %d times, want none: a denied command derives nothing", len(h.protector.calls))
	}
}

func TestBeginPrecedesAuthorizationAndEndClosesTheSequence(t *testing.T) {
	h, _ := newInstrumentedHarness(t)

	if _, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register()); err != nil {
		t.Fatalf("RegisterOrganization() error = %v, want nil", err)
	}

	observed := h.rec.Observed()
	if len(observed) < 4 || observed[0] != "begin" || observed[1] != "authorize" {
		t.Fatalf("observed = %v, want begin then authorize first (TRC-16)", observed)
	}
	if got := observed[len(observed)-2:]; got[0] != "end" || got[1] != "audit" {
		t.Fatalf("observed = %v, want the sequence to close with end then audit", observed)
	}
}
