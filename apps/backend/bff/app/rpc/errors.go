package rpc

import (
	"context"
	"errors"
	"net/http"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Failure struct {
	Status     int
	Code       string
	Message    string
	RetryAfter time.Duration
}

const overloadRetryAfter = time.Second

var (
	InternalFailure    = Failure{Status: http.StatusInternalServerError, Code: "internal-failure", Message: "the request could not be completed"}
	deadlineFailure    = Failure{Status: http.StatusGatewayTimeout, Code: "deadline-exceeded", Message: "the deadline of the request expired before the context answered"}
	unavailableFailure = Failure{Status: http.StatusServiceUnavailable, Code: "unavailable", Message: "the context is unavailable"}
	CanceledFailure    = Failure{Status: kernelgrpc.StatusClientClosedRequest, Code: "client-closed-request", Message: "the caller went away before the context answered"}

	// The kernel's own admission refusal asks for the same second
	// (http@v1.0.0-rc.5 admission.go:74).
	OverloadedFailure = Failure{Status: unavailableFailure.Status, Code: unavailableFailure.Code, Message: unavailableFailure.Message, RetryAfter: overloadRetryAfter}

	// The HTTP status never lives here: it comes from kernelgrpc.HTTPStatus,
	// the one table GRP-14 fixes.
	codeNames = map[codes.Code]struct{ code, message string }{
		codes.NotFound:           {"not-found", "no resource with this id"},
		codes.Aborted:            {"version-conflict", "a concurrent write advanced the resource; replay the request"},
		codes.AlreadyExists:      {"already-exists", "the resource already exists"},
		codes.DeadlineExceeded:   {deadlineFailure.Code, deadlineFailure.Message},
		codes.Unavailable:        {unavailableFailure.Code, unavailableFailure.Message},
		codes.ResourceExhausted:  {"admission-refused", "the context refused the call; retry later"},
		codes.InvalidArgument:    {"invalid-request", "the context refused the request as invalid"},
		codes.FailedPrecondition: {"failed-precondition", "the resource is not in a state that accepts this request"},
		codes.OutOfRange:         {"out-of-range", "a value of the request is outside the accepted range"},
		codes.PermissionDenied:   {"permission-denied", "the caller may not perform this operation"},
		codes.Unauthenticated:    {"unauthenticated", "the caller is not authenticated"},
		codes.Unimplemented:      {"unimplemented", "the context does not implement this operation"},
		codes.Canceled:           {CanceledFailure.Code, CanceledFailure.Message},
	}

	// Read before the code: FailedPrecondition and Aborted also carry other
	// meanings than the idempotency outcome (IDM-04, IDM-07).
	reasonFailures = map[string]Failure{
		kernelgrpc.ReasonReusedIdempotencyKey:   {Status: http.StatusUnprocessableEntity, Code: "reused-idempotency-key", Message: "the idempotency key was already used with another request"},
		kernelgrpc.ReasonInFlightIdempotencyKey: {Status: http.StatusConflict, Code: "in-flight-idempotency-key", Message: "a request with this idempotency key is still in progress; retry with the same key"},
		kernelgrpc.ReasonMissingIdempotencyKey:  {Status: http.StatusBadRequest, Code: "missing-idempotency-key", Message: "the request requires an idempotency key"},
		kernelgrpc.ReasonInvalidIdempotencyKey:  {Status: http.StatusBadRequest, Code: "invalid-idempotency-key", Message: "the idempotency key is outside the accepted format"},
	}
)

// The local deadline, breaker and bulkhead errors carry no gRPC status, so
// status.FromError would report them as Unknown and turn a timeout or a
// refused dependency into a 500 (RES-12, RES-14).
func Classify(err error) Failure {
	switch {
	case errors.Is(err, deadline.ErrNoDeadline):
		return InternalFailure
	case errors.Is(err, deadline.ErrDeadlineExhausted), errors.Is(err, resilience.ErrDeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return deadlineFailure
	case errors.Is(err, context.Canceled):
		return CanceledFailure
	case errors.Is(err, resilience.ErrBreakerOpen), errors.Is(err, resilience.ErrBulkheadSaturated):
		return unavailableFailure
	}
	if failure, ok := reasonFailures[kernelgrpc.ReasonOf(err)]; ok {
		return failure
	}
	s, isStatus := status.FromError(err)
	if !isStatus {
		return InternalFailure
	}
	named, mapped := codeNames[s.Code()]
	if !mapped {
		return InternalFailure
	}
	return Failure{Status: kernelgrpc.HTTPStatus(s.Code()), Code: named.code, Message: named.message}
}

func Category(err error) string { return kernelgrpc.Category(err) }

// The kernel answers 401 to any error of Authenticate (http@v1.0.0-rc.5
// identity.go:37-40); the edge recognizes this one to answer its Failure instead.
type PermissionsUnavailable struct {
	Failure Failure
	Cause   error
}

func (e *PermissionsUnavailable) Error() string {
	return "rpc: effective permissions unavailable: " + e.Cause.Error()
}

func (e *PermissionsUnavailable) Unwrap() error { return e.Cause }
