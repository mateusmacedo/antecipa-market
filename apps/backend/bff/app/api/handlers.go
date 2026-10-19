package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

const maxBodyBytes = 1 << 16

var (
	idFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

	errUnrepresentable = errors.New("api: the context answered what the contract cannot represent")
)

type handlers struct {
	identity rpc.Identity
}

type rejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// decodeBody reads exactly one JSON object of the contract: unknown fields and
// trailing data are refused, so nothing half-parsed reaches a context.
func decodeBody(w http.ResponseWriter, r *http.Request, into any) error {
	stream := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	stream.DisallowUnknownFields()
	if err := stream.Decode(into); err != nil {
		return err
	}
	if err := stream.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("api: trailing data after the request body")
	}
	return nil
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if idFormat.MatchString(id) {
		return id, true
	}
	writeRejection(r, w, http.StatusBadRequest, "invalid-request", name+" must have 1 to 128 characters of [A-Za-z0-9._:-]")
	return "", false
}

func instantQuery(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	query := r.URL.Query()
	if !query.Has(name) {
		return 0, true
	}
	instant, err := strconv.ParseInt(query.Get(name), 10, 64)
	if err != nil || instant < 0 {
		writeRejection(r, w, http.StatusBadRequest, "invalid-request", name+" must be an integer of at least 0")
		return 0, false
	}
	return instant, true
}

func writeRejection(r *http.Request, w http.ResponseWriter, status int, code, message string) {
	writeJSON(r, w, status, rejection{Code: code, Message: message})
}

func writeFailure(r *http.Request, w http.ResponseWriter, failure rpc.Failure) {
	if seconds := int(failure.RetryAfter.Seconds()); seconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	writeRejection(r, w, failure.Status, failure.Code, failure.Message)
}

func writeError(r *http.Request, w http.ResponseWriter, err error) {
	failure := rpc.Classify(err)
	if failure.Status >= http.StatusInternalServerError {
		tracing.RecordError(trace.SpanFromContext(r.Context()), rpc.Category(err))
	}
	writeFailure(r, w, failure)
}

type decoder[Req any] func(w http.ResponseWriter, r *http.Request) (Req, bool)

func endpoint[Req, Resp any](decode decoder[Req], call func(context.Context, Req) (Resp, error), present func(http.ResponseWriter, *http.Request, Resp)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, ok := decode(w, r)
		if !ok {
			return
		}
		resp, err := call(r.Context(), req)
		if err != nil {
			writeError(r, w, err)
			return
		}
		present(w, r, resp)
	}
}

func fromPath[Req any](name string, build func(id string) Req) decoder[Req] {
	return func(w http.ResponseWriter, r *http.Request) (Req, bool) {
		id, ok := pathID(w, r, name)
		if !ok {
			var none Req
			return none, false
		}
		return build(id), true
	}
}

func fromPathAndInstant[Req any](name, instant string, build func(id string, at int64) Req) decoder[Req] {
	return func(w http.ResponseWriter, r *http.Request) (Req, bool) {
		var none Req
		id, ok := pathID(w, r, name)
		if !ok {
			return none, false
		}
		at, ok := instantQuery(w, r, instant)
		if !ok {
			return none, false
		}
		return build(id, at), true
	}
}

func view[Resp any](present func(Resp) (any, bool)) func(http.ResponseWriter, *http.Request, Resp) {
	return func(w http.ResponseWriter, r *http.Request, resp Resp) {
		body, representable := present(resp)
		if !representable {
			writeError(r, w, errUnrepresentable)
			return
		}
		writeJSON(r, w, http.StatusOK, body)
	}
}

// WHY: the status line is already on the wire when the body fails, so the only
// thing left is to say so on the span; silence makes a truncated answer look
// like a success in every dashboard.
func writeJSON(r *http.Request, w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		tracing.RecordError(trace.SpanFromContext(r.Context()), kernelgrpc.Category(err))
	}
}

// WHY: the published contract fixes Rejection as the body of every 4xx, and
// the kernel's default refusal is plain text, which no generated client parses.
func refuseAsRejection(w http.ResponseWriter, r *http.Request, status int, reason string) {
	code := "admission-refused"
	if status == http.StatusNotFound {
		code = "not-found"
	}
	writeRejection(r, w, status, code, reason)
}
