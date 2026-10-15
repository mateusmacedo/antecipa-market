package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	testRepresentationID = domain.RepresentationID("rep-100")
	testValidFrom        = kernel.Instant(1_755_000_000_000_000_000)
	testValidUntil       = kernel.Instant(1_786_536_000_000_000_000)
)

func registerRepresentation() application.RegisterRepresentation {
	return application.RegisterRepresentation{
		RepresentationID:    testRepresentationID,
		OrganizationID:      testOrganizationID,
		PersonID:            testPersonID,
		PowerType:           domain.PowerAttorneyInFact,
		SourceType:          domain.SourcePowerOfAttorney,
		SourceEvidenceID:    "evd-100",
		ScopeCurrency:       "BRL",
		ScopeMaxAmountMinor: 150_000,
		ScopeActTypes:       []string{"assignment", "signature"},
		ValidFrom:           testValidFrom,
		ValidUntil:          testValidUntil,
		ProposedByUserID:    "user-sub-1",
	}
}

func declaredRepresentation(id domain.RepresentationID, organization domain.OrganizationID) domain.RepresentationSnapshot {
	return domain.RepresentationSnapshot{
		ID:                  id,
		OrganizationID:      organization,
		PersonID:            testPersonID,
		PowerType:           domain.PowerAttorneyInFact,
		SourceType:          domain.SourcePowerOfAttorney,
		SourceEvidenceID:    "evd-100",
		ScopeCurrency:       "BRL",
		ScopeMaxAmountMinor: 150_000,
		ScopeActTypes:       []string{"assignment", "signature"},
		ValidFrom:           testValidFrom,
		ValidUntil:          testValidUntil,
		VerificationStatus:  domain.RepresentationDeclared,
		ProposedByUserID:    "user-sub-1",
	}
}

func withOrganizationAndPerson(t *testing.T, h *harness, status domain.OrganizationStatus) {
	t.Helper()
	withOrganization(t, h, status)
	seed(t, h, withExecution(t, context.Background()), personTable, testPersonID, registeredPerson(testPersonID, testTenant))
	h.rec.Reset()
}

func TestRegisterRepresentationWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withOrganizationAndPerson(t, h, domain.OrganizationActive)

	if _, err := h.service.RegisterRepresentation(withExecution(t, context.Background()), registerRepresentation()); err != nil {
		t.Fatalf("RegisterRepresentation() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"organizations.Load",
		"persons.Load",
		"representations.Load",
		"representations.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2, spec note †)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRegisterRepresentationDeclaresItWithoutAnEvent(t *testing.T) {
	h := newHarness(t)
	withOrganizationAndPerson(t, h, domain.OrganizationActive)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RegisterRepresentation(ctx, registerRepresentation())

	if err != nil {
		t.Fatalf("RegisterRepresentation() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.RegisterRepresentationResponse{RepresentationID: testRepresentationID}) {
		t.Fatalf("Response() = %+v", got)
	}
	stored, version, err := h.service.RepresentationReader.Load(ctx, testRepresentationID)
	if err != nil || version != 1 || !stored.Equal(declaredRepresentation(testRepresentationID, testOrganizationID)) {
		t.Fatalf("Load() = %+v v%d, %v; want the declared representation at v1", stored, version, err)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none: the verified fact is born in VerifyRepresentation", got)
	}
}

func TestRegisterRepresentationRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *harness)
		mutate  func(*application.RegisterRepresentation)
		code    kernel.Code
	}{
		{"source evidence missing", withActive, func(c *application.RegisterRepresentation) { c.SourceEvidenceID = "" }, domain.CodeRepresentationSourceEvidenceMissing},
		{"valid until not after valid from", withActive, func(c *application.RegisterRepresentation) { c.ValidUntil = c.ValidFrom }, domain.CodeRepresentationValidityInvalid},
		{"amount without currency", withActive, func(c *application.RegisterRepresentation) { c.ScopeCurrency = "" }, domain.CodeRepresentationScopeInvalid},
		{"organization not found", func(t *testing.T, h *harness) {
			seed(t, h, withExecution(t, context.Background()), personTable, testPersonID, registeredPerson(testPersonID, testTenant))
		}, func(*application.RegisterRepresentation) {}, domain.CodeRepresentationOrganizationNotActive},
		{"organization offboarded", func(t *testing.T, h *harness) { withOrganizationAndPerson(t, h, domain.OrganizationOffboarded) },
			func(*application.RegisterRepresentation) {}, domain.CodeRepresentationOrganizationNotActive},
		{"person not found", func(t *testing.T, h *harness) { withOrganization(t, h, domain.OrganizationActive) },
			func(*application.RegisterRepresentation) {}, domain.CodeRepresentationPersonNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.prepare(t, h)
			cmd := registerRepresentation()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.RegisterRepresentation(ctx, cmd)

			if err != nil {
				t.Fatalf("RegisterRepresentation() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := h.service.RepresentationReader.Load(ctx, testRepresentationID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
		})
	}
}

func withActive(t *testing.T, h *harness) {
	t.Helper()
	withOrganizationAndPerson(t, h, domain.OrganizationActive)
}

func TestRegisterRepresentationOfAPersonOfAnotherTenantIsNotFound(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	seed(t, h, withTenant(t, context.Background(), "globex"), personTable, testPersonID, registeredPerson(testPersonID, "globex"))

	outcome, err := h.service.RegisterRepresentation(withExecution(t, context.Background()), registerRepresentation())

	if err != nil {
		t.Fatalf("RegisterRepresentation() error = %v, want a rejection", err)
	}
	if rej, refused := outcome.Rejection(); !refused || rej.Code() != domain.CodeRepresentationPersonNotFound {
		t.Fatalf("outcome = %+v, want %s (IDN-13)", outcome, domain.CodeRepresentationPersonNotFound)
	}
}

func TestEachFailureOfRegisterRepresentationCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
	}{
		{"organization load", []option{withLoadError(errBoom)}},
		{"person load", []option{withTableFaults("persons", serviceskit.Faults{Load: errBoom})}},
		{"load", []option{withTableFaults("representations", serviceskit.Faults{Load: errBoom})}},
		{"save", []option{withTableFaults("representations", serviceskit.Faults{Save: errBoom})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withOrganizationAndPerson(t, h, domain.OrganizationActive)

			_, err := h.service.RegisterRepresentation(withExecution(t, context.Background()), registerRepresentation())

			if want := "application: register representation rep-100: boom"; !errors.Is(err, errBoom) || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

func TestTheSameKeyWithAnotherRepresentationIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.RegisterRepresentation){
		"organization":    func(c *application.RegisterRepresentation) { c.OrganizationID = "org-other" },
		"person":          func(c *application.RegisterRepresentation) { c.PersonID = "per-other" },
		"power type":      func(c *application.RegisterRepresentation) { c.PowerType = domain.PowerOperator },
		"source type":     func(c *application.RegisterRepresentation) { c.SourceType = domain.SourceBoardResolution },
		"source evidence": func(c *application.RegisterRepresentation) { c.SourceEvidenceID = "evd-other" },
		"scope currency":  func(c *application.RegisterRepresentation) { c.ScopeCurrency = "USD" },
		"scope amount":    func(c *application.RegisterRepresentation) { c.ScopeMaxAmountMinor++ },
		"scope act type":  func(c *application.RegisterRepresentation) { c.ScopeActTypes[0] = "other" },
		"scope act types": func(c *application.RegisterRepresentation) { c.ScopeActTypes = c.ScopeActTypes[:1] },
		"valid from":      func(c *application.RegisterRepresentation) { c.ValidFrom++ },
		"valid until":     func(c *application.RegisterRepresentation) { c.ValidUntil = 0 },
		"proposer":        func(c *application.RegisterRepresentation) { c.ProposedByUserID = "user-sub-2" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withOrganizationAndPerson(t, h, domain.OrganizationActive)
			if _, err := h.service.RegisterRepresentation(withKey(t, context.Background(), "k-reused"), registerRepresentation()); err != nil {
				t.Fatalf("first RegisterRepresentation() = %v, want nil", err)
			}
			second := registerRepresentation()
			mutate(&second)

			_, err := h.service.RegisterRepresentation(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second RegisterRepresentation() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
