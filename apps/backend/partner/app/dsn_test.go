package app_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app"
)

func runtimeParams(t *testing.T, dsn string) map[string]string {
	t.Helper()
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig(%q) = %v", dsn, err)
	}
	return cfg.RuntimeParams
}

func TestTheAPIBoundsTheLockWaitAndTheIdleTransactionOfItsPool(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dsn      string
		lock     string
		idleInTx string
	}{
		{"url", "postgres://partner:secret@db:5432/partner?sslmode=disable", "2s", "30s"},
		{"keyword and value", "host=db port=5432 user=partner dbname=partner sslmode=disable", "2s", "30s"},
		{"explicit lock timeout", "postgres://partner:secret@db:5432/partner?sslmode=disable&lock_timeout=5s", "5s", "30s"},
		{"explicit idle timeout", "host=db user=partner dbname=partner idle_in_transaction_session_timeout=1min", "2s", "1min"},
		{"several hosts", "postgres://partner:secret@db1:5432,db2:5432/partner?sslmode=disable", "2s", "30s"},
		{"url without a query", "postgres://partner:secret@db:5432/partner", "2s", "30s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", tc.dsn, "GRPC_INSECURE", "true"))
			if err != nil {
				t.Fatalf("FromEnv() = %v, want nil", err)
			}

			params := runtimeParams(t, cfg.DSN)
			if params["lock_timeout"] != tc.lock || params["idle_in_transaction_session_timeout"] != tc.idleInTx {
				t.Fatalf("runtime params of %q = %v, want lock_timeout %s and idle_in_transaction_session_timeout %s", cfg.DSN, params, tc.lock, tc.idleInTx)
			}
			if got := runtimeParams(t, cfg.DSN)["sslmode"]; got != "" {
				t.Fatalf("sslmode leaked into the runtime params: %q", got)
			}
		})
	}
}

// A timeout given through options, in the DSN or by PGOPTIONS, is the
// environment's: a startup parameter appended after it would win.
func TestATimeoutGivenThroughOptionsIsKept(t *testing.T) {
	for _, tc := range []struct {
		name string
		dsn  string
		env  string
	}{
		{"options in the DSN", "postgres://partner:secret@db:5432/partner?options=-c%20lock_timeout%3D7s", ""},
		{"PGOPTIONS", "postgres://partner:secret@db:5432/partner", "-c lock_timeout=7s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PGOPTIONS", tc.env)
			cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", tc.dsn, "GRPC_INSECURE", "true"))
			if err != nil {
				t.Fatalf("FromEnv() = %v, want nil", err)
			}

			params := runtimeParams(t, cfg.DSN)
			if _, appended := params["lock_timeout"]; appended || params["idle_in_transaction_session_timeout"] != "30s" {
				t.Fatalf("runtime params of %q = %v, want the lock timeout of options kept and the idle timeout added", cfg.DSN, params)
			}
		})
	}
}

// The relay holds its claim open while the broker answers, so the bound on an
// idle transaction is left to the role that takes the guard (ADR-067).
func TestTheRelayKeepsItsDSNAsTheEnvironmentGivesIt(t *testing.T) {
	const dsn = "postgres://partner:secret@db:5432/partner?sslmode=disable"
	cfg, err := app.FromEnv(app.RoleRelay, lookup("PG_DSN", dsn,
		"KAFKA_BROKERS", "b:9092", "KAFKA_INSECURE", "true", "KAFKA_PARTNER_TOPIC", "t", "KAFKA_PARTNER_DLQ", "d", "KAFKA_GROUP", "g"))
	if err != nil {
		t.Fatalf("FromEnv(relay) = %v, want nil", err)
	}

	if cfg.DSN != dsn {
		t.Fatalf("FromEnv(relay).DSN = %q, want %q unchanged", cfg.DSN, dsn)
	}
}

func TestAMalformedDSNIsRefusedAtStartup(t *testing.T) {
	if _, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", "postgres://%zz", "GRPC_INSECURE", "true")); err == nil {
		t.Fatal("FromEnv() = nil, want the malformed DSN refused before the pool opens")
	}
}
