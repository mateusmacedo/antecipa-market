package app_test

import (
	"context"
	"slices"
	"testing"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
	partnerv1 "github.com/mateusmacedo/antecipa-market/apps/backend/partner/contract/gen/go/company/partner/event/v1"
)

func TestEachBoundaryAdmitsTheOneProducerOfItsChannels(t *testing.T) {
	cfg := consumerConfig()

	partner, own := app.PartnerBoundary(cfg, true), app.OwnBoundary(cfg, true)

	if !slices.Equal(partner.Sources, []string{"urn:dmpf:reference-partner"}) || partner.Transport != kernelapp.TransportVerified {
		t.Fatalf("partner boundary = %+v, want the Partner's source over a verified transport", partner)
	}
	if !slices.Equal(own.Sources, []string{"urn:dmpf:reference-gateway"}) || own.Transport != kernelapp.TransportVerified {
		t.Fatalf("own boundary = %+v, want the source this context's relay stamps over a verified transport", own)
	}
}

// IDN-04: a boundary is verified only on what the transport proved, never on a
// variable saying TLS is on.
func TestABoundaryWithoutAnAuthenticatedClientIsDevelopmentOnly(t *testing.T) {
	cfg := consumerConfig()
	for name, boundary := range map[string]kernelapp.Boundary{
		"partner.events":   app.PartnerBoundary(cfg, false),
		"the own channels": app.OwnBoundary(cfg, false),
	} {
		if boundary.Transport != kernelapp.TransportDevelopmentOnly {
			t.Errorf("%s: Transport = %q, want development-only", name, boundary.Transport)
		}
	}
}

func TestTheAdmittedPartnerIsReadFromTheEnvironment(t *testing.T) {
	pairs := append(consumerVariables(), "PARTNER_SOURCE", "urn:dmpf:another-partner")

	cfg, err := app.FromEnv(app.RoleConsumer, lookup(pairs...))

	if err != nil || cfg.PartnerSource != "urn:dmpf:another-partner" {
		t.Fatalf("FromEnv() = %q, %v; want the source PARTNER_SOURCE declares", cfg.PartnerSource, err)
	}
}

// Only this context's relay publishes its four channels: a message of a type
// the channel accepts, from any other source, is contained at the boundary and
// never reaches its handler; partner.events refuses this context's relay too.
func TestEveryChannelRefusesAForeignSourceBeforeItsHandler(t *testing.T) {
	relay, partner := consumerConfig().Relay.Source, consumerConfig().PartnerSource
	tests := map[string]struct {
		source    string
		eventType string
		message   proto.Message
	}{
		"gateway.events":         {partner, schemaMismatchType, &eventv1.ExternalSchemaMismatchDetected{InteractionId: "int-1"}},
		"gateway.dispatch":       {partner, dispatchRequestedType, &eventv1.ExternalCommandDispatchRequested{IdempotencyKey: "idem-1"}},
		"gateway.interactions":   {partner, "com.company.gateway.external-response-captured.v1", &eventv1.ExternalResponseCaptured{InteractionId: "int-1"}},
		"gateway.normalizations": {partner, "com.company.gateway.external-interaction-stored.v1", &eventv1.ExternalInteractionStored{InteractionId: "int-1"}},
		"partner.events":         {relay, "com.company.partner.capability-suspended.v1", &partnerv1.CapabilitySuspended{PartnerId: "prt-1"}},
	}
	subscriptions := subscribed(t, service(memory.New()))
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			handler, contained := &countingHandler{}, &quarantine{}
			consumer := subscriptions[name].Consumer
			consumer.Handle, consumer.Containment = handler.handle, contained

			_ = kernelapp.Sink{Consumer: consumer}.Handle(context.Background(), ownRaw(t, tc.source, tc.eventType, tc.message), 1, &ack{})

			if handler.calls != 0 || len(contained.contained) != 1 || contained.contained[0].Reason != ports.ReasonUntrustedBoundary {
				t.Fatalf("handled %d, contained %+v; want the message refused at the boundary", handler.calls, contained.contained)
			}
		})
	}
}
