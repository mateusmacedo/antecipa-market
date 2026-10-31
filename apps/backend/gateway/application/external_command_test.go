package application_test

import (
	"context"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func createCommand(key domain.IdempotencyKey, payloadVersion int) application.CreateExternalCommand {
	return application.CreateExternalCommand{
		IdempotencyKey: key, TenantID: "tnt-1", ConnectorKey: testConnector, Operation: "submitEffect", AssetType: "duplicata",
		BusinessKey: "eff-1", PayloadVersion: payloadVersion, Payload: `{"amount":100}`, PayloadHash: "h-" + string(key),
	}
}

// ready is a connector able to receive submitEffect: registered, enabled by
// the Partner and declared in a profile in force.
func ready(t *testing.T, operations ...domain.OperationCapability) *harness {
	t.Helper()
	h := newHarness(t)
	h.registered(t)
	h.enabled(t, "submitEffect", true, 1)
	if len(operations) == 0 {
		operations = []domain.OperationCapability{simulatorOperation("submitEffect")}
	}
	h.declared(t, operations...)
	return h
}

func (h *harness) created(t *testing.T, key domain.IdempotencyKey, payloadVersion int) domain.ExternalCommandSnapshot {
	t.Helper()
	must(h.service.CreateExternalCommand(withExecution(t), createCommand(key, payloadVersion))).accepted(t)
	return h.command(t, key)
}

// force rewrites a stored command, for a state only the partner's side of the
// cycle reaches.
func (h *harness) force(t *testing.T, key domain.IdempotencyKey, change func(*domain.ExternalCommandSnapshot)) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) port.Repository[domain.IdempotencyKey, domain.ExternalCommandSnapshot] {
		return commandsTable.Repository(tx)
	})
	err := uow.Within(withExecution(t), func(ctx context.Context, repo port.Repository[domain.IdempotencyKey, domain.ExternalCommandSnapshot]) error {
		s, version, err := repo.Load(ctx, key)
		if err != nil {
			return err
		}
		change(&s)
		return repo.Save(ctx, key, s, version)
	})
	if err != nil {
		t.Fatalf("force %s: %v", key, err)
	}
	h.fakes.MarkSeeded()
}

func TestCreateExternalCommandIsDurableBeforeAnySubmission(t *testing.T) {
	h := ready(t)

	got := must(h.service.CreateExternalCommand(withExecution(t), createCommand(testCommand, 1))).accepted(t)

	if got != (domain.CommandStatusResponse{IdempotencyKey: testCommand, Status: domain.StatusCreated}) {
		t.Fatalf("Response() = %+v", got)
	}
	s := h.command(t, testCommand)
	want := domain.CommandProfileOf(2, simulatorOperation("submitEffect"), true)
	if s.Status != domain.StatusCreated || s.Profile != want || len(s.OutboundKey) != 32 || s.Submissions != 0 {
		t.Fatalf("command = %+v, want created with the declaration copied and a 128-bit outbound key", s)
	}
	entries := h.entries()
	if len(entries) != 1 {
		t.Fatalf("outbox = %+v, want only the dispatch intent", entries)
	}
	intent := entries[0]
	if intent.Intent.Destination != application.DispatchDestination || intent.Intent.PartitionKey != string(testCommand) ||
		intent.Event != (domain.ExternalCommandDispatchRequested{IdempotencyKey: testCommand, ConnectorKey: testConnector, Operation: "submitEffect", At: testOccurred}) {
		t.Fatalf("outbox entry = %+v", intent)
	}
}

func TestCreateExternalCommandCopiesTheMostRestrictiveStateQuery(t *testing.T) {
	h := newHarness(t)
	h.registered(t)
	h.enabled(t, "submitEffect", false, 1)
	h.declared(t, simulatorOperation("submitEffect"))

	s := h.created(t, testCommand, 1)

	if s.Profile.StateQuerySupported {
		t.Fatalf("Profile = %+v, want no state query when the Partner homologated none", s.Profile)
	}
}

func TestCreateExternalCommandRefusesWithoutAnyExternalCall(t *testing.T) {
	unsupported := simulatorOperation("submitEffect")
	unsupported.Supported = false
	tests := []struct {
		name  string
		given func(t *testing.T) *harness
		code  kernel.Code
	}{
		{"a connector without the capability enabled", func(t *testing.T) *harness {
			h := newHarness(t)
			h.registered(t)
			h.declared(t, simulatorOperation("submitEffect"))
			return h
		}, domain.CodeExternalCommandCapabilityNotEnabled},
		{"an operation the profile in force does not support", func(t *testing.T) *harness { return ready(t, unsupported) },
			domain.CodeExternalCommandCapabilityNotEnabled},
		{"no connector at all", func(t *testing.T) *harness { return newHarness(t) }, domain.CodeExternalCommandCapabilityNotEnabled},
		{"a suspended operation", func(t *testing.T) *harness {
			h := ready(t)
			must(h.service.SuspendOperation(withExecution(t), application.SuspendOperation{ConnectorKey: testConnector, Operation: "submitEffect", Cause: domain.CauseSchemaMismatch})).accepted(t)
			return h
		}, domain.CodeExternalCommandOperationSuspended},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.given(t)

			outcome, err := h.service.CreateExternalCommand(withExecution(t), createCommand(testCommand, 1))

			must(outcome, err).refused(t, string(tc.code))
			if n := len(h.entries()); n != 0 {
				t.Fatalf("outbox has %d entries, want no dispatch intent", n)
			}
		})
	}
}

// Only the writes of the ports are operations of an ExternalCommand (spec L457):
// a read the profile supports and the Partner enabled still has no cycle here.
func TestCreateExternalCommandRefusesAnOperationThatIsNotAWrite(t *testing.T) {
	h := ready(t, simulatorOperation("submitEffect"), simulatorOperation("queryAsset"))
	h.enabled(t, "queryAsset", true, 1)
	cmd := createCommand(testCommand, 1)
	cmd.Operation = "queryAsset"

	outcome, err := h.service.CreateExternalCommand(withExecution(t), cmd)

	must(outcome, err).refused(t, string(domain.CodeExternalCommandCapabilityNotEnabled))
	if n := len(h.entries()); n != 0 {
		t.Fatalf("outbox has %d entries, want no dispatch intent", n)
	}
}

func TestCreateExternalCommandRefusesAnEmptyBusinessKeyOrPayload(t *testing.T) {
	h := ready(t)
	noKey := createCommand(testCommand, 1)
	noKey.BusinessKey = ""
	noPayload := createCommand(testCommand, 1)
	noPayload.Payload = ""

	outcome, err := h.service.CreateExternalCommand(withExecution(t), noKey)
	must(outcome, err).refused(t, string(domain.CodeExternalCommandBusinessKeyEmpty))
	outcome, err = h.service.CreateExternalCommand(withExecution(t), noPayload)
	must(outcome, err).refused(t, string(domain.CodeExternalCommandPayloadEmpty))
}

func TestCreateExternalCommandReturnsTheExistingCommandWithoutASecondIntent(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)

	got := must(h.service.CreateExternalCommand(withExecution(t), createCommand(testCommand, 1))).accepted(t)

	if got.Status != domain.StatusCreated || len(h.entries()) != 1 {
		t.Fatalf("Response() = %+v with %d outbox entries, want the existing command and one intent", got, len(h.entries()))
	}
}

func TestCreateExternalCommandRefusesAnotherPayloadUnderATakenKey(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	other := createCommand(testCommand, 1)
	other.PayloadHash = "h-other"

	outcome, err := h.service.CreateExternalCommand(withExecution(t), other)

	must(outcome, err).refused(t, string(domain.CodeExternalCommandIdempotencyKeyConflict))
}

func TestCreateExternalCommandGuardsTheSeries(t *testing.T) {
	tests := []struct {
		name    string
		first   func(*domain.ExternalCommandSnapshot)
		refused bool
	}{
		{"a version still created", func(*domain.ExternalCommandSnapshot) {}, true},
		{"a version sent and confirmed", func(s *domain.ExternalCommandSnapshot) { s.Status = domain.StatusConfirmed }, true},
		{"a version in a retryable failure", func(s *domain.ExternalCommandSnapshot) { s.Status = domain.StatusRetryableFailure }, true},
		{"a version cancelled", func(s *domain.ExternalCommandSnapshot) { s.Status = domain.StatusCancelled }, false},
		{"a version refused for its validation", func(s *domain.ExternalCommandSnapshot) {
			s.Status, s.FailureCategory = domain.StatusFinalFailure, domain.CategoryFinalValidation
		}, false},
		{"a version refused for a business rule", func(s *domain.ExternalCommandSnapshot) {
			s.Status, s.FailureCategory = domain.StatusFinalFailure, domain.CategoryFinalBusiness
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := ready(t)
			first := h.created(t, "idem-v1", 1)
			h.force(t, "idem-v1", tc.first)

			outcome, err := h.service.CreateExternalCommand(withExecution(t), createCommand("idem-v2", 2))

			if tc.refused {
				must(outcome, err).refused(t, string(domain.CodeExternalCommandContentChangedAfterSend))
				return
			}
			must(outcome, err).accepted(t)
			if second := h.command(t, "idem-v2"); second.OutboundKey == first.OutboundKey || second.Status != domain.StatusCreated {
				t.Fatalf("second version = %+v, want created under another outbound key", second)
			}
		})
	}
}

func TestCancelExternalCommandCancelsWhatWasNeverSent(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)

	must(h.service.CancelExternalCommand(withExecution(t), application.CancelExternalCommand{IdempotencyKey: testCommand})).accepted(t)

	if got := h.command(t, testCommand).Status; got != domain.StatusCancelled {
		t.Fatalf("Status = %q, want cancelled", got)
	}
}

func TestCancelExternalCommandRefusesACommandAlreadySent(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.force(t, testCommand, func(s *domain.ExternalCommandSnapshot) { s.Status, s.Submissions = domain.StatusAcknowledged, 1 })

	outcome, err := h.service.CancelExternalCommand(withExecution(t), application.CancelExternalCommand{IdempotencyKey: testCommand})

	must(outcome, err).refused(t, string(domain.CodeExternalCommandAlreadySent))
}

func TestFindExternalCommandsByBusinessKeyNarrowsToTheConnector(t *testing.T) {
	h := ready(t)
	h.created(t, testCommand, 1)
	h.force(t, testCommand, func(s *domain.ExternalCommandSnapshot) { s.Status = domain.StatusCancelled })
	h.created(t, "idem-2", 2)

	found, err := h.service.FindExternalCommandsByBusinessKey(withExecution(t), testConnector, "eff-1")
	if err != nil || len(found) != 2 {
		t.Fatalf("FindExternalCommandsByBusinessKey() = %+v, %v", found, err)
	}
	other, err := h.service.FindExternalCommandsByBusinessKey(withExecution(t), "prt-2:production", "eff-1")
	if err != nil || len(other) != 0 {
		t.Fatalf("another connector found %+v, %v", other, err)
	}
	byStatus, err := h.service.FindExternalCommandsByStatus(withExecution(t), domain.StatusCancelled, "")
	if err != nil || len(byStatus) != 1 || byStatus[0].Key != testCommand {
		t.Fatalf("FindExternalCommandsByStatus() = %+v, %v", byStatus, err)
	}
	command, err := h.service.FindExternalCommand(withExecution(t), "idem-2")
	if err != nil || command.PayloadVersion != 2 {
		t.Fatalf("FindExternalCommand() = %+v, %v", command, err)
	}
}
