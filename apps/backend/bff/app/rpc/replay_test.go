package rpc_test

import (
	"context"
	"slices"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/rpc"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func replayedHeader(value string) metadata.MD {
	return metadata.Pairs(kernelgrpc.ReplayedHeader, value)
}

func onAttempt(per map[int]metadata.MD) func(int) metadata.MD {
	return func(n int) metadata.MD { return per[n] }
}

func failingFirst(code codes.Code) func(int) (proto.Message, error) {
	return func(n int) (proto.Message, error) {
		if n == 1 {
			return nil, status.Error(code, "refused")
		}
		return &servicev1.FindOrganizationResponse{}, nil
	}
}

func TestTheReplayHeaderMarksTheRequestOnlyOnASuccessfulAnswer(t *testing.T) {
	cases := map[string]struct {
		header metadata.MD
		code   codes.Code
		slot   bool
		want   bool
	}{
		"header true":           {header: replayedHeader("true"), slot: true, want: true},
		"no header":             {slot: true},
		"another value":         {header: replayedHeader("TRUE"), slot: true},
		"repeated header":       {header: metadata.Pairs(kernelgrpc.ReplayedHeader, "true", kernelgrpc.ReplayedHeader, "true"), slot: true},
		"no slot":               {header: replayedHeader("true")},
		"header with a failure": {header: replayedHeader("true"), code: codes.NotFound, slot: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := (&fakeIdentity{}).header("FindOrganization", onAttempt(map[int]metadata.MD{1: tc.header}))
			if tc.code != codes.OK {
				fake.answer("FindOrganization", func(int) (proto.Message, error) { return nil, status.Error(tc.code, "refused") })
			}
			identity := identityClient(t, fake)
			ctx := withDeadline(t, 2*time.Second)
			if tc.slot {
				ctx = rpc.WithReplaySlot(ctx)
			}

			_, err := identity.FindOrganization(ctx, &servicev1.FindOrganizationRequest{OrganizationId: "org-1"})

			if status.Code(err) != tc.code {
				t.Fatalf("FindOrganization() = %v, want %s", err, tc.code)
			}
			if got := rpc.Replayed(ctx); got != tc.want {
				t.Fatalf("Replayed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestARetriedCommandSendsTheSameKeyAndReflectsOnlyTheFinalAttempt(t *testing.T) {
	cases := map[string]struct {
		headers map[int]metadata.MD
		want    bool
	}{
		"no replay on either attempt":         {headers: nil, want: false},
		"replay only on the failed attempt":   {headers: map[int]metadata.MD{1: replayedHeader("true")}, want: false},
		"replay only on the answered attempt": {headers: map[int]metadata.MD{2: replayedHeader("true")}, want: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := (&fakeIdentity{}).answer("FindOrganization", failingFirst(codes.Unavailable)).header("FindOrganization", onAttempt(tc.headers))
			identity := identityClient(t, fake)
			ctx := rpc.WithReplaySlot(rpc.WithCall(retry.WithBudget(withDeadline(t, 2*time.Second)), rpc.Call{IdempotencyKey: "derived-1"}))

			if _, err := identity.FindOrganization(ctx, &servicev1.FindOrganizationRequest{OrganizationId: "org-1"}); err != nil {
				t.Fatalf("FindOrganization() = %v, want success after one retry", err)
			}

			calls := fake.callsTo("FindOrganization")
			if len(calls) != 2 {
				t.Fatalf("FindOrganization reached the identity %d times, want 2", len(calls))
			}
			for i, call := range calls {
				if got := call.md.Get(kernelgrpc.IdempotencyKey); !slices.Equal(got, []string{"derived-1"}) {
					t.Fatalf("attempt %d carried %s = %v, want the same derived key on every attempt (GRP-09)", i+1, kernelgrpc.IdempotencyKey, got)
				}
			}
			if got := rpc.Replayed(ctx); got != tc.want {
				t.Fatalf("Replayed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReplayedIsFalseOnABareContext(t *testing.T) {
	if rpc.Replayed(context.Background()) || rpc.Replayed(rpc.WithReplaySlot(context.Background())) {
		t.Fatal("Replayed() = true before any answer was marked")
	}
}
