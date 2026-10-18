package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

const signalHelper = "BFF_SIGNAL_HELPER"

func lookup(pairs ...string) func(string) string {
	values := map[string]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values[pairs[i]] = pairs[i+1]
	}
	return func(name string) string { return values[name] }
}

const sentinelPassword = "sentinel-redis-password"

func TestRunRefusesToStartWithoutConfiguration(t *testing.T) {
	cases := []struct {
		name  string
		env   func(string) string
		named string
	}{
		{"identity target", lookup("GRPC_INSECURE", "true", "AUTH_DEV_MOCK", "true", "REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0"), "IDENTITY_GRPC_TARGET"},
		{"transport policy", lookup("IDENTITY_GRPC_TARGET", "i:9090", "AUTH_DEV_MOCK", "true", "REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0"), "GRPC_CA_FILE"},
		{"revocation store with the mock", lookup("IDENTITY_GRPC_TARGET", "i:9090", "GRPC_INSECURE", "true", "AUTH_DEV_MOCK", "true"), "REDIS_URL"},
		{"mock alongside an issuer", lookup("IDENTITY_GRPC_TARGET", "i:9090", "GRPC_INSECURE", "true", "AUTH_DEV_MOCK", "true",
			"OIDC_ISSUER", "http://keycloak.localhost:8180/realms/dmpf", "REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0"), "AUTH_DEV_MOCK"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errOut bytes.Buffer

			code := run(options{lookup: tc.env}, &errOut)

			if code != exitUsage {
				t.Fatalf("run() = %d, want %d (usage)", code, exitUsage)
			}
			if !strings.Contains(errOut.String(), tc.named) {
				t.Fatalf("stderr = %q, want %s named", errOut.String(), tc.named)
			}
		})
	}
}

func TestRunRefusesAnUnusableRevocationStoreWithoutEchoingIt(t *testing.T) {
	for name, url := range map[string]string{
		"not a redis URL":          "http://revocation:" + sentinelPassword + "@redis:6379/0",
		"database other than zero": "redis://revocation:" + sentinelPassword + "@redis:6379/1",
	} {
		t.Run(name, func(t *testing.T) {
			var errOut bytes.Buffer

			code := run(options{lookup: lookup("IDENTITY_GRPC_TARGET", "i:9090", "GRPC_INSECURE", "true", "AUTH_DEV_MOCK", "true", "REDIS_URL", url)}, &errOut)

			if code != exitUsage || !strings.Contains(errOut.String(), "REDIS_URL") {
				t.Fatalf("run() = %d with stderr %q, want %d naming REDIS_URL", code, errOut.String(), exitUsage)
			}
			if strings.Contains(errOut.String(), sentinelPassword) {
				t.Fatalf("stderr = %q, want the URL left out", errOut.String())
			}
		})
	}
}

func TestHealthcheckAnswersOnlyHealthyOrUnhealthy(t *testing.T) {
	edge := func(code int) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }))
		t.Cleanup(srv.Close)
		return srv.Listener.Addr().String()
	}
	configured := func(addr string) func(string) string {
		return lookup("GRPC_INSECURE", "true", "AUTH_DEV_MOCK", "true", "ADMIN_ADDR", addr,
			"IDENTITY_GRPC_TARGET", "i:9090", "REDIS_URL", "redis://revocation:secret@127.0.0.1:1/0")
	}
	cases := []struct {
		name string
		env  func(string) string
		want int
	}{
		{"edge ready", configured(edge(http.StatusNoContent)), exitOK},
		{"edge not ready", configured(edge(http.StatusServiceUnavailable)), exitFailure},
		{"no configuration", lookup(), exitFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var errOut bytes.Buffer

			if code := run(options{lookup: tc.env, args: []string{"healthcheck"}}, &errOut); code != tc.want {
				t.Fatalf("run(healthcheck) = %d, want %d (stderr %q)", code, tc.want, errOut.String())
			}
		})
	}
}

func TestSignalHelperProcess(t *testing.T) {
	if os.Getenv(signalHelper) != "1" {
		t.Skip("runs only as the child of TestASecondSignalEndsADrainThatNeverFinishes")
	}
	os.Exit(serve(os.Stderr, func(ctx context.Context) error {
		fmt.Println("serving")
		<-ctx.Done()
		fmt.Println("draining")
		time.Sleep(time.Hour)
		return nil
	}))
}

func TestASecondSignalEndsADrainThatNeverFinishes(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSignalHelperProcess$", "-test.count=1", "-test.timeout=30s")
	child.Env = append(os.Environ(), signalHelper+"=1")
	out, err := child.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() = %v", err)
	}
	if err := child.Start(); err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() { _ = child.Process.Kill() })
	printed := make(chan string, 8)
	go func() {
		lines := bufio.NewScanner(out)
		for lines.Scan() {
			printed <- lines.Text()
		}
		close(printed)
	}()
	await := func(want string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case line, open := <-printed:
				if !open {
					t.Fatalf("the child exited before printing %q", want)
				}
				if line == want {
					return
				}
			case <-deadline:
				t.Fatalf("the child never printed %q", want)
			}
		}
	}

	await("serving")
	if err := child.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("first SIGTERM = %v", err)
	}
	await("draining")
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()
	for deadline := time.After(5 * time.Second); ; {
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-exited:
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
				t.Fatalf("child ended with %v, want it killed by the second SIGTERM", err)
			}
			return
		case <-deadline:
			t.Fatal("the child kept draining after a second SIGTERM")
		case <-time.After(100 * time.Millisecond):
		}
	}
}
