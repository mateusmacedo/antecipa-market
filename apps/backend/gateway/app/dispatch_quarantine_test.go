package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	gatewayports "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

const dispatchRequestedType = "com.company.gateway.external-command-dispatch-requested.v1"

// upstream answers every submission as the partner's 503 would, the retryable
// class whose redelivery the consumer bounds.
type upstream struct {
	gatewayports.NoPartnerAdapters
	calls *int
}

func (u upstream) Command(domain.ConnectorKey, string, *gatewayports.CaptureCollector) (ports.ExternalCommand, error) {
	return u, nil
}

func (u upstream) Submit(context.Context, ports.OutboundKey, []byte) (ports.ExternalClass, ports.Observation, error) {
	*u.calls++
	return ports.ClassRetryableUpstream, ports.Observation{}, errors.New("partner: 503")
}

type quarantine struct{ contained []ports.Contained }

func (q *quarantine) Quarantine(_ context.Context, c ports.Contained) error {
	q.contained = append(q.contained, c)
	return nil
}

func seedEnabledConnector(t *testing.T, store *memory.Store) {
	t.Helper()
	seedConnector(t, store)
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Repository[domain.ConnectorKey, domain.ConnectorSnapshot] {
		return connectorsTable.Repository(tx)
	})
	if err := uow.Within(withExecution(t), func(ctx context.Context, repo ports.Repository[domain.ConnectorKey, domain.ConnectorSnapshot]) error {
		s, version, err := repo.Load(ctx, "prt-1:production")
		if err != nil {
			return err
		}
		connector := domain.FromConnectorSnapshot(s)
		if _, rej := connector.EnableOperation(domain.EnableOperation{
			CapabilityID: "cap-1", Operation: "submitEffect", StateQuerySupported: true, Version: 1, At: occurred,
		}); rej != nil {
			t.Fatalf("EnableOperation() = %v", rej)
		}
		return repo.Save(ctx, "prt-1:production", connector.Snapshot(), version)
	}); err != nil {
		t.Fatalf("enable connector: %v", err)
	}
}

func rawIntent(t *testing.T) []byte {
	t.Helper()
	env := packed(t, dispatchRequestedType, &eventv1.ExternalCommandDispatchRequested{
		IdempotencyKey: "idem-1", ConnectorId: "prt-1:production", Operation: "submitEffect",
	})
	tenant := string(testTenant)
	env.TenantID = &tenant
	env.Subject, env.PartitionKey, env.CorrelationID, env.CausationID = "idem-1", "idem-1", "corr-1", "evt-1"
	env.TraceParent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	ce, err := envelope.Encode(env)
	if err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	out, err := proto.MarshalOptions{Deterministic: true}.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal() = %v", err)
	}
	return out
}

// L666-668: the intent the partner keeps answering with a retryable class is
// redelivered up to the consumer's limit and then quarantined, while the
// command stays created, still cancellable.
func TestAnIntentThatExhaustsTheRedeliveryIsQuarantinedAndTheCommandStaysCreated(t *testing.T) {
	store := memory.New()
	seedEnabledConnector(t, store)
	seedCreatedCommand(t, store)
	calls := 0
	service := internalService(store)
	service.Partners = upstream{calls: &calls}
	service.Attempts = memory.NewAttemptStore(store)
	service.ConnectorReader = connectorsTable.Reader(store)
	service.Budget = application.DispatchBudget{
		StaleAfter:    int64(staleAfter),
		SettleContext: func(ctx context.Context) (context.Context, context.CancelFunc) { return context.WithCancel(ctx) },
	}
	cfg := app.Defaults(app.RoleConsumer)
	cfg.DispatchTopic, cfg.DispatchDLQ, cfg.Group = "gateway.dispatch", "gateway.dispatch.dlq", app.ConsumerName
	ch := app.DispatchChannel(cfg)
	contained := &quarantine{}
	consumer := kernelapp.Consumer{
		Name: application.DispatchConsumer, MaxAttempts: 3, Handle: app.DispatchHandler(service),
		Accepts: []string{dispatchRequestedType}, Containment: contained, Clock: memory.FixedClock{At: occurred},
		Timeout:  10 * time.Second,
		Boundary: kernelapp.Boundary{Transport: kernelapp.TransportDevelopmentOnly, Sources: []string{"urn:dmpf:reference-partner"}},
		Locale:   "en", Tracer: noop.NewTracerProvider().Tracer(""),
		Channel: kernelapp.Channel{Address: ch.Address, Group: ch.Group, Carries: channel.WireTypes(ch)},
	}
	if err := consumer.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	sink := kernelapp.Sink{Consumer: consumer}

	for attempt := 1; attempt <= 3; attempt++ {
		_ = sink.Handle(context.Background(), rawIntent(t), attempt, &ack{})
	}

	if calls != 3 || len(contained.contained) != 1 || contained.contained[0].Reason != ports.ReasonAttemptsExhausted {
		t.Fatalf("calls = %d, contained = %+v, want three submissions and the intent quarantined once exhausted", calls, contained.contained)
	}
	command, _, err := commandsTable.Reader(store).Load(withExecution(t), "idem-1")
	if err != nil || command.Status != domain.StatusCreated || command.Submissions != 0 {
		t.Fatalf("command = %+v, %v, want created, with nothing applied, so it can still be cancelled", command, err)
	}
	if _, rej := domain.FromExternalCommandSnapshot(command).Cancel(domain.CancelExternalCommand{At: occurred}); rej != nil {
		t.Fatalf("Cancel() = %v, want the quarantined command cancellable", rej)
	}
}
