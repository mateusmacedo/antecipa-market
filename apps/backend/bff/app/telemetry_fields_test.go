package app_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
)

var sentinels = []string{"sentinel-client-key", "sentinel-tenant", "sentinel-redis-password"}

func configWithSecrets() app.Config {
	cfg := app.Defaults()
	cfg.IdentityTarget = "identity:9090"
	cfg.CAFile = "/tls/ca.crt"
	cfg.ServerName = "identity.internal"
	cfg.ClientCertFile = "/tls/client.crt"
	cfg.ClientKeyFile = "/tls/sentinel-client-key.pem"
	cfg.CORSOrigins = []string{"https://app.example"}
	cfg.MetricTenants = []string{"sentinel-tenant-a", "sentinel-tenant-b"}
	cfg.IdentityContractPath = "/contracts/identity.yaml"
	cfg.DrainDelay = 5 * time.Second
	cfg.Auth = authn.Config{Issuer: "https://issuer.example", Audience: realmAudience, TenantClaim: realmTenantClaim,
		PermissionClaims: []string{"permissions"}, DiscoveryTimeout: 5 * time.Second}
	var err error
	if cfg.Redis, err = redis.FromEnv(lookup("REDIS_URL", "redis://revocation:sentinel-redis-password@redis:6379/0")); err != nil {
		panic(err)
	}
	return cfg
}

func TestTheProcessConfiguredRecordCarriesTheAllowlistOfTheEdge(t *testing.T) {
	record, _ := edgeTelemetry.ProcessConfigured(t, configWithSecrets())

	got := obstest.SettingKeys(record)
	want := []string{"admin_addr", "authn.audience", "authn.dev_mock", "authn.discovery_timeout", "authn.issuer", "authn.permission_claims",
		"authn.revocation", "authn.tenant_claim", "cors_origins", "drain_delay", "grpc_ca_file", "grpc_client_cert_file",
		"grpc_client_key_file", "grpc_insecure", "grpc_server_name", "http_addr", "identity_grpc_target", "metric_tenants",
		"openapi_identity_path", "redis.addr", "redis.command_timeout", "redis.dial_timeout", "redis.tls", "redis.username"}
	if !slices.Equal(got, want) {
		t.Fatalf("dmpf.config.* = %v, want exactly the settings of the edge %v", got, want)
	}
}

func TestTheProcessConfiguredRecordDeclaresTheDenyListTheEdgeServesWith(t *testing.T) {
	cfg, closeRevocations, err := app.WithRevocations(configWithSecrets())
	if err != nil {
		t.Fatalf("WithRevocations() = %v", err)
	}
	t.Cleanup(func() { _ = closeRevocations() })

	record, _ := edgeTelemetry.ProcessConfigured(t, cfg)

	if got := record["dmpf.config.authn.revocation"].GetStringValue(); got != "enabled" {
		t.Fatalf("dmpf.config.authn.revocation = %q, want enabled: the edge never serves without the deny-list", got)
	}
	if got := record["dmpf.config.authn.audience"].GetStringValue(); got != realmAudience {
		t.Fatalf("dmpf.config.authn.audience = %q, want %s", got, realmAudience)
	}
}

func TestRunDeclaresTheDenyListInTheProcessConfiguredRecord(t *testing.T) {
	addr, collector := obstest.StartCollector(t)
	obstest.UseOnlyTheOTelEnvironmentOf(t, map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + addr,
		"OTEL_TRACES_EXPORTER":        "none",
		"OTEL_METRICS_EXPORTER":       "none",
		"OTEL_LOGS_EXPORTER":          "otlp",
		"OTEL_RESOURCE_ATTRIBUTES":    "service.version=test,service.instance.id=bff-1",
	})
	cfg := edgeConfig(t, devMock, "true", "HTTP_ADDR", obstest.UnusedLoopbackAddr(t), "ADMIN_ADDR", obstest.UnusedLoopbackAddr(t))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx, cfg) }()

	var notReady error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		probing, stop := context.WithTimeout(context.Background(), requestLimit)
		notReady = app.Probe(probing, cfg)
		stop()
		if notReady == nil {
			break
		}
	}
	if notReady != nil {
		t.Fatalf("Probe() = %v, want the edge started by Run ready", notReady)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() = %v, want nil on shutdown", err)
		}
	case <-time.After(drainLimit):
		t.Fatalf("Run() did not return in %v after the cancel", drainLimit)
	}

	revocation := ""
	for _, record := range collector.LogRecords() {
		if record.GetBody().GetStringValue() != "process configured" {
			continue
		}
		for _, attribute := range record.GetAttributes() {
			if attribute.GetKey() == "dmpf.config.authn.revocation" {
				revocation = attribute.GetValue().GetStringValue()
			}
		}
	}
	if revocation != "enabled" {
		t.Fatalf("dmpf.config.authn.revocation emitted by Run = %q, want enabled", revocation)
	}
}

func TestTheProcessConfiguredRecordCarriesNoSecret(t *testing.T) {
	record, out := edgeTelemetry.ProcessConfigured(t, configWithSecrets())

	for _, sentinel := range sentinels {
		if strings.Contains(out, sentinel) {
			t.Errorf("the telemetry carries %q (LOG-09): %s", sentinel, out)
		}
	}
	if got := record["dmpf.config.metric_tenants"]; got.GetIntValue() != 2 {
		t.Errorf("dmpf.config.metric_tenants = %v, want the count 2 and not the names", got)
	}
	if got := record["dmpf.config.grpc_client_key_file"].GetStringValue(); got != "set" {
		t.Errorf("dmpf.config.grpc_client_key_file = %v, want set", got)
	}
}

func TestTheTelemetryDeclaresTheApiRole(t *testing.T) {
	edgeTelemetry.RunRolesDeclared(t)
}

func TestARecordNamesTheExecutionOfTheCallFromTheBaggage(t *testing.T) {
	edgeTelemetry.RunTenantBaggage(t)
}
