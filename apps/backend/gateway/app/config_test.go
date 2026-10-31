package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

func TestTheAPIRunsWithItsDefaults(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true"))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	if cfg.API.GRPCAddr != ":9090" || cfg.Service != "gateway" || !cfg.API.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, gateway, insecure", cfg)
	}
	if cfg.Relay.ShutdownGrace != observability.ShutdownGrace {
		t.Fatalf("Relay.ShutdownGrace = %v, want the kernel's %v", cfg.Relay.ShutdownGrace, observability.ShutdownGrace)
	}
}

func TestTheAPIRequiresATransportPolicy(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "GRPC_INSECURE") {
		t.Fatalf("FromEnv() = %v, want the missing transport policy named", err)
	}
}

func TestTheAPIRefusesANonPositivePurgeBatchUnderItsOwnSentinel(t *testing.T) {
	cfg := app.Defaults(app.RoleAPI)
	cfg.DSN, cfg.API.GRPCInsecure = "postgres://x", true
	cfg.Policies.PurgeBatch = 0

	err := cfg.Validate()

	if want := "gateway: invalid idempotency or purge policy: PurgeBatch 0"; !errors.Is(err, app.ErrInvalidPolicy) || err.Error() != want {
		t.Fatalf("Validate() = %v, want %q", err, want)
	}
}

func TestTheRelayNamesEachMissingVariable(t *testing.T) {
	full := []string{
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"KAFKA_GATEWAY_TOPIC", "gateway", "KAFKA_GATEWAY_DLQ", "gateway.dlq",
		"KAFKA_GATEWAY_DISPATCH_TOPIC", "gateway.dispatch", "KAFKA_GATEWAY_DISPATCH_DLQ", "gateway.dispatch.dlq", "KAFKA_GROUP", "g",
	}
	if _, err := app.FromEnv(app.RoleRelay, lookup(full...)); err != nil {
		t.Fatalf("FromEnv(full) = %v, want nil", err)
	}
	for i := 0; i < len(full); i += 2 {
		variable := full[i]
		t.Run("without "+variable, func(t *testing.T) {
			pairs := append(append([]string{}, full[:i]...), full[i+2:]...)

			_, err := app.FromEnv(app.RoleRelay, lookup(pairs...))

			if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestTheConsumerNamesEachMissingVariable(t *testing.T) {
	full := []string{
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_INSECURE", "true",
		"KAFKA_PARTNER_TOPIC", "partner.events", "KAFKA_PARTNER_DLQ", "partner.events.dlq", "KAFKA_GROUP", "gateway",
	}
	cfg, err := app.FromEnv(app.RoleConsumer, lookup(full...))
	if err != nil || cfg.PartnerSource != "urn:dmpf:reference-partner" {
		t.Fatalf("FromEnv(full) = %+v, %v, want the default Partner source", cfg.PartnerSource, err)
	}
	for _, variable := range []string{"PG_DSN", "KAFKA_BROKERS", "KAFKA_PARTNER_TOPIC", "KAFKA_PARTNER_DLQ", "KAFKA_GROUP"} {
		t.Run("without "+variable, func(t *testing.T) {
			var pairs []string
			for i := 0; i < len(full); i += 2 {
				if full[i] != variable {
					pairs = append(pairs, full[i], full[i+1])
				}
			}

			_, err := app.FromEnv(app.RoleConsumer, lookup(pairs...))

			if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("FromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestTheConsumerKeepsItsInboxForTheWholeRedeliveryWindow(t *testing.T) {
	cfg := app.Defaults(app.RoleConsumer)
	cfg.DSN, cfg.Brokers, cfg.KafkaInsecure, cfg.PartnerTopic, cfg.PartnerDLQ, cfg.Group = "postgres://x", []string{"b:9092"}, true, "p", "p.dlq", "g"
	cfg.Policies.InboxRetention = time.Hour

	if err := cfg.Validate(); !errors.Is(err, app.ErrInvalidPolicy) || !strings.Contains(err.Error(), "INB-14") {
		t.Fatalf("Validate() = %v, want the retention below the redelivery window refused", err)
	}
}

func TestAnUnknownRoleNamesTheThreeRoles(t *testing.T) {
	_, err := app.FromEnv("dispatcher", lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrUnknownRole) || !strings.Contains(err.Error(), "api|relay|consumer") {
		t.Fatalf("FromEnv(dispatcher) = %v, want a refusal naming api|relay|consumer", err)
	}
}

func TestDefaultsAreTheValuesOfARoleWithoutEnvironment(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)

	if cfg.Role != app.RoleRelay || cfg.API.GRPCAddr != ":9090" || cfg.Relay.BatchSize == 0 {
		t.Fatalf("Defaults(relay) = %+v, want the role, :9090 and a relay configuration", cfg)
	}
}

func TestTheDefaultsKeepCommandsAndThePurgeWithinTheirRetention(t *testing.T) {
	want := kernelapp.Policies{
		IdempotencyWait:      time.Second,
		IdempotencyRetention: 24 * time.Hour,
		OutboxRetention:      168 * time.Hour,
		InboxRetention:       192 * time.Hour,
		PurgeInterval:        15 * time.Minute,
		PurgeBatch:           1000,
	}
	if got := app.Defaults(app.RoleAPI).Policies; got != want {
		t.Fatalf("Policies = %+v, want %+v", got, want)
	}
}

func TestTheIdentityOfTheProcessIsLeftToTheEnvironment(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "GRPC_INSECURE", "true"))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	if telemetry := app.TelemetryOf(cfg); telemetry.Version != "" || telemetry.Instance != "" {
		t.Fatalf("TelemetryOf() declares version %q and instance %q, want neither: both come from OTEL_RESOURCE_ATTRIBUTES (RF-E3)", telemetry.Version, telemetry.Instance)
	}
}

func TestFromEnvHandsTheMigratingDSNToTheAPIOnly(t *testing.T) {
	const migrator = "postgres://gateway_migrator:secret@db/gateway"
	api, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "MIGRATE", "true", "PG_MIGRATE_DSN", migrator, "GRPC_INSECURE", "true"))
	if err != nil {
		t.Fatalf("FromEnv(api) = %v, want nil", err)
	}
	if api.MigrateDSN != migrator {
		t.Fatalf("FromEnv(api).MigrateDSN = %q, want %q", api.MigrateDSN, migrator)
	}

	relay, err := app.FromEnv(app.RoleRelay, lookup("PG_DSN", "postgres://x", "MIGRATE", "true", "PG_MIGRATE_DSN", migrator,
		"KAFKA_BROKERS", "b:9092", "KAFKA_INSECURE", "true", "KAFKA_GATEWAY_TOPIC", "t", "KAFKA_GATEWAY_DLQ", "d",
		"KAFKA_GATEWAY_DISPATCH_TOPIC", "dt", "KAFKA_GATEWAY_DISPATCH_DLQ", "dd", "KAFKA_GROUP", "g"))
	if err != nil {
		t.Fatalf("FromEnv(relay) = %v, want nil", err)
	}
	if relay.MigrateDSN != "" {
		t.Fatalf("FromEnv(relay).MigrateDSN = %q, want it never read by a role that does not migrate", relay.MigrateDSN)
	}
}

func TestTheAPIReadsTheMigratingDSNOnlyWhenItMigrates(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "PG_MIGRATE_DSN", "postgres://gateway_migrator:secret@db/gateway", "GRPC_INSECURE", "true"))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	if cfg.MigrateDSN != "" {
		t.Fatalf("FromEnv().MigrateDSN = %q without MIGRATE, want it left unread", cfg.MigrateDSN)
	}
}

func TestTheMigratingAPIRequiresTheMigratingDSN(t *testing.T) {
	_, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://x", "MIGRATE", "true", "GRPC_INSECURE", "true"))

	if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), "PG_MIGRATE_DSN") {
		t.Fatalf("FromEnv() = %v, want the missing PG_MIGRATE_DSN named", err)
	}
}
