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

func suspend() application.SuspendUser {
	return application.SuspendUser{UserID: testSubject, Reason: "acesso indevido"}
}

func TestSuspendUserWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withAccount(t, h, activeAccount(testSubject))

	if _, err := h.service.SuspendUser(withExecution(t, context.Background()), suspend()); err != nil {
		t.Fatalf("SuspendUser() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"user_accounts.Load",
		"user_accounts.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestSuspendUserSuspendsTheAccountAndEnqueuesTheFactInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	withAccount(t, h, activeAccount(testSubject))
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.SuspendUser(ctx, suspend())

	if err != nil || outcome.Response() != (domain.SuspendUserResponse{UserID: testSubject}) {
		t.Fatalf("SuspendUser() = %+v, %v; want accepted", outcome, err)
	}
	stored, version, err := userAccountTable.Reader(h.store).Load(ctx, testSubject)
	if err != nil || version != 2 || !stored.Equal(suspendedAccount(testSubject)) {
		t.Fatalf("Load() = %+v v%d, %v; want the suspended account at v2", stored, version, err)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeUserAccount || entry.AggregateID != string(testSubject) ||
		entry.AggregateVersion != 2 || entry.Intent.Destination != application.Destination ||
		entry.Intent.PartitionKey != string(testSubject) {
		t.Fatalf("outbox entry = %+v, want the suspension of the account at version 2", entry)
	}
	want := domain.UserSuspended{UserID: testSubject, Reason: "acesso indevido", At: testOccurred}
	if got, ok := entry.Event.(domain.UserSuspended); !ok || got != want {
		t.Fatalf("event = %+v, want %+v", entry.Event, want)
	}
}

func TestSuspendUserRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   domain.UserAccountSnapshot
		reason string
		code   kernel.Code
	}{
		{"account suspended", suspendedAccount(testSubject), "acesso indevido", domain.CodeUserAccountNotActive},
		{"reason empty", activeAccount(testSubject), "", domain.CodeUserAccountReasonEmpty},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			withAccount(t, h, tc.held)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.SuspendUser(ctx, application.SuspendUser{UserID: testSubject, Reason: tc.reason})

			if err != nil {
				t.Fatalf("SuspendUser() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if stored, version, _ := userAccountTable.Reader(h.store).Load(ctx, testSubject); version != 1 || !stored.Equal(tc.held) {
				t.Fatalf("Load() = %+v v%d, want the account untouched at v1", stored, version)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
		})
	}
}

func TestSuspendingAnUnknownAccountIsNotFound(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withTenant(t, context.Background(), "globex"), userAccountTable, testSubject, activeAccount(testSubject))

	_, err := h.service.SuspendUser(withExecution(t, context.Background()), suspend())

	if want := "application: suspend user user-sub-100: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("SuspendUser() = %v, want %q: the account of another tenant does not exist here", err, want)
	}
}

func TestEachFailureOfSuspendUserCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		want    string
	}{
		{"load", []option{withTableFaults("user_accounts", serviceskit.Faults{Load: errBoom})}, "application: suspend user user-sub-100: boom"},
		{"save", []option{withTableFaults("user_accounts", serviceskit.Faults{Save: errBoom})}, "application: suspend user user-sub-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, "application: suspend user user-sub-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withAccount(t, h, activeAccount(testSubject))

			_, err := h.service.SuspendUser(withExecution(t, context.Background()), suspend())

			if !errors.Is(err, errBoom) || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestTheSameKeyWithAnotherSuspensionIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.SuspendUser){
		"user":   func(c *application.SuspendUser) { c.UserID = "user-sub-other" },
		"reason": func(c *application.SuspendUser) { c.Reason = "outro motivo" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withAccount(t, h, activeAccount(testSubject))
			if _, err := h.service.SuspendUser(withKey(t, context.Background(), "k-reused"), suspend()); err != nil {
				t.Fatalf("first SuspendUser() = %v, want nil", err)
			}
			second := suspend()
			mutate(&second)

			_, err := h.service.SuspendUser(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second SuspendUser() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
