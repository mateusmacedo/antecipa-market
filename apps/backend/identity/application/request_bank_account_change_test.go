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
	testBankAccountChangeID   = domain.BankAccountChangeID("bac-100")
	testDestinationEvidenceID = "evd-200"
	testRequesterID           = "user-sub-1"
	testApproverID            = "user-sub-2"
)

func requestBankAccountChange() application.RequestBankAccountChange {
	return application.RequestBankAccountChange{
		BankAccountChangeID:   testBankAccountChangeID,
		OrganizationID:        testOrganizationID,
		DestinationEvidenceID: testDestinationEvidenceID,
		RequestedByUserID:     testRequesterID,
	}
}

func requestedBankAccountChange(id domain.BankAccountChangeID, organization domain.OrganizationID) domain.BankAccountChangeSnapshot {
	return domain.BankAccountChangeSnapshot{
		ID:                    id,
		OrganizationID:        organization,
		DestinationEvidenceID: testDestinationEvidenceID,
		RequestedByUserID:     testRequesterID,
		Status:                domain.BankAccountChangeStatusRequested,
		RequestedAt:           testOccurred,
	}
}

func TestRequestBankAccountChangeWalksTheStepsInOrder(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)

	if _, err := h.service.RequestBankAccountChange(withExecution(t, context.Background()), requestBankAccountChange()); err != nil {
		t.Fatalf("RequestBankAccountChange() error = %v, want nil", err)
	}

	want := []string{
		"authorize",
		"clock.Now",
		"ids.NewMessageID",
		"within",
		"commands.Register",
		"organizations.Load",
		"bank_account_changes.Load",
		"bank_account_changes.Save",
		"outbox.Enqueue",
		"commands.Complete",
		"commit",
	}
	if !slices.Equal(h.rec.Observed(), want) {
		t.Fatalf("sequence mismatch (FND-04 §3.2, spec note †)\ngot:  %v\nwant: %v", h.rec.Observed(), want)
	}
}

func TestRequestBankAccountChangeRequestsItAndEnqueuesTheRequestInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	ctx := withExecution(t, context.Background())

	outcome, err := h.service.RequestBankAccountChange(ctx, requestBankAccountChange())

	if err != nil || outcome.Response() != (domain.RequestBankAccountChangeResponse{BankAccountChangeID: testBankAccountChangeID}) {
		t.Fatalf("RequestBankAccountChange() = %+v, %v; want accepted", outcome, err)
	}
	stored, version, err := bankAccountChangeTable.Reader(h.store).Load(ctx, testBankAccountChangeID)
	if want := requestedBankAccountChange(testBankAccountChangeID, testOrganizationID); err != nil || version != 1 || !stored.Equal(want) {
		t.Fatalf("Load() = %+v v%d, %v; want %+v at v1", stored, version, err, want)
	}
	entries := h.store.Entries()
	if len(entries) != 1 {
		t.Fatalf("outbox entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.AggregateType != application.AggregateTypeBankAccountChange || entry.AggregateID != string(testBankAccountChangeID) ||
		entry.AggregateVersion != 1 || entry.Intent.PartitionKey != string(testBankAccountChangeID) {
		t.Fatalf("outbox entry = %+v, want the request of the change at version 1", entry)
	}
	want := domain.BankAccountChangeRequested{
		BankAccountChangeID:   testBankAccountChangeID,
		OrganizationID:        testOrganizationID,
		DestinationEvidenceID: testDestinationEvidenceID,
		RequestedByUserID:     testRequesterID,
		At:                    testOccurred,
	}
	if got, ok := entry.Event.(domain.BankAccountChangeRequested); !ok || got != want {
		t.Fatalf("event = %+v, want %+v", entry.Event, want)
	}
}

func TestRequestBankAccountChangeRejectsEachPreconditionWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *harness)
		mutate  func(*application.RequestBankAccountChange)
		code    kernel.Code
	}{
		{"destination evidence missing", activeClient, func(c *application.RequestBankAccountChange) { c.DestinationEvidenceID = "" },
			domain.CodeBankAccountChangeDestinationEvidenceMissing},
		{"organization not found", func(*testing.T, *harness) {}, func(*application.RequestBankAccountChange) {},
			domain.CodeBankAccountChangeOrganizationNotActive},
		{"organization suspended", func(t *testing.T, h *harness) { withOrganization(t, h, domain.OrganizationSuspended) },
			func(*application.RequestBankAccountChange) {}, domain.CodeBankAccountChangeOrganizationNotActive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.prepare(t, h)
			cmd := requestBankAccountChange()
			tc.mutate(&cmd)
			ctx := withExecution(t, context.Background())

			outcome, err := h.service.RequestBankAccountChange(ctx, cmd)

			if err != nil {
				t.Fatalf("RequestBankAccountChange() error = %v, want a rejection", err)
			}
			if rej, refused := outcome.Rejection(); !refused || rej.Code() != tc.code {
				t.Fatalf("outcome = %+v, want the rejection %s", outcome, tc.code)
			}
			if _, _, err := bankAccountChangeTable.Reader(h.store).Load(ctx, testBankAccountChangeID); !errors.Is(err, port.ErrNotFound) {
				t.Fatalf("Load() = %v, want ErrNotFound: a rejection writes nothing", err)
			}
			if got := h.outboxLen(); got != 0 {
				t.Fatalf("outbox = %d entries, want none", got)
			}
		})
	}
}

func TestRequestBankAccountChangeOfAnOrganizationOfAnotherTenantIsNotActive(t *testing.T) {
	h := newHarness(t)
	h.seedOrganization(t, withTenant(t, context.Background(), "globex"), registeredSnapshot(testOrganizationID, "globex", testJurisdiction))

	outcome, err := h.service.RequestBankAccountChange(withExecution(t, context.Background()), requestBankAccountChange())

	if rej, refused := outcome.Rejection(); err != nil || !refused || rej.Code() != domain.CodeBankAccountChangeOrganizationNotActive {
		t.Fatalf("RequestBankAccountChange() = %+v, %v; want %s (IDN-13)", outcome, err, domain.CodeBankAccountChangeOrganizationNotActive)
	}
}

func TestEachFailureOfRequestBankAccountChangeCarriesTheUseCasePrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []option
		want    string
	}{
		{"organization load", []option{withLoadError(errBoom)}, "application: request bank account change bac-100: boom"},
		{"load", []option{withTableFaults("bank_account_changes", serviceskit.Faults{Load: errBoom})}, "application: request bank account change bac-100: boom"},
		{"save", []option{withTableFaults("bank_account_changes", serviceskit.Faults{Save: errBoom})}, "application: request bank account change bac-100: boom"},
		{"enqueue", []option{withEnqueueError(errBoom)}, "application: request bank account change bac-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			withOrganization(t, h, domain.OrganizationActive)

			_, err := h.service.RequestBankAccountChange(withExecution(t, context.Background()), requestBankAccountChange())

			if !errors.Is(err, errBoom) || err.Error() != tc.want {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRequestingAnExistingBankAccountChangeCarriesThePrefix(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)
	seed(t, h, withExecution(t, context.Background()), bankAccountChangeTable, testBankAccountChangeID,
		requestedBankAccountChange(testBankAccountChangeID, testOrganizationID))

	_, err := h.service.RequestBankAccountChange(withExecution(t, context.Background()), requestBankAccountChange())

	if want := "application: request bank account change bac-100: ports: aggregate already exists"; !errors.Is(err, port.ErrAlreadyExists) || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestARepeatedRequestBankAccountChangeReplaysWithoutASecondEffect(t *testing.T) {
	h := newHarness(t)
	withOrganization(t, h, domain.OrganizationActive)

	first, err := h.service.RequestBankAccountChange(withKey(t, context.Background(), "k-request"), requestBankAccountChange())
	if err != nil {
		t.Fatalf("first RequestBankAccountChange() = %v, want nil", err)
	}
	again, err := h.service.RequestBankAccountChange(withKey(t, context.Background(), "k-request"), requestBankAccountChange())
	if err != nil || again.Response() != first.Response() {
		t.Fatalf("replay = %+v, %v; want %+v", again.Response(), err, first.Response())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1: the replay produced a second event", got)
	}
}

func TestTheSameKeyWithAnotherBankAccountChangeRequestIsAMismatch(t *testing.T) {
	for name, mutate := range map[string]func(*application.RequestBankAccountChange){
		"bank account change":  func(c *application.RequestBankAccountChange) { c.BankAccountChangeID = "bac-other" },
		"organization":         func(c *application.RequestBankAccountChange) { c.OrganizationID = "org-other" },
		"destination evidence": func(c *application.RequestBankAccountChange) { c.DestinationEvidenceID = "evd-other" },
		"requester":            func(c *application.RequestBankAccountChange) { c.RequestedByUserID = "user-sub-other" },
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			withOrganization(t, h, domain.OrganizationActive)
			if _, err := h.service.RequestBankAccountChange(withKey(t, context.Background(), "k-reused"), requestBankAccountChange()); err != nil {
				t.Fatalf("first RequestBankAccountChange() = %v, want nil", err)
			}
			second := requestBankAccountChange()
			mutate(&second)

			_, err := h.service.RequestBankAccountChange(withKey(t, context.Background(), "k-reused"), second)

			if !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second RequestBankAccountChange() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}
