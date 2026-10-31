//go:build integration

package app_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
)

func TestASessionOfTheAPIPoolCarriesTheTimeouts(t *testing.T) {
	cfg, err := app.FromEnv(app.RoleAPI, lookup("PG_DSN", appkit.OpenPool(t).Config().ConnString(), "GRPC_INSECURE", "true"))
	if err != nil {
		t.Fatalf("FromEnv() = %v, want nil", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.DSN)
	if err != nil {
		t.Fatalf("pgxpool.New() = %v", err)
	}
	t.Cleanup(pool.Close)

	for setting, want := range map[string]string{"lock_timeout": "2s", "idle_in_transaction_session_timeout": "30s"} {
		var got string
		if err := pool.QueryRow(context.Background(), "SHOW "+setting).Scan(&got); err != nil || got != want {
			t.Fatalf("SHOW %s = %q, %v; want %q", setting, got, err, want)
		}
	}
}
