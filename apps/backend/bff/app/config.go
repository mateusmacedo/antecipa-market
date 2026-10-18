package app

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var (
	ErrMissingVariable = errors.New("bff: required variable is not set")
	ErrInvalidVariable = envconfig.ErrInvalidVariable
)

const (
	envHTTPAddr             = "HTTP_ADDR"
	envAdminAddr            = "ADMIN_ADDR"
	envIdentityTarget       = "IDENTITY_GRPC_TARGET"
	envGRPCInsecure         = "GRPC_INSECURE"
	envGRPCCAFile           = "GRPC_CA_FILE"
	envGRPCServerName       = "GRPC_SERVER_NAME"
	envGRPCClientCertFile   = "GRPC_CLIENT_CERT_FILE"
	envGRPCClientKeyFile    = "GRPC_CLIENT_KEY_FILE"
	envCORSOrigins          = "CORS_ORIGINS"
	envMetricTenants        = "METRIC_TENANTS"
	envIdentityContractPath = "OPENAPI_IDENTITY_PATH"
	envDrainDelay           = "DRAIN_DELAY"
	envRedisURL             = "REDIS_URL"
)

// WHY: terminationGracePeriodSeconds 30 (identity/deploy/k8s/base/deployment-api.yaml:71)
// covers the drain plus two observability.ShutdownGrace windows in sequence:
// the HTTP Shutdown and the telemetry flush deferred by boot.Boot.
const maxDrainDelay = 10 * time.Second

type Config struct {
	HTTPAddr  string
	AdminAddr string

	IdentityTarget string
	GRPCInsecure   bool
	CAFile         string
	ServerName     string
	ClientCertFile string
	ClientKeyFile  string

	CORSOrigins          []string
	IdentityContractPath string

	Signals boot.Signals

	Service string

	Admission admission.Limit

	// MetricTenants is the allowlist of MET-07: the tenants that keep their own
	// admission bucket and label. Every other tenant shares "other".
	MetricTenants []string
	RouteBudget   deadline.Budget

	// DrainDelay is how long the edge keeps serving, already unready, before
	// closing the listener: the time the load balancer takes to stop routing.
	DrainDelay time.Duration

	Auth  authn.Config
	Redis redis.Config
}

func Defaults() Config {
	return Config{
		HTTPAddr:  ":8080",
		AdminAddr: ":8090",
		Service:   "bff",
		Admission: admission.Limit{PerSecond: 50, Burst: 100, Concurrency: 32},
		RouteBudget: deadline.Budget{
			Dependency:        "identity",
			Method:            "route",
			Limit:             2 * time.Second,
			Slack:             200 * time.Millisecond,
			EstimatedDuration: 200 * time.Millisecond,
		},
	}
}

func FromEnv(lookup func(string) string) (Config, error) {
	cfg := Defaults()

	cfg.HTTPAddr = envconfig.OrDefault(lookup(envHTTPAddr), cfg.HTTPAddr)
	cfg.AdminAddr = envconfig.OrDefault(lookup(envAdminAddr), cfg.AdminAddr)
	cfg.IdentityTarget = lookup(envIdentityTarget)
	cfg.CAFile = lookup(envGRPCCAFile)
	cfg.ServerName = lookup(envGRPCServerName)
	cfg.ClientCertFile = lookup(envGRPCClientCertFile)
	cfg.ClientKeyFile = lookup(envGRPCClientKeyFile)
	cfg.CORSOrigins = envconfig.SplitList(lookup(envCORSOrigins))
	cfg.MetricTenants = envconfig.SplitList(lookup(envMetricTenants))
	cfg.IdentityContractPath = lookup(envIdentityContractPath)

	var err error
	if cfg.GRPCInsecure, err = envconfig.ParseBool(envGRPCInsecure, lookup(envGRPCInsecure)); err != nil {
		return Config{}, err
	}
	if cfg.Auth, err = authn.ReadEnv(lookup); err != nil {
		return Config{}, err
	}
	if cfg.Redis, err = readRedis(lookup); err != nil {
		return Config{}, err
	}

	if cfg.Signals, err = boot.SignalsFromEnv(lookup); err != nil {
		return Config{}, err
	}
	if value := lookup(envDrainDelay); value != "" {
		if cfg.DrainDelay, err = time.ParseDuration(value); err != nil || cfg.DrainDelay < 0 {
			return Config{}, fmt.Errorf("%w: %s=%q is not a non-negative duration", ErrInvalidVariable, envDrainDelay, value)
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if err := validateListeners(c.HTTPAddr, c.AdminAddr); err != nil {
		return err
	}
	if c.DrainDelay > maxDrainDelay {
		return fmt.Errorf("%w: %s=%s exceeds %s", ErrInvalidVariable, envDrainDelay, c.DrainDelay, maxDrainDelay)
	}
	switch {
	case c.IdentityTarget == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envIdentityTarget)
	case !c.GRPCInsecure && c.CAFile == "":
		return fmt.Errorf("%w: %s=true or %s", ErrMissingVariable, envGRPCInsecure, envGRPCCAFile)
	case c.CAFile != "" && (c.ClientCertFile == "" || c.ClientKeyFile == ""):
		return fmt.Errorf("%w: %s and %s", ErrMissingVariable, envGRPCClientCertFile, envGRPCClientKeyFile)
	}
	if err := c.RouteBudget.Validate(); err != nil {
		return err
	}
	if err := c.Auth.Validate(); err != nil {
		return err
	}
	return validateRedis(c.Redis)
}

func readRedis(lookup func(string) string) (redis.Config, error) {
	if lookup(envRedisURL) == "" {
		return redis.Config{}, nil
	}
	return redis.FromEnv(lookup)
}

func validateRedis(c redis.Config) error {
	if c.Options == nil {
		return fmt.Errorf("%w: %s", ErrMissingVariable, envRedisURL)
	}
	return c.Validate()
}

type listener struct {
	host string
	ip   net.IP
	port int
}

func listenerOf(name, addr string) (listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return listener{}, fmt.Errorf("%w: %s=%q is not host:port", ErrInvalidVariable, name, addr)
	}
	number := 0
	if port != "" {
		if number, err = net.LookupPort("tcp", port); err != nil {
			return listener{}, fmt.Errorf("%w: %s=%q is not host:port", ErrInvalidVariable, name, addr)
		}
	}
	return listener{host: host, ip: net.ParseIP(host), port: number}, nil
}

func (l listener) wildcard() bool { return l.host == "" || l.ip != nil && l.ip.IsUnspecified() }

func (l listener) sameHost(other listener) bool {
	if l.ip != nil && other.ip != nil {
		return l.ip.Equal(other.ip)
	}
	return l.host == other.host
}

func validateListeners(httpAddr, adminAddr string) error {
	public, err := listenerOf(envHTTPAddr, httpAddr)
	if err != nil {
		return err
	}
	admin, err := listenerOf(envAdminAddr, adminAddr)
	if err != nil {
		return err
	}
	if public.port != 0 && public.port == admin.port && (public.sameHost(admin) || public.wildcard() || admin.wildcard()) {
		return fmt.Errorf("%w: %s=%q and %s=%q share a listener", ErrInvalidVariable, envHTTPAddr, httpAddr, envAdminAddr, adminAddr)
	}
	return nil
}
