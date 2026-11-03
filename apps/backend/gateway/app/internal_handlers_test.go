package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	gatewayports "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

var interactionsTable = memory.Table[domain.InteractionID, domain.ExternalInteractionSnapshot]{Name: "external_interactions"}

// internalService is the service the internal handlers reach, over memory.
func internalService(store *memory.Store) application.Service {
	return application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{
				ExternalCommands: commandsTable.Repository(tx), ExternalInteractions: interactionsTable.Repository(tx),
				Ledger: tx.AttemptLedger(), Outbox: tx.Outbox(), Commands: tx.CommandInbox(application.CommandConsumer),
				DispatchInbox: tx.Inbox(application.DispatchConsumer), InteractionInbox: tx.Inbox(application.InteractionConsumer),
				NormalizationInbox: tx.Inbox(application.NormalizationConsumer),
			}
		}),
		CommandReader:     commandsTable.Reader(store),
		InteractionReader: interactionsTable.Reader(store),
		Clock:             memory.FixedClock{At: occurred},
		IDs:               &memory.SequenceIDs{Prefix: "m-"},
		Authorize:         usecase.AllowAll[application.Operation](),
		Idempotency:       usecase.IdempotencyPolicy{Wait: int64(time.Second), Retention: int64(24 * time.Hour), Digest: serviceskit.FoldDigest},
		DerivedKeys:       application.DerivedKeyPolicy{Digest: serviceskit.FoldDigest},
		Partners:          gatewayports.NoPartnerAdapters{},
	}
}

func internalReceipt(eventType string) ports.Receipt {
	return ports.Receipt{MessageID: "evt-1", MessageType: eventType, PayloadHash: "h", ReceivedAt: occurred}
}

func TestTheInteractionHandlerRecordsTheOutboundInteractionOfACapture(t *testing.T) {
	store := memory.New()
	handle := app.InteractionHandler(internalService(store))
	captured := &eventv1.ExternalResponseCaptured{
		InteractionId: "int-1", CommandId: "idem-1", ConnectorId: "prt-1:production", Operation: "submitEffect",
		RequestRef: "interactions/prt-1:production/int-1/request", RequestVersionId: "vq", RequestHash: "hq",
		ResponseRef: "interactions/prt-1:production/int-1/response", ResponseVersionId: "vs", ResponseHash: "hs",
		ContentType: "application/json", Step: eventv1.Step_STEP_SUSPEND, MismatchKind: eventv1.MismatchKind_MISMATCH_KIND_VERSION,
		MismatchDetail: "schema 3", PartnerDeclaredVersion: "3.0", At: timestamppb.New(time.Unix(0, int64(occurred))),
	}
	env := packed(t, "com.company.gateway.external-response-captured.v1", captured)

	for i, want := range []usecase.Disposition{usecase.R1D1, usecase.R2} {
		if disposition, err := handle(withExecution(t), internalReceipt(env.Type), env); err != nil || disposition != want {
			t.Fatalf("delivery %d = %v, %v, want %v", i+1, disposition, err, want)
		}
	}
	got, _, err := interactionsTable.Reader(store).Load(withExecution(t), "int-1")
	if err != nil || got.Direction != domain.DirectionOutbound || got.Source != domain.InteractionSubmitResponse ||
		got.CapturedStep != domain.StepSuspend || got.ContentType != "application/json" || got.Response.VersionID != "vs" ||
		got.Mismatch != (domain.Mismatch{Kind: domain.MismatchVersion, Detail: "schema 3", PartnerDeclaredVersion: "3.0"}) {
		t.Fatalf("interaction = %+v, %v, want the outbound record of the capture", got, err)
	}
}

// The intent and the handover name what their handler reads before any call;
// a missing one is a read that has not caught up, redelivered.
func TestTheDispatchAndNormalizationHandlersReachTheirUseCase(t *testing.T) {
	tests := map[string]struct {
		handle func(application.Service) kernelapp.Handler
		env    func(*testing.T) envelope.Envelope
	}{
		"dispatch": {app.DispatchHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-command-dispatch-requested.v1", &eventv1.ExternalCommandDispatchRequested{
				IdempotencyKey: "idem-1", ConnectorId: "prt-1:production", Operation: "submitEffect",
			})
		}},
		"normalization": {app.NormalizationHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-interaction-stored.v1", &eventv1.ExternalInteractionStored{
				InteractionId: "int-1", ConnectorId: "prt-1:production", NormalizationRevision: 1,
			})
		}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			env := tt.env(t)
			disposition, err := tt.handle(internalService(memory.New()))(withExecution(t), internalReceipt(env.Type), env)

			if disposition != usecase.R1D3 || !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("Handle() = %v, %v, want the read redelivered", disposition, err)
			}
		})
	}
}

func TestTheInternalHandlersRefuseWhatIsNotTheirContractWithoutAUnitOfWork(t *testing.T) {
	tests := map[string]struct {
		handle func(application.Service) kernelapp.Handler
		env    func(*testing.T) envelope.Envelope
	}{
		"dispatch without key": {app.DispatchHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-command-dispatch-requested.v1", &eventv1.ExternalCommandDispatchRequested{})
		}},
		"dispatch of another type": {app.DispatchHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-interaction-stored.v1", &eventv1.ExternalInteractionStored{InteractionId: "int-1"})
		}},
		"capture without step": {app.InteractionHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-response-captured.v1", &eventv1.ExternalResponseCaptured{InteractionId: "int-1", CommandId: "idem-1"})
		}},
		"capture without interaction": {app.InteractionHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-response-captured.v1", &eventv1.ExternalResponseCaptured{CommandId: "idem-1", Step: eventv1.Step_STEP_CONFIRM})
		}},
		"handover without revision": {app.NormalizationHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-interaction-stored.v1", &eventv1.ExternalInteractionStored{InteractionId: "int-1"})
		}},
		"handover of another payload": {app.NormalizationHandler, func(t *testing.T) envelope.Envelope {
			return packed(t, "com.company.gateway.external-interaction-stored.v1", &eventv1.ExternalCommandDispatchRequested{IdempotencyKey: "idem-1"})
		}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			store := memory.New()
			env := tt.env(t)

			disposition, err := tt.handle(internalService(store))(withExecution(t), internalReceipt(env.Type), env)

			if disposition != usecase.R1D4 || usecase.CategoryOf(err) != string(usecase.Validation) {
				t.Fatalf("Handle() = %v, %v, want R1×D4 by Validation", disposition, err)
			}
			if store.WithinCalls() != 0 {
				t.Fatalf("a unit of work was opened for a terminal payload")
			}
		})
	}
}

// D-P6: the route covers the partner and both writes of the raw store, so the
// stale cut the dispatch and the sweep share outlasts a submission whose writes
// are slow but within their deadline.
func TestTheDispatchBudgetReservesBothWritesOfTheRawStore(t *testing.T) {
	partner := app.PartnerRoute()
	budget, err := app.DispatchBudget(partner)
	if err != nil {
		t.Fatalf("DispatchBudget() = %v", err)
	}
	put, declared := app.RawStoreSheet().Deadline.Get()
	if !declared || budget.Route.Limit != partner.Limit+2*put {
		t.Fatalf("route limit = %v, want the partner's %v and two writes of %v", budget.Route.Limit, partner.Limit, put)
	}
	slowest := partner.Limit + 2*put + budget.Settle
	if budget.StaleAfter() <= slowest || budget.Processing > budget.StaleAfter() {
		t.Fatalf("stale cut = %v, processing = %v, want the cut past the slowest submission %v and the delivery within it",
			budget.StaleAfter(), budget.Processing, slowest)
	}
	cut := app.DispatchBudgetOf(budget)
	if cut.StaleAfter != int64(budget.StaleAfter()) || cut.SettleContext == nil {
		t.Fatalf("DispatchBudgetOf() = %+v, want the cut of the budget", cut)
	}
	ctx, cancel := cut.SettleContext(context.Background())
	defer cancel()
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > budget.Settle {
		t.Fatalf("settle deadline = %v, %v, want within %v", deadline, ok, budget.Settle)
	}
}
