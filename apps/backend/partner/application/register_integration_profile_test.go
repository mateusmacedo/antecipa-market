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

func TestRegisterIntegrationProfileWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterIntegrationProfile(withExecution(t, context.Background()), registerProfile()); err != nil {
		t.Fatalf("RegisterIntegrationProfile() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"integration_profiles.LoadByPartner",
		"integration_profiles.Load",
		"integration_profiles.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRegisterIntegrationProfileOpensTheSeriesWithTheReferenceOnlyAndNoEvent(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterIntegrationProfile(ctx, registerProfile())

	if err != nil {
		t.Fatalf("RegisterIntegrationProfile() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.RegisterIntegrationProfileResponse{ProfileID: testProfileID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.profileRead.Load(ctx, testProfileID)
	want := domain.IntegrationProfileSnapshot{
		ID: testProfileID, PartnerID: testPartnerID, Environment: domain.EnvironmentProduction, Version: 1,
		ChangeKind: domain.ChangeInitial, Protocols: []string{"https", "mtls"}, Endpoint: "https://partner.example/api",
		SchemaVersion: "v3", SecretRef: "vault://partners/prt-100/api-key", CertificateRef: "cert-100",
		CertificateExpiresAt: kernel.Instant(testOccurred) + 90*day, EffectiveFrom: kernel.Instant(testOccurred),
	}
	if err != nil || version != 1 || !snap.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", snap, version, err, want)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: registering a profile emits nothing", got)
	}
	if want := []domain.PartnerID{testPartnerID}; !slices.Equal(h.profiles.asked, want) {
		t.Fatalf("the series read asked for %v, want %v", h.profiles.asked, want)
	}
}

func TestRegisterIntegrationProfileSupersedesTheLastVersionOfItsEnvironmentOnly(t *testing.T) {
	h := newHarness(t)
	h.profiles.hold(testPartnerID,
		profileVersion("ipf-1", domain.EnvironmentProduction, 1, "", 90*day),
		profileVersion("ipf-2", domain.EnvironmentProduction, 2, "ipf-1", 90*day),
		profileVersion("ipf-h3", domain.EnvironmentHomologation, 3, "ipf-h2", 90*day),
	)
	cmd := registerProfile()
	cmd.ProfileID, cmd.SupersedesID, cmd.ChangeKind = "ipf-3", "ipf-2", domain.ChangeAnnounced
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterIntegrationProfile(ctx, cmd)

	if err != nil {
		t.Fatalf("RegisterIntegrationProfile() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterIntegrationProfile() rejected with %s, want accepted", rej.Code())
	}
	if snap, _, err := h.profileRead.Load(ctx, "ipf-3"); err != nil || snap.Version != 3 || snap.SupersedesID != "ipf-2" {
		t.Fatalf("Load() = %+v, %v; want version 3 superseding ipf-2", snap, err)
	}
}

func TestRegisterIntegrationProfileOpensAnotherEnvironmentAsAFirstVersion(t *testing.T) {
	h := newHarness(t)
	h.profiles.hold(testPartnerID, profileVersion("ipf-h1", domain.EnvironmentHomologation, 1, "", 90*day))

	outcome, err := h.service.RegisterIntegrationProfile(withExecution(t, context.Background()), registerProfile())

	if err != nil {
		t.Fatalf("RegisterIntegrationProfile() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterIntegrationProfile() rejected with %s: the series is the partner and the environment", rej.Code())
	}
}

func TestRegisterIntegrationProfileRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*harness, *application.RegisterIntegrationProfile)
		code    kernel.Code
	}{
		{"endpoint empty", func(_ *harness, c *application.RegisterIntegrationProfile) { c.Endpoint = "" },
			domain.CodeIntegrationProfileEndpointEmpty},
		{"without the secret reference", func(_ *harness, c *application.RegisterIntegrationProfile) { c.SecretRef = "" },
			domain.CodeIntegrationProfileSecretRefMissing},
		{"certificate expired before the instant", func(_ *harness, c *application.RegisterIntegrationProfile) {
			c.CertificateExpiresAt = kernel.Instant(testOccurred) - day
		}, domain.CodeIntegrationProfileCertificateExpiryInvalid},
		{"supersedes a profile of another environment", func(h *harness, c *application.RegisterIntegrationProfile) {
			h.profiles.hold(testPartnerID, profileVersion("ipf-h1", domain.EnvironmentHomologation, 1, "", 90*day))
			c.SupersedesID = "ipf-h1"
		}, domain.CodeIntegrationProfileSupersedesInvalid},
		{"another first version of the environment", func(h *harness, _ *application.RegisterIntegrationProfile) {
			h.profiles.hold(testPartnerID, profileVersion("ipf-1", domain.EnvironmentProduction, 1, "", 90*day))
		}, domain.CodeIntegrationProfileSupersedesInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			cmd := registerProfile()
			tc.prepare(h, &cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.RegisterIntegrationProfile(ctx, cmd)

			if err != nil {
				t.Fatalf("RegisterIntegrationProfile() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := h.profileRead.Load(ctx, testProfileID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func TestRegisterIntegrationProfileCountsACrossTenantAccessAsAnEmptySeries(t *testing.T) {
	h := newHarness(t)
	h.profiles.err = port.CrossTenantAccess{Object: "integration_profiles?partner_id=prt-100", ContextTenant: testTenant, DataTenant: "globex"}

	outcome, err := h.service.RegisterIntegrationProfile(withExecution(t, context.Background()), registerProfile())

	if err != nil {
		t.Fatalf("RegisterIntegrationProfile() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterIntegrationProfile() rejected with %s, want the first version of an empty series", rej.Code())
	}
}
