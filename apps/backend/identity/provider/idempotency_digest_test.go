package provider_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

var testCanonical = []byte("identity.RegisterPerson|per-1|Ana Sintetica|123.456.789-09")

func newIdempotencyDigest(t *testing.T, key []byte) func([]byte) port.Fingerprint {
	t.Helper()
	digest, err := provider.NewIdempotencyDigest(key)
	if err != nil {
		t.Fatalf("NewIdempotencyDigest() = %v", err)
	}
	return digest
}

func hexOf(f port.Fingerprint) string { return hex.EncodeToString(f[:]) }

// The vectors were computed apart from this package, by HMAC-SHA256 over the
// label prefixed by its length as a big-endian uint32, then the canonical. An
// inbox entry is compared by this digest, so they must not move.
func TestTheIdempotencyDigestMatchesTheFixedVectors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		key       []byte
		canonical []byte
		want      string
	}{
		{"a fingerprint with the document in clear", testKey, testCanonical, "e6df66369b050fef2150c2c41c89914d73b2ec523390926f267f0d5e725c1876"},
		{"the empty canonical", testKey, nil, "0a9ad6e17bcdd377b5f915fea0f7aa2c25f014bde8dd8aa8ccf7a61c46e77687"},
		{"another key", otherTestKey, testCanonical, "49dd37ecb4c44c4f3646d19f7b9c56384b10eac6ade8c56daeb6b955b5eda252"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hexOf(newIdempotencyDigest(t, tc.key)(tc.canonical)); got != tc.want {
				t.Fatalf("digest = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestTheIdempotencyDigestIsNotTheUnkeyedSHA256OfTheCanonical(t *testing.T) {
	got := newIdempotencyDigest(t, testKey)(testCanonical)

	if unkeyed := sha256.Sum256(testCanonical); got == port.Fingerprint(unkeyed) {
		t.Fatalf("digest = %x, the unkeyed SHA-256 a brute force over the document recomputes", got)
	}
}

// The canonical below is, byte for byte, the input the identifier protector
// MACs under the same key: only the leading label keeps the two apart.
func TestTheIdempotencyDigestDoesNotCollideWithTheIdentifierHash(t *testing.T) {
	var protectorInput []byte
	for _, field := range []string{"v1", "acme", string(domain.TaxIDTypeCPF), "12345678909"} {
		protectorInput = binary.BigEndian.AppendUint32(protectorInput, uint32(len(field)))
		protectorInput = append(protectorInput, field...)
	}
	identifier := protect(t, newProtector(t, testKey, 1), "acme", domain.TaxIDTypeCPF, "123.456.789-09")
	mac := hmac.New(sha256.New, testKey)
	mac.Write(protectorInput)
	if hex.EncodeToString(mac.Sum(nil)) != identifier.Hash {
		t.Fatalf("the crafted canonical is not the protector input, so the test would prove nothing")
	}

	if got := hexOf(newIdempotencyDigest(t, testKey)(protectorInput)); got == identifier.Hash {
		t.Fatalf("digest = %s, the identifier hash of the same bytes under the same key", got)
	}
}

func TestTheIdempotencyDigestRefusesAKeyShorterThan32Bytes(t *testing.T) {
	_, err := provider.NewIdempotencyDigest(testKey[:31])

	if !errors.Is(err, provider.ErrIdentifierKeyTooShort) {
		t.Fatalf("NewIdempotencyDigest(31 bytes) = %v, want ErrIdentifierKeyTooShort", err)
	}
}

func TestTheIdempotencyDigestKeepsItsOwnCopyOfTheKey(t *testing.T) {
	key := append([]byte(nil), testKey...)
	digest := newIdempotencyDigest(t, key)
	before := digest(testCanonical)

	for i := range key {
		key[i] = 'x'
	}

	if after := digest(testCanonical); after != before {
		t.Fatalf("digest moved from %x to %x when the caller reused its buffer", before, after)
	}
}
