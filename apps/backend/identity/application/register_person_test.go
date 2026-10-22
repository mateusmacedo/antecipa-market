package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	testPersonID = domain.PersonID("per-100")
	testFullName = "Pessoa Sintetica da Silva"
	testDocument = "123.456.789-09"
)

func registerPerson() application.RegisterPerson {
	return application.RegisterPerson{PersonID: testPersonID, FullName: testFullName, Document: testDocument}
}

func registeredPerson(id domain.PersonID, tenant port.TenantID) domain.PersonSnapshot {
	return domain.PersonSnapshot{
		ID:                     id,
		FullName:               testFullName,
		DocumentHash:           fakeHash(tenant, domain.TaxIDTypePersonDocument, testDocument),
		DocumentHashKeyVersion: 1,
		DocumentMasked:         "***.***.*89-09",
		CreatedAt:              testOccurred,
	}
}

func TestRegisterPersonWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterPerson(withExecution(t, context.Background()), registerPerson()); err != nil {
		t.Fatalf("RegisterPerson() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"protector.Protect",
		"persons.Load",
		"persons.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2, spec note ††)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRegisterPersonCreatesItWithHashAndMaskAndNoEvent(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterPerson(ctx, registerPerson())

	if err != nil {
		t.Fatalf("RegisterPerson() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.RegisterPersonResponse{PersonID: testPersonID}) {
		t.Fatalf("Response() = %+v", got)
	}
	stored, version, err := h.service.PersonReader.Load(ctx, testPersonID)
	if err != nil || version != 1 || !stored.Equal(registeredPerson(testPersonID, testTenant)) {
		t.Fatalf("Load() = %+v v%d, %v; want the registered person at v1", stored, version, err)
	}
	// The readable hash of fakeProtector carries the value by design.
	stored.DocumentHash = ""
	if printed := fmt.Sprintf("%+v", stored); strings.Contains(printed, testDocument) || strings.Contains(printed, "12345678909") {
		t.Fatalf("stored person %s carries the document (spec note ‡‡)", printed)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: RegisterPerson emits no event", got)
	}
}

func TestRegisterPersonProtectsTheDocumentUnderItsOwnTypeAndTheTenant(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.RegisterPerson(withTenant(t, context.Background(), "globex"), registerPerson()); err != nil {
		t.Fatalf("RegisterPerson() error = %v, want nil", err)
	}

	if want := []protectCall{{tenant: "globex", kind: domain.TaxIDTypePersonDocument, value: testDocument}}; !slices.Equal(h.protector.calls, want) {
		t.Fatalf("Protect calls = %+v, want %+v", h.protector.calls, want)
	}
}

func TestRegisterPersonRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*application.RegisterPerson)
		held   bool
		code   kernel.Code
	}{
		{"full name empty", func(c *application.RegisterPerson) { c.FullName = "" }, false, domain.CodePersonFullNameEmpty},
		{"document empty", func(c *application.RegisterPerson) { c.Document = "" }, false, domain.CodePersonDocumentEmpty},
		{"document already registered in the tenant", func(*application.RegisterPerson) {}, true, domain.CodePersonDuplicateDocument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			cmd := registerPerson()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())
			if tc.held {
				seed(t, h, ctx, personTable, "per-held", registeredPerson("per-held", testTenant))
			}

			outcome, err := h.service.RegisterPerson(ctx, cmd)

			if err != nil {
				t.Fatalf("RegisterPerson() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := h.service.PersonReader.Load(ctx, testPersonID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func TestRegisterPersonAcceptsTheSameDocumentInAnotherTenant(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withTenant(t, context.Background(), "globex"), personTable, "per-held", registeredPerson("per-held", "globex"))

	outcome, err := h.service.RegisterPerson(withExecution(t, context.Background()), registerPerson())

	if err != nil {
		t.Fatalf("RegisterPerson() error = %v, want nil", err)
	}
	if rej, refused := outcome.Rejection(); refused {
		t.Fatalf("RegisterPerson() rejected with %s, want accepted", rej.Code())
	}
}

func TestEachFailureOfRegisterPersonCarriesThePrefixAndNoDocument(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		prepare func(*harness)
		wantMsg string
	}{
		{"protect", nil, func(h *harness) { h.protector.err = errBoom }, "application: register person per-100: boom"},
		{"load", []option{withTableFaults("persons", serviceskit.Faults{Load: errBoom})}, nil, "application: register person per-100: boom"},
		{"save", []option{withTableFaults("persons", serviceskit.Faults{Save: errBoom})}, nil, "application: register person per-100: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.prepare != nil {
				tc.prepare(h)
			}

			_, err := h.service.RegisterPerson(withExecution(t, context.Background()), registerPerson())

			if !errors.Is(err, errBoom) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q", err, tc.wantMsg)
			}
		})
	}
}

func TestRegisteringAnExistingPersonCarriesThePrefix(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())
	seed(t, h, ctx, personTable, testPersonID, registeredPerson(testPersonID, "globex"))

	_, err := h.service.RegisterPerson(withExecution(t, context.Background()), registerPerson())

	if want := "application: register person per-100: ports: aggregate already exists"; !errors.Is(err, port.ErrAlreadyExists) || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestTheSameKeyWithAnotherPersonIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.RegisterPerson){
		"full name": func(c *application.RegisterPerson) { c.FullName += " Junior" },
		"document":  func(c *application.RegisterPerson) { c.Document = "987.654.321-00" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.service.RegisterPerson(withKey(t, context.Background(), "k-reused"), registerPerson()); err != nil {
				t.Fatalf("first RegisterPerson() = %v, want nil", err)
			}
			second := registerPerson()
			mutate(&second)

			_, err := h.service.RegisterPerson(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second RegisterPerson() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
