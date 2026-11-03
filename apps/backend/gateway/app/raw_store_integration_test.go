//go:build integration

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"go.opentelemetry.io/otel/propagation"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// bucketShape is how a test bucket is born: Object Lock is granted only at
// creation, and it turns versioning on.
type bucketShape int

const (
	lockedBucket bucketShape = iota
	plainBucket
	versionedBucket
	defaultRetentionBucket
)

var bucketSequence atomic.Int64

func testRegion() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	return "us-east-1"
}

// createBucket makes a bucket of its own for the test in the emulator of
// S3_ENDPOINT, the way the provisioning outside the context would.
func createBucket(t *testing.T, shape bucketShape) string {
	t.Helper()
	endpoint := tb.Env(t, "S3_ENDPOINT")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resolved, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(testRegion()))
	if err != nil {
		t.Fatalf("LoadDefaultConfig() = %v", err)
	}
	client := awss3.NewFromConfig(resolved, func(o *awss3.Options) {
		o.BaseEndpoint, o.UsePathStyle = &endpoint, true
	})
	name := fmt.Sprintf("gateway-it-%d-%d", time.Now().UnixNano()%1_000_000_000_000, bucketSequence.Add(1))
	in := &awss3.CreateBucketInput{Bucket: &name}
	if shape == lockedBucket || shape == defaultRetentionBucket {
		in.ObjectLockEnabledForBucket = aws.Bool(true)
	}
	if _, err := client.CreateBucket(ctx, in); err != nil {
		t.Fatalf("CreateBucket(%s) = %v", name, err)
	}
	switch shape {
	case versionedBucket:
		if _, err := client.PutBucketVersioning(ctx, &awss3.PutBucketVersioningInput{
			Bucket: &name, VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatusEnabled},
		}); err != nil {
			t.Fatalf("PutBucketVersioning(%s) = %v", name, err)
		}
	case defaultRetentionBucket:
		if _, err := client.PutObjectLockConfiguration(ctx, &awss3.PutObjectLockConfigurationInput{
			Bucket: &name, ObjectLockConfiguration: &types.ObjectLockConfiguration{
				ObjectLockEnabled: types.ObjectLockEnabledEnabled,
				Rule: &types.ObjectLockRule{DefaultRetention: &types.DefaultRetention{
					Mode: types.ObjectLockRetentionModeGovernance, Days: aws.Int32(1),
				}},
			},
		}); err != nil {
			t.Fatalf("PutObjectLockConfiguration(%s) = %v", name, err)
		}
	}
	return name
}

// testRawStoreEnv points a role at a bucket of the emulator, with the short
// wait a suite affords.
func testRawStoreEnv(t *testing.T, bucket string) RawStoreEnv {
	t.Helper()
	endpoint := tb.Env(t, "S3_ENDPOINT")
	return RawStoreEnv{
		Endpoint: endpoint, Bucket: bucket, Region: testRegion(),
		Insecure: !strings.HasPrefix(endpoint, "https://"), PathStyle: true,
		ReadyAttempts: 2, ReadyInterval: 100 * time.Millisecond,
	}
}

func TestTheRawStoreOfARoleKeepsEachWriteAsAVersionOfTheBucket(t *testing.T) {
	env := testRawStoreEnv(t, createBucket(t, lockedBucket))
	ctx := execctx.WithExecution(t, context.Background(), execctx.WithSlot(), execctx.AutoKey(), execctx.WithTenant("platform"))
	store, err := OpenRawStore(ctx, env, nil)
	if err != nil {
		t.Fatalf("OpenRawStore() = %v", err)
	}
	key := ports.RawKeyOf("prt-1:production", "int-1", ports.RawRequest)

	first, err := store.Put(ctx, key, ports.RawBody{ContentType: "application/json", Body: []byte(`{"attempt":1}`)})
	if err != nil {
		t.Fatalf("Put(first) = %v", err)
	}
	second, err := store.Put(ctx, key, ports.RawBody{ContentType: "application/json", Body: []byte(`{"attempt":2}`)})
	if err != nil {
		t.Fatalf("Put(second) = %v", err)
	}

	if first.Ref != key || first.VersionID == "" || first.VersionID == second.VersionID || len(first.Hash) != 64 || first.Hash == second.Hash {
		t.Fatalf("Put() = %+v then %+v; want the key, two versions and the SHA-256 of each body", first, second)
	}
	for payload, want := range map[domain.RawPayload]string{first: `{"attempt":1}`, second: `{"attempt":2}`} {
		got, err := store.Get(ctx, payload)
		if err != nil || string(got.Body) != want || got.ContentType != "application/json" {
			t.Fatalf("Get(%s) = %+v, %v; want %s", payload.VersionID, got, err, want)
		}
	}
	tampered := domain.RawPayload{Ref: first.Ref, VersionID: first.VersionID, Hash: second.Hash}
	if _, err := store.Get(ctx, tampered); !errors.Is(err, ports.ErrRawPayloadDefect) {
		t.Fatalf("Get(another hash) = %v, want ErrRawPayloadDefect", err)
	}
}

// A bucket that does not keep the versions under lock, or that would retain
// the writes the port declares free, refuses the start of both roles.
func TestARoleRefusesToStartOverABucketThatIsNotImmutable(t *testing.T) {
	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Disabled: true, Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{ServiceName: "gateway", ServiceVersion: "dev", ServiceInstanceID: "gateway-raw-1", Role: string(RoleAPI)},
	})
	if err != nil {
		t.Fatalf("otelboot.Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	shapes := map[string]bucketShape{
		"without versioning":       plainBucket,
		"versioned without lock":   versionedBucket,
		"with a default retention": defaultRetentionBucket,
	}
	for name, shape := range shapes {
		bucket := createBucket(t, shape)
		for _, role := range []Role{RoleAPI, RoleConsumer} {
			t.Run(string(role)+" "+name, func(t *testing.T) {
				cfg := Defaults(role)
				cfg.RawStore = testRawStoreEnv(t, bucket)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				err := RunWith(ctx, cfg, runtime)

				if !errors.Is(err, s3.ErrBucketNotImmutable) {
					t.Fatalf("RunWith() = %v, want the start refused with ErrBucketNotImmutable", err)
				}
			})
		}
	}

	t.Run("a bucket that was never provisioned", func(t *testing.T) {
		cfg := Defaults(RoleAPI)
		cfg.RawStore = testRawStoreEnv(t, fmt.Sprintf("gateway-it-absent-%d", bucketSequence.Add(1)))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := RunWith(ctx, cfg, runtime); err == nil || ctx.Err() != nil {
			t.Fatalf("RunWith() = %v, want the start refused after the bounded wait", err)
		}
	})
}
