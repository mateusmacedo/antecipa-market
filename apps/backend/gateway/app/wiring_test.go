package app

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
)

func channelConfig() Config {
	cfg := Defaults(RoleRelay)
	cfg.GatewayTopic, cfg.GatewayDLQ = "gateway.events", "gateway.events.dlq"
	cfg.DispatchTopic, cfg.DispatchDLQ = "gateway.dispatch", "gateway.dispatch.dlq"
	cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group = "partner.events", "partner.events.dlq", "gateway"
	return cfg
}

func TestTheRelayAddressesEachChannelByItsPhysicalTopic(t *testing.T) {
	cfg := channelConfig()
	catalog, err := channel.NewCatalog(GatewayChannel(cfg), DispatchChannel(cfg))
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}

	for destination, topic := range map[string]string{
		application.Destination:         cfg.GatewayTopic,
		application.DispatchDestination: cfg.DispatchTopic,
	} {
		if got := catalog.AddressOf(destination); got != topic {
			t.Errorf("AddressOf(%q) = %q, want the physical topic %q", destination, got, topic)
		}
	}
	if topic := catalog.AddressOf("uncatalogued.events"); topic != "" {
		t.Fatalf("AddressOf(uncatalogued.events) = %q, want empty: the send never names a logical channel", topic)
	}
}

// The catalogue declares the base type and the kernel appends the major; the
// mapper, the Sink and the publisher must agree on the versioned value.
func TestEachChannelCarriesTheTypeItsWriterOrReaderUses(t *testing.T) {
	cfg := channelConfig()
	cases := map[string]struct {
		ch          channel.Channel
		eventType   string
		contract    string
		containment string
	}{
		"gateway.events": {GatewayChannel(cfg), "com.company.gateway.external-command-submitted.v1",
			"company/gateway/event/v1/external_command_submitted.proto#ExternalCommandSubmitted", cfg.GatewayDLQ},
		"gateway.dispatch": {DispatchChannel(cfg), "com.company.gateway.external-command-dispatch-requested.v1",
			"company/gateway/event/v1/external_command_dispatch_requested.proto#ExternalCommandDispatchRequested", cfg.DispatchDLQ},
		"partner.events": {PartnerChannel(cfg), capabilityEnabledEvent,
			"company/partner/event/v1/capability_enabled.proto#CapabilityEnabled", cfg.PartnerDLQ},
	}
	for name, c := range cases {
		if got := channel.EventTypeOf(c.ch); got != c.eventType {
			t.Errorf("%s: EventTypeOf() = %q, want %q", name, got, c.eventType)
		}
		if c.ch.ContractRef != c.contract || c.ch.Containment != c.containment {
			t.Errorf("%s: channel = %+v, want contract %q and containment %q", name, c.ch, c.contract, c.containment)
		}
	}
}
