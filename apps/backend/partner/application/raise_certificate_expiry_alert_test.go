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

func seedProfile(t *testing.T, h *harness, ctx context.Context, s domain.IntegrationProfileSnapshot) {
	t.Helper()
	seed(t, h, ctx, profileTable, s.ID, s)
}

func expiringProfile(expiresIn kernel.Instant, lastAlerted int) domain.IntegrationProfileSnapshot {
	s := profileVersion(testProfileID, domain.EnvironmentProduction, 1, "", expiresIn)
	s.LastAlertedThresholdDays = lastAlerted
	return s
}

func TestRaiseCertificateExpiryAlertWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedProfile(t, h, ctx, expiringProfile(25*day, 0))

	if _, err := h.service.RaiseCertificateExpiryAlert(ctx, raiseAlert(30)); err != nil {
		t.Fatalf("RaiseCertificateExpiryAlert() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"integration_profiles.Load",
		"integration_profiles.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRaiseCertificateExpiryAlertRecordsTheThresholdAndEnqueuesTheExpiry(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seedProfile(t, h, ctx, expiringProfile(25*day, 0))

	outcome, err := h.service.RaiseCertificateExpiryAlert(ctx, raiseAlert(30))

	if err != nil {
		t.Fatalf("RaiseCertificateExpiryAlert() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.RaiseCertificateExpiryAlertResponse{ProfileID: testProfileID}) {
		t.Fatalf("Response() = %+v", got)
	}
	if snap, version, err := h.profileRead.Load(ctx, testProfileID); err != nil || version != 2 || snap.LastAlertedThresholdDays != 30 {
		t.Fatalf("Load() = %+v v%d, %v; want the threshold of 30 days at version 2", snap, version, err)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeIntegrationProfile || entry.AggregateID != string(testProfileID) ||
		entry.AggregateVersion != 2 || entry.Intent.Destination != application.Destination || entry.Intent.PartitionKey != string(testProfileID) {
		t.Fatalf("outbox entry = %+v, want the expiry of the profile at version 2 keyed by it", entry)
	}
	event := domain.PartnerCertificateExpiring{
		ProfileID: testProfileID, PartnerID: testPartnerID, Environment: domain.EnvironmentProduction,
		CertificateExpiresAt: kernel.Instant(testOccurred) + 25*day, ThresholdDays: 30, At: kernel.Instant(testOccurred),
	}
	if got, ok := entry.Event.(domain.PartnerCertificateExpiring); !ok || got != event {
		t.Fatalf("event = %+v, want %+v", entry.Event, event)
	}
}

func TestRaiseCertificateExpiryAlertRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name      string
		held      domain.IntegrationProfileSnapshot
		threshold int
		code      kernel.Code
	}{
		{"threshold of 45 days", expiringProfile(25*day, 0), 45, domain.CodeIntegrationProfileInvalidAlertThreshold},
		{"certificate expiring in 90 days", expiringProfile(90*day, 0), 60, domain.CodeIntegrationProfileThresholdNotReached},
		{"60 days after 30 were alerted", expiringProfile(25*day, 30), 60, domain.CodeIntegrationProfileAlreadyAlerted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			ctx := withExecution(t, context.Background())
			seedProfile(t, h, ctx, tc.held)

			outcome, err := h.service.RaiseCertificateExpiryAlert(ctx, raiseAlert(tc.threshold))

			if err != nil {
				t.Fatalf("RaiseCertificateExpiryAlert() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if snap, version, _ := h.profileRead.Load(ctx, testProfileID); version != 1 || !snap.Equal(tc.held) || h.outboxLen() != 0 {
				t.Fatalf("Load() = %+v v%d with %d outbox entries, want the seeded profile untouched", snap, version, h.outboxLen())
			}
		})
	}
}

func TestAlertingAnUnknownProfileIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.RaiseCertificateExpiryAlert(withExecution(t, context.Background()), raiseAlert(30))

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("RaiseCertificateExpiryAlert() = %v, want ErrNotFound", err)
	}
}

func TestARepeatedAlertUnderItsDeterministicKeyReplaysWithoutASecondEvent(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	seedProfile(t, h, withExecution(t, context.Background()), expiringProfile(25*day, 0))
	key := "cert-alert." + string(testProfileID) + ".30.20250817"
	first, err := h.service.RaiseCertificateExpiryAlert(withKey(t, context.Background(), key), raiseAlert(30))
	if err != nil {
		t.Fatalf("first RaiseCertificateExpiryAlert() = %v, want nil", err)
	}

	again, err := h.service.RaiseCertificateExpiryAlert(withKey(t, context.Background(), key), raiseAlert(30))

	if err != nil || again.Response() != first.Response() {
		t.Fatalf("replay = %+v, %v; want %+v", again.Response(), err, first.Response())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want the single PartnerCertificateExpiring", got)
	}
	if len(instr.audits) != 1 {
		t.Fatalf("Audit called %d times, want 1: a replay is not a new fact", len(instr.audits))
	}
}

func TestTheSameAlertUnderANewKeyDecidesAgainAndIsAlreadyAlerted(t *testing.T) {
	h := newHarness(t)
	seedProfile(t, h, withExecution(t, context.Background()), expiringProfile(25*day, 0))
	if _, err := h.service.RaiseCertificateExpiryAlert(withKey(t, context.Background(), "k-first"), raiseAlert(30)); err != nil {
		t.Fatalf("first RaiseCertificateExpiryAlert() = %v, want nil", err)
	}

	again, err := h.service.RaiseCertificateExpiryAlert(withKey(t, context.Background(), "k-second"), raiseAlert(30))

	if err != nil {
		t.Fatalf("RaiseCertificateExpiryAlert() = %v, want a rejection", err)
	}
	if rej, refused := again.Rejection(); !refused || rej.Code() != domain.CodeIntegrationProfileAlreadyAlerted || h.outboxLen() != 1 {
		t.Fatalf("outcome = %+v with %d outbox entries, want already-alerted and one event", again, h.outboxLen())
	}
}
