package app

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

func TestACancelledPermissionsHopIsClientClosedInEitherForm(t *testing.T) {
	for name, err := range map[string]error{
		"a gRPC status": status.Error(codes.Canceled, "gone"),
		"a raw context": fmt.Errorf("hop: %w", context.Canceled),
	} {
		t.Run(name, func(t *testing.T) {
			if got := permissionsFailure(err); got != rpc.CanceledFailure {
				t.Fatalf("permissionsFailure(%v) = %+v, want %+v", err, got, rpc.CanceledFailure)
			}
		})
	}
}
