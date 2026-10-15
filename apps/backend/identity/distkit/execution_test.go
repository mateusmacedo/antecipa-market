//go:build integration && distributed

package distkit_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

// The gRPC edge mounts the tenant and no subject, which is what the
// authorization of this context admits by the tenant alone.
func withExecution(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	return execctx.WithExecution(t, ctx, execctx.AutoKey(), execctx.WithoutSubject())
}
