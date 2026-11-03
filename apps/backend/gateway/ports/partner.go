package ports

import (
	"context"
	"errors"
	"fmt"
	"sync"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// ErrPartnerAdapterUnavailable is a connector and operation with no adapter:
// the caller fails closed as an unavailable dependency, with no external call.
var ErrPartnerAdapterUnavailable = errors.New("ports: no partner adapter for the connector and operation")

// PartnerAdapters resolves, per connector and operation, the adapter of each
// way the context talks to a partner. The command is bound to the collector of
// the message being dispatched, which its Submit fills.
type PartnerAdapters interface {
	Command(connector domain.ConnectorKey, operation string, collector *CaptureCollector) (port.ExternalCommand, error)
	Reader(connector domain.ConnectorKey, operation string) (PartnerReader, error)
	Normalizer(connector domain.ConnectorKey, operation string) (InteractionNormalizer, error)
}

// PartnerReader answers in raw bytes, unlike the kernel's ExternalState: the
// caller keeps both sides in the RawStore before recording the interaction,
// and the finding comes from the normalizer, not from the call.
type PartnerReader interface {
	QueryState(ctx context.Context, key port.OutboundKey) (RawExchange, error)
	Read(ctx context.Context, parameters []byte) (RawExchange, error)
}

// InteractionNormalizer is pure over the versioned raw payload, so a
// redelivery of one revision never reads it two ways.
type InteractionNormalizer interface {
	Normalize(source domain.InteractionSource, exchange RawExchange) (Reading, error)
}

// Reading is a normalized raw payload. CommandKey is the outbound key the
// partner echoes, which the caller resolves to the command; ReadingHash is the
// SHA-256 of Canonical, both empty outside a normalized outcome.
type Reading struct {
	Outcome     domain.InteractionStatus
	CommandKey  port.OutboundKey
	Finding     port.Finding
	ExternalRef string
	AsOf        port.Instant
	References  []domain.LinkExternalReference
	Mismatch    domain.Mismatch
	Canonical   []byte
	ReadingHash string
}

// NoPartnerAdapters is the runtime's PartnerAdapters until the providers are
// mapped (#5): every connector and operation fails closed.
type NoPartnerAdapters struct{}

func (NoPartnerAdapters) Command(connector domain.ConnectorKey, operation string, _ *CaptureCollector) (port.ExternalCommand, error) {
	return nil, unavailable(connector, operation)
}

func (NoPartnerAdapters) Reader(connector domain.ConnectorKey, operation string) (PartnerReader, error) {
	return nil, unavailable(connector, operation)
}

func (NoPartnerAdapters) Normalizer(connector domain.ConnectorKey, operation string) (InteractionNormalizer, error) {
	return nil, unavailable(connector, operation)
}

func unavailable(connector domain.ConnectorKey, operation string) error {
	return fmt.Errorf("%w: %s %s", ErrPartnerAdapterUnavailable, connector, operation)
}

// Capture is what the adapter of one submission kept: the outbound
// interaction id it minted, both raw payloads, the content type of the
// response and, under ClassSchemaMismatch, how the response broke.
type Capture struct {
	InteractionID domain.InteractionID
	Request       domain.RawPayload
	Response      domain.RawPayload
	ContentType   string
	Mismatch      domain.Mismatch
}

// CaptureCollector carries the capture of one message from the adapter's
// Submit to the Apply of the same Dispatch, because the kernel's Submit returns
// only class, observation and error (decision 17). Begin starts each call over.
type CaptureCollector struct {
	mu        sync.Mutex
	capture   Capture
	responded bool
	lost      error
}

func (c *CaptureCollector) Begin(id domain.InteractionID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capture, c.responded, c.lost = Capture{InteractionID: id}, false, nil
}

func (c *CaptureCollector) Requested(raw domain.RawPayload) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capture.Request = raw
}

func (c *CaptureCollector) Responded(raw domain.RawPayload, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capture.Response, c.capture.ContentType, c.responded = raw, contentType, true
}

func (c *CaptureCollector) Broke(mismatch domain.Mismatch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.capture.Mismatch = mismatch
}

// ResponseLost records the failure that kept the response out of the store:
// the outcome of the submission stands, without a capture (decision 17).
func (c *CaptureCollector) ResponseLost(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lost = err
}

// Captured is the capture of the last call when both raw payloads were kept.
func (c *CaptureCollector) Captured() (Capture, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.responded || c.capture.Request.Ref == "" {
		return Capture{}, false
	}
	return c.capture, true
}

func (c *CaptureCollector) LostResponse() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lost
}
