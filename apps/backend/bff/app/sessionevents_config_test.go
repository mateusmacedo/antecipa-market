package app_test

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
)

var sessionEventsEnv = []string{
	"OIDC_ISSUER", "http://keycloak:8080/realms/dmpf",
	"OIDC_REVOCATION_TTL_SECONDS", "300",
	"OIDC_BACKCHANNEL_CLIENT_ID", "dmpf-portal",
	"OIDC_SESSION_EVENTS_CLIENT_ID", "dmpf-session-events",
	"OIDC_SESSION_EVENTS_CLIENT_SECRET", "sentinel-polling-secret",
	"REDIS_URL", "redis://revocation:sentinel-redis-password@redis:6379/0",
}

var sessionEventsTelemetry = obstest.Subject[app.SessionEventsConfig, string]{
	Service:  "bff",
	Roles:    []string{"session-events"},
	API:      "session-events",
	Defaults: func(string) app.SessionEventsConfig { return app.SessionEventsDefaults() },
	FromEnv: func(_ string, getenv func(string) string) (app.SessionEventsConfig, error) {
		return app.SessionEventsFromEnv(getenv)
	},
	TelemetryOf: app.SessionEventsTelemetryOf,
	Scope:       reflect.TypeFor[app.Config]().PkgPath(),
}

func TestSessionEventsFromEnvNeedsNoVariableOfTheAPI(t *testing.T) {
	cfg, err := app.SessionEventsFromEnv(lookup(sessionEventsEnv...))

	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v, want a start without gRPC target, transport policy or AUTH_DEV_MOCK", err)
	}
	if cfg.ListenAddr != ":8091" || cfg.AdminAddr != ":8092" || cfg.Polling.Interval != 5*time.Second {
		t.Fatalf("cfg = %+v, want :8091, :8092 and a 5s polling interval by default", cfg)
	}
	if cfg.Feed.Issuer != "http://keycloak:8080/realms/dmpf" || cfg.Feed.TTL != 5*time.Minute || cfg.Feed.Skew != time.Minute {
		t.Fatalf("cfg.Feed = %+v, want the issuer, a 300s TTL and the 60s default skew", cfg.Feed)
	}
	if cfg.Backchannel.ClientID != "dmpf-portal" || cfg.Polling.ClientID != "dmpf-session-events" || cfg.Polling.ClientSecret != "sentinel-polling-secret" {
		t.Fatalf("cfg = %+v, want both clients read", cfg)
	}
	if cfg.Redis.Options == nil || cfg.Redis.Options.Addr != "redis:6379" {
		t.Fatalf("cfg.Redis = %v, want redis:6379", cfg.Redis)
	}
}

func TestSessionEventsFromEnvReadsItsOwnAddressesAndInterval(t *testing.T) {
	cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv),
		"SESSION_EVENTS_ADDR", "127.0.0.1:9101", "ADMIN_ADDR", "127.0.0.1:9102", "OIDC_SESSION_EVENTS_INTERVAL_SECONDS", "2", "DRAIN_DELAY", "3s")...))

	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:9101" || cfg.AdminAddr != "127.0.0.1:9102" || cfg.Polling.Interval != 2*time.Second || cfg.DrainDelay != 3*time.Second {
		t.Fatalf("cfg = %+v, want the declared addresses, interval and drain", cfg)
	}
}

func TestSessionEventsFromEnvNamesEachMissingVariable(t *testing.T) {
	for _, variable := range []string{"OIDC_ISSUER", "OIDC_REVOCATION_TTL_SECONDS", "OIDC_BACKCHANNEL_CLIENT_ID",
		"OIDC_SESSION_EVENTS_CLIENT_ID", "OIDC_SESSION_EVENTS_CLIENT_SECRET", "REDIS_URL"} {
		t.Run(variable, func(t *testing.T) {
			_, err := app.SessionEventsFromEnv(lookup(without(sessionEventsEnv, variable)...))

			if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("SessionEventsFromEnv() = %v, want %s named", err, variable)
			}
		})
	}
}

func TestSessionEventsFromEnvRefusesAnInvalidValue(t *testing.T) {
	for variable, value := range map[string]string{
		"OIDC_SESSION_EVENTS_INTERVAL_SECONDS": "0",
		"OIDC_REVOCATION_TTL_SECONDS":          "soon",
		"ADMIN_ADDR":                           "8092",
		"SESSION_EVENTS_ADDR":                  "8091",
		"DRAIN_DELAY":                          "-1s",
		"OIDC_DISCOVERY_TIMEOUT_SECONDS":       "never",
	} {
		t.Run(variable, func(t *testing.T) {
			_, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), variable, value)...))

			if !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), variable) {
				t.Fatalf("SessionEventsFromEnv(%s=%s) = %v, want ErrInvalidVariable naming it", variable, value, err)
			}
		})
	}
}

func TestSessionEventsFromEnvBoundsTheDrainByTheTerminationGrace(t *testing.T) {
	cases := []struct {
		value string
		want  error
	}{
		{"10s", nil},
		{"10s1ms", app.ErrInvalidVariable},
		{"1m", app.ErrInvalidVariable},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			_, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), "DRAIN_DELAY", tc.value)...))

			if !errors.Is(err, tc.want) || err != nil && !strings.Contains(err.Error(), "DRAIN_DELAY") {
				t.Fatalf("SessionEventsFromEnv(DRAIN_DELAY=%s) = %v, want %v naming DRAIN_DELAY", tc.value, err, tc.want)
			}
		})
	}
}

func TestSessionEventsFromEnvBoundsTheSecondsByTheLargestDuration(t *testing.T) {
	largest := strconv.FormatInt(math.MaxInt64/int64(time.Second), 10)
	beyond := strconv.FormatInt(math.MaxInt64/int64(time.Second)+1, 10)
	cases := []struct {
		variable string
		value    string
		want     error
	}{
		{"OIDC_SESSION_EVENTS_INTERVAL_SECONDS", largest, app.ErrInvalidVariable},
		{"OIDC_SESSION_EVENTS_INTERVAL_SECONDS", beyond, app.ErrInvalidVariable},
		{"OIDC_DISCOVERY_TIMEOUT_SECONDS", largest, nil},
		{"OIDC_DISCOVERY_TIMEOUT_SECONDS", beyond, app.ErrInvalidVariable},
		{"OIDC_DISCOVERY_TIMEOUT_SECONDS", "-1", app.ErrInvalidVariable},
	}
	for _, tc := range cases {
		t.Run(tc.variable+"="+tc.value, func(t *testing.T) {
			cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), tc.variable, tc.value)...))

			if !errors.Is(err, tc.want) {
				t.Fatalf("SessionEventsFromEnv() = %v, want %v", err, tc.want)
			}
			if err != nil && !strings.Contains(err.Error(), tc.variable) {
				t.Fatalf("SessionEventsFromEnv() = %v, want %s named", err, tc.variable)
			}
			if err == nil && (cfg.Polling.Interval <= 0 || cfg.Backchannel.DiscoveryTimeout <= 0) {
				t.Fatalf("cfg = %+v, want positive durations", cfg)
			}
		})
	}
}

func TestSessionEventsFromEnvRefusesTheReceiverAndTheAdministrationOnOneListener(t *testing.T) {
	for _, pair := range [][2]string{{":9000", ":9000"}, {":9000", "127.0.0.1:9000"}, {"[::1]:9000", "[0:0:0:0:0:0:0:1]:9000"}} {
		t.Run(pair[0]+" "+pair[1], func(t *testing.T) {
			_, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), "SESSION_EVENTS_ADDR", pair[0], "ADMIN_ADDR", pair[1])...))

			if !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "SESSION_EVENTS_ADDR") || !strings.Contains(err.Error(), "ADMIN_ADDR") {
				t.Fatalf("SessionEventsFromEnv() = %v, want the shared listener refused naming both variables", err)
			}
		})
	}
}

func TestSessionEventsFromEnvAcceptsTwoEphemeralPorts(t *testing.T) {
	if _, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), "SESSION_EVENTS_ADDR", "127.0.0.1:0", "ADMIN_ADDR", "127.0.0.1:0")...)); err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v, want port 0 on both listeners accepted", err)
	}
}

func TestTheSessionEventsRecordCarriesItsSettingsAndNoSecret(t *testing.T) {
	cfg, err := app.SessionEventsFromEnv(lookup(sessionEventsEnv...))
	if err != nil {
		t.Fatalf("SessionEventsFromEnv() = %v", err)
	}

	record, out := sessionEventsTelemetry.ProcessConfigured(t, cfg)

	want := []string{"admin_addr", "backchannel_client_id", "discovery_timeout", "drain_delay", "issuer", "polling_client_id",
		"polling_client_secret", "polling_interval", "redis.addr", "redis.command_timeout", "redis.dial_timeout", "redis.tls",
		"redis.username", "revocation_skew", "revocation_ttl", "session_events_addr"}
	if got := obstest.SettingKeys(record); !slices.Equal(got, want) {
		t.Fatalf("dmpf.config.* = %v, want exactly %v", got, want)
	}
	for _, sentinel := range []string{"sentinel-polling-secret", "sentinel-redis-password"} {
		if strings.Contains(out, sentinel) {
			t.Errorf("the telemetry carries %q (LOG-09): %s", sentinel, out)
		}
	}
}

func TestTheTelemetryDeclaresTheSessionEventsRole(t *testing.T) {
	sessionEventsTelemetry.RunRolesDeclared(t)
}

func TestTheSessionEventsAdministrationAddressIsTheOneTheRoleListensOn(t *testing.T) {
	for _, env := range [][]string{nil, {"ADMIN_ADDR", "127.0.0.1:9102"}} {
		cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), env...)...))
		if err != nil {
			t.Fatalf("SessionEventsFromEnv() = %v", err)
		}

		if got := app.SessionEventsAdminAddr(lookup(env...)); got != cfg.AdminAddr {
			t.Fatalf("SessionEventsAdminAddr(%v) = %q, want %q, where the role listens", env, got, cfg.AdminAddr)
		}
	}
	if got := app.SessionEventsAdminAddr(lookup()); got != ":8092" {
		t.Fatalf("SessionEventsAdminAddr() = %q, want :8092 by default", got)
	}
}

func TestSessionEventsFromEnvKeepsTwoCyclesInsideTheRevocationWindow(t *testing.T) {
	cases := []struct {
		env  []string
		want error
	}{
		{nil, nil},
		{[]string{"OIDC_SESSION_EVENTS_INTERVAL_SECONDS", "180"}, nil},
		{[]string{"OIDC_SESSION_EVENTS_INTERVAL_SECONDS", "181"}, app.ErrInvalidVariable},
		{[]string{"OIDC_SESSION_EVENTS_INTERVAL_SECONDS", "600"}, app.ErrInvalidVariable},
		{[]string{"OIDC_SESSION_EVENTS_INTERVAL_SECONDS", "40", "OIDC_REVOCATION_TTL_SECONDS", "60", "OIDC_REVOCATION_SKEW_SECONDS", "0"}, app.ErrInvalidVariable},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.env, "="), func(t *testing.T) {
			cfg, err := app.SessionEventsFromEnv(lookup(append(slices.Clone(sessionEventsEnv), tc.env...)...))

			if !errors.Is(err, tc.want) || err != nil && !strings.Contains(err.Error(), "OIDC_SESSION_EVENTS_INTERVAL_SECONDS") {
				t.Fatalf("SessionEventsFromEnv() = %v, want %v naming OIDC_SESSION_EVENTS_INTERVAL_SECONDS", err, tc.want)
			}
			if err == nil && 2*cfg.Polling.Interval > cfg.Feed.TTL+cfg.Feed.Skew {
				t.Fatalf("cfg = %+v, want two polling cycles inside TTL + skew", cfg)
			}
		})
	}
}
