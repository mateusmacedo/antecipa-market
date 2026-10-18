package api_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
)

func derived(subject, key string) string {
	digest := sha256.Sum256([]byte(subject + "\x00" + key))
	return hex.EncodeToString(digest[:])
}

func TestTheAccessLogCarriesTheClientAndTheDerivedKey(t *testing.T) {
	f := newFixture(t)

	f.post(t, "/probes/p-1/act", `{}`)

	record := onlyAccessLog(t, f)
	if record["dmpf.idempotency_key"] != "k-1" || record["dmpf.idempotency_key.derived"] != derived("tester", "k-1") {
		t.Fatalf("access log = %v, want the client key and the key derived from the subject", record)
	}
}

func TestTheAccessLogFlagsAnInvalidKeyWithoutRecordingIt(t *testing.T) {
	f := newFixture(t)

	f.do(t, http.MethodPost, "/probes/p-1/act", strings.NewReader(`{}`),
		"Idempotency-Key", "k 1\nforged=true", "Content-Type", "application/json", "Authorization", testCredential)

	record := onlyAccessLog(t, f)
	if _, logged := record["dmpf.idempotency_key"]; logged || record["dmpf.idempotency_key.invalid"] != true {
		t.Fatalf("access log = %v, want dmpf.idempotency_key.invalid and no dmpf.idempotency_key: the header is client input", record)
	}
}

func TestCORSExposesTheReplayHeader(t *testing.T) {
	f := newFixture(t, withCORS("https://app.example"))

	rec := f.do(t, http.MethodGet, "/probes/p-1", nil, "Origin", "https://app.example")

	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposed, "Idempotent-Replayed") || !strings.Contains(exposed, "X-Correlation-ID") {
		t.Fatalf("Access-Control-Expose-Headers = %q, want X-Correlation-ID and Idempotent-Replayed", exposed)
	}
}
