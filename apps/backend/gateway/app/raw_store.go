package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/envconfig"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

const (
	envS3Endpoint  = "S3_ENDPOINT"
	envS3Bucket    = "S3_BUCKET"
	envAWSRegion   = "AWS_REGION"
	envS3KMSKeyARN = "S3_KMS_KEY_ARN"
	envS3Insecure  = "S3_INSECURE"
	envS3PathStyle = "S3_PATH_STYLE"
)

// RawStoreEnv is the bucket of the raw payloads, which the api and the
// consumer open: versioned, under Object Lock and with no default retention,
// provisioned outside the context (decision 16). Credentials come from the
// SDK's default chain; Insecure and PathStyle exist for the emulator.
type RawStoreEnv struct {
	Endpoint  string
	Bucket    string
	Region    string
	KMSKeyARN string
	Insecure  bool
	PathStyle bool

	// ReadyAttempts and ReadyInterval bound how long a role waits for the
	// bucket at startup before refusing to start.
	ReadyAttempts int
	ReadyInterval time.Duration
}

func readRawStoreEnv(lookup func(string) string, defaults RawStoreEnv) (RawStoreEnv, error) {
	env := defaults
	env.Endpoint = lookup(envS3Endpoint)
	env.Bucket = lookup(envS3Bucket)
	env.Region = lookup(envAWSRegion)
	env.KMSKeyARN = lookup(envS3KMSKeyARN)
	var err error
	if env.Insecure, err = envconfig.ParseBool(envS3Insecure, lookup(envS3Insecure)); err != nil {
		return RawStoreEnv{}, err
	}
	if env.PathStyle, err = envconfig.ParseBool(envS3PathStyle, lookup(envS3PathStyle)); err != nil {
		return RawStoreEnv{}, err
	}
	return env, nil
}

func (e RawStoreEnv) Missing() []string {
	var missing []string
	if e.Bucket == "" {
		missing = append(missing, envS3Bucket)
	}
	if e.Region == "" {
		missing = append(missing, envAWSRegion)
	}
	return missing
}

// Validate refuses what the kernel's store would refuse at startup, so the
// refusal names the configuration and not a failed run.
func (e RawStoreEnv) Validate() error {
	if err := e.storeConfig(aws.Config{Region: e.Region}, nil).Validate(); err != nil {
		return fmt.Errorf("%w: raw store: %w", ErrInvalidVariable, err)
	}
	if e.ReadyAttempts < 1 || e.ReadyInterval <= 0 {
		return fmt.Errorf("%w: raw store: ReadyAttempts %d and ReadyInterval %v must be positive", ErrInvalidVariable, e.ReadyAttempts, e.ReadyInterval)
	}
	return nil
}

// Encryption is AES256 unless a KMS key is declared, by its ARN (DAT-08).
func (e RawStoreEnv) Encryption() ports.Encryption {
	if e.KMSKeyARN == "" {
		return ports.Encryption{Algorithm: "AES256"}
	}
	return ports.Encryption{Algorithm: "aws:kms", KeyRef: e.KMSKeyARN}
}

// RawStoreSheet is the RES-21 sheet of the raw store; its deadline bounds each
// Put, which the dispatch budget reserves twice per submission.
func RawStoreSheet() resilience.Sheet { return resilience.Defaults("s3") }

// storeConfig scopes the store to the tenant, because every call runs in the
// platform tenant (decision 1), and declares no ceiling, so no write carries a
// retention until 04:G11.
func (e RawStoreEnv) storeConfig(resolved aws.Config, rt *otelboot.Runtime) s3.Config {
	cfg := s3.Config{
		AWS:                        resolved,
		Bucket:                     e.Bucket,
		Endpoint:                   e.Endpoint,
		InsecureForDevelopmentOnly: e.Insecure,
		UsePathStyle:               e.PathStyle,
		Declaration:                ports.ObjectStoreDeclaration{Scope: ports.ScopeTenant, Encryption: e.Encryption()},
		Clock:                      clock.System(),
		Sheet:                      RawStoreSheet(),
	}
	if rt != nil {
		cfg.Tracer, cfg.Instruments, cfg.LoggerProvider = rt.Tracer(), rt.Instruments(), rt.LoggerProvider()
	}
	return cfg
}

func (e RawStoreEnv) settings() []slog.Attr {
	return []slog.Attr{
		slog.String("s3_endpoint", e.Endpoint),
		slog.String("s3_bucket", e.Bucket),
		slog.String("aws_region", e.Region),
		slog.String("s3_encryption", e.Encryption().Algorithm),
		slog.Bool("s3_insecure", e.Insecure),
		slog.Bool("s3_path_style", e.PathStyle),
	}
}

// OpenRawStore builds the raw store over the kernel's S3 provider and returns
// it only once its bucket is confirmed; the context receives the writer and
// the reader, never retention nor purge.
func OpenRawStore(ctx context.Context, env RawStoreEnv, rt *otelboot.Runtime) (provider.RawStore, error) {
	resolved, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(env.Region))
	if err != nil {
		return provider.RawStore{}, fmt.Errorf("gateway: raw store: aws configuration: %w", err)
	}
	store, err := s3.New(env.storeConfig(resolved, rt))
	if err != nil {
		return provider.RawStore{}, fmt.Errorf("gateway: raw store: %w", err)
	}
	if err := awaitReady(ctx, store, env.ReadyAttempts, env.ReadyInterval); err != nil {
		return provider.RawStore{}, fmt.Errorf("gateway: raw store: %w", err)
	}
	return provider.NewRawStore(store, store), nil
}

type readiness interface {
	Ready(ctx context.Context) error
}

// awaitReady repeats Ready, which never repeats itself, up to attempts. A
// bucket that does not keep versions under lock is refused at once: waiting
// does not version nor lock a bucket, the provisioning does.
func awaitReady(ctx context.Context, store readiness, attempts int, interval time.Duration) error {
	var err error
	for attempt := 1; ; attempt++ {
		if err = store.Ready(ctx); err == nil || errors.Is(err, s3.ErrBucketNotImmutable) || attempt >= attempts {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %w", ctx.Err(), err)
		case <-time.After(interval):
		}
	}
}
