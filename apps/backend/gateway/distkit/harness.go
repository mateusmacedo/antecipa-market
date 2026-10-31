//go:build integration && distributed

package distkit

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
)

// Spec runs the relay of this context from its own configuration. The harness
// creates one topic and collects from it, so it carries the dispatch intents
// the vector enqueues; gateway.events goes to the topic the test creates.
var Spec = dist.Spec{
	Name: "gateway",
	Pool: appkit.PoolOptions,
	Roles: map[dist.Role]dist.RoleFunc{dist.RoleRelay: dist.Relay(func(ctx context.Context, getenv func(string) string) error {
		cfg, err := app.FromEnv(app.RoleRelay, oneTopic(getenv))
		if err != nil {
			return err
		}
		return app.Run(ctx, cfg)
	})},
}

// EnvEventsTopic and EnvEventsDLQ name the topic of gateway.events: the
// catalogue refuses two channels on one address (KFK-01c).
const (
	EnvEventsTopic = "DISTKIT_GATEWAY_EVENTS_TOPIC"
	EnvEventsDLQ   = "DISTKIT_GATEWAY_EVENTS_DLQ"
)

func oneTopic(getenv func(string) string) func(string) string {
	return func(name string) string {
		switch name {
		case "KAFKA_GATEWAY_DISPATCH_TOPIC":
			return getenv("KAFKA_GATEWAY_TOPIC")
		case "KAFKA_GATEWAY_DISPATCH_DLQ":
			return getenv("KAFKA_GATEWAY_DLQ")
		case "KAFKA_GATEWAY_TOPIC":
			return getenv(EnvEventsTopic)
		case "KAFKA_GATEWAY_DLQ":
			return getenv(EnvEventsDLQ)
		default:
			return getenv(name)
		}
	}
}

func RunRole(t *testing.T) { dist.RunRole(t, Spec) }
