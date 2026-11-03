package provider_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

const rawTenant = port.TenantID("platform-tenant")

var rawKey = ports.RawKeyOf("prt-1:production", "int-1", ports.RawResponse)

func rawContext(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithSlot(), execctx.AutoKey(), execctx.WithTenant(rawTenant))
}

func memoryRawStore(t *testing.T) (provider.RawStore, *memory.ObjectBucket) {
	t.Helper()
	bucket := memory.NewObjectBucket(memory.FixedClock{At: 1_700_000_000_000_000_000})
	store, err := memory.NewObjectStore(bucket, port.ObjectStoreDeclaration{Scope: port.ScopeTenant, Encryption: port.Encryption{Algorithm: "AES256"}})
	if err != nil {
		t.Fatalf("NewObjectStore() = %v", err)
	}
	return provider.NewRawStore(store, store), bucket
}

func sha(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestTheRawStoreKeepsEachWriteAsAVersionReadBackByItsReference(t *testing.T) {
	store, bucket := memoryRawStore(t)
	ctx := rawContext(t)
	first, second := []byte(`{"status":202}`), []byte(`{"status":200}`)

	one, err := store.Put(ctx, rawKey, ports.RawBody{ContentType: "application/json", Body: first})
	if err != nil {
		t.Fatalf("Put(first) = %v", err)
	}
	two, err := store.Put(ctx, rawKey, ports.RawBody{ContentType: "application/json", Body: second})
	if err != nil {
		t.Fatalf("Put(second) = %v", err)
	}

	if one.Ref != rawKey || one.Hash != sha(first) || one.VersionID == "" || one.VersionID == two.VersionID {
		t.Fatalf("Put() = %+v then %+v; want the key, a version of its own and the SHA-256 of the body", one, two)
	}
	for payload, want := range map[domain.RawPayload][]byte{one: first, two: second} {
		got, err := store.Get(ctx, payload)
		if err != nil || string(got.Body) != string(want) || got.ContentType != "application/json" {
			t.Fatalf("Get(%s) = %+v, %v; want %s", payload.VersionID, got, err, want)
		}
	}
	if keys := bucket.PhysicalKeys(); !slices.Equal(keys, []string{"tenant/" + string(rawTenant) + "/" + rawKey}) {
		t.Fatalf("physical keys = %v, want the raw key under the tenant of the call", keys)
	}
}

func TestTheRawStoreTellsADefectFromAnUnavailableStore(t *testing.T) {
	store, _ := memoryRawStore(t)
	ctx := rawContext(t)
	kept, err := store.Put(ctx, rawKey, ports.RawBody{ContentType: "application/json", Body: []byte(`{}`)})
	if err != nil {
		t.Fatalf("Put() = %v", err)
	}

	for name, call := range map[string]func() error{
		"another content under the recorded hash": func() error {
			_, err := store.Get(ctx, domain.RawPayload{Ref: kept.Ref, VersionID: kept.VersionID, Hash: sha([]byte("other"))})
			return err
		},
		"a version that is not there": func() error {
			_, err := store.Get(ctx, domain.RawPayload{Ref: kept.Ref, VersionID: "missing", Hash: kept.Hash})
			return err
		},
		"a key that escapes its prefix": func() error {
			_, err := store.Put(ctx, "../escape", ports.RawBody{Body: []byte(`{}`)})
			return err
		},
		"a call outside any execution": func() error {
			_, err := store.Put(context.Background(), rawKey, ports.RawBody{Body: []byte(`{}`)})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ports.ErrRawPayloadDefect) || errors.Is(err, ports.ErrRawStoreUnavailable) {
				t.Fatalf("err = %v, want ErrRawPayloadDefect only", err)
			}
		})
	}

	down := errors.New("connection refused")
	broken := provider.NewRawStore(failingStore{err: down}, failingStore{err: down})
	if _, err := broken.Put(ctx, rawKey, ports.RawBody{Body: []byte(`{}`)}); !errors.Is(err, ports.ErrRawStoreUnavailable) || !errors.Is(err, down) {
		t.Fatalf("Put() on a store down = %v, want ErrRawStoreUnavailable keeping the cause", err)
	}
	if _, err := broken.Get(ctx, kept); !errors.Is(err, ports.ErrRawStoreUnavailable) {
		t.Fatalf("Get() on a store down = %v, want ErrRawStoreUnavailable", err)
	}
}

type failingStore struct{ err error }

func (f failingStore) Put(context.Context, port.PutRequest) (port.ObjectVersion, error) {
	return port.ObjectVersion{}, f.err
}

func (f failingStore) Get(context.Context, port.ObjectRef) (port.Object, error) {
	return port.Object{}, f.err
}

func (f failingStore) Head(context.Context, port.ObjectRef) (port.ObjectInfo, error) {
	return port.ObjectInfo{}, f.err
}
