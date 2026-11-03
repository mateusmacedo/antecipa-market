package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
)

// rawStoreVars is the bucket a role that keeps raw payloads needs.
var rawStoreVars = []string{"S3_BUCKET", "gateway-interactions", "AWS_REGION", "us-east-1"}

func TestTheRolesThatKeepRawPayloadsNameTheMissingBucketAndRegion(t *testing.T) {
	roles := map[app.Role][]string{
		app.RoleAPI: {"PG_DSN", "postgres://x", "GRPC_INSECURE", "true"},
		app.RoleConsumer: {
			"PG_DSN", "postgres://x", "KAFKA_BROKERS", "b:9092", "KAFKA_INSECURE", "true", "KAFKA_GROUP", "gateway",
			"KAFKA_PARTNER_TOPIC", "partner.events", "KAFKA_PARTNER_DLQ", "partner.events.dlq",
		},
	}
	for role, base := range roles {
		for i := 0; i < len(rawStoreVars); i += 2 {
			variable := rawStoreVars[i]
			t.Run(string(role)+" without "+variable, func(t *testing.T) {
				pairs := append(append([]string{}, base...), rawStoreVars...)
				pairs = append(pairs, variable, "")

				_, err := app.FromEnv(role, lookup(pairs...))

				if !errors.Is(err, app.ErrMissingVariable) || !strings.Contains(err.Error(), variable) {
					t.Fatalf("FromEnv() = %v, want %s named", err, variable)
				}
			})
		}
	}
}

func TestTheRelayOpensNoRawStore(t *testing.T) {
	cfg := app.Defaults(app.RoleRelay)
	cfg.DSN, cfg.Brokers, cfg.KafkaInsecure, cfg.Group = "postgres://x", []string{"b:9092"}, true, "gateway"
	cfg.GatewayTopic, cfg.GatewayDLQ, cfg.DispatchTopic, cfg.DispatchDLQ = "a", "a.dlq", "b", "b.dlq"
	cfg.InteractionTopic, cfg.InteractionDLQ, cfg.NormalizationTopic, cfg.NormalizationDLQ = "c", "c.dlq", "d", "d.dlq"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want the relay to start without a bucket", err)
	}
}

func TestTheRawStoreDeclaresAES256UnlessAKMSKeyIsGivenByItsARN(t *testing.T) {
	api := []string{"PG_DSN", "postgres://x", "GRPC_INSECURE", "true"}

	cfg, err := app.FromEnv(app.RoleAPI, lookup(append(api, rawStoreVars...)...))
	if err != nil || cfg.RawStore.Encryption() != (port.Encryption{Algorithm: "AES256"}) {
		t.Fatalf("FromEnv() = %+v, %v; want AES256", cfg.RawStore, err)
	}
	arn := "arn:aws:kms:us-east-1:000000000000:key/raw"
	cfg, err = app.FromEnv(app.RoleAPI, lookup(append(append(api, rawStoreVars...), "S3_KMS_KEY_ARN", arn)...))
	if err != nil || cfg.RawStore.Encryption() != (port.Encryption{Algorithm: "aws:kms", KeyRef: arn}) {
		t.Fatalf("FromEnv() = %+v, %v; want aws:kms under the ARN", cfg.RawStore, err)
	}
}

func TestTheRawStoreRefusesWhatTheKernelStoreWouldRefuseAtStartup(t *testing.T) {
	api := append([]string{"PG_DSN", "postgres://x", "GRPC_INSECURE", "true"}, rawStoreVars...)
	for name, tc := range map[string]struct {
		pairs []string
		want  error
	}{
		"path style without an endpoint": {[]string{"S3_PATH_STYLE", "true"}, s3.ErrIncompleteConfig},
		"plaintext without the opt-out":  {[]string{"S3_ENDPOINT", "http://localhost:4566"}, s3.ErrTLSRequired},
		"a KMS key not given by its ARN": {[]string{"S3_KMS_KEY_ARN", "alias/raw"}, s3.ErrIncompleteConfig},
		"an opt-out that is no boolean":  {[]string{"S3_INSECURE", "sometimes"}, app.ErrInvalidVariable},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := app.FromEnv(app.RoleAPI, lookup(append(append([]string{}, api...), tc.pairs...)...))

			if !errors.Is(err, tc.want) || !errors.Is(err, app.ErrInvalidVariable) {
				t.Fatalf("FromEnv() = %v, want %v as an invalid variable", err, tc.want)
			}
		})
	}
	cfg, err := app.FromEnv(app.RoleAPI, lookup(append(api, "S3_ENDPOINT", "http://localhost:4566", "S3_INSECURE", "true", "S3_PATH_STYLE", "true")...))
	if err != nil || !cfg.RawStore.Insecure || !cfg.RawStore.PathStyle {
		t.Fatalf("FromEnv(emulator) = %+v, %v; want the emulator accepted by explicit opt-out", cfg.RawStore, err)
	}
}

type readiness struct {
	answers []error
	calls   int
}

func (r *readiness) Ready(context.Context) error {
	r.calls++
	if r.calls > len(r.answers) {
		return nil
	}
	return r.answers[r.calls-1]
}

func TestTheStartupWaitsABoundedTimeForTheBucket(t *testing.T) {
	unreachable := errors.New("dial tcp: connection refused")
	notLocked := s3.ErrBucketNotImmutable

	for name, tc := range map[string]struct {
		answers []error
		calls   int
		want    error
	}{
		"ready after a transient failure": {[]error{unreachable}, 2, nil},
		"unreachable past the attempts":   {[]error{unreachable, unreachable, unreachable}, 3, unreachable},
		"a bucket that is not immutable":  {[]error{notLocked}, 1, notLocked},
	} {
		t.Run(name, func(t *testing.T) {
			store := &readiness{answers: tc.answers}

			err := app.AwaitReady(context.Background(), store, 3, time.Millisecond)

			if !errors.Is(err, tc.want) || (tc.want == nil && err != nil) || store.calls != tc.calls {
				t.Fatalf("AwaitReady() = %v after %d calls, want %v after %d", err, store.calls, tc.want, tc.calls)
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.AwaitReady(ctx, &readiness{answers: []error{unreachable}}, 3, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("AwaitReady(cancelled) = %v, want the cancellation", err)
	}
}
