package app

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
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
// mapper, the consumer and the publisher must agree on the versioned value.
func TestEachChannelCarriesTheTypesItsWriterOrReaderUses(t *testing.T) {
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
		if err := c.ch.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", name, err)
		}
		index := slices.Index(channel.WireTypes(c.ch), c.eventType)
		if index < 0 {
			t.Errorf("%s: WireTypes() = %v, want it to carry %q", name, channel.WireTypes(c.ch), c.eventType)
			continue
		}
		if got := c.ch.Events[index].ContractRef; got != c.contract || c.ch.Containment != c.containment {
			t.Errorf("%s: contract %q and containment %q, want %q and %q", name, got, c.ch.Containment, c.contract, c.containment)
		}
	}
}

func TestEveryTypeTheMapperPublishesIsOnItsChannel(t *testing.T) {
	cfg := channelConfig()
	integration := channel.WireTypes(GatewayChannel(cfg))
	dispatch := channel.WireTypes(DispatchChannel(cfg))
	events := map[string]kernel.DomainEvent{
		"submitted": domain.ExternalCommandSubmitted{}, "acknowledged": domain.ExternalCommandAcknowledged{},
		"confirmed": domain.ExternalCommandConfirmed{}, "failed": domain.ExternalCommandFailed{},
		"outcome unknown": domain.ExternalOutcomeUnknown{}, "ambiguity expired": domain.ExternalCommandAmbiguityExpired{},
	}
	for name, event := range events {
		mapped, err := provider.Mapper{}.Map(event)
		if err != nil {
			t.Fatalf("Map(%s) = %v, want nil", name, err)
		}
		if !slices.Contains(integration, mapped.Type) {
			t.Errorf("Map(%s).Type = %q, outside gateway.events %v: the relay would publish a type it does not declare (KFK-03b)", name, mapped.Type, integration)
		}
	}
	if len(integration) != len(events) {
		t.Errorf("gateway.events carries %v, want exactly the %d types the mapper publishes", integration, len(events))
	}
	mapped, err := provider.Mapper{}.Map(domain.ExternalCommandDispatchRequested{})
	if err != nil || !slices.Equal(dispatch, []string{mapped.Type}) {
		t.Fatalf("gateway.dispatch carries %v, want the dispatch intent %q alone (KFK-03), err %v", dispatch, mapped.Type, err)
	}
}
