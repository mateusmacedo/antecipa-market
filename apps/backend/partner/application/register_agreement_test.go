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

func TestRegisterAgreementWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterAgreement(withExecution(t, context.Background()), registerAgreement()); err != nil {
		t.Fatalf("RegisterAgreement() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"partner_agreements.LoadByPartner",
		"partner_agreements.Load",
		"partner_agreements.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRegisterAgreementOpensTheSeriesAsADraftWithoutAnEvent(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterAgreement(ctx, registerAgreement())

	if err != nil {
		t.Fatalf("RegisterAgreement() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.RegisterAgreementResponse{AgreementID: testAgreementID}) {
		t.Fatalf("Response() = %+v", got)
	}
	snap, version, err := h.agreeRead.Load(ctx, testAgreementID)
	want := draftAgreement(testAgreementID, 1, "")
	want.InstrumentTemplateID = "tpl-1"
	if err != nil || version != 1 || !snap.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v v1", snap, version, err, want)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: registering an agreement emits nothing", got)
	}
	if want := []domain.PartnerID{testPartnerID}; !slices.Equal(h.agreements.asked, want) {
		t.Fatalf("the series read asked for %v, want %v", h.agreements.asked, want)
	}
}

func TestRegisterAgreementSupersedesTheLastVersionOfTheSeries(t *testing.T) {
	h := newHarness(t)
	h.agreements.hold(testPartnerID, draftAgreement("agr-2", 2, "agr-1"), draftAgreement("agr-1", 1, ""))
	cmd := registerAgreement()
	cmd.AgreementID, cmd.SupersedesID = "agr-3", "agr-2"
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterAgreement(ctx, cmd)

	if err != nil {
		t.Fatalf("RegisterAgreement() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterAgreement() rejected with %s, want accepted", rej.Code())
	}
	if snap, _, err := h.agreeRead.Load(ctx, "agr-3"); err != nil || snap.Version != 3 || snap.SupersedesID != "agr-2" {
		t.Fatalf("Load() = %+v, %v; want version 3 superseding agr-2", snap, err)
	}
}

func TestRegisterAgreementRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*harness, *application.RegisterAgreement)
		code    kernel.Code
	}{
		{"valid until before valid from", func(_ *harness, c *application.RegisterAgreement) { c.ValidUntil = c.ValidFrom - 1 },
			domain.CodePartnerAgreementValidityInvalid},
		{"supersedes the first of versions 1 and 2", func(h *harness, c *application.RegisterAgreement) {
			h.agreements.hold(testPartnerID, draftAgreement("agr-1", 1, ""), draftAgreement("agr-2", 2, "agr-1"))
			c.SupersedesID = "agr-1"
		}, domain.CodePartnerAgreementSupersedesInvalid},
		{"another first version of the series", func(h *harness, _ *application.RegisterAgreement) {
			h.agreements.hold(testPartnerID, draftAgreement("agr-1", 1, ""))
		}, domain.CodePartnerAgreementSupersedesInvalid},
		{"supersedes in an empty series", func(_ *harness, c *application.RegisterAgreement) { c.SupersedesID = "agr-0" },
			domain.CodePartnerAgreementSupersedesInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			cmd := registerAgreement()
			tc.prepare(h, &cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.RegisterAgreement(ctx, cmd)

			if err != nil {
				t.Fatalf("RegisterAgreement() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := h.agreeRead.Load(ctx, testAgreementID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func TestRegisterAgreementCountsACrossTenantAccessAsAnEmptySeries(t *testing.T) {
	h := newHarness(t)
	h.agreements.err = port.CrossTenantAccess{Object: "partner_agreements?partner_id=prt-100", ContextTenant: testTenant, DataTenant: "globex"}

	outcome, err := h.service.RegisterAgreement(withExecution(t, context.Background()), registerAgreement())

	if err != nil {
		t.Fatalf("RegisterAgreement() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterAgreement() rejected with %s, want the first version of an empty series", rej.Code())
	}
}

func TestARepeatedRegisterAgreementReplaysWithoutANewRow(t *testing.T) {
	h := newHarness(t)
	first, err := h.service.RegisterAgreement(withKey(t, context.Background(), "k-agreement"), registerAgreement())
	if err != nil {
		t.Fatalf("first RegisterAgreement() = %v, want nil", err)
	}
	h.rec.Reset()

	again, err := h.service.RegisterAgreement(withKey(t, context.Background(), "k-agreement"), registerAgreement())

	if err != nil || again.Response() != first.Response() {
		t.Fatalf("replay = %+v, %v; want %+v", again.Response(), err, first.Response())
	}
	if slices.Contains(h.rec.Observed(), "partner_agreements.Save") || slices.Contains(h.rec.Observed(), "partner_agreements.LoadByPartner") {
		t.Fatalf("sequence = %v: the replay reached the series or the store", h.rec.Observed())
	}
}
