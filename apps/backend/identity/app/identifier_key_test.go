package app_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app"
)

const sentinelKey = "sentinel-identifier-key-must-never-leak"

var telemetrySubject = obstest.Subject[app.Config, app.Role]{
	Service:     "identity",
	Roles:       app.Roles,
	API:         app.RoleAPI,
	Defaults:    app.Defaults,
	FromEnv:     app.FromEnv,
	TelemetryOf: app.TelemetryOf,
	Scope:       reflect.TypeFor[app.Config]().PkgPath(),
}

func TestTheAPIReadsTheIdentifierKeyAndItsVersion(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup(apiPairs("IDENTITY_IDENTIFIER_HMAC_KEY_VERSION", "3")...))

	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil: a key of exactly 32 bytes is enough", err)
	}
	if cfg.IdentifierKeyVersion != 3 {
		t.Fatalf("IdentifierKeyVersion = %d, want 3", cfg.IdentifierKeyVersion)
	}
}

func TestTheAPINamesEachMissingIdentifierVariable(t *testing.T) {
	for _, variable := range []string{"IDENTITY_IDENTIFIER_HMAC_KEY", "IDENTITY_IDENTIFIER_HMAC_KEY_VERSION"} {
		t.Run("without "+variable, func(t *testing.T) {
			_, err := app.FromEnv(app.RoleAPI, lookup(apiPairs(variable, "")...))

			if !errors.Is(err, app.ErrMissingVariable) || !strings.HasSuffix(err.Error(), ": "+variable) {
				t.Fatalf("FromEnv() = %v, want %s named as missing", err, variable)
			}
		})
	}
}

func TestTheAPIRefusesAnIdentifierKeyShorterThan32Bytes(t *testing.T) {
	short := "sentinel-short-key-of-31-bytes."

	_, err := app.FromEnv(app.RoleAPI, lookup(apiPairs("IDENTITY_IDENTIFIER_HMAC_KEY", short)...))

	if len(short) != 31 {
		t.Fatalf("the short key has %d bytes, want 31", len(short))
	}
	if !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "IDENTITY_IDENTIFIER_HMAC_KEY:") {
		t.Fatalf("FromEnv() = %v, want IDENTITY_IDENTIFIER_HMAC_KEY refused as invalid", err)
	}
	if strings.Contains(err.Error(), short) {
		t.Fatalf("the refusal %q carries the key", err)
	}
}

func TestTheAPIRefusesAKeyVersionThatIsNotAPositiveInteger(t *testing.T) {
	for _, value := range []string{"0", "-1", "one"} {
		t.Run(value, func(t *testing.T) {
			_, err := app.FromEnv(app.RoleAPI, lookup(apiPairs("IDENTITY_IDENTIFIER_HMAC_KEY_VERSION", value)...))

			if !errors.Is(err, app.ErrInvalidVariable) || !strings.Contains(err.Error(), "IDENTITY_IDENTIFIER_HMAC_KEY_VERSION") {
				t.Fatalf("FromEnv() = %v, want the version refused as invalid", err)
			}
		})
	}
}

func TestTheRelayReadsNoIdentifierKey(t *testing.T) {
	_, err := app.FromEnv(app.RoleRelay, lookup(
		"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_INSECURE", "true",
		"KAFKA_IDENTITY_TOPIC", "identity.events", "KAFKA_IDENTITY_DLQ", "identity.events.dlq", "KAFKA_GROUP", "g",
	))

	if err != nil {
		t.Fatalf("FromEnv(relay) = %v, want nil: only the api protects identifiers", err)
	}
}

func TestTheIdentifierKeyNeverReachesAFormattedValue(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup(apiPairs("IDENTITY_IDENTIFIER_HMAC_KEY", sentinelKey)...))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	var text, structured bytes.Buffer
	slog.New(slog.NewTextHandler(&text, nil)).Info("configured", "cfg", cfg, "key", cfg.IdentifierKey)
	slog.New(slog.NewJSONHandler(&structured, nil)).Info("configured", "cfg", cfg, "key", cfg.IdentifierKey)

	outputs := map[string]string{
		"%v":        fmt.Sprintf("cfg=%v", cfg),
		"%+v":       fmt.Sprintf("cfg=%+v", cfg),
		"%#v":       fmt.Sprintf("cfg=%#v", cfg),
		"%s":        fmt.Sprintf("key=%s", cfg.IdentifierKey),
		"%q":        fmt.Sprintf("key=%q", cfg.IdentifierKey),
		"%x":        fmt.Sprintf("key=%x", cfg.IdentifierKey),
		"String":    cfg.IdentifierKey.String(),
		"slog text": text.String(),
		"slog json": structured.String(),
	}
	for name, out := range outputs {
		if strings.Contains(out, sentinelKey) || strings.Contains(out, hex.EncodeToString([]byte(sentinelKey))) {
			t.Errorf("%s carries the identifier key: %s", name, out)
		}
	}
}

func TestTheProcessConfiguredRecordCarriesOnlyThePresenceOfTheKey(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup(apiPairs("IDENTITY_IDENTIFIER_HMAC_KEY", sentinelKey)...))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}

	record, out := telemetrySubject.ProcessConfigured(t, cfg)

	if strings.Contains(out, sentinelKey) {
		t.Fatalf("the telemetry carries the identifier key (LOG-09): %s", out)
	}
	if got := record["dmpf.config.identity_identifier_hmac_key"].GetStringValue(); got != "set" {
		t.Errorf("dmpf.config.identity_identifier_hmac_key = %v, want set", got)
	}
	if got := record["dmpf.config.identity_identifier_hmac_key_version"].GetIntValue(); got != 1 {
		t.Errorf("dmpf.config.identity_identifier_hmac_key_version = %v, want 1", got)
	}
}
