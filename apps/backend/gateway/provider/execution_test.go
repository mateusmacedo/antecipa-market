//go:build integration

package provider_test

import (
	"context"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

const platform = port.TenantID("platform")

func withExecution(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.AutoKey(), execctx.WithTenant(platform))
}

func withTenant(t *testing.T, tenant port.TenantID) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.AutoKey(), execctx.WithTenant(tenant))
}
