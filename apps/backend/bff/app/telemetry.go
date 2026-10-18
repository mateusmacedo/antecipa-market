package app

import (
	"log/slog"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

const processRole = "api"

func TelemetryOf(cfg Config) boot.Telemetry {
	return boot.Telemetry{
		Service:  cfg.Service,
		Role:     processRole,
		Signals:  cfg.Signals,
		Class:    tracing.ClassWrite,
		Settings: settings(cfg),
	}
}

func settings(cfg Config) []slog.Attr {
	return []slog.Attr{
		slog.String("http_addr", cfg.HTTPAddr),
		slog.String("admin_addr", cfg.AdminAddr),
		slog.String("identity_grpc_target", cfg.IdentityTarget),
		slog.Bool("grpc_insecure", cfg.GRPCInsecure),
		slog.String("grpc_ca_file", cfg.CAFile),
		slog.String("grpc_server_name", cfg.ServerName),
		slog.String("grpc_client_cert_file", cfg.ClientCertFile),
		slog.String("grpc_client_key_file", presence(cfg.ClientKeyFile)),
		slog.String("cors_origins", strings.Join(cfg.CORSOrigins, ",")),
		slog.Int("metric_tenants", len(cfg.MetricTenants)),
		slog.String("openapi_identity_path", cfg.IdentityContractPath),
		slog.String("drain_delay", cfg.DrainDelay.String()),
		slog.Any("authn", cfg.Auth),
		slog.Any("redis", cfg.Redis),
	}
}

func presence(value string) string {
	if value == "" {
		return "unset"
	}
	return "set"
}
