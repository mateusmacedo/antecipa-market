package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app"
)

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

const testIdentifierKey = "0123456789abcdef0123456789abcdef"

func apiPairs(extra ...string) []string {
	return append([]string{
		"PG_DSN", "postgres://x", "GRPC_INSECURE", "true",
		"IDENTITY_IDENTIFIER_HMAC_KEY", testIdentifierKey, "IDENTITY_IDENTIFIER_HMAC_KEY_VERSION", "1",
	}, extra...)
}

func TestTheAPIRunsWithItsDefaults(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup(apiPairs()...))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	if cfg.API.GRPCAddr != ":9090" || cfg.Service != "identity" || !cfg.API.GRPCInsecure {
		t.Fatalf("cfg = %+v, want :9090, identity, insecure", cfg)
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
	cfg.IdentifierKey, cfg.IdentifierKeyVersion = app.NewSecret(testIdentifierKey), 1
	cfg.Policies.PurgeBatch = 0

	err := cfg.Validate()

	if want := "identity: invalid idempotency or purge policy: PurgeBatch 0"; !errors.Is(err, app.ErrInvalidPolicy) || err.Error() != want {
		t.Fatalf("Validate() = %v, want %q", err, want)
	}
}

func TestTheRelayNamesEachMissingVariable(t *testing.T) {
	full := []string{
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_SASL_MECHANISM", "SCRAM-SHA-256",
		"KAFKA_IDENTITY_TOPIC", "identity", "KAFKA_IDENTITY_DLQ", "identity.dlq", "KAFKA_GROUP", "g",
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

func TestIdentityRefusesTheConsumerRole(t *testing.T) {
	_, err := app.FromEnv("consumer", lookup("PG_DSN", "postgres://x"))

	if !errors.Is(err, app.ErrUnknownRole) || !strings.Contains(err.Error(), "consumer") || !strings.Contains(err.Error(), "api|relay") {
		t.Fatalf("FromEnv(consumer) = %v, want a refusal naming the role and api|relay", err)
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
		PurgeInterval:        15 * time.Minute,
		PurgeBatch:           1000,
	}
	if got := app.Defaults(app.RoleAPI).Policies; got != want {
		t.Fatalf("Policies = %+v, want %+v", got, want)
	}
}

func TestTheRelayRetriesThroughABrokerOutageOfHalfAnHour(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay).Relay
	if cfg.MaxAttempts <= 0 {
		t.Fatalf("MaxAttempts = %d, want a ceiling so a broken record reaches failed", cfg.MaxAttempts)
	}
	backoff := relay.ExponentialBackoff(cfg.BackoffBase, cfg.BackoffCeiling, nil)
	var shortest time.Duration
	for attempt := 1; attempt < cfg.MaxAttempts; attempt++ {
		shortest += backoff(attempt)
	}
	if shortest < 30*time.Minute {
		t.Fatalf("shortest retry window = %v, want at least 30m before a record goes to failed", shortest)
	}
}

func TestTheIdentityOfTheProcessIsLeftToTheEnvironment(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup(apiPairs()...))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	if telemetry := app.TelemetryOf(cfg); telemetry.Version != "" || telemetry.Instance != "" {
		t.Fatalf("TelemetryOf() declares version %q and instance %q, want neither: both come from OTEL_RESOURCE_ATTRIBUTES (RF-E3)", telemetry.Version, telemetry.Instance)
	}
}
