package app

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

// Role selects which process the binary becomes: one role per process, so the
// outbox relay never shares a process with the request path.
type Role string

const (
	RoleAPI   Role = "api"
	RoleRelay Role = "relay"
)

// Roles lists the accepted values of --role; identity consumes no channel.
var Roles = []Role{RoleAPI, RoleRelay}

var (
	ErrUnknownRole     = errors.New("identity: unknown role")
	ErrMissingVariable = errors.New("identity: required variable is not set")
	ErrInvalidVariable = envconfig.ErrInvalidVariable
	ErrInvalidPolicy   = errors.New("identity: invalid idempotency or purge policy")
)

const (
	envDSN           = "PG_DSN"
	envMigrate       = "MIGRATE"
	envBrokers       = "KAFKA_BROKERS"
	envKafkaInsecure = "KAFKA_INSECURE"
	envGroup         = "KAFKA_GROUP"

	envIdentityTopic = "KAFKA_IDENTITY_TOPIC"
	envIdentityDLQ   = "KAFKA_IDENTITY_DLQ"

	envMetricTenants = "METRIC_TENANTS"

	envIdentifierKey        = "IDENTITY_IDENTIFIER_HMAC_KEY"
	envIdentifierKeyVersion = "IDENTITY_IDENTIFIER_HMAC_KEY_VERSION"
)

// Config is every operational value the two roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN     string
	Migrate bool

	API           kernelgrpc.APIEnv
	Admission     admission.Limit
	MetricTenants []string

	Brokers       []string
	KafkaInsecure bool

	// KafkaAuth is the principal this process presents to the broker, required
	// whenever TLS is on: a consumed message counts as authenticated only when
	// the transport that delivered it authenticated the producer.
	KafkaAuth kafka.ClientAuth
	Group     string

	IdentityTopic string
	IdentityDLQ   string

	IdentifierKey        Secret
	IdentifierKeyVersion int

	Signals boot.Signals

	Service string

	Relay relay.Config

	Policies kernelapp.Policies
}

// Defaults are the values a role runs with when the environment says nothing.
func Defaults(role Role) Config {
	return Config{
		Role:    role,
		API:     kernelgrpc.APIEnv{GRPCAddr: ":9090"},
		Service: "identity",
		Relay: relay.Config{
			Source:         "urn:dmpf:reference-identity",
			Interval:       500 * time.Millisecond,
			BatchSize:      50,
			Lease:          30 * time.Second,
			Concurrency:    4,
			MaxAttempts:    24,
			BackoffBase:    200 * time.Millisecond,
			BackoffCeiling: 5 * time.Minute,
			ShutdownGrace:  observability.ShutdownGrace,
			System:         semconv.MessagingSystemKafka.Value.AsString(),
		},
		Admission: admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},
		Policies: kernelapp.Policies{
			IdempotencyWait:      time.Second,
			IdempotencyRetention: 24 * time.Hour,
			OutboxRetention:      168 * time.Hour,
			PurgeInterval:        15 * time.Minute,
			PurgeBatch:           1000,
		},
	}
}

// FromEnv reads the variables through lookup, applies Defaults and validates
// for the role.
func FromEnv(role Role, lookup func(string) string) (Config, error) {
	cfg := Defaults(role)

	var err error
	cfg.DSN = lookup(envDSN)
	if cfg.API, err = kernelgrpc.ReadAPIEnv(lookup, cfg.API.GRPCAddr); err != nil {
		return Config{}, err
	}
	cfg.Brokers = envconfig.SplitList(lookup(envBrokers))
	cfg.KafkaAuth = kafka.ReadClientAuth(lookup)
	cfg.MetricTenants = envconfig.SplitList(lookup(envMetricTenants))
	cfg.IdentityTopic = lookup(envIdentityTopic)
	cfg.IdentityDLQ = lookup(envIdentityDLQ)
	cfg.Group = lookup(envGroup)
	cfg.IdentifierKey = NewSecret(lookup(envIdentifierKey))
	if cfg.IdentifierKeyVersion, err = envconfig.ParsePositive(envIdentifierKeyVersion, lookup(envIdentifierKeyVersion), 0); err != nil {
		return Config{}, err
	}

	flags := []struct {
		variable string
		into     *bool
	}{
		{envKafkaInsecure, &cfg.KafkaInsecure},
		{envMigrate, &cfg.Migrate},
	}
	for _, flag := range flags {
		if *flag.into, err = envconfig.ParseBool(flag.variable, lookup(flag.variable)); err != nil {
			return Config{}, err
		}
	}

	if cfg.Signals, err = boot.SignalsFromEnv(lookup); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate refuses a configuration the role could not start with, naming the
// variable that is missing.
func (c Config) Validate() error {
	missing, err := c.requirements()
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", ErrMissingVariable, missing[0])
	}
	if c.Role == RoleAPI {
		if _, err := c.identifierProtector(); err != nil {
			return err
		}
	}
	if err := c.Policies.Validate(false, ErrInvalidPolicy); err != nil {
		return err
	}
	return c.Relay.Validate()
}

func (c Config) requirements() ([]string, error) {
	var missing []string
	require := func(variable string, absent bool) {
		if absent {
			missing = append(missing, variable)
		}
	}
	require(envDSN, c.DSN == "")
	switch c.Role {
	case RoleAPI:
		missing = append(missing, c.API.Missing()...)
		require(envIdentifierKey, c.IdentifierKey.value == "")
		require(envIdentifierKeyVersion, c.IdentifierKeyVersion == 0)
		return missing, nil
	case RoleRelay:
		require(envBrokers, len(c.Brokers) == 0)
		require(envIdentityTopic, c.IdentityTopic == "")
		require(envIdentityDLQ, c.IdentityDLQ == "")
		require(envGroup, c.Group == "")
		return append(missing, c.KafkaAuth.Missing(c.KafkaInsecure)...), nil
	case "consumer":
		return nil, fmt.Errorf("%w: %q: the identity context consumes no channel (use %s)", ErrUnknownRole, c.Role, roleList())
	default:
		return nil, fmt.Errorf("%w: %q (use one of %s)", ErrUnknownRole, c.Role, roleList())
	}
}

// identifierProtector runs the key rule of the provider over the configuration,
// so Validate refuses at startup what the wiring would refuse.
func (c Config) identifierProtector() (*provider.IdentifierProtector, error) {
	protector, err := provider.NewIdentifierProtector([]byte(c.IdentifierKey.value), c.IdentifierKeyVersion)
	switch {
	case errors.Is(err, provider.ErrIdentifierKeyVersionInvalid):
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidVariable, envIdentifierKeyVersion, err)
	case err != nil:
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidVariable, envIdentifierKey, err)
	}
	return protector, nil
}

// Secret renders as redact.Placeholder under every fmt verb and in slog, so a
// Config printed whole, or a field logged by mistake, never carries the value.
type Secret struct{ value string }

func NewSecret(value string) Secret { return Secret{value: value} }

func (Secret) String() string { return redact.Placeholder }

func (Secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redact.Placeholder) }

func (Secret) LogValue() slog.Value { return slog.StringValue(redact.Placeholder) }

func roleList() string {
	names := make([]string, len(Roles))
	for i, role := range Roles {
		names[i] = string(role)
	}
	return strings.Join(names, "|")
}

func apiRole(cfg Config) serve.APIRole {
	return serve.APIRole{DSN: cfg.DSN, Server: cfg.API, Migrate: cfg.Migrate, Admission: cfg.Admission, MetricTenants: cfg.MetricTenants, Policies: cfg.Policies}
}

func relayRole(cfg Config) serve.RelayRole {
	return serve.RelayRole{DSN: cfg.DSN, Brokers: cfg.Brokers, KafkaInsecure: cfg.KafkaInsecure, KafkaAuth: cfg.KafkaAuth, Relay: cfg.Relay, Policies: cfg.Policies}
}
