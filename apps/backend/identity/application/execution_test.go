package application_test

import (
	"context"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

func withExecution(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	return execctx.WithExecution(t, ctx, execctx.WithSlot(), execctx.AutoKey())
}

func withTenant(t *testing.T, ctx context.Context, tenant port.TenantID) context.Context {
	t.Helper()
	return execctx.WithExecution(t, ctx, execctx.WithSlot(), execctx.AutoKey(), execctx.WithTenant(tenant))
}

func withKey(t *testing.T, ctx context.Context, key string) context.Context {
	t.Helper()
	return execctx.WithKey(t, ctx, key, execctx.WithSlot())
}
