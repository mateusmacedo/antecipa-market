package app_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/redis"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app"
)

type injectedChecker struct{}

func (injectedChecker) Revoked(context.Context, authn.SessionRef) (authn.RevocationKind, error) {
	return "", nil
}

func TestWithRevocationsInstallsTheRedisDenyList(t *testing.T) {
	cfg, closeRevocations, err := app.WithRevocations(configWithSecrets())
	if err != nil {
		t.Fatalf("WithRevocations() = %v", err)
	}
	t.Cleanup(func() { _ = closeRevocations() })

	if _, installed := cfg.Auth.Revocations.(*redis.RevocationStore); !installed {
		t.Fatalf("cfg.Auth.Revocations = %T, want the redis deny-list", cfg.Auth.Revocations)
	}
}

func TestWithRevocationsPreservesAnInjectedChecker(t *testing.T) {
	cfg := configWithSecrets()
	cfg.Auth.Revocations = injectedChecker{}

	got, closeRevocations, err := app.WithRevocations(cfg)
	if err != nil {
		t.Fatalf("WithRevocations() = %v", err)
	}
	if err := closeRevocations(); err != nil {
		t.Fatalf("close = %v, want nothing to close for a checker it did not open", err)
	}

	if _, kept := got.Auth.Revocations.(injectedChecker); !kept {
		t.Fatalf("cfg.Auth.Revocations = %T, want the injected checker kept", got.Auth.Revocations)
	}
}

func TestWithRevocationsIsIdempotent(t *testing.T) {
	first, closeFirst, err := app.WithRevocations(configWithSecrets())
	if err != nil {
		t.Fatalf("WithRevocations() = %v", err)
	}
	t.Cleanup(func() { _ = closeFirst() })

	second, closeSecond, err := app.WithRevocations(first)
	if err != nil {
		t.Fatalf("WithRevocations() again = %v", err)
	}
	if err := closeSecond(); err != nil {
		t.Fatalf("second close = %v, want nothing to close", err)
	}

	if second.Auth.Revocations != first.Auth.Revocations {
		t.Fatal("WithRevocations() again replaced the checker, want the one installed first")
	}
}
