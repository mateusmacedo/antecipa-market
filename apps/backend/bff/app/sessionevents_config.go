package app

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
)

const (
	envSessionEventsAddr         = "SESSION_EVENTS_ADDR"
	envIssuer                    = "OIDC_ISSUER"
	envRevocationTTL             = "OIDC_REVOCATION_TTL_SECONDS"
	envBackchannelClientID       = "OIDC_BACKCHANNEL_CLIENT_ID"
	envSessionEventsClientID     = "OIDC_SESSION_EVENTS_CLIENT_ID"
	envSessionEventsClientSecret = "OIDC_SESSION_EVENTS_CLIENT_SECRET"
	envSessionEventsInterval     = "OIDC_SESSION_EVENTS_INTERVAL_SECONDS"
	envDiscoveryTimeout          = "OIDC_DISCOVERY_TIMEOUT_SECONDS"
)

const (
	sessionEventsRole = "session-events"

	defaultPollingIntervalSeconds  = 5
	defaultDiscoveryTimeoutSeconds = 10
	defaultPollingRequestTimeout   = 3 * time.Second
)

type SessionEventsConfig struct {
	ListenAddr string
	AdminAddr  string
	Service    string
	Signals    boot.Signals
	DrainDelay time.Duration

	Feed        authn.FeedConfig
	Backchannel authn.BackchannelLogoutConfig
	Polling     authn.SessionPollingConfig
	Redis       redis.Config
}

func SessionEventsDefaults() SessionEventsConfig {
	return SessionEventsConfig{
		ListenAddr:  ":8091",
		AdminAddr:   ":8092",
		Service:     "bff",
		Backchannel: authn.BackchannelLogoutConfig{DiscoveryTimeout: defaultDiscoveryTimeoutSeconds * time.Second},
		Polling:     authn.SessionPollingConfig{Interval: defaultPollingIntervalSeconds * time.Second, RequestTimeout: defaultPollingRequestTimeout},
	}
}

func SessionEventsAdminAddr(lookup func(string) string) string {
	return envconfig.OrDefault(lookup(envAdminAddr), SessionEventsDefaults().AdminAddr)
}

func SessionEventsFromEnv(lookup func(string) string) (SessionEventsConfig, error) {
	cfg := SessionEventsDefaults()

	cfg.ListenAddr = envconfig.OrDefault(lookup(envSessionEventsAddr), cfg.ListenAddr)
	cfg.AdminAddr = SessionEventsAdminAddr(lookup)
	cfg.Backchannel.ClientID = lookup(envBackchannelClientID)
	cfg.Polling.ClientID = lookup(envSessionEventsClientID)
	cfg.Polling.ClientSecret = lookup(envSessionEventsClientSecret)

	var err error
	if cfg.Feed, err = authn.ReadRevocationFeedEnv(lookup); err != nil {
		return SessionEventsConfig{}, err
	}
	if cfg.Polling.Interval, err = parsePositiveSeconds(envSessionEventsInterval, lookup(envSessionEventsInterval), cfg.Polling.Interval); err != nil {
		return SessionEventsConfig{}, err
	}
	if cfg.Backchannel.DiscoveryTimeout, err = parsePositiveSeconds(envDiscoveryTimeout, lookup(envDiscoveryTimeout), cfg.Backchannel.DiscoveryTimeout); err != nil {
		return SessionEventsConfig{}, err
	}
	if cfg.Redis, err = readRedis(lookup); err != nil {
		return SessionEventsConfig{}, err
	}
	if cfg.Signals, err = boot.SignalsFromEnv(lookup); err != nil {
		return SessionEventsConfig{}, err
	}
	if value := lookup(envDrainDelay); value != "" {
		if cfg.DrainDelay, err = time.ParseDuration(value); err != nil || cfg.DrainDelay < 0 {
			return SessionEventsConfig{}, fmt.Errorf("%w: %s=%q is not a non-negative duration", ErrInvalidVariable, envDrainDelay, value)
		}
	}

	if err := cfg.Validate(); err != nil {
		return SessionEventsConfig{}, err
	}
	return cfg, nil
}

func (c SessionEventsConfig) Validate() error {
	if err := validateListeners(envSessionEventsAddr, c.ListenAddr, envAdminAddr, c.AdminAddr); err != nil {
		return err
	}
	if c.DrainDelay > maxDrainDelay {
		return fmt.Errorf("%w: %s=%s exceeds %s", ErrInvalidVariable, envDrainDelay, c.DrainDelay, maxDrainDelay)
	}
	switch {
	case c.Feed.Issuer == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envIssuer)
	case c.Feed.TTL <= 0:
		return fmt.Errorf("%w: %s", ErrMissingVariable, envRevocationTTL)
	case c.Backchannel.ClientID == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envBackchannelClientID)
	case c.Polling.ClientID == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envSessionEventsClientID)
	case c.Polling.ClientSecret == "":
		return fmt.Errorf("%w: %s", ErrMissingVariable, envSessionEventsClientSecret)
	case c.Polling.Interval > c.Feed.TTL/2+c.Feed.Skew/2:
		return fmt.Errorf("%w: %s=%s exceeds half the revocation window (%s + skew %s)",
			ErrInvalidVariable, envSessionEventsInterval, c.Polling.Interval, c.Feed.TTL, c.Feed.Skew)
	}
	return validateRedis(c.Redis)
}

func parsePositiveSeconds(variable, value string, fallback time.Duration) (time.Duration, error) {
	if value == "" {
		return fallback, nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 || seconds > math.MaxInt64/int64(time.Second) {
		return 0, fmt.Errorf("%w: %s=%q is not a number of seconds from 1 up to the largest duration",
			ErrInvalidVariable, variable, value)
	}
	return time.Duration(seconds) * time.Second, nil
}

func SessionEventsTelemetryOf(cfg SessionEventsConfig) boot.Telemetry {
	return boot.Telemetry{
		Service: cfg.Service,
		Role:    sessionEventsRole,
		Signals: cfg.Signals,
		Class:   tracing.ClassWrite,
		Settings: []slog.Attr{
			slog.String("session_events_addr", cfg.ListenAddr),
			slog.String("admin_addr", cfg.AdminAddr),
			slog.String("issuer", cfg.Feed.Issuer),
			slog.String("revocation_ttl", cfg.Feed.TTL.String()),
			slog.String("revocation_skew", cfg.Feed.Skew.String()),
			slog.String("backchannel_client_id", cfg.Backchannel.ClientID),
			slog.String("discovery_timeout", cfg.Backchannel.DiscoveryTimeout.String()),
			slog.String("polling_client_id", cfg.Polling.ClientID),
			slog.String("polling_client_secret", presence(cfg.Polling.ClientSecret)),
			slog.String("polling_interval", cfg.Polling.Interval.String()),
			slog.String("drain_delay", cfg.DrainDelay.String()),
			slog.Any("redis", cfg.Redis),
		},
	}
}
