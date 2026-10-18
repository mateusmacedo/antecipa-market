// Command bff runs the public REST edge of the workspace, configured only
// through the environment.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

const (
	healthcheckCommand = "healthcheck"
	probeTimeout       = 3 * time.Second
)

func main() {
	os.Exit(run(options{lookup: os.Getenv, args: os.Args[1:]}, os.Stderr))
}

type options struct {
	lookup func(string) string
	args   []string
}

func run(o options, errOut io.Writer) int {
	if len(o.args) > 0 && o.args[0] == healthcheckCommand {
		return healthcheck(o.lookup, errOut)
	}
	cfg, err := app.FromEnv(o.lookup)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "bff: %v\n", err)
		return exitUsage
	}
	return serve(errOut, func(ctx context.Context) error { return app.Run(ctx, cfg) })
}

func serve(errOut io.Writer, start func(context.Context) error) int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	// WHY: stop restores the default disposition (os/signal.NotifyContext), so a
	// second signal ends a drain that would otherwise only stop at SIGKILL.
	context.AfterFunc(ctx, stop)

	if err := start(ctx); err != nil {
		_, _ = fmt.Fprintf(errOut, "bff: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// WHY: Docker's HEALTHCHECK reads only 0 and 1 and reserves 2, so a
// configuration the edge could not start with is a failure, not a usage error.
func healthcheck(lookup func(string) string, errOut io.Writer) int {
	cfg, err := app.FromEnv(lookup)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "bff healthcheck: %v\n", err)
		return exitFailure
	}
	return probe(cfg.AdminAddr, errOut)
}

func probe(adminAddr string, errOut io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	if err := app.Probe(ctx, app.Config{AdminAddr: adminAddr}); err != nil {
		_, _ = fmt.Fprintf(errOut, "bff healthcheck: %v\n", err)
		return exitFailure
	}
	return exitOK
}
