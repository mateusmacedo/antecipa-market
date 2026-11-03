package app

import (
	"reflect"
	"testing"

	gatewayports "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

func TestTheAPIRoleIsHandedEverySettingOfConfig(t *testing.T) {
	cfg := Defaults(RoleAPI)
	cfg.DSN, cfg.Migrate, cfg.MetricTenants = "postgres://gateway@db/gateway", true, []string{"acme"}
	cfg.MigrateDSN = "postgres://gateway_migrator@db/gateway"

	v := reflect.ValueOf(apiRole(cfg))
	for i := range v.NumField() {
		if v.Field(i).IsZero() {
			t.Errorf("%s.%s is zero, want it set from Config", v.Type().Name(), v.Type().Field(i).Name)
		}
	}
}

// Decision 21: no Config the environment builds carries a partner adapter, so
// every way to a partner fails closed until the providers are mapped.
func TestEveryRoleOfTheEnvironmentHasNoPartnerAdapter(t *testing.T) {
	for _, role := range Roles {
		adapters := Defaults(role).partnerAdapters()
		if _, none := adapters.(gatewayports.NoPartnerAdapters); !none {
			t.Errorf("%s: partner adapters = %T, want NoPartnerAdapters", role, adapters)
		}
	}
}
