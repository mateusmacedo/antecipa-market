package rpc_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

func idempotencyStatus(t *testing.T, err error) error {
	t.Helper()
	mapped, ok := kernelgrpc.IdempotencyStatus(err)
	if !ok {
		t.Fatalf("IdempotencyStatus(%v) mapped nothing", err)
	}
	return mapped
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"reused key", idempotencyStatus(t, ports.ErrIdempotencyMismatch), http.StatusUnprocessableEntity, "reused-idempotency-key"},
		{"key in flight", idempotencyStatus(t, ports.ErrIdempotencyInFlight), http.StatusConflict, "in-flight-idempotency-key"},
		{"missing key", kernelgrpc.KeyStatus(kernelgrpc.ReasonMissingIdempotencyKey), http.StatusBadRequest, "missing-idempotency-key"},
		{"invalid key", kernelgrpc.KeyStatus(kernelgrpc.ReasonInvalidIdempotencyKey), http.StatusBadRequest, "invalid-idempotency-key"},
		{"already exists", idempotencyStatus(t, ports.ErrAlreadyExists), http.StatusConflict, "already-exists"},
		{"not found", status.Error(codes.NotFound, "row absent in identity"), http.StatusNotFound, "not-found"},
		{"version conflict", status.Error(codes.Aborted, "version 3"), http.StatusConflict, "version-conflict"},
		{"remote deadline", status.Error(codes.DeadlineExceeded, "late"), http.StatusGatewayTimeout, "deadline-exceeded"},
		{"local context deadline", fmt.Errorf("call: %w", context.DeadlineExceeded), http.StatusGatewayTimeout, "deadline-exceeded"},
		{"local exhausted deadline", fmt.Errorf("%w: identity", deadline.ErrDeadlineExhausted), http.StatusGatewayTimeout, "deadline-exceeded"},
		{"resilience deadline", fmt.Errorf("%w: identity.FindOrganization has no time left", resilience.ErrDeadlineExceeded), http.StatusGatewayTimeout, "deadline-exceeded"},
		{"missing deadline", deadline.ErrNoDeadline, http.StatusInternalServerError, "internal-failure"},
		{"unavailable", status.Error(codes.Unavailable, "dial tcp"), http.StatusServiceUnavailable, "unavailable"},
		{"open breaker", fmt.Errorf("grpc: identity: %w", resilience.ErrBreakerOpen), http.StatusServiceUnavailable, "unavailable"},
		{"saturated bulkhead", fmt.Errorf("grpc: identity: %w", resilience.ErrBulkheadSaturated), http.StatusServiceUnavailable, "unavailable"},
		{"admission", status.Error(codes.ResourceExhausted, "limit"), http.StatusTooManyRequests, "admission-refused"},
		{"invalid argument", status.Error(codes.InvalidArgument, "organization_id"), http.StatusBadRequest, "invalid-request"},
		{"failed precondition", status.Error(codes.FailedPrecondition, "suspended"), http.StatusBadRequest, "failed-precondition"},
		{"out of range", status.Error(codes.OutOfRange, "valid_at"), http.StatusBadRequest, "out-of-range"},
		{"permission denied", status.Error(codes.PermissionDenied, "role"), http.StatusForbidden, "permission-denied"},
		{"unauthenticated", status.Error(codes.Unauthenticated, "peer"), http.StatusUnauthorized, "unauthenticated"},
		{"unimplemented", status.Error(codes.Unimplemented, "method"), http.StatusNotImplemented, "unimplemented"},
		{"canceled", status.Error(codes.Canceled, "gone"), kernelgrpc.StatusClientClosedRequest, "client-closed-request"},
		{"local context canceled", fmt.Errorf("call: %w", context.Canceled), kernelgrpc.StatusClientClosedRequest, "client-closed-request"},
		{"internal", status.Error(codes.Internal, "panic: nil map"), http.StatusInternalServerError, "internal-failure"},
		{"unknown", status.Error(codes.Unknown, "surprise"), http.StatusInternalServerError, "internal-failure"},
		{"plain error", errors.New("pgx: connection refused"), http.StatusInternalServerError, "internal-failure"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rpc.Classify(tc.err)
			if got.Status != tc.status || got.Code != tc.code || got.Message == "" {
				t.Fatalf("Classify() = %+v, want %d %q with a public message", got, tc.status, tc.code)
			}
		})
	}
}

func TestClassifyNeverEchoesWhatTheIdentitySaid(t *testing.T) {
	got := rpc.Classify(status.Error(codes.NotFound, "row absent in identity.organizations"))

	if got.Message == "row absent in identity.organizations" {
		t.Fatalf("Classify().Message = %q, want a public message (ERR-20)", got.Message)
	}
}

func TestCategoryIsTheOneOfTheKernel(t *testing.T) {
	for _, err := range []error{status.Error(codes.NotFound, "x"), status.Error(codes.Unavailable, "x"), errors.New("x")} {
		if got, want := rpc.Category(err), kernelgrpc.Category(err); got != want {
			t.Fatalf("Category(%v) = %q, want %q", err, got, want)
		}
	}
}
