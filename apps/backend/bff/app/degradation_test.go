package app_test

import (
	"context"
	"io"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

const (
	requestLimit = 2 * time.Second
	drainLimit   = 5 * time.Second
)

func startEdgeExportingTo(t *testing.T, collector string) app.Config {
	t.Helper()
	env := maps.Clone(obstest.ReadEnvFile(t, manifestPath))
	maps.Copy(env, map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + collector,
		"OTEL_METRIC_EXPORT_INTERVAL": "10",
		"OTEL_BSP_SCHEDULE_DELAY":     "10",
		"OTEL_BSP_MAX_QUEUE_SIZE":     "8",
		"OTEL_BLRP_SCHEDULE_DELAY":    "10",
		"OTEL_BLRP_MAX_QUEUE_SIZE":    "8",
		"HTTP_ADDR":                   obstest.UnusedLoopbackAddr(t),
		"ADMIN_ADDR":                  obstest.UnusedLoopbackAddr(t),
		"OPENAPI_IDENTITY_PATH":       writeContract(t),
	})
	obstest.UseOnlyTheOTelEnvironmentOf(t, env)
	cfg, err := app.FromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("FromEnv() over %s = %v", manifestPath, err)
	}

	runtime, err := boot.StartTelemetry(context.Background(), app.TelemetryOf(cfg))
	if err != nil {
		t.Fatalf("StartTelemetry() = %v", err)
	}
	t.Cleanup(func() {
		grace, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		obstest.ReturnsWithin(t, drainLimit, "the telemetry shutdown", func() { _ = runtime.Shutdown(grace) })
	})
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- app.RunWith(ctx, cfg, runtime) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("RunWith() = %v, want nil on shutdown", err)
			}
		case <-time.After(drainLimit):
			t.Errorf("the drain did not return in %v with the export failing, want the telemetry never to hold it (NF Degradação)", drainLimit)
		}
	})

	var notReady error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		probing, stop := context.WithTimeout(context.Background(), requestLimit)
		notReady = app.Probe(probing, cfg)
		stop()
		if notReady == nil {
			return cfg
		}
	}
	t.Fatalf("Probe() = %v, want the edge ready with the export failing", notReady)
	return cfg
}

func TestAFailingExportNeverFailsNorHoldsARequestOfTheEdge(t *testing.T) {
	for name, collector := range map[string]func(testing.TB) *obstest.DownCollector{
		"unreachable endpoint": obstest.UnreachableCollector,
		"stalled collector":    obstest.StallingCollector,
	} {
		t.Run(name, func(t *testing.T) {
			down := collector(t)
			cfg := startEdgeExportingTo(t, down.Addr)
			client := &http.Client{Timeout: requestLimit}
			t.Cleanup(client.CloseIdleConnections)

			for range cfg.Admission.Burst {
				began := time.Now()
				res, err := client.Get("http://" + cfg.HTTPAddr + api.IdentityContractPath)
				if err != nil {
					t.Fatalf("GET %s = %v after %v with the export failing, want 200 within %v: telemetry never fails nor holds a request (NF Degradação)",
						api.IdentityContractPath, err, time.Since(began).Round(time.Millisecond), requestLimit)
				}
				body, read := io.ReadAll(res.Body)
				_ = res.Body.Close()
				if res.StatusCode != http.StatusOK || read != nil || string(body) != contractBody {
					t.Fatalf("GET %s = %d %q (%v), want 200 with the contract", api.IdentityContractPath, res.StatusCode, body, read)
				}
			}
			down.RequireEverySignalExporting(t)
		})
	}
}
