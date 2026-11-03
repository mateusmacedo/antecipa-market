package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

func channelConfig() Config {
	cfg := Defaults(RoleRelay)
	cfg.GatewayTopic, cfg.GatewayDLQ = "gateway.events", "gateway.events.dlq"
	cfg.DispatchTopic, cfg.DispatchDLQ = "gateway.dispatch", "gateway.dispatch.dlq"
	cfg.InteractionTopic, cfg.InteractionDLQ = "gateway.interactions", "gateway.interactions.dlq"
	cfg.NormalizationTopic, cfg.NormalizationDLQ = "gateway.normalizations", "gateway.normalizations.dlq"
	cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group = "partner.events", "partner.events.dlq", "gateway"
	return cfg
}

func TestTheRelayAddressesEachChannelByItsPhysicalTopic(t *testing.T) {
	cfg := channelConfig()
	catalog, err := RelayCatalog(cfg)
	if err != nil {
		t.Fatalf("RelayCatalog() = %v, want nil", err)
	}

	for destination, topic := range map[string]string{
		application.Destination:              cfg.GatewayTopic,
		application.DispatchDestination:      cfg.DispatchTopic,
		application.InteractionDestination:   cfg.InteractionTopic,
		application.NormalizationDestination: cfg.NormalizationTopic,
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
		"gateway.interactions": {InteractionChannel(cfg), "com.company.gateway.external-response-captured.v1",
			"company/gateway/event/v1/external_response_captured.proto#ExternalResponseCaptured", cfg.InteractionDLQ},
		"gateway.normalizations": {NormalizationChannel(cfg), "com.company.gateway.external-interaction-stored.v1",
			"company/gateway/event/v1/external_interaction_stored.proto#ExternalInteractionStored", cfg.NormalizationDLQ},
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
		"schema mismatch": domain.ExternalSchemaMismatchDetected{}, "uncorrelated": domain.ExternalInteractionUncorrelated{},
		"reprocessed": domain.ExternalInteractionReprocessed{},
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
	internal := map[string]struct {
		carries []string
		event   kernel.DomainEvent
	}{
		"gateway.dispatch":       {dispatch, domain.ExternalCommandDispatchRequested{}},
		"gateway.interactions":   {channel.WireTypes(InteractionChannel(cfg)), domain.ExternalResponseCaptured{}},
		"gateway.normalizations": {channel.WireTypes(NormalizationChannel(cfg)), domain.ExternalInteractionStored{}},
	}
	for name, c := range internal {
		mapped, err := provider.Mapper{}.Map(c.event)
		if err != nil || !slices.Equal(c.carries, []string{mapped.Type}) {
			t.Errorf("%s carries %v, want %q alone (KFK-03), err %v", name, c.carries, mapped.Type, err)
		}
	}
}

// The consumer reads back each channel of this context the relay publishes, at
// the same address, with the same types and dead letter, under a group of its
// own apart from the one of partner.events.
func TestTheConsumerReadsBackWhatTheRelayPublishes(t *testing.T) {
	cfg := channelConfig()
	cfg.GatewayGroup, cfg.DispatchGroup = "gateway.events", "gateway.dispatch"
	cfg.InteractionGroup, cfg.NormalizationGroup = "gateway.interactions", "gateway.normalizations"
	published, err := RelayCatalog(cfg)
	if err != nil {
		t.Fatalf("RelayCatalog() = %v", err)
	}
	read := consumedChannels(cfg)
	if _, err := channel.NewCatalog(read.all()...); err != nil {
		t.Fatalf("NewCatalog(consumed) = %v, want the five channels on five addresses", err)
	}

	for _, ch := range []channel.Channel{read.events, read.dispatch, read.interactions, read.normalizations} {
		relayed := published[ch.Name]
		if ch.Address != relayed.Address || ch.Containment != relayed.Containment || !slices.Equal(channel.WireTypes(ch), channel.WireTypes(relayed)) {
			t.Errorf("%s: read at %s (%s) carrying %v, want the relay's %s (%s) carrying %v", ch.Name, ch.Address, ch.Containment,
				channel.WireTypes(ch), relayed.Address, relayed.Containment, channel.WireTypes(relayed))
		}
		if ch.Group == "" || ch.Group == cfg.Group {
			t.Errorf("%s: group %q, want one of its own", ch.Name, ch.Group)
		}
	}
	if read.partner.Group != cfg.Group {
		t.Fatalf("partner.events group = %q, want the role's %q, whose offsets it keeps", read.partner.Group, cfg.Group)
	}
}

func TestTheConsumerWaitsForTheAttemptLedgerTheDispatchWrites(t *testing.T) {
	grants := consumerGrants()
	for _, want := range append(postgres.KernelGrants(postgres.Outbox, postgres.Inbox, postgres.ExternalAttempt), provider.Grants...) {
		if !slices.ContainsFunc(grants, func(g postgres.Grant) bool { return g.Table == want.Table }) {
			t.Errorf("consumerGrants() = %v, want %s waited for", grants, want.Table)
		}
	}
}

// The first loop of the role that ends stops the others; only the
// cancellation is a clean exit.
func TestTheLoopsOfTheRoleStopTogether(t *testing.T) {
	broken := errors.New("broker gone")
	waiting := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

	if err := together(context.Background(), waiting, func(context.Context) error { return broken }, waiting); !errors.Is(err, broken) {
		t.Fatalf("together() = %v, want the failure of the loop that ended", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := together(ctx, waiting, waiting); err != nil {
		t.Fatalf("together(cancelled) = %v, want a clean exit", err)
	}
}
