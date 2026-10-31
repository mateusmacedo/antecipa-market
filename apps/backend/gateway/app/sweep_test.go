package app_test

import (
	"context"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	outboundKey = ports.OutboundKey("0123456789abcdef0123456789abcdef")
	staleAfter  = ports.Instant(10_000_000_000)
)

var commandsTable = memory.Table[domain.IdempotencyKey, domain.ExternalCommandSnapshot]{Name: "external_commands"}

type byOutboundKey struct{ store *memory.Store }

func (r byOutboundKey) LoadByOutboundKey(ctx context.Context, key ports.OutboundKey) ([]domain.ExternalCommandSnapshot, error) {
	s, _, err := commandsTable.Reader(r.store).Load(ctx, "idem-1")
	if err != nil || s.OutboundKey != string(key) {
		return nil, nil
	}
	return []domain.ExternalCommandSnapshot{s}, nil
}

func seedCreatedCommand(t *testing.T, store *memory.Store) {
	t.Helper()
	command := domain.NewExternalCommand("idem-1")
	operation := domain.OperationCapability{
		Operation: "submitEffect", Supported: true, StateQuerySupported: true, IdempotencySupport: domain.IdempotencyKeyHeader,
		Efficacy: domain.EfficacyOnSubmit, Quiescence: 1, MaxQueries: 3, KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32,
		QueryInterval: 60, AmbiguityDeadline: 3600,
	}
	if _, rej := command.Create(domain.CreateExternalCommand{
		TenantID: "tnt-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", BusinessKey: "eff-1", PayloadVersion: 1,
		Payload: "{}", PayloadHash: "h", Profile: domain.CommandProfileOf(1, operation, true), OutboundKey: string(outboundKey),
		CapabilityGate: domain.GateAllowed, SeriesOpen: true, At: occurred,
	}); rej != nil {
		t.Fatalf("Create() = %v", rej)
	}
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Repository[domain.IdempotencyKey, domain.ExternalCommandSnapshot] {
		return commandsTable.Repository(tx)
	})
	if err := uow.Within(withExecution(t), func(ctx context.Context, repo ports.Repository[domain.IdempotencyKey, domain.ExternalCommandSnapshot]) error {
		return repo.Save(ctx, "idem-1", command.Snapshot(), 0)
	}); err != nil {
		t.Fatalf("seed command: %v", err)
	}
}

// The kernel's sweep finds the attempt a crash left begun and hands it to the
// context's Abandon: ClassUnknownSent is presumed and nobody is called (RES-43).
func TestTheKernelSweepAbandonsAStaleAttemptThroughTheContext(t *testing.T) {
	store := memory.New()
	seedCreatedCommand(t, store)
	ledger := memory.NewAttemptStore(store)
	if _, err := ledger.Begin(withExecution(t), outboundKey, "m-1", occurred); err != nil {
		t.Fatalf("Begin() = %v", err)
	}
	service := application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{ExternalCommands: commandsTable.Repository(tx), Ledger: tx.AttemptLedger(), Outbox: tx.Outbox()}
		}),
		CommandsByOutboundKey: byOutboundKey{store: store},
		Clock:                 memory.FixedClock{At: occurred + staleAfter},
		IDs:                   &memory.SequenceIDs{Prefix: "m-"},
		Authorize:             usecase.AllowAll[application.Operation](),
	}
	sweep, err := kernelapp.SweepExternalAttempts(kernelapp.ExternalSweep{
		Store: ledger, Abandon: service.AbandonExternalAttempt, Timeout: 5 * time.Second, Locale: "en",
	})
	if err != nil {
		t.Fatalf("SweepExternalAttempts() = %v", err)
	}

	page, err := sweep(context.Background(), occurred, ports.AttemptCursor{}, 10)

	if err != nil || page.Swept != 1 {
		t.Fatalf("sweep = %+v, %v, want the begun attempt closed", page, err)
	}
	command, _, err := commandsTable.Reader(store).Load(withExecution(t), "idem-1")
	if err != nil || command.Status != domain.StatusUnknownOutcome || command.FailureCategory != domain.CategoryUnknownSent || command.Submissions != 1 {
		t.Fatalf("command = %+v, %v, want unknown_outcome presumed sent", command, err)
	}
	var names []string
	for _, entry := range store.Entries() {
		names = append(names, entry.Event.EventName())
	}
	if len(names) != 2 || names[0] != "gateway.external-command-submitted" || names[1] != "gateway.external-outcome-unknown" {
		t.Fatalf("outbox = %v, want submitted then outcome unknown", names)
	}
}
