package app_test

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

const (
	poisonedUser = "poisoned"
	heldUser     = "held"

	probeCredential = `{"sub":"probe","tenant":"acme"}`
)

var routePermissions = []string{"identity:find-effective-permissions", "identity:find-organization"}

func authorized(credential string) []string { return []string{"Authorization", "Bearer " + credential} }

func statusOf(addr, path string, headers []string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set(headers[0], headers[1])
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = res.Body.Close()
	return res.StatusCode, nil
}

func TestARouteThatOpensTheBreakerOfTheIdentityLeavesThePermissionsHopAnswering(t *testing.T) {
	identity := serveIdentity(t)
	identity.answeringTo(func(_ context.Context, user string) (*servicev1.FindEffectivePermissionsResponse, error) {
		if user == poisonedUser {
			return nil, status.Error(codes.Internal, "poison row")
		}
		return &servicev1.FindEffectivePermissionsResponse{Permissions: routePermissions}, nil
	})
	e := startEdge(t, devMock, "true", "IDENTITY_GRPC_TARGET", identity.addr)
	poisoned := "/identity/users/" + poisonedUser + "/permissions"

	opened := false
	for range 60 {
		reached := identity.askedFor(poisonedUser)
		e.request(t, http.MethodGet, poisoned, authorized(claimingCredential)...)
		if identity.askedFor(poisonedUser) == reached {
			opened = true
			break
		}
	}
	if !opened {
		t.Fatal("the breaker of the route connection never opened")
	}

	hops := identity.askedFor("tester")
	answered := e.request(t, http.MethodGet, poisoned, authorized(claimingCredential)...)

	if identity.askedFor("tester") != hops+1 {
		t.Fatalf("the permissions hop never reached the identity while the route breaker was open (answered %d)", answered)
	}
	if answered != http.StatusServiceUnavailable {
		t.Fatalf("GET %s = %d, want 503 from the open breaker of the route", poisoned, answered)
	}
	e.stop(t)
}

func TestASaturatedRoutePoolLeavesThePermissionsHopAnswering(t *testing.T) {
	identity := serveIdentity(t)
	release := make(chan struct{})
	var held atomic.Int32
	identity.answeringTo(func(ctx context.Context, user string) (*servicev1.FindEffectivePermissionsResponse, error) {
		if user == heldUser {
			held.Add(1)
			select {
			case <-release:
			case <-ctx.Done():
			}
			return &servicev1.FindEffectivePermissionsResponse{}, nil
		}
		return &servicev1.FindEffectivePermissionsResponse{Permissions: routePermissions}, nil
	})
	e := startEdge(t, devMock, "true", "IDENTITY_GRPC_TARGET", identity.addr)

	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if _, err := statusOf(e.addr, "/identity/users/"+heldUser+"/permissions", authorized(claimingCredential)); err != nil {
				t.Errorf("held request = %v", err)
			}
		})
	}
	if !eventually(func() bool { return held.Load() >= 16 }) {
		close(release)
		wg.Wait()
		t.Fatalf("only %d route calls reached the identity, want the 16 of the pool", held.Load())
	}

	answered, err := statusOf(e.addr, "/identity/organizations/org-1", authorized(probeCredential))
	close(release)
	wg.Wait()

	if err != nil {
		t.Fatalf("probe request = %v", err)
	}
	if n := identity.askedFor("probe"); n != 1 {
		t.Fatalf("the permissions hop reached the identity %d times while the route pool was full, want 1 (answered %d)", n, answered)
	}
	if answered != http.StatusServiceUnavailable {
		t.Fatalf("probe = %d, want 503 from the saturated route pool", answered)
	}
	e.stop(t)
}
