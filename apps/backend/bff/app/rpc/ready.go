package rpc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var ErrNotReady = errors.New("rpc: contexts not ready")

type Readiness map[string]*grpc.ClientConn

func (r Readiness) Check(ctx context.Context) error {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		notReady []string
	)
	for name, conn := range r {
		wg.Go(func() {
			if !ready(ctx, conn) {
				mu.Lock()
				notReady = append(notReady, name)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if len(notReady) == 0 {
		return nil
	}
	slices.Sort(notReady)
	return fmt.Errorf("%w: %s", ErrNotReady, strings.Join(notReady, ", "))
}

// With the health check of the service config (GRP-13), round_robin reports
// READY only once a backend answers SERVING; calling Health/Check directly
// would be refused by the method policy, which declares no such method (GRP-16).
func ready(ctx context.Context, conn *grpc.ClientConn) bool {
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return true
		case connectivity.Shutdown, connectivity.TransientFailure:
			return false
		case connectivity.Idle:
			conn.Connect()
		}
		if !conn.WaitForStateChange(ctx, state) {
			return false
		}
	}
}
