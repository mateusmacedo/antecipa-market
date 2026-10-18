package app

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

func TestTheRelayPublishesThroughKafka(t *testing.T) {
	if system := Defaults(RoleRelay).Relay.System; system != "kafka" {
		t.Fatalf("Relay.System = %q, want %q", system, "kafka")
	}
}

func relayConfig() Config {
	cfg := Defaults(RoleRelay)
	cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group = "partner.events", "partner.events.dlq", "partner"
	return cfg
}

func TestTheRelayAddressesThePartnerChannelByItsPhysicalTopic(t *testing.T) {
	cfg := relayConfig()
	catalog, err := channel.NewCatalog(PartnerChannel(cfg))
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}

	if application.Destination != "partner.events" {
		t.Fatalf("Destination = %q, want the partner.events channel", application.Destination)
	}
	if topic := catalog.AddressOf(application.Destination); topic != cfg.PartnerTopic {
		t.Fatalf("AddressOf(%q) = %q, want the physical topic %q", application.Destination, topic, cfg.PartnerTopic)
	}
	if topic := catalog.AddressOf("uncatalogued.events"); topic != "" {
		t.Fatalf("AddressOf(uncatalogued.events) = %q, want empty: the send never names a logical channel", topic)
	}
}

func TestThePartnerChannelCarriesTheTypeTheMapperWrites(t *testing.T) {
	cfg := relayConfig()

	ch := PartnerChannel(cfg)

	if got := channel.EventTypeOf(ch); got != "com.company.partner.partner-registered.v1" {
		t.Fatalf("EventTypeOf() = %q, want the type the outbox mapper writes", got)
	}
	if ch.ContractRef != "company/partner/event/v1/partner_registered.proto#PartnerRegistered" || ch.Containment != cfg.PartnerDLQ {
		t.Fatalf("channel = %+v, want the PartnerRegistered contract and the DLQ as containment", ch)
	}
}

func TestProductionAsksTheDirectoryThatFailsClosed(t *testing.T) {
	directory := productionDirectory()

	if _, ok := directory.(provider.UnconfiguredOrganizationDirectory); !ok {
		t.Fatalf("productionDirectory() = %T, want provider.UnconfiguredOrganizationDirectory until the Identity client exists", directory)
	}
	if _, err := directory.FindOrganization(context.Background(), "acme", "org-1"); !errors.Is(err, provider.ErrOrganizationDirectoryUnconfigured) {
		t.Fatalf("FindOrganization() = %v, want the technical refusal", err)
	}
}

func TestTheAuditTrailOfTheServiceReachesTheLoggerProviderOfTheRuntime(t *testing.T) {
	obstest.AuditTrail(t, "partner", func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error {
		service, err := NewPartnerService(pool, pool, rt, Defaults(RoleAPI), productionDirectory())
		if err != nil {
			return err
		}
		if service.Directory != productionDirectory() || service.Idempotency.Digest == nil {
			t.Errorf("service = %+v, want the directory it was handed and the kernel idempotency policy", service)
		}
		if service.Rollout != (provider.RolloutBucket{}) || service.CapabilitiesByPartner == nil {
			t.Errorf("service = %+v, want the SHA-256 rollout bucket and the capabilities of the partner", service)
		}
		if service.ProductsByPartner == nil {
			t.Errorf("service = %+v, want the products of the partner for the series of DefineProduct", service)
		}
		service.Instrumentation.Audit(ctx, port.AuditEvent{Object: "prt-9", Action: "read", Outcome: port.OutcomeAccepted})
		return nil
	})
}

func TestNewServiceWiresTheReadersOfEveryQuery(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://partner@127.0.0.1:1/partner")
	if err != nil {
		t.Fatalf("pgxpool.New() = %v", err)
	}
	t.Cleanup(pool.Close)

	service := NewService(pool, pool, productionDirectory(), provider.RolloutBucket{}, nil, nil, Waits{})

	if service.PartnerReader == nil || service.PartnersByRole == nil || service.ProfilesByEnvironment == nil ||
		service.PartnersByOrganization == nil || service.AgreementsByPartner == nil || service.PoliciesByPartner == nil {
		t.Fatalf("service = %+v, want a reader behind every query", service)
	}
}
