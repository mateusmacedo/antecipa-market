package app

import (
	"errors"
	"fmt"
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
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Role selects which of the three processes the binary becomes (BLK-02: the
// relay never shares a process with the request path).
type Role string

const (
	RoleAPI      Role = "api"
	RoleRelay    Role = "relay"
	RoleConsumer Role = "consumer"
)

// Roles lists the accepted values of --role, in the order the usage prints them.
var Roles = []Role{RoleAPI, RoleRelay, RoleConsumer}

var (
	ErrUnknownRole     = errors.New("gateway: unknown role")
	ErrMissingVariable = errors.New("gateway: required variable is not set")
	ErrInvalidVariable = envconfig.ErrInvalidVariable
	ErrInvalidPolicy   = errors.New("gateway: invalid idempotency or purge policy")
)

const (
	envDSN           = "PG_DSN"
	envMigrateDSN    = "PG_MIGRATE_DSN"
	envMigrate       = "MIGRATE"
	envBrokers       = "KAFKA_BROKERS"
	envKafkaInsecure = "KAFKA_INSECURE"
	envGroup         = "KAFKA_GROUP"

	envGatewayTopic  = "KAFKA_GATEWAY_TOPIC"
	envGatewayDLQ    = "KAFKA_GATEWAY_DLQ"
	envDispatchTopic = "KAFKA_GATEWAY_DISPATCH_TOPIC"
	envDispatchDLQ   = "KAFKA_GATEWAY_DISPATCH_DLQ"
	envPartnerTopic  = "KAFKA_PARTNER_TOPIC"
	envPartnerDLQ    = "KAFKA_PARTNER_DLQ"
	envPartnerSource = "PARTNER_SOURCE"

	envMetricTenants = "METRIC_TENANTS"
)

// Config is every operational value the three roles need, resolved once at
// startup from the environment.
type Config struct {
	Role Role

	DSN        string
	MigrateDSN string
	Migrate    bool

	API           kernelgrpc.APIEnv
	Admission     admission.Limit
	MetricTenants []string

	Brokers       []string
	KafkaInsecure bool

	// KafkaAuth is the principal this process presents to the
	// broker, required whenever TLS is on (IDN-04).
	KafkaAuth kafka.ClientAuth
	Group     string

	// GatewayTopic and DispatchTopic address the two channels the relay
	// publishes to; PartnerTopic the one the consumer reads.
	GatewayTopic  string
	GatewayDLQ    string
	DispatchTopic string
	DispatchDLQ   string
	PartnerTopic  string
	PartnerDLQ    string

	// PartnerSource is the producer the consumer admits on partner.events, by
	// the envelope's source attribute (CTX-27, IDN-04).
	PartnerSource string

	Signals boot.Signals

	Service string

	Relay relay.Config
	Wait  time.Duration

	// ConsumerTimeout is this consumer's own time policy, which CTX-28 makes
	// mandatory: the adapter mounts a deadline per attempt from it.
	ConsumerTimeout time.Duration

	Policies kernelapp.Policies
}

// Defaults are the values a role runs with when the environment says nothing.
func Defaults(role Role) Config {
	return Config{
		Role:    role,
		API:     kernelgrpc.APIEnv{GRPCAddr: ":9090"},
		Service: "gateway",
		Relay: relay.Config{
			Source:         "urn:dmpf:reference-gateway",
			Interval:       500 * time.Millisecond,
			BatchSize:      50,
			Lease:          30 * time.Second,
			Concurrency:    4,
			MaxAttempts:    10,
			BackoffBase:    200 * time.Millisecond,
			BackoffCeiling: 30 * time.Second,
			ShutdownGrace:  observability.ShutdownGrace,
			System:         semconv.MessagingSystemKafka.Value.AsString(),
		},
		PartnerSource:   "urn:dmpf:reference-partner",
		Wait:            2 * time.Second,
		ConsumerTimeout: 30 * time.Second,
		Admission:       admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},
		Policies: kernelapp.Policies{
			IdempotencyWait:      time.Second,
			IdempotencyRetention: 24 * time.Hour,
			OutboxRetention:      168 * time.Hour,
			InboxRetention:       192 * time.Hour,
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
	cfg.GatewayTopic = lookup(envGatewayTopic)
	cfg.GatewayDLQ = lookup(envGatewayDLQ)
	cfg.DispatchTopic = lookup(envDispatchTopic)
	cfg.DispatchDLQ = lookup(envDispatchDLQ)
	cfg.PartnerTopic = lookup(envPartnerTopic)
	cfg.PartnerDLQ = lookup(envPartnerDLQ)
	cfg.PartnerSource = envconfig.OrDefault(lookup(envPartnerSource), cfg.PartnerSource)
	cfg.Group = lookup(envGroup)

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
	if role == RoleAPI && cfg.Migrate {
		cfg.MigrateDSN = lookup(envMigrateDSN)
	}
	if role == RoleAPI && cfg.DSN != "" {
		if cfg.DSN, err = withSessionTimeouts(cfg.DSN); err != nil {
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
	// The api keeps the kernel's attempt ledger and the consumer its message
	// inbox, so both retentions are validated whatever the role.
	if err := c.Policies.Validate(true, ErrInvalidPolicy); err != nil {
		return err
	}
	// INB-14: an entry purged before the broker stops redelivering its message
	// would let that message through as new.
	if window := PartnerChannel(c).Redelivery.UpperBound; c.Role == RoleConsumer && c.Policies.InboxRetention < window {
		return fmt.Errorf("%w: InboxRetention %v is below the redelivery window %v (INB-14)", ErrInvalidPolicy, c.Policies.InboxRetention, window)
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
		require(envMigrateDSN, c.Migrate && c.MigrateDSN == "")
		return append(missing, c.API.Missing()...), nil
	case RoleRelay:
		require(envBrokers, len(c.Brokers) == 0)
		require(envGatewayTopic, c.GatewayTopic == "")
		require(envGatewayDLQ, c.GatewayDLQ == "")
		require(envDispatchTopic, c.DispatchTopic == "")
		require(envDispatchDLQ, c.DispatchDLQ == "")
		require(envGroup, c.Group == "")
		return append(missing, c.KafkaAuth.Missing(c.KafkaInsecure)...), nil
	case RoleConsumer:
		require(envBrokers, len(c.Brokers) == 0)
		require(envPartnerTopic, c.PartnerTopic == "")
		require(envPartnerDLQ, c.PartnerDLQ == "")
		require(envGroup, c.Group == "")
		return append(missing, c.KafkaAuth.Missing(c.KafkaInsecure)...), nil
	default:
		return nil, fmt.Errorf("%w: %q (use one of %s)", ErrUnknownRole, c.Role, roleList())
	}
}

func roleList() string {
	names := make([]string, len(Roles))
	for i, role := range Roles {
		names[i] = string(role)
	}
	return strings.Join(names, "|")
}

func apiRole(cfg Config) serve.APIRole {
	return serve.APIRole{DSN: cfg.DSN, MigrateDSN: cfg.MigrateDSN, Server: cfg.API, Migrate: cfg.Migrate, Admission: cfg.Admission, MetricTenants: cfg.MetricTenants, Policies: cfg.Policies}
}
