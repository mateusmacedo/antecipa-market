package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"go.opentelemetry.io/otel/trace"
)

const maxBodyBytes = 1 << 16

type handlers struct{}

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

func writeRejection(r *http.Request, w http.ResponseWriter, status int, code, message string) {
	writeJSON(r, w, status, rejection{Code: code, Message: message})
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
