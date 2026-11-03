package ports

import (
	"context"
	"errors"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

type RawPart string

const (
	RawRequest  RawPart = "request"
	RawResponse RawPart = "response"
)

// RawKeyOf is the key of decision 16. The interaction id is minted or derived
// in the idempotency key alphabet, so it never adds a segment to the key.
func RawKeyOf(connector domain.ConnectorKey, interaction domain.InteractionID, part RawPart) string {
	return "interactions/" + string(connector) + "/" + string(interaction) + "/" + string(part)
}

type RawBody struct {
	ContentType string
	Body        []byte
}

// RawExchange is one call to a partner in bytes; an inbound interaction has
// only the request.
type RawExchange struct {
	Request  RawBody
	Response RawBody
}

// RawStore keeps raw payloads in the kernel's object storage port. The
// realization computes the SHA-256 Put returns, because the application block
// imports no crypto; Get reads one version back.
type RawStore interface {
	Put(ctx context.Context, key string, body RawBody) (domain.RawPayload, error)
	Get(ctx context.Context, payload domain.RawPayload) (RawBody, error)
}

var (
	// ErrRawStoreUnavailable is transient: a store not ready or unreachable,
	// or a write of unknown outcome whose version stays orphan (decision 16).
	ErrRawStoreUnavailable = errors.New("ports: raw store unavailable")

	// ErrRawPayloadDefect does not heal by repeating: an invalid key, a body
	// that does not hash to the record, or a recorded version that is gone.
	ErrRawPayloadDefect = errors.New("ports: raw payload defect")
)
