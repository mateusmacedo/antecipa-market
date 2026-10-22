package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
)

var errBoom = errors.New("boom")

func TestEachFailureOfRegisterCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		wantIs  error
		wantMsg string
	}{
		{"protect", nil, func(h *harness) { h.protector.err = errBoom }, errBoom, "application: register organization org-100: boom"},
		{"load", []option{withLoadError(errBoom)}, nil, errBoom, "application: register organization org-100: boom"},
		{"save", []option{withSaveError(errBoom)}, nil, errBoom, "application: register organization org-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, nil, errBoom, "application: register organization org-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.prepare != nil {
				tc.prepare(h)
			}

			_, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register())

			if !errors.Is(err, tc.wantIs) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q matching %v", err, tc.wantMsg, tc.wantIs)
			}
			if h.outboxLen() != 0 {
				t.Fatalf("outbox = %d entries, want none after a failure", h.outboxLen())
			}
		})
	}
}

func TestRegisterAlreadyExistingCarriesThePrefix(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	h.seedOrganization(t, ctx, registeredSnapshot(testOrganizationID, testTenant, "PT"))

	_, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register())

	if want := "application: register organization org-100: ports: aggregate already exists"; !errors.Is(err, port.ErrAlreadyExists) || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestNoFailureMessageCarriesTheTaxID(t *testing.T) {
	h := newHarness(t, withSaveError(errBoom))
	h.taxIDs.err = errBoom

	_, err := h.service.RegisterOrganization(withExecution(t, context.Background()), register())
	_, findErr := h.service.FindOrganizationByTaxID(withExecution(t, context.Background()), application.FindOrganizationByTaxID{
		TaxIDType: register().TaxIDType, TaxID: alphanumericCNPJ, Jurisdiction: testJurisdiction,
	})

	for _, e := range []error{err, findErr} {
		if e == nil || strings.Contains(e.Error(), alphanumericCNPJ) {
			t.Fatalf("err = %v, want a failure that does not carry the tax id", e)
		}
	}
}
