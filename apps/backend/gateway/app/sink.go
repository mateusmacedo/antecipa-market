package app

import (
	"context"
	"log/slog"
	"slices"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Sink is the bridge of FND-06 §11 plus the subscription: partner.events
// carries every event of the Partner and this consumer takes a closed set; the
// rest is acknowledged as received, never quarantined, nor written to the inbox.
type Sink struct {
	Consumer   kernelapp.Consumer
	EventTypes []string
	Logger     *slog.Logger
}

// Handle hands to the adapter, which opens the process span (RF-B9), every
// delivery of the set, and also one whose envelope does not even unmarshal:
// classifying it is the adapter's.
func (s Sink) Handle(ctx context.Context, raw []byte, attempt int, ack ports.Acknowledger) error {
	env, err := envelope.Unmarshal(raw)
	if err == nil && !slices.Contains(s.EventTypes, env.Type) {
		if s.Logger != nil {
			s.Logger.DebugContext(ctx, "delivery of another event type acknowledged",
				string(semconv.CloudEventsEventTypeKey), env.Type, string(semconv.MessagingMessageIDKey), env.ID)
		}
		return ack.Ack(ctx)
	}
	_, err = s.Consumer.Consume(ctx, kernelapp.Delivery{Raw: raw, Attempt: attempt}, ack)
	return err
}
