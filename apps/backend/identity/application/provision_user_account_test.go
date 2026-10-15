package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const testSubject = domain.UserID("user-sub-100")

func provision() application.ProvisionUserAccount {
	return application.ProvisionUserAccount{Subject: testSubject, PersonID: testPersonID}
}

func activeAccount(subject domain.UserID) domain.UserAccountSnapshot {
	return domain.UserAccountSnapshot{
		Subject:       subject,
		PersonID:      testPersonID,
		Status:        domain.UserAccountActive,
		ProvisionedAt: testOccurred,
	}
}

func suspendedAccount(subject domain.UserID) domain.UserAccountSnapshot {
	s := activeAccount(subject)
	s.Status = domain.UserAccountSuspended
	s.SuspendedAt = testOccurred
	s.SuspensionReason = "acesso indevido"
	return s
}

func withAccount(t *testing.T, h *harness, account domain.UserAccountSnapshot) {
	t.Helper()
	seed(t, h, withExecution(t, context.Background()), userAccountTable, account.Subject, account)
	h.rec.Reset()
}

func TestProvisionUserAccountWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)

	if _, err := h.service.ProvisionUserAccount(withExecution(t, context.Background()), provision()); err != nil {
		t.Fatalf("ProvisionUserAccount() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"user_accounts.Load",
		"user_accounts.Save",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestProvisionUserAccountActivatesAnUnknownSubjectWithoutAnEvent(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.ProvisionUserAccount(ctx, provision())

	if err != nil {
		t.Fatalf("ProvisionUserAccount() error = %v, want nil", err)
	}
	if got := outcome.Response(); got != (domain.ProvisionUserAccountResponse{UserID: testSubject}) {
		t.Fatalf("Response() = %+v, want the subject", got)
	}
	stored, version, err := userAccountTable.Reader(h.store).Load(ctx, testSubject)
	if err != nil || version != 1 || !stored.Equal(activeAccount(testSubject)) {
		t.Fatalf("Load() = %+v v%d, %v; want the active account at v1", stored, version, err)
	}
	if got := h.outboxLen(); got != 0 {
		t.Fatalf("outbox = %d entries, want none", got)
	}
}

func TestAReplayedProvisionAnswersTheStoredResponseWithoutWriting(t *testing.T) {
	h := newHarness(t)
	first, err := h.service.ProvisionUserAccount(withKey(t, context.Background(), "k-provision"), provision())
	if err != nil {
		t.Fatalf("first ProvisionUserAccount() = %v, want nil", err)
	}
	ctx := withKey(t, context.Background(), "k-provision")

	again, err := h.service.ProvisionUserAccount(ctx, provision())

	if err != nil || again.Response() != first.Response() {
		t.Fatalf("replay = %+v, %v; want %+v", again.Response(), err, first.Response())
	}
	if outcome, _ := port.IdempotencyOutcomeFrom(ctx); outcome != port.IdempotencyReplayed {
		t.Fatalf("idempotency outcome = %v, want replayed", outcome)
	}
	if _, version, _ := userAccountTable.Reader(h.store).Load(ctx, testSubject); version != 1 {
		t.Fatalf("version = %d, want 1: a replay writes nothing", version)
	}
}

func TestProvisionUnderANewKeyAcceptsWithoutChangingTheAccount(t *testing.T) {
	for name, held := range map[string]domain.UserAccountSnapshot{
		"active":    activeAccount(testSubject),
		"suspended": suspendedAccount(testSubject),
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withAccount(t, h, held)
			cmd := provision()
			cmd.PersonID = "per-other"
			ctx := withKey(t, context.Background(), "k-new")

			outcome, err := h.service.ProvisionUserAccount(ctx, cmd)

			if err != nil || outcome.Response() != (domain.ProvisionUserAccountResponse{UserID: testSubject}) {
				t.Fatalf("ProvisionUserAccount() = %+v, %v; want accepted", outcome, err)
			}
			stored, version, err := userAccountTable.Reader(h.store).Load(ctx, testSubject)
			if err != nil || version != 2 || !stored.Equal(held) {
				t.Fatalf("Load() = %+v v%d, %v; want the same state at v2: the kernel saves every decided aggregate", stored, version, err)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
		})
	}
}

func TestProvisionUserAccountIsPerTenant(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withTenant(t, context.Background(), "globex"), userAccountTable, testSubject, suspendedAccount(testSubject))
	ctx := withExecution(t, context.Background())

	if _, err := h.service.ProvisionUserAccount(ctx, provision()); err != nil {
		t.Fatalf("ProvisionUserAccount() error = %v, want nil", err)
	}

	if stored, version, err := userAccountTable.Reader(h.store).Load(ctx, testSubject); err != nil || version != 1 || !stored.Equal(activeAccount(testSubject)) {
		t.Fatalf("Load() = %+v v%d, %v; want a fresh active account in this tenant", stored, version, err)
	}
	globex := withTenant(t, context.Background(), "globex")
	if stored, _, err := userAccountTable.Reader(h.store).Load(globex, testSubject); err != nil || stored.Status != domain.UserAccountSuspended {
		t.Fatalf("Load() in globex = %+v, %v; want the account of the other tenant untouched", stored, err)
	}
}

func TestProvisionUserAccountRejectsAnEmptySubjectWithoutWriting(t *testing.T) {
	h := newHarness(t)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.ProvisionUserAccount(ctx, application.ProvisionUserAccount{Subject: ""})

	if err != nil {
		t.Fatalf("ProvisionUserAccount() error = %v, want a rejection", err)
	}
	if rej, refused := outcome.Rejection(); !refused || rej.Code() != domain.CodeUserAccountSubjectEmpty {
		t.Fatalf("outcome = %+v, want %s", outcome, domain.CodeUserAccountSubjectEmpty)
	}
	if _, _, err := userAccountTable.Reader(h.store).Load(ctx, ""); !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
	}
}

func TestProvisionUserAccountWithoutAKeyIsRefusedBeforeAnyEffect(t *testing.T) {
	h := newHarness(t)
	ctx := port.WithExecutionContext(context.Background(), execctx.Execution(t))

	_, err := h.service.ProvisionUserAccount(ctx, provision())

	if !errors.Is(err, port.ErrIdempotencyKeyAbsent) {
		t.Fatalf("ProvisionUserAccount() = %v, want ErrIdempotencyKeyAbsent", err)
	}
	if slices.Contains(h.rec.Observed(), "user_accounts.Load") {
		t.Fatalf("sequence = %v: a command without a key reached the account", h.rec.Observed())
	}
}

func TestTheSameKeyWithAnotherProvisionIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.ProvisionUserAccount){
		"subject": func(c *application.ProvisionUserAccount) { c.Subject = "user-sub-other" },
		"person":  func(c *application.ProvisionUserAccount) { c.PersonID = "per-other" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.service.ProvisionUserAccount(withKey(t, context.Background(), "k-reused"), provision()); err != nil {
				t.Fatalf("first ProvisionUserAccount() = %v, want nil", err)
			}
			second := provision()
			mutate(&second)

			_, err := h.service.ProvisionUserAccount(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second ProvisionUserAccount() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}

func TestEachFailureOfProvisionUserAccountCarriesTheUseCasePrefix(t *testing.T) {
	for name, faults := range map[string]serviceskit.Faults{
		"load": {Load: errBoom},
		"save": {Save: errBoom},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, withTableFaults("user_accounts", faults))

			_, err := h.service.ProvisionUserAccount(withExecution(t, context.Background()), provision())

			if want := "application: provision user account user-sub-100: boom"; !errors.Is(err, errBoom) || err.Error() != want {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}
