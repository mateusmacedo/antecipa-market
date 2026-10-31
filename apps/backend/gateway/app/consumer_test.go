package app_test

import (
	"context"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	partnerv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

const (
	occurred   = ports.Instant(1_755_432_000_000_000_000)
	testTenant = ports.TenantID("platform")
)

var connectorsTable = memory.Table[domain.ConnectorKey, domain.ConnectorSnapshot]{Name: "connectors"}

func service(store *memory.Store) application.Service {
	return application.Service{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) application.Resources {
			return application.Resources{Connectors: connectorsTable.Repository(tx), Outbox: tx.Outbox(), Inbox: tx.Inbox(app.ConsumerName)}
		}),
		ConnectorReader: connectorsTable.Reader(store),
		Clock:           memory.FixedClock{At: occurred},
		IDs:             &memory.SequenceIDs{Prefix: "m-"},
		Authorize:       usecase.AllowAll[application.Operation](),
		Consumer:        app.ConsumerName,
	}
}

func withExecution(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithTenant(testTenant))
}

func seedConnector(t *testing.T, store *memory.Store) {
	t.Helper()
	connector := domain.NewConnector("prt-1:production")
	if _, rej := connector.Register(domain.RegisterConnector{PartnerID: "prt-1", Environment: domain.EnvironmentProduction, CredentialRef: "vault://x", IntegrationVersion: 1, At: occurred}); rej != nil {
		t.Fatalf("Register() = %v", rej)
	}
	uow := memory.NewUnitOfWork(store, func(tx *memory.Tx) ports.Repository[domain.ConnectorKey, domain.ConnectorSnapshot] {
		return connectorsTable.Repository(tx)
	})
	if err := uow.Within(withExecution(t), func(ctx context.Context, repo ports.Repository[domain.ConnectorKey, domain.ConnectorSnapshot]) error {
		return repo.Save(ctx, "prt-1:production", connector.Snapshot(), 0)
	}); err != nil {
		t.Fatalf("seed connector: %v", err)
	}
}

func packed(t *testing.T, eventType string, msg proto.Message) envelope.Envelope {
	t.Helper()
	payload, typeURL, err := envelope.Pack(msg)
	if err != nil {
		t.Fatalf("Pack() = %v", err)
	}
	return envelope.Envelope{
		ID: "evt-1", Source: "urn:dmpf:reference-partner", SpecVersion: envelope.SpecVersion, Type: eventType,
		Time: timestamppb.New(time.Unix(0, int64(occurred))), DataSchema: typeURL, DataContentType: envelope.ContentType, Payload: payload,
	}
}

func receipt() ports.Receipt {
	return ports.Receipt{MessageID: "evt-1", MessageType: "com.company.partner.capability-suspended.v1", PayloadHash: "h", ReceivedAt: occurred}
}

func TestTheHandlerSuspendsTheOperationOfACapabilitySuspended(t *testing.T) {
	store := memory.New()
	seedConnector(t, store)
	handle := app.Handler(service(store))

	disposition, err := handle(withExecution(t), receipt(), packed(t, "com.company.partner.capability-suspended.v1", &partnerv1.CapabilitySuspended{
		CapabilityId: "cap-1", PartnerId: "prt-1", Operation: "submitEffect", Environment: partnerv1.Environment_ENVIRONMENT_PRODUCTION, Version: 1,
	}))

	if err != nil || disposition != usecase.R1D1 {
		t.Fatalf("Handle() = %v, %v, want R1×D1", disposition, err)
	}
	connector, _, err := connectorsTable.Reader(store).Load(withExecution(t), "prt-1:production")
	if err != nil || len(connector.Suspensions) != 1 || connector.Suspensions[0].Cause != domain.CausePartner {
		t.Fatalf("connector = %+v, %v, want the partner's suspension", connector, err)
	}
}

func TestTheHandlerEnablesTheOperationOfACapabilityEnabled(t *testing.T) {
	store := memory.New()
	seedConnector(t, store)

	disposition, err := app.Handler(service(store))(withExecution(t), receipt(), packed(t, "com.company.partner.capability-enabled.v1", &partnerv1.CapabilityEnabled{
		CapabilityId: "cap-1", PartnerId: "prt-1", Operation: "submitEffect", AssetType: "duplicata",
		Environment: partnerv1.Environment_ENVIRONMENT_PRODUCTION, Version: 2, StateQuerySupported: true,
	}))

	if err != nil || disposition != usecase.R1D1 {
		t.Fatalf("Handle() = %v, %v, want R1×D1", disposition, err)
	}
}

// A payload outside the contract is terminal and opens no unit of work.
func TestTheHandlerRefusesWhatIsNotItsContractWithoutAUnitOfWork(t *testing.T) {
	store := memory.New()
	handle := app.Handler(service(store))
	tests := map[string]envelope.Envelope{
		"no environment": packed(t, "com.company.partner.capability-suspended.v1", &partnerv1.CapabilitySuspended{PartnerId: "prt-1", Operation: "submitEffect"}),
		"an environment outside the enum": packed(t, "com.company.partner.capability-suspended.v1", &partnerv1.CapabilitySuspended{
			PartnerId: "prt-1", Operation: "submitEffect", Environment: partnerv1.Environment(99),
		}),
		"another payload": packed(t, "com.company.partner.capability-enabled.v1", &partnerv1.CapabilitySuspended{
			PartnerId: "prt-1", Environment: partnerv1.Environment_ENVIRONMENT_PRODUCTION,
		}),
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			disposition, err := handle(withExecution(t), receipt(), env)

			if disposition != usecase.R1D4 || err == nil || usecase.CategoryOf(err) != string(usecase.Validation) {
				t.Fatalf("Handle() = %v, %v, want R1×D4 by Validation", disposition, err)
			}
			if store.WithinCalls() != 0 {
				t.Fatalf("a unit of work was opened for a terminal payload")
			}
		})
	}
}

type ack struct{ acks int }

func (a *ack) Ack(context.Context) error     { a.acks++; return nil }
func (a *ack) Release(context.Context) error { return nil }

type countingHandler struct{ calls int }

func (h *countingHandler) handle(context.Context, ports.Receipt, envelope.Envelope) (usecase.Disposition, error) {
	h.calls++
	return usecase.R1D1, nil
}

type containment struct{}

func (containment) Quarantine(context.Context, ports.Contained) error { return nil }

func raw(t *testing.T, eventType string, msg proto.Message) []byte {
	t.Helper()
	env := packed(t, eventType, msg)
	env.Subject, env.PartitionKey, env.CorrelationID, env.CausationID = "cap-1", "cap-1", "corr-1", "evt-1"
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

func TestTheSinkTakesTheClosedSetAndAcknowledgesTheRest(t *testing.T) {
	handler := &countingHandler{}
	sink := app.Sink{
		Consumer: kernelapp.Consumer{
			Name: app.ConsumerName, MaxAttempts: 2, Handle: handler.handle, Containment: containment{}, Clock: memory.FixedClock{At: occurred},
			Timeout: 10 * time.Second, Locale: "en", Tracer: noop.NewTracerProvider().Tracer(""),
			Boundary: kernelapp.Boundary{Transport: kernelapp.TransportDevelopmentOnly, Sources: []string{"urn:dmpf:reference-partner"}},
			Channel:  kernelapp.Channel{Address: "partner.events", Group: app.ConsumerName},
		},
		EventTypes: app.PartnerEventTypes,
	}

	other := &ack{}
	if err := sink.Handle(context.Background(), raw(t, "com.company.partner.partner-registered.v1", &partnerv1.PartnerRegistered{PartnerId: "prt-1"}), 1, other); err != nil {
		t.Fatalf("Handle(partner-registered) = %v", err)
	}
	if other.acks != 1 || handler.calls != 0 {
		t.Fatalf("another type: acks %d, handled %d; want acknowledged untouched", other.acks, handler.calls)
	}

	taken := &ack{}
	if err := sink.Handle(context.Background(), raw(t, "com.company.partner.capability-suspended.v1", &partnerv1.CapabilitySuspended{PartnerId: "prt-1"}), 1, taken); err != nil {
		t.Fatalf("Handle(capability-suspended) = %v", err)
	}
	if handler.calls != 1 || taken.acks != 1 {
		t.Fatalf("a type of the set: handled %d, acks %d; want handed to the adapter and acknowledged after it", handler.calls, taken.acks)
	}
}
