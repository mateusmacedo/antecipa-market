package app_test

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

type edgeRequest struct {
	name       string
	method     string
	path       string
	body       string
	headers    map[string]string
	omittedKey string
}

var pathParameter = regexp.MustCompile(`\{[^}]+\}`)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func requestsFor(route kernelhttp.Route) []edgeRequest {
	path := pathParameter.ReplaceAllStringFunc(route.Path, func(string) string { return newUUID() })
	if route.Method == http.MethodGet {
		return []edgeRequest{{name: route.Name, method: route.Method, path: path}}
	}
	header := route.IdempotencyKey
	if header == "" {
		header = api.IdempotencyHeader
	}
	return []edgeRequest{
		{name: route.Name + " with its idempotency key", method: route.Method, path: path, body: "{}",
			headers: map[string]string{header: newUUID(), "Content-Type": "application/json"}},
		{name: route.Name + " without its idempotency key", method: route.Method, path: path, body: "{}",
			headers: map[string]string{"Content-Type": "application/json"}, omittedKey: header},
	}
}

var uuidShape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestTheEdgeRequestsFillEveryPathParameterWithAUUID(t *testing.T) {
	requests := requestsFor(kernelhttp.Route{Name: "findMembership", Method: http.MethodGet, Path: "/identity/organizations/{organizationId}/memberships/{membershipId}"})

	if len(requests) != 1 || requests[0].body != "" || len(requests[0].headers) != 0 {
		t.Fatalf("requestsFor(GET) = %+v, want one bare request", requests)
	}
	segments := strings.Split(requests[0].path, "/")
	if len(segments) != 6 || !uuidShape.MatchString(segments[3]) || !uuidShape.MatchString(segments[5]) || segments[3] == segments[5] {
		t.Fatalf("path = %q, want each parameter replaced by its own UUID", requests[0].path)
	}
}

func TestTheEdgeRequestsOfACommandCarryAndOmitTheIdempotencyKey(t *testing.T) {
	for _, tc := range []struct {
		route  kernelhttp.Route
		header string
	}{
		{kernelhttp.Route{Name: "registerOrganization", Method: http.MethodPost, Path: "/identity/organizations"}, api.IdempotencyHeader},
		{kernelhttp.Route{Name: "approve", Method: http.MethodPost, Path: "/identity/changes/{id}/approve", IdempotencyKey: "X-Request-Key"}, "X-Request-Key"},
	} {
		t.Run(tc.route.Name, func(t *testing.T) {
			requests := requestsFor(tc.route)

			if len(requests) != 2 {
				t.Fatalf("requestsFor(POST) = %+v, want the complete request and one without the key", requests)
			}
			complete, keyless := requests[0], requests[1]
			if complete.body != "{}" || keyless.body != "{}" || complete.method != http.MethodPost || keyless.method != http.MethodPost {
				t.Fatalf("requests = %+v, want both POST with the body {}", requests)
			}
			if !uuidShape.MatchString(complete.headers[tc.header]) {
				t.Fatalf("complete headers = %v, want %s carrying a valid key", complete.headers, tc.header)
			}
			if _, carried := keyless.headers[tc.header]; carried || keyless.omittedKey != tc.header || complete.omittedKey != "" {
				t.Fatalf("keyless = %+v and complete = %+v, want only the keyless one omitting %s", keyless, complete, tc.header)
			}
			if strings.Contains(complete.path, "{") || complete.path != keyless.path {
				t.Fatalf("paths = %q and %q, want one filled path for both", complete.path, keyless.path)
			}
		})
	}
}
