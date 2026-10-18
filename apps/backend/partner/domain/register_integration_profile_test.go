package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRegisterIntegrationProfileOpensTheSeriesOfTheEnvironmentAtVersionOne(t *testing.T) {
	p := domain.NewIntegrationProfile(profileID)

	acc, rej := p.Register(registerProfileCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RegisterIntegrationProfileResponse{ProfileID: profileID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() = %+v, want none", acc.Events())
	}
	want := domain.IntegrationProfileSnapshot{
		ID:                   profileID,
		PartnerID:            partnerID,
		Environment:          domain.EnvironmentProduction,
		Version:              1,
		ChangeKind:           domain.ChangeInitial,
		Protocols:            []string{"https", "mtls"},
		Endpoint:             "https://partner.example/api",
		SchemaVersion:        "v3",
		SecretRef:            "vault://partners/prt-100/api-key",
		CertificateRef:       "cert-100",
		CertificateExpiresAt: at + 90*day,
		EffectiveFrom:        at,
	}
	if got := p.Snapshot(); !got.Equal(want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
}

func TestRegisterIntegrationProfileSupersedesTheLastVersionOfTheEnvironment(t *testing.T) {
	cmd := registerProfileCommand()
	cmd.ChangeKind, cmd.SupersedesID = domain.ChangeAnnounced, "ipf-2"
	cmd.Head = domain.ProfileHead{ID: "ipf-2", Version: 2}
	p := domain.NewIntegrationProfile("ipf-3")

	_, rej := p.Register(cmd)

	tb.RequireAccepted(t, rej)
	if s := p.Snapshot(); s.Version != 3 || s.SupersedesID != "ipf-2" || s.ChangeKind != domain.ChangeAnnounced || s.LastAlertedThresholdDays != 0 {
		t.Fatalf("Snapshot() = %+v, want version 3 superseding ipf-2, announced and never alerted", s)
	}
}

func TestRegisterIntegrationProfileKeepsItsOwnCopyOfTheProtocols(t *testing.T) {
	cmd := registerProfileCommand()
	p := domain.NewIntegrationProfile(profileID)
	if _, rej := p.Register(cmd); rej != nil {
		t.Fatalf("Register rejected: %v", rej)
	}

	cmd.Protocols[0] = "ftp"
	p.Snapshot().Protocols[1] = "smtp"

	if got := p.Snapshot().Protocols; !slices.Equal(got, []string{"https", "mtls"}) {
		t.Fatalf("Protocols = %v, want the protocols of the command, untouched by the caller", got)
	}
}

func TestRegisterIntegrationProfileRejectsEachPrecondition(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.RegisterIntegrationProfile)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"endpoint empty", func(c *domain.RegisterIntegrationProfile) { c.Endpoint = "" },
			domain.CodeIntegrationProfileEndpointEmpty, nil},
		{"endpoint blank", func(c *domain.RegisterIntegrationProfile) { c.Endpoint = "   " },
			domain.CodeIntegrationProfileEndpointEmpty, nil},
		{"secret reference missing", func(c *domain.RegisterIntegrationProfile) { c.SecretRef = "" },
			domain.CodeIntegrationProfileSecretRefMissing, nil},
		{"secret reference blank", func(c *domain.RegisterIntegrationProfile) { c.SecretRef = " " },
			domain.CodeIntegrationProfileSecretRefMissing, nil},
		{"certificate expiring at the instant", func(c *domain.RegisterIntegrationProfile) { c.CertificateExpiresAt = c.At },
			domain.CodeIntegrationProfileCertificateExpiryInvalid, nil},
		{"certificate already expired", func(c *domain.RegisterIntegrationProfile) { c.CertificateExpiresAt = c.At - day },
			domain.CodeIntegrationProfileCertificateExpiryInvalid, nil},
		{"supersedes in an empty series", func(c *domain.RegisterIntegrationProfile) { c.SupersedesID = "ipf-0" },
			domain.CodeIntegrationProfileSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "ipf-0"}}},
		{"another first version of the environment", func(c *domain.RegisterIntegrationProfile) {
			c.Head = domain.ProfileHead{ID: "ipf-1", Version: 1}
		}, domain.CodeIntegrationProfileSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: ""}}},
		{"supersedes a version that is not the last", func(c *domain.RegisterIntegrationProfile) {
			c.SupersedesID, c.Head = "ipf-1", domain.ProfileHead{ID: "ipf-2", Version: 2}
		}, domain.CodeIntegrationProfileSupersedesInvalid, []kernel.Detail{{Key: "supersedes", Value: "ipf-1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registerProfileCommand()
			tt.mutate(&cmd)
			p := domain.NewIntegrationProfile(profileID)
			before := p.Snapshot()

			acc, rej := p.Register(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			requireProfileUnchanged(t, before, p.Snapshot())
		})
	}
}

func TestRegisterIntegrationProfileReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.RegisterIntegrationProfile{SupersedesID: "ipf-0", At: at}
	want := []kernel.Code{
		domain.CodeIntegrationProfileEndpointEmpty,
		domain.CodeIntegrationProfileSecretRefMissing,
		domain.CodeIntegrationProfileCertificateExpiryInvalid,
		domain.CodeIntegrationProfileSupersedesInvalid,
	}
	fixes := []func(*domain.RegisterIntegrationProfile){
		func(c *domain.RegisterIntegrationProfile) { c.Endpoint = "https://partner.example/api" },
		func(c *domain.RegisterIntegrationProfile) { c.SecretRef = "vault://partners/prt-100/api-key" },
		func(c *domain.RegisterIntegrationProfile) { c.CertificateExpiresAt = at + day },
		func(*domain.RegisterIntegrationProfile) {},
	}
	for i, code := range want {
		acc, rej := domain.NewIntegrationProfile(profileID).Register(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
}
