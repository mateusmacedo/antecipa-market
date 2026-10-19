package app_test

import (
	"testing"

	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

func TestEveryIdentityMethodLimitIsBelowTheRouteLimit(t *testing.T) {
	route := app.Defaults().RouteBudget
	cfg := rpc.IdentityConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})

	for method, policy := range cfg.Methods {
		if policy.Budget.Limit >= route.Limit {
			t.Errorf("%s: limit %v, want below the route limit %v (GRP-17)", method, policy.Budget.Limit, route.Limit)
		}
	}
}

func TestThePermissionsHopLimitIsBelowTheRouteLimit(t *testing.T) {
	route := app.Defaults().RouteBudget
	cfg := rpc.PermissionsConfig(rpc.Options{Insecure: true, Clock: obsclock.System()})

	for method, policy := range cfg.Methods {
		if policy.Budget.Limit >= route.Limit {
			t.Errorf("%s: limit %v, want below the route limit %v (GRP-17)", method, policy.Budget.Limit, route.Limit)
		}
	}
}
