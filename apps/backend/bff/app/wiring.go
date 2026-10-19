package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	obsclock "github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/admission"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	maxHeaderBytes    = 1 << 16

	keyDrainDelay = "dmpf.drain.delay"

	identityContext    = "identity"
	commandsContext    = "identity-commands"
	permissionsContext = "identity-permissions"
)

func Run(ctx context.Context, cfg Config) error {
	cfg, closeRevocations, err := WithRevocations(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeRevocations() }()
	return boot.Boot(ctx, TelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunWith(ctx, cfg, rt)
	})
}

// WHY: Run calls this before boot.Boot so "process configured" reports the
// deny-list it serves with; redis.NewClient does not reach the server.
func WithRevocations(cfg Config) (Config, func() error, error) {
	if cfg.Auth.Revocations != nil {
		return cfg, func() error { return nil }, nil
	}
	client, err := redis.NewClient(cfg.Redis)
	if err != nil {
		return cfg, nil, err
	}
	cfg.Auth.Revocations = redis.NewRevocationStore(client, obsclock.System())
	return cfg, client.Close, nil
}

// Authenticator resolves how this process verifies identity. The development
// mock is only reachable through the opt-out the config already refused to
// combine with an issuer, so one start never has two ways of resolving it.
func Authenticator(ctx context.Context, cfg Config, identity rpc.Identity) (api.Authenticator, error) {
	if cfg.Auth.DevMock {
		return NewPermissions(authn.DevAuthenticator{}, identity), nil
	}
	verifier, err := authn.NewVerifier(ctx, cfg.Auth)
	if err != nil {
		return nil, err
	}
	return NewPermissions(verifier, identity), nil
}

func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	logger := rt.LoggerFor(reflect.TypeFor[Config]().PkgPath())
	opts, err := ClientOptions(cfg)
	if err != nil {
		return err
	}
	opts.Tracer, opts.MeterProvider = rt.Tracer(), rt.MeterProvider()
	opts.Instruments, opts.LoggerProvider = rt.Instruments(), rt.LoggerProvider()
	conn, err := rpc.Dial(cfg.IdentityTarget, rpc.IdentityConfig(opts))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	commandsConn, err := rpc.Dial(cfg.IdentityTarget, rpc.CommandsConfig(opts))
	if err != nil {
		return err
	}
	defer func() { _ = commandsConn.Close() }()
	permissionsConn, err := rpc.Dial(cfg.IdentityTarget, rpc.PermissionsConfig(opts))
	if err != nil {
		return err
	}
	defer func() { _ = permissionsConn.Close() }()

	ctrl, err := admissionFor(cfg)
	if err != nil {
		return err
	}
	cfg, closeRevocations, err := WithRevocations(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeRevocations() }()
	cfg.Auth.LoggerProvider = rt.LoggerProvider()
	cfg.Auth.MeterProvider = rt.MeterProvider()
	identity := rpc.NewIdentity(conn, commandsConn)
	authenticator, err := Authenticator(ctx, cfg, rpc.NewIdentity(permissionsConn, permissionsConn))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	var draining atomic.Bool
	options := api.Options{
		Budget:        cfg.RouteBudget,
		Authenticator: authenticator,
		CORSOrigins:   cfg.CORSOrigins,
		Logger:        rt.LoggerFor(reflect.TypeFor[api.Options]().PkgPath()),
		Ready:         rpc.Readiness{identityContext: conn, commandsContext: commandsConn, permissionsContext: permissionsConn}.Check,
		Draining:      draining.Load,
	}
	if options.IdentityContract, err = readContract(cfg.IdentityContractPath); err != nil {
		return err
	}
	handler, err := api.NewHandler(identity, ctrl, rt.Instruments(), options)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	adminListener, err := net.Listen("tcp", cfg.AdminAddr)
	if err != nil {
		_ = listener.Close()
		return err
	}
	server, admin := newServer(handler, rt), newServer(api.NewAdminHandler(options), rt)
	failed := make(chan error, 2)
	go func() { failed <- server.Serve(listener) }()
	go func() { failed <- admin.Serve(adminListener) }()
	logger.LogAttrs(ctx, slog.LevelInfo, "http listening", serverAddress(listener.Addr())...)
	logger.LogAttrs(ctx, slog.LevelInfo, "admin listening", serverAddress(adminListener.Addr())...)

	select {
	case <-ctx.Done():
		draining.Store(true)
		if cfg.DrainDelay > 0 {
			logger.InfoContext(ctx, "http draining", keyDrainDelay, cfg.DrainDelay.String())
			time.Sleep(cfg.DrainDelay)
		}
		grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), observability.ShutdownGrace)
		defer cancel()
		err := server.Shutdown(grace)
		return errors.Join(err, admin.Shutdown(grace))
	case err := <-failed:
		_, _ = server.Close(), admin.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// admissionFor builds no controller while the edge mounts no route: the
// kernel refuses an empty limit map (admission.ErrNoRoutes).
func admissionFor(cfg Config) (*admission.Controller, error) {
	limits := api.Limits(cfg.Admission)
	if len(limits) == 0 {
		return nil, nil
	}
	return admission.NewController(limits, cfg.MetricTenants, admission.DefaultMaxKeys)
}

func ClientOptions(cfg Config) (rpc.Options, error) {
	opts := rpc.Options{Clock: obsclock.System()}
	if cfg.CAFile == "" {
		if !cfg.GRPCInsecure {
			return rpc.Options{}, ErrInsecureNotDeclared
		}
		opts.Insecure = true
		return opts, nil
	}
	clientTLS, err := rpc.ClientTLS(cfg.CAFile, cfg.ServerName, cfg.ClientCertFile, cfg.ClientKeyFile)
	if err != nil {
		return rpc.Options{}, err
	}
	opts.TLS = clientTLS
	return opts, nil
}

func newServer(handler http.Handler, rt *otelboot.Runtime) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		// WHY: a nil ErrorLog sends net/http's own errors to the default logger
		// on stderr, around the redacting handler this process installs.
		ErrorLog: serverErrorLog(rt),
	}
}

var serverStatement = regexp.MustCompile(`^(?:[a-z][a-z0-9]*: )*(?:[A-Za-z][A-Za-z_., -]*)?`)

func serverErrorLog(rt *otelboot.Runtime) *log.Logger {
	return log.New(serverErrors{logger: rt.LoggerFor(reflect.TypeFor[http.Server]().PkgPath())}, "", 0)
}

type serverErrors struct{ logger *slog.Logger }

func (s serverErrors) Write(message []byte) (int, error) {
	s.logger.LogAttrs(context.Background(), slog.LevelWarn, redact.WithoutValues(string(message), serverStatement))
	return len(message), nil
}

func serverAddress(addr net.Addr) []slog.Attr {
	host, port, err := net.SplitHostPort(addr.String())
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil {
		return []slog.Attr{slog.String(string(semconv.ServerAddressKey), addr.String())}
	}
	return []slog.Attr{slog.String(string(semconv.ServerAddressKey), host), slog.Int(string(semconv.ServerPortKey), number)}
}

func readContract(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("openapi: %w", err)
	}
	if len(document) == 0 {
		return nil, fmt.Errorf("openapi: %s is empty", path)
	}
	return document, nil
}
