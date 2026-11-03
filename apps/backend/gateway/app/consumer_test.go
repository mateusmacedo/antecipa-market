package app_test

import (
	"context"
	"slices"
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
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
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
			return application.Resources{
				Connectors: connectorsTable.Repository(tx), Outbox: tx.Outbox(), Inbox: tx.Inbox(app.ConsumerName),
				EventsInbox: tx.Inbox(application.EventsConsumer),
			}
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

func partnerConsumer(t *testing.T, handler *countingHandler) kernelapp.Consumer {
	t.Helper()
	consumer := subscribed(t, service(memory.New()))["partner.events"].Consumer
	if handler != nil {
		consumer.Handle = handler.handle
	}
	return consumer
}

// consumerConfig names every channel the consumer role reads.
func consumerConfig() app.Config {
	cfg := app.Defaults(app.RoleConsumer)
	cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group = "partner.events", "partner.events.dlq", app.ConsumerName
	cfg.GatewayTopic, cfg.GatewayDLQ, cfg.GatewayGroup = "gateway.events", "gateway.events.dlq", "gateway.events"
	cfg.DispatchTopic, cfg.DispatchDLQ, cfg.DispatchGroup = "gateway.dispatch", "gateway.dispatch.dlq", "gateway.dispatch"
	cfg.InteractionTopic, cfg.InteractionDLQ, cfg.InteractionGroup = "gateway.interactions", "gateway.interactions.dlq", "gateway.interactions"
	cfg.NormalizationTopic, cfg.NormalizationDLQ, cfg.NormalizationGroup = "gateway.normalizations", "gateway.normalizations.dlq", "gateway.normalizations"
	return cfg
}

// subscribed is the subscriptions of the consumer role over a service, by the
// logical name of their channel.
func subscribed(t *testing.T, service application.Service) map[string]app.Subscription {
	t.Helper()
	budget, err := app.DispatchBudget(app.PartnerRoute())
	if err != nil {
		t.Fatalf("DispatchBudget() = %v", err)
	}
	byChannel := map[string]app.Subscription{}
	for _, s := range app.Subscriptions(consumerConfig(), app.ConsumerParts{
		Service: service, Budget: budget, Containment: containment{}, Clock: memory.FixedClock{At: occurred},
		Telemetry: app.ConsumerTelemetry{Tracer: noop.NewTracerProvider().Tracer("")},
	}) {
		byChannel[s.Channel.Name] = s
	}
	return byChannel
}

func TestTheConsumerAcceptsTheCapabilityEventsAmongTheElevenOfThePartnerChannel(t *testing.T) {
	consumer := partnerConsumer(t, nil)

	if want := []string{"com.company.partner.capability-enabled.v1", "com.company.partner.capability-suspended.v1"}; !slices.Equal(consumer.Accepts, want) {
		t.Fatalf("Accepts = %v, want %v: the handler unpacks only the capability events (KFK-03c)", consumer.Accepts, want)
	}
	if got := len(consumer.Channel.Carries); got != 11 {
		t.Fatalf("Carries = %v, want the eleven types the Partner publishes on partner.events (KFK-03b)", consumer.Channel.Carries)
	}
	if err := consumer.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil: the subscription stays within the channel", err)
	}
}

func TestTheSinkHandsTheAcceptedTypesAndDiscardsTheRestOfTheChannel(t *testing.T) {
	handler := &countingHandler{}
	sink := kernelapp.Sink{Consumer: partnerConsumer(t, handler)}

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
		t.Fatalf("a type of the set: handled %d, acks %d; want handed to the handler and acknowledged after it", handler.calls, taken.acks)
	}
}

// Each channel of the role is read under a group and into a message inbox of
// its own, accepting only the type its handler takes (KFK-03c), from the one
// producer it trusts; the dispatch runs under the budget of a submission.
func TestEachSubscriptionReadsItsChannelUnderAGroupAndAnInboxOfItsOwn(t *testing.T) {
	cfg := consumerConfig()
	budget, err := app.DispatchBudget(app.PartnerRoute())
	if err != nil {
		t.Fatalf("DispatchBudget() = %v", err)
	}
	want := map[string]struct {
		inbox   string
		accepts []string
		source  string
	}{
		"partner.events": {app.ConsumerName,
			[]string{"com.company.partner.capability-enabled.v1", "com.company.partner.capability-suspended.v1"}, cfg.PartnerSource},
		"gateway.events":         {application.EventsConsumer, []string{schemaMismatchType}, cfg.Relay.Source},
		"gateway.dispatch":       {application.DispatchConsumer, []string{dispatchRequestedType}, cfg.Relay.Source},
		"gateway.interactions":   {application.InteractionConsumer, []string{"com.company.gateway.external-response-captured.v1"}, cfg.Relay.Source},
		"gateway.normalizations": {application.NormalizationConsumer, []string{"com.company.gateway.external-interaction-stored.v1"}, cfg.Relay.Source},
	}
	subscriptions := subscribed(t, service(memory.New()))
	if len(subscriptions) != len(want) {
		t.Fatalf("subscriptions = %v, want the %d channels of the role", subscriptions, len(want))
	}
	groups := map[string]string{}
	for name, w := range want {
		s := subscriptions[name]
		c := s.Consumer
		if err := c.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", name, err)
		}
		if c.Name != w.inbox || !slices.Equal(c.Accepts, w.accepts) || !slices.Equal(c.Boundary.Sources, []string{w.source}) {
			t.Errorf("%s: inbox %q, accepts %v, sources %v; want %q, %v, %v", name, c.Name, c.Accepts, c.Boundary.Sources, w.inbox, w.accepts, w.source)
		}
		if c.Channel.Group != s.Channel.Group || c.Channel.Address != s.Channel.Address || c.MaxAttempts != s.Channel.Retry.MaxAttempts {
			t.Errorf("%s: consumer channel %+v and limit %d, want the subscription's %s %s and %d",
				name, c.Channel, c.MaxAttempts, s.Channel.Address, s.Channel.Group, s.Channel.Retry.MaxAttempts)
		}
		if other, taken := groups[s.Channel.Group]; taken {
			t.Errorf("%s and %s share the group %q: a rebalance of one would pause the other", name, other, s.Channel.Group)
		}
		groups[s.Channel.Group] = name
		timeout := cfg.ConsumerTimeout
		if name == "gateway.dispatch" {
			timeout = budget.Processing
		}
		if c.Timeout != timeout {
			t.Errorf("%s: timeout %v, want %v", name, c.Timeout, timeout)
		}
	}
	if dispatch := subscriptions["gateway.dispatch"]; dispatch.ProcessingDeadline != budget.Processing {
		t.Fatalf("dispatch deadline = %v, want the processing of the budget %v", dispatch.ProcessingDeadline, budget.Processing)
	}
}

const schemaMismatchType = "com.company.gateway.external-schema-mismatch-detected.v1"

// ownRaw is a message of this context's relay as the broker delivers it.
func ownRaw(t *testing.T, source, eventType string, msg proto.Message) []byte {
	t.Helper()
	env := packed(t, eventType, msg)
	tenant := string(testTenant)
	env.Source, env.TenantID = source, &tenant
	env.Subject, env.PartitionKey, env.CorrelationID, env.CausationID = "int-1", "int-1", "corr-1", "evt-0"
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

// L570-572 at the consumer: the mismatch on gateway.events suspends the
// operation, the other eight types of the channel are acknowledged untouched,
// and a mismatch from another producer is refused at the boundary.
func TestTheEventsSubscriptionSuspendsOnTheMismatchAlone(t *testing.T) {
	store := memory.New()
	seedEnabledConnector(t, store)
	contained := &quarantine{}
	consumer := subscribed(t, service(store))["gateway.events"].Consumer
	consumer.Containment = contained
	sink := kernelapp.Sink{Consumer: consumer}
	relay := consumerConfig().Relay.Source
	mismatch := &eventv1.ExternalSchemaMismatchDetected{
		InteractionId: "int-1", ConnectorId: "prt-1:production", ConnectorKey: "prt-1:production", Operation: "submitEffect", AssetType: "duplicata",
	}

	confirmed := &ack{}
	if err := sink.Handle(context.Background(), ownRaw(t, relay, "com.company.gateway.external-command-confirmed.v1",
		&eventv1.ExternalCommandConfirmed{IdempotencyKey: "idem-1"}), 1, confirmed); err != nil || confirmed.acks != 1 {
		t.Fatalf("Handle(confirmed) = %v with %d acks, want acknowledged untouched", err, confirmed.acks)
	}
	foreign := &ack{}
	_ = sink.Handle(context.Background(), ownRaw(t, "urn:dmpf:reference-partner", schemaMismatchType, mismatch), 1, foreign)
	if len(contained.contained) != 1 || contained.contained[0].Reason != ports.ReasonUntrustedBoundary {
		t.Fatalf("contained = %+v, want the mismatch of another producer refused at the boundary", contained.contained)
	}
	connector, _, err := connectorsTable.Reader(store).Load(withExecution(t), "prt-1:production")
	if err != nil || len(connector.Suspensions) != 0 {
		t.Fatalf("connector = %+v, %v, want nothing suspended yet", connector, err)
	}

	taken := &ack{}
	if err := sink.Handle(context.Background(), ownRaw(t, relay, schemaMismatchType, mismatch), 1, taken); err != nil || taken.acks != 1 {
		t.Fatalf("Handle(mismatch) = %v with %d acks, want handled and acknowledged", err, taken.acks)
	}
	connector, _, err = connectorsTable.Reader(store).Load(withExecution(t), "prt-1:production")
	want := []domain.OperationSuspension{{Operation: "submitEffect", Cause: domain.CauseSchemaMismatch, Since: occurred}}
	if err != nil || !slices.Equal(connector.Suspensions, want) {
		t.Fatalf("suspensions = %+v, %v, want %+v", connector.Suspensions, err, want)
	}
}
