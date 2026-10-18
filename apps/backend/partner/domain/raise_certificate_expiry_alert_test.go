package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRaiseExpiryAlertRecordsTheThresholdAndAnnouncesTheExpiry(t *testing.T) {
	p := profileExpiringIn(t, 25*day)

	acc, rej := p.RaiseExpiryAlert(raiseAlert(30))

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RaiseCertificateExpiryAlertResponse{ProfileID: profileID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	want := domain.PartnerCertificateExpiring{
		ProfileID:            profileID,
		PartnerID:            partnerID,
		Environment:          domain.EnvironmentProduction,
		CertificateExpiresAt: at + 25*day,
		ThresholdDays:        30,
		At:                   at,
	}
	if events := acc.Events(); len(events) != 1 || events[0] != want {
		t.Fatalf("Events() = %+v, want [%+v]", events, want)
	}
	if got := p.Snapshot().LastAlertedThresholdDays; got != 30 {
		t.Fatalf("LastAlertedThresholdDays = %d, want 30", got)
	}
}

func TestRaiseExpiryAlertAcceptsTheThresholdOnTheInstantItIsReached(t *testing.T) {
	_, rej := profileExpiringIn(t, 30*day).RaiseExpiryAlert(raiseAlert(30))

	tb.RequireAccepted(t, rej)
}

func TestRaiseExpiryAlertWalksDownTheThresholds(t *testing.T) {
	p := profileExpiringIn(t, 5*day)

	for _, threshold := range []int{60, 30, 7} {
		_, rej := p.RaiseExpiryAlert(raiseAlert(threshold))

		tb.RequireAccepted(t, rej)
		if got := p.Snapshot().LastAlertedThresholdDays; got != threshold {
			t.Fatalf("LastAlertedThresholdDays = %d, want %d", got, threshold)
		}
	}
}

func TestRaiseExpiryAlertRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name      string
		profile   func(*testing.T) *domain.IntegrationProfile
		threshold int
		code      kernel.Code
		details   []kernel.Detail
	}{
		{"threshold of 45 days", func(t *testing.T) *domain.IntegrationProfile { return profileExpiringIn(t, 25*day) }, 45,
			domain.CodeIntegrationProfileInvalidAlertThreshold, []kernel.Detail{{Key: "thresholdDays", Value: "45"}}},
		{"threshold of zero days", func(t *testing.T) *domain.IntegrationProfile { return profileExpiringIn(t, 25*day) }, 0,
			domain.CodeIntegrationProfileInvalidAlertThreshold, []kernel.Detail{{Key: "thresholdDays", Value: "0"}}},
		{"certificate expiring in 90 days", func(t *testing.T) *domain.IntegrationProfile { return profileExpiringIn(t, 90*day) }, 60,
			domain.CodeIntegrationProfileThresholdNotReached, nil},
		{"one nanosecond before the threshold", func(t *testing.T) *domain.IntegrationProfile { return profileExpiringIn(t, 30*day+1) }, 30,
			domain.CodeIntegrationProfileThresholdNotReached, nil},
		{"60 days after 30 were alerted", func(t *testing.T) *domain.IntegrationProfile { return alertedProfile(t, 25*day, 30) }, 60,
			domain.CodeIntegrationProfileAlreadyAlerted, nil},
		{"the same threshold again", func(t *testing.T) *domain.IntegrationProfile { return alertedProfile(t, 25*day, 30) }, 30,
			domain.CodeIntegrationProfileAlreadyAlerted, nil},
		{"30 days after 7 were alerted", func(t *testing.T) *domain.IntegrationProfile { return alertedProfile(t, 5*day, 7) }, 30,
			domain.CodeIntegrationProfileAlreadyAlerted, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.profile(t)
			before := p.Snapshot()

			acc, rej := p.RaiseExpiryAlert(raiseAlert(tt.threshold))

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireProfileUnchanged(t, before, p.Snapshot())
		})
	}
}

func TestRaiseExpiryAlertReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	p := alertedProfile(t, 90*day, 7)

	for _, step := range []struct {
		threshold int
		code      kernel.Code
	}{
		{45, domain.CodeIntegrationProfileInvalidAlertThreshold},
		{60, domain.CodeIntegrationProfileThresholdNotReached},
	} {
		acc, rej := p.RaiseExpiryAlert(raiseAlert(step.threshold))
		tb.RequireRejected(t, acc, rej, step.code)
	}
	acc, rej := alertedProfile(t, 5*day, 7).RaiseExpiryAlert(raiseAlert(60))
	tb.RequireRejected(t, acc, rej, domain.CodeIntegrationProfileAlreadyAlerted)
}
