//go:build integration

package app_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

const (
	e2eTimeout = 90 * time.Second
	pollEvery  = 100 * time.Millisecond

	portAttempts = 3

	modulePrefix = "github.com/mateusmacedo/antecipa-market/apps/backend/"

	identityContractFile = "apps/backend/identity/contract/openapi/v1/openapi.yaml"
)

type topology struct {
	root string

	bffAddr   string
	adminAddr string

	processes []*dist.Process
	pick      func(*testing.T) string
}

type binaries struct{ bff, identity string }

func buildBinaries(t *testing.T) binaries {
	t.Helper()
	root := workspaceRoot(t)
	dir := t.TempDir()
	out := binaries{bff: filepath.Join(dir, "bff"), identity: filepath.Join(dir, "identity")}
	for path, pkg := range map[string]string{
		out.bff:      modulePrefix + "bff/cmd",
		out.identity: modulePrefix + "identity/cmd",
	} {
		cmd := exec.Command("go", "build", "-race", "-o", path, pkg)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build -race %s: %v\n%s", pkg, err, output)
		}
	}
	return out
}

func workspaceRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOWORK").Output()
	if err != nil {
		t.Fatalf("go env GOWORK: %v", err)
	}
	work := strings.TrimSpace(string(out))
	if work == "" || work == "off" {
		t.Fatal("the e2e builds the bff and the identity through go.work; GOWORK is unset")
	}
	return filepath.Dir(work)
}

func (top *topology) start(t *testing.T, name, binary string, env map[string]string, args ...string) *dist.Process {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var p *dist.Process
	t.Cleanup(func() {
		if t.Failed() && p != nil {
			t.Logf("%s logs:\n%s", name, p.Output())
		}
	})
	p = dist.Supervise(t, name, cmd)
	top.processes = append(top.processes, p)
	return p
}

func (top *topology) exited() *dist.Process {
	for _, p := range top.processes {
		select {
		case <-p.Done():
			return p
		default:
		}
	}
	return nil
}

func (top *topology) await(what string, ready func() error) error {
	var last error
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		if last = ready(); last == nil {
			return nil
		}
		if p := top.exited(); p != nil {
			return fmt.Errorf("%s exited (%v) before %s\n%s", p.Name(), p.Err(), what, p.Output())
		}
		time.Sleep(pollEvery)
	}
	return fmt.Errorf("%s did not happen within %v: %v", what, e2eTimeout, last)
}

func (top *topology) startOnFreePort(t *testing.T, name, binary, addrKey string, env map[string]string, ready func(addr string) error, args ...string) string {
	t.Helper()
	return top.startOnFreePorts(t, name, binary, []string{addrKey}, env, func(addrs map[string]string) error {
		return ready(addrs[addrKey])
	}, args...)[addrKey]
}

func (top *topology) startOnFreePorts(t *testing.T, name, binary string, addrKeys []string, env map[string]string, ready func(addrs map[string]string) error, args ...string) map[string]string {
	t.Helper()
	for attempt := 1; ; attempt++ {
		addrs := map[string]string{}
		for _, key := range addrKeys {
			addrs[key] = top.pickAddr(t)
		}
		p := top.start(t, name, binary, merge(env, addrs), args...)
		err := ready(addrs)
		if err == nil {
			return addrs
		}
		if attempt == portAttempts || !lostItsPort(p) {
			t.Fatal(err)
		}
		top.processes = slices.DeleteFunc(top.processes, func(q *dist.Process) bool { return q == p })
	}
}

func lostItsPort(p *dist.Process) bool {
	select {
	case <-p.Done():
		return strings.Contains(p.Output(), syscall.EADDRINUSE.Error())
	default:
		return false
	}
}

func (top *topology) pickAddr(t *testing.T) string {
	t.Helper()
	if top.pick != nil {
		return top.pick(t)
	}
	return freeAddr(t)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().String()
}

func (top *topology) boot(t *testing.T, bin binaries) {
	t.Helper()
	top.root = workspaceRoot(t)
	quiet := quietTelemetry(t)

	pki := tb.NewPKI(t)
	bffCert, bffKey := pki.Client(t, "bff")
	identityCert, identityKey := pki.Server(t, "identity-api")
	probe := mutualProbe(t, pki.CAFile, bffCert, bffKey)

	identityAddr := top.startOnFreePort(t, "identity api", bin.identity, "GRPC_ADDR", merge(quiet, map[string]string{
		"GRPC_TLS_CERT_FILE": identityCert, "GRPC_TLS_KEY_FILE": identityKey,
		"GRPC_CLIENT_CA_FILE": pki.CAFile, "GRPC_TRUSTED_CLIENTS": tb.Identity("bff"),
		"PG_DSN": pg.DSN(t, "e2e_identity"), "MIGRATE": "true",
		"IDENTITY_IDENTIFIER_HMAC_KEY": "e2e-identity-identifier-hmac-key-0001", "IDENTITY_IDENTIFIER_HMAC_KEY_VERSION": "1",
		"OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-identity-api",
	}), func(addr string) error { return top.waitServing(addr, rpc.IdentityServiceName, probe) }, "--role", "api")

	addrs := top.startOnFreePorts(t, "bff", bin.bff, []string{"HTTP_ADDR", "ADMIN_ADDR"}, merge(quiet, map[string]string{
		"OTEL_RESOURCE_ATTRIBUTES": "service.version=e2e,service.instance.id=e2e-bff",
		"GRPC_CA_FILE":             pki.CAFile, "GRPC_SERVER_NAME": "localhost",
		"GRPC_CLIENT_CERT_FILE": bffCert, "GRPC_CLIENT_KEY_FILE": bffKey,
		"IDENTITY_GRPC_TARGET":  "dns:///" + identityAddr,
		"AUTH_DEV_MOCK":         "true",
		"REDIS_URL":             tb.Env(t, "REDIS_URL"),
		"OPENAPI_IDENTITY_PATH": filepath.Join(top.root, identityContractFile),
	}), func(addrs map[string]string) error {
		return top.await("bff ready", func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			return app.Probe(ctx, app.Config{AdminAddr: addrs["ADMIN_ADDR"]})
		})
	})
	top.bffAddr, top.adminAddr = addrs["HTTP_ADDR"], addrs["ADMIN_ADDR"]
}

func quietTelemetry(t *testing.T) map[string]string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	var accepted atomic.Int64
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			_ = conn.Close()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		if got := accepted.Load(); got != 0 {
			t.Errorf("the topology opened %d connections to the OTLP endpoint, want none with every exporter declared none", got)
		}
	})
	return map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://" + listener.Addr().String(),
		"OTEL_TRACES_EXPORTER":        "none",
		"OTEL_METRICS_EXPORTER":       "none",
		"OTEL_LOGS_EXPORTER":          "none",
	}
}

func (top *topology) waitServing(addr, service string, creds credentials.TransportCredentials) error {
	conn, err := grpc.NewClient("dns:///"+addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("grpc.NewClient(%s) = %w", addr, err)
	}
	defer func() { _ = conn.Close() }()
	client := healthpb.NewHealthClient(conn)
	return top.await(service+" at "+addr+" turning SERVING", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		res, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
		if err == nil && res.GetStatus() != healthpb.HealthCheckResponse_SERVING {
			err = fmt.Errorf("status %v", res.GetStatus())
		}
		return err
	})
}

// WHY: the identity answers nothing, health included, to a workload it does not
// trust (IDN-03), so the probe presents the edge's own certificate.
func mutualProbe(t *testing.T, caFile, certFile, keyFile string) credentials.TransportCredentials {
	t.Helper()
	config, err := rpc.ClientTLS(caFile, "localhost", certFile, keyFile)
	if err != nil {
		t.Fatalf("ClientTLS() = %v", err)
	}
	return credentials.NewTLS(config)
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

var errNotYet = errors.New("the condition does not hold yet")

func TestAProcessThatDiesAtStartFailsTheBootAtOnce(t *testing.T) {
	top := &topology{}
	top.start(t, "dying identity", "/bin/sh", nil, "-c", "echo refused >&2; exit 3")
	began := time.Now()

	err := top.await("the dying identity serving", func() error { return errNotYet })

	if err == nil || !strings.Contains(err.Error(), "dying identity exited") || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("await() = %v, want the death of the process named with its output", err)
	}
	if elapsed := time.Since(began); elapsed > 10*time.Second {
		t.Fatalf("await() took %v, want the death noticed long before the %v timeout", elapsed, e2eTimeout)
	}
}

const (
	holderAddrKey  = "HARNESS_HOLDER_ADDR"
	holderGreeting = "holding"
)

func TestAProcessThatLosesItsPortStartsAgainOnAnother(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	t.Cleanup(func() { _ = taken.Close() })
	picked := false
	top := &topology{pick: func(t *testing.T) string {
		if picked {
			return freeAddr(t)
		}
		picked = true
		return taken.Addr().String()
	}}

	addr := top.startOnFreePort(t, "holder", os.Args[0], holderAddrKey, nil, func(addr string) error {
		return top.await("holder greeting at "+addr, func() error { return greeting(addr) })
	}, "-test.run=^TestHoldingTheAddressItWasGiven$")

	if addr == taken.Addr().String() {
		t.Fatalf("startOnFreePort() = %s, want a port other than the one taken before the bind", addr)
	}
	if p := top.exited(); p != nil {
		t.Fatalf("%s still counts as exited after starting again on %s", p.Name(), addr)
	}
}

func TestHoldingTheAddressItWasGiven(t *testing.T) {
	addr := os.Getenv(holderAddrKey)
	if addr == "" {
		t.Skip("runs only as the process TestAProcessThatLosesItsPortStartsAgainOnAnother starts")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_, _ = io.WriteString(conn, holderGreeting)
			_ = conn.Close()
		}
	}()
	terminated := make(chan os.Signal, 1)
	signal.Notify(terminated, syscall.SIGTERM)
	<-terminated
	_ = listener.Close()
}

func greeting(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	got, err := io.ReadAll(conn)
	if err != nil {
		return err
	}
	if string(got) != holderGreeting {
		return fmt.Errorf("greeting = %q, want %q", got, holderGreeting)
	}
	return nil
}
