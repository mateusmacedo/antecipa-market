package rpc

import (
	"context"
	"slices"
	"sync/atomic"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"google.golang.org/grpc/metadata"
)

type replayKey struct{}

func WithReplaySlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, replayKey{}, new(atomic.Bool))
}

func Replayed(ctx context.Context) bool {
	slot, ok := ctx.Value(replayKey{}).(*atomic.Bool)
	return ok && slot.Load()
}

func markReplayed(ctx context.Context, header metadata.MD) {
	if slot, ok := ctx.Value(replayKey{}).(*atomic.Bool); ok && slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		slot.Store(true)
	}
}
