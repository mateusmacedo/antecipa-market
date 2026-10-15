package app_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/channel"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

func TestTheRelayPublishesThroughKafka(t *testing.T) {
	if system := app.Defaults(app.RoleRelay).Relay.System; system != "kafka" {
		t.Fatalf("Relay.System = %q, want %q", system, "kafka")
	}
}

func TestTheRelayAddressesTheIdentityChannelByItsPhysicalTopic(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)
	cfg.IdentityTopic, cfg.IdentityDLQ, cfg.Group = "identity.events", "identity.events.dlq", "identity"
	catalog, err := channel.NewCatalog(app.IdentityChannel(cfg))
	if err != nil {
		t.Fatalf("NewCatalog() = %v, want nil", err)
	}

	if application.Destination != "identity.events" {
		t.Fatalf("Destination = %q, want the identity.events channel", application.Destination)
	}
	if topic := catalog.AddressOf(application.Destination); topic != cfg.IdentityTopic {
		t.Fatalf("AddressOf(%q) = %q, want the physical topic %q (RF-B7)", application.Destination, topic, cfg.IdentityTopic)
	}
	if topic := catalog.AddressOf("uncatalogued.events"); topic != "" {
		t.Fatalf("AddressOf(uncatalogued.events) = %q, want empty: the send never names a logical channel", topic)
	}
}

func TestTheIdentityChannelCarriesTheTypeTheMapperWrites(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)
	cfg.IdentityTopic, cfg.IdentityDLQ, cfg.Group = "identity.events", "identity.events.dlq", "identity"

	ch := app.IdentityChannel(cfg)

	if got := channel.EventTypeOf(ch); got != "com.company.identity.organization-registered.v1" {
		t.Fatalf("EventTypeOf() = %q, want the type the outbox mapper writes (PTB-03)", got)
	}
	if ch.ContractRef != "company/identity/event/v1/organization_registered.proto#OrganizationRegistered" || ch.Containment != cfg.IdentityDLQ {
		t.Fatalf("channel = %+v, want the OrganizationRegistered contract and the DLQ as containment", ch)
	}
}

func apiConfig(key string) app.Config {
	cfg := app.Defaults(app.RoleAPI)
	cfg.IdentifierKey, cfg.IdentifierKeyVersion = app.NewSecret(key), 1
	return cfg
}

func TestTheAuditTrailOfTheServiceReachesTheLoggerProviderOfTheRuntime(t *testing.T) {
	obstest.AuditTrail(t, "identity", func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error {
		service, err := app.NewIdentityService(pool, pool, rt, apiConfig(testIdentifierKey))
		if err != nil {
			return err
		}
		service.Instrumentation.Audit(ctx, port.AuditEvent{Object: "org-9", Action: "read", Outcome: port.OutcomeAccepted})
		return nil
	})
}

func TestTheServiceRefusesTheKeyTheConfigurationRefuses(t *testing.T) {
	_, err := app.NewIdentityService(nil, nil, nil, apiConfig("short"))

	if !errors.Is(err, app.ErrInvalidVariable) {
		t.Fatalf("NewIdentityService() = %v, want the key refused before any provider is built", err)
	}
}

var personCanonical = []byte("identity.RegisterPerson|per-1|Ana Sintetica|123.456.789-09")

func keyedDigest(t *testing.T) func([]byte) port.Fingerprint {
	t.Helper()
	digest, err := provider.NewIdempotencyDigest([]byte(testIdentifierKey))
	if err != nil {
		t.Fatalf("NewIdempotencyDigest() = %v", err)
	}
	return digest
}

func TestTheIdempotencyPolicyDigestsUnderTheIdentifierKey(t *testing.T) {
	cfg := apiConfig(testIdentifierKey)

	policy, err := app.IdempotencyPolicy(cfg)

	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v, want nil", err)
	}
	got := policy.Digest(personCanonical)
	if got == port.Fingerprint(sha256.Sum256(personCanonical)) {
		t.Fatalf("Digest() = %x, the unkeyed SHA-256 of the kernel default", got)
	}
	if want := keyedDigest(t)(personCanonical); got != want {
		t.Fatalf("Digest() = %x, want the HMAC under the identifier key %x", got, want)
	}
	if policy.Wait != int64(cfg.Policies.IdempotencyWait) || policy.Retention != int64(cfg.Policies.IdempotencyRetention) {
		t.Fatalf("policy = wait %d, retention %d, want the configured %v and %v", policy.Wait, policy.Retention,
			cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	}
}

func TestTheIdempotencyPolicyRefusesTheKeyTheConfigurationRefuses(t *testing.T) {
	short := "sentinel-short-key"

	_, err := app.IdempotencyPolicy(apiConfig(short))

	if !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "IDENTITY_IDENTIFIER_HMAC_KEY:") {
		t.Fatalf("IdempotencyPolicy() = %v, want IDENTITY_IDENTIFIER_HMAC_KEY refused as invalid", err)
	}
	if strings.Contains(err.Error(), short) {
		t.Fatalf("the refusal %q carries the key", err)
	}
}

func TestTheServiceRunsEveryCommandUnderTheKeyedDigest(t *testing.T) {
	var got port.Fingerprint
	obstest.AuditTrail(t, "identity", func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error {
		service, err := app.NewIdentityService(pool, pool, rt, apiConfig(testIdentifierKey))
		if err != nil {
			return err
		}
		got = service.Idempotency.Digest(personCanonical)
		service.Instrumentation.Audit(ctx, port.AuditEvent{Object: "per-1", Action: "read", Outcome: port.OutcomeAccepted})
		return nil
	})

	if want := keyedDigest(t)(personCanonical); got != want {
		t.Fatalf("service digest = %x, want the HMAC under the identifier key %x", got, want)
	}
}
