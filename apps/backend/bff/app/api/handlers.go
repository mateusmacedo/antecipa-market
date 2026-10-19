package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"unicode/utf8"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

const (
	maxBodyBytes = 1 << 16

	lateBody = "the request body did not arrive within the deadline of the request"
)

var (
	errUnrepresentable = errors.New("api: the context answered what the contract cannot represent")
	errNoActor         = errors.New("api: a command reached its handler without an authenticated subject")
	errNotUnicode      = errors.New("api: the body is not Unicode text")
)

type handlers struct {
	identity rpc.Identity
}

type rejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type refusal interface {
	GetCode() string
	GetMessage() string
}

// decodeBody reads exactly one JSON object of the contract: unknown fields and
// trailing data are refused, so nothing half-parsed reaches a context.
func decodeBody(r *http.Request, of *shape, into any) error {
	raw, err := readBody(r)
	if err != nil {
		return err
	}
	if !utf8.Valid(raw) || unpairedSurrogate(raw) {
		return errNotUnicode
	}
	if err := exactShape(raw, of); err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

func readBody(r *http.Request) ([]byte, error) {
	size := 512
	if r.ContentLength > 0 {
		size = int(min(r.ContentLength, maxBodyBytes)) + 1
	}
	raw := make([]byte, 0, size)
	for {
		n, err := r.Body.Read(raw[len(raw):cap(raw)])
		raw = raw[:len(raw)+n]
		if errors.Is(err, io.EOF) {
			return raw, nil
		}
		if err != nil {
			return nil, err
		}
		if len(raw) == cap(raw) {
			raw = append(raw, 0)[:len(raw)]
		}
	}
}

func bodyRefusal(err error, contract string) string {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		return tooLargeMessage(tooLarge.Limit)
	case errors.Is(err, os.ErrDeadlineExceeded):
		return lateBody
	}
	return "the body is not the " + contract + " of the contract"
}

func tooLargeMessage(limit int64) string {
	if limit == 0 {
		return "this operation takes no request body"
	}
	return "the request body exceeds the " + strconv.FormatInt(limit, 10) + " bytes this operation accepts"
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if validIdentifier(id) {
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

func writeRefusal(r *http.Request, w http.ResponseWriter, refused refusal) {
	writeRejection(r, w, http.StatusUnprocessableEntity, refused.GetCode(), refused.GetMessage())
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

func fromBody[Body, Req any](contract, invalid string, valid func(Body) bool, build func(Body) Req) decoder[Req] {
	of := mustShapeOf(reflect.TypeFor[*Body]())
	return func(w http.ResponseWriter, r *http.Request) (Req, bool) {
		var none Req
		var body Body
		if err := decodeBody(r, of, &body); err != nil {
			writeRejection(r, w, http.StatusBadRequest, "invalid-request", bodyRefusal(err, contract))
			return none, false
		}
		if !valid(body) {
			writeRejection(r, w, http.StatusBadRequest, "invalid-request", invalid)
			return none, false
		}
		return build(body), true
	}
}

func fromPathAndBody[Body, Req any](name, contract, invalid string, valid func(Body) bool, build func(id string, body Body) Req) decoder[Req] {
	body := fromBody(contract, invalid, valid, func(b Body) Body { return b })
	return func(w http.ResponseWriter, r *http.Request) (Req, bool) {
		var none Req
		id, ok := pathID(w, r, name)
		if !ok {
			return none, false
		}
		decoded, ok := body(w, r)
		if !ok {
			return none, false
		}
		return build(id, decoded), true
	}
}

func fromPathWithoutBody[Req any](name string, build func(id string) Req) decoder[Req] {
	return func(w http.ResponseWriter, r *http.Request) (Req, bool) {
		var none Req
		id, ok := pathID(w, r, name)
		if !ok {
			return none, false
		}
		var first [1]byte
		if n, err := io.ReadFull(r.Body, first[:]); n != 0 || !errors.Is(err, io.EOF) {
			refusal := tooLargeMessage(0)
			if errors.Is(err, os.ErrDeadlineExceeded) {
				refusal = lateBody
			}
			writeRejection(r, w, http.StatusBadRequest, "invalid-request", refusal)
			return none, false
		}
		return build(id), true
	}
}

func acting[Req any](decode decoder[Req], assign func(req Req, actor string)) decoder[Req] {
	return func(w http.ResponseWriter, r *http.Request) (Req, bool) {
		var none Req
		actor, ok := actorFrom(r.Context())
		if !ok {
			writeError(r, w, errNoActor)
			return none, false
		}
		req, ok := decode(w, r)
		if !ok {
			return none, false
		}
		assign(req, actor)
		return req, true
	}
}

func actorFrom(ctx context.Context) (string, bool) {
	execution, ok := kernelhttp.ExecutionContextFrom(ctx)
	if !ok {
		return "", false
	}
	subject, ok := execution.Subject()
	return string(subject), ok
}

func outcome[Resp any](status int, present func(Resp) (reference, refusal)) func(http.ResponseWriter, *http.Request, Resp) {
	return func(w http.ResponseWriter, r *http.Request, resp Resp) {
		created, refused := present(resp)
		switch {
		case refused != nil && refused.GetCode() != "":
			writeRefusal(r, w, refused)
		case refused == nil && validIdentifier(created.id):
			writeJSON(r, w, status, created)
		default:
			writeError(r, w, errUnrepresentable)
		}
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
	if status < http.StatusInternalServerError && rpc.Replayed(r.Context()) {
		w.Header().Set(ReplayedHeader, "true")
	}
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
