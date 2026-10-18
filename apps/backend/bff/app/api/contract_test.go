package api_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/bff/app/api"
)

func TestTheContractIsServedWithAValidatorAndCompressed(t *testing.T) {
	document := strings.Repeat("openapi: 3.1.0\n", 200)
	f := newFixture(t, withoutRoutes(), withIdentityContract(document))

	plain := f.do(t, http.MethodGet, api.IdentityContractPath, nil)
	etag := plain.Header().Get("ETag")
	if plain.Code != http.StatusOK || plain.Body.String() != document || etag == "" {
		t.Fatalf("GET %s = %d with ETag %q, want 200 with the contract and a validator", api.IdentityContractPath, plain.Code, etag)
	}
	if vary := plain.Header().Values("Vary"); !strings.Contains(strings.Join(vary, ","), "Accept-Encoding") {
		t.Fatalf("Vary = %v, want Accept-Encoding", vary)
	}

	revalidated := f.do(t, http.MethodGet, api.IdentityContractPath, nil, "If-None-Match", etag)
	if revalidated.Code != http.StatusNotModified || revalidated.Body.Len() != 0 {
		t.Fatalf("GET with If-None-Match %s = %d with %d bytes, want 304 without a body", etag, revalidated.Code, revalidated.Body.Len())
	}

	compressed := f.do(t, http.MethodGet, api.IdentityContractPath, nil, "Accept-Encoding", "br, gzip")
	if compressed.Header().Get("Content-Encoding") != "gzip" || compressed.Header().Get("ETag") == etag {
		t.Fatalf("GET with gzip accepted = Content-Encoding %q, ETag %q; want gzip under its own validator",
			compressed.Header().Get("Content-Encoding"), compressed.Header().Get("ETag"))
	}
	reader, err := gzip.NewReader(compressed.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader() = %v", err)
	}
	if body, err := io.ReadAll(reader); err != nil || string(body) != document {
		t.Fatalf("decompressed contract = %d bytes (%v), want the %d of the document", len(body), err, len(document))
	}
	if compressed.Body.Len() >= len(document) {
		t.Fatalf("compressed body = %d bytes, want fewer than the %d of the document", compressed.Body.Len(), len(document))
	}

	compressedETag := compressed.Header().Get("ETag")
	if rec := f.do(t, http.MethodGet, api.IdentityContractPath, nil, "Accept-Encoding", "gzip", "If-None-Match", compressedETag); rec.Code != http.StatusNotModified {
		t.Fatalf("GET gzip with If-None-Match %s = %d, want 304", compressedETag, rec.Code)
	}
	if rec := f.do(t, http.MethodGet, api.IdentityContractPath, nil, "Accept-Encoding", "gzip", "If-None-Match", etag); rec.Code != http.StatusOK {
		t.Fatalf("GET gzip with the plain validator = %d, want 200: each representation has its own", rec.Code)
	}
	if rec := f.do(t, http.MethodGet, api.IdentityContractPath, nil, "Range", "bytes=0-9"); rec.Code != http.StatusPartialContent || rec.Body.String() != document[:10] {
		t.Fatalf("GET with Range bytes=0-9 = %d %q, want 206 with the first 10 bytes", rec.Code, rec.Body.String())
	}
	for _, header := range []string{"gzip;q=0", "gzip; Q=0", "gzip;foo=1;q=0"} {
		if rec := f.do(t, http.MethodGet, api.IdentityContractPath, nil, "Accept-Encoding", header); rec.Header().Get("Content-Encoding") != "" {
			t.Fatalf("GET with Accept-Encoding %q = Content-Encoding %q, want the plain contract", header, rec.Header().Get("Content-Encoding"))
		}
	}

	refused := f.do(t, http.MethodGet, api.IdentityContractPath, nil, "Accept-Encoding", "gzip;q=0")
	if refused.Header().Get("Content-Encoding") != "" || refused.Body.String() != document {
		t.Fatalf("GET with gzip;q=0 = Content-Encoding %q, want the plain contract", refused.Header().Get("Content-Encoding"))
	}

	head := f.do(t, http.MethodHead, api.IdentityContractPath, nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("ETag") != etag {
		t.Fatalf("HEAD %s = %d with %d bytes, want 200 with the validator and no body", api.IdentityContractPath, head.Code, head.Body.Len())
	}
}
