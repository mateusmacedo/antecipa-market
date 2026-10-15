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

func approveBankAccountChange() application.ApproveBankAccountChange {
	return application.ApproveBankAccountChange{BankAccountChangeID: testBankAccountChangeID, ApprovedByUserID: testApproverID}
}

func approvedBankAccountChange(id domain.BankAccountChangeID, organization domain.OrganizationID) domain.BankAccountChangeSnapshot {
	s := requestedBankAccountChange(id, organization)
	s.Status = domain.BankAccountChangeStatusApproved
	s.ApprovedByUserID = testApproverID
	s.ApprovedAt = testOccurred
	return s
}

func withBankAccountChange(t *testing.T, h *harness, change domain.BankAccountChangeSnapshot) {
	t.Helper()
	seed(t, h, withExecution(t, context.Background()), bankAccountChangeTable, change.ID, change)
	h.rec.Reset()
}

func TestApproveBankAccountChangeWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withBankAccountChange(t, h, requestedBankAccountChange(testBankAccountChangeID, testOrganizationID))

	if _, err := h.service.ApproveBankAccountChange(withExecution(t, context.Background()), approveBankAccountChange()); err != nil {
		t.Fatalf("ApproveBankAccountChange() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"bank_account_changes.Load",
		"bank_account_changes.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestApproveBankAccountChangeApprovesItAndEnqueuesTheApprovalInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	withBankAccountChange(t, h, requestedBankAccountChange(testBankAccountChangeID, testOrganizationID))
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.ApproveBankAccountChange(ctx, approveBankAccountChange())

	if err != nil || outcome.Response() != (domain.ApproveBankAccountChangeResponse{BankAccountChangeID: testBankAccountChangeID}) {
		t.Fatalf("ApproveBankAccountChange() = %+v, %v; want accepted", outcome, err)
	}
	stored, version, err := bankAccountChangeTable.Reader(h.store).Load(ctx, testBankAccountChangeID)
	if want := approvedBankAccountChange(testBankAccountChangeID, testOrganizationID); err != nil || version != 2 || !stored.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v at v2", stored, version, err, want)
	}
	entries := h.store.Entries()
	if len(entries) != 1 || entries[0].AggregateType != application.AggregateTypeBankAccountChange ||
		entries[0].AggregateVersion != 2 || entries[0].Intent.PartitionKey != string(testBankAccountChangeID) {
		t.Fatalf("outbox entries = %+v, want the approval of the change at version 2", entries)
	}
	want := domain.BankAccountChangeApproved{
		BankAccountChangeID: testBankAccountChangeID,
		OrganizationID:      testOrganizationID,
		ApprovedByUserID:    testApproverID,
		At:                  testOccurred,
	}
	if got, ok := entries[0].Event.(domain.BankAccountChangeApproved); !ok || got != want {
		t.Fatalf("event = %+v, want %+v", entries[0].Event, want)
	}
}

func TestApproveBankAccountChangeRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name   string
		held   domain.BankAccountChangeSnapshot
		mutate func(*application.ApproveBankAccountChange)
		code   kernel.Code
	}{
		{"change already approved", approvedBankAccountChange(testBankAccountChangeID, testOrganizationID),
			func(*application.ApproveBankAccountChange) {}, domain.CodeBankAccountChangeNotRequested},
		{"approved by the requester", requestedBankAccountChange(testBankAccountChangeID, testOrganizationID),
			func(c *application.ApproveBankAccountChange) { c.ApprovedByUserID = testRequesterID }, domain.CodeBankAccountChangeSameActorApproval},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			withBankAccountChange(t, h, tc.held)
			cmd := approveBankAccountChange()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.ApproveBankAccountChange(ctx, cmd)

			if err != nil {
				t.Fatalf("ApproveBankAccountChange() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if stored, version, _ := bankAccountChangeTable.Reader(h.store).Load(ctx, testBankAccountChangeID); version != 1 || !stored.Equal(tc.held) {
				t.Fatalf("Load() = %+v v%d, want the change untouched at v1", stored, version)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
		})
	}
}

func TestApprovingAnUnknownBankAccountChangeIsNotFound(t *testing.T) {
	h := newHarness(t)

	_, err := h.service.ApproveBankAccountChange(withExecution(t, context.Background()), approveBankAccountChange())

	if want := "application: approve bank account change bac-100: ports: aggregate not found"; !errors.Is(err, port.ErrNotFound) || err.Error() != want {
		t.Fatalf("ApproveBankAccountChange() = %v, want %q", err, want)
	}
}

func TestApprovingABankAccountChangeOfAnotherTenantIsNotFound(t *testing.T) {
	h := newHarness(t)
	seed(t, h, withTenant(t, context.Background(), "globex"), bankAccountChangeTable, testBankAccountChangeID,
		requestedBankAccountChange(testBankAccountChangeID, testOrganizationID))

	_, err := h.service.ApproveBankAccountChange(withExecution(t, context.Background()), approveBankAccountChange())

	if !errors.Is(err, port.ErrNotFound) {
		t.Fatalf("ApproveBankAccountChange() = %v, want ErrNotFound (IDN-13)", err)
	}
}

func TestEachFailureOfApproveBankAccountChangeCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		want    string
	}{
		{"load", []option{withTableFaults("bank_account_changes", serviceskit.Faults{Load: errBoom})}, "application: approve bank account change bac-100: boom"},
		{"save", []option{withTableFaults("bank_account_changes", serviceskit.Faults{Save: errBoom})}, "application: approve bank account change bac-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, "application: approve bank account change bac-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withBankAccountChange(t, h, requestedBankAccountChange(testBankAccountChangeID, testOrganizationID))

			_, err := h.service.ApproveBankAccountChange(withExecution(t, context.Background()), approveBankAccountChange())

			if !errors.Is(err, errBoom) || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestTheSameKeyWithAnotherApprovalIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.ApproveBankAccountChange){
		"bank account change": func(c *application.ApproveBankAccountChange) { c.BankAccountChangeID = "bac-other" },
		"approver":            func(c *application.ApproveBankAccountChange) { c.ApprovedByUserID = "user-sub-other" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withBankAccountChange(t, h, requestedBankAccountChange(testBankAccountChangeID, testOrganizationID))
			if _, err := h.service.ApproveBankAccountChange(withKey(t, context.Background(), "k-reused"), approveBankAccountChange()); err != nil {
				t.Fatalf("first ApproveBankAccountChange() = %v, want nil", err)
			}
			second := approveBankAccountChange()
			mutate(&second)

			_, err := h.service.ApproveBankAccountChange(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second ApproveBankAccountChange() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
