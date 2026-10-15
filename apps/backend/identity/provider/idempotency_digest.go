package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// idempotencyDigestLabel leads every input, length-prefixed, so no digest of a
// command fingerprint equals an identifier hash made under the same key.
const idempotencyDigestLabel = "identity-organization/idempotency-fingerprint/v1"

// NewIdempotencyDigest replaces the unkeyed SHA-256 of the kernel policy: the
// fingerprint of RegisterPerson holds the document in clear, and the inbox
// keeps its digest (spec, Idempotência). The key is the identifier key.
func NewIdempotencyDigest(key []byte) (func(canonical []byte) port.Fingerprint, error) {
	if len(key) < minIdentifierKeyBytes {
		return nil, ErrIdentifierKeyTooShort
	}
	secret := append([]byte(nil), key...)
	label := binary.BigEndian.AppendUint32(nil, uint32(len(idempotencyDigestLabel)))
	label = append(label, idempotencyDigestLabel...)
	return func(canonical []byte) port.Fingerprint {
		mac := hmac.New(sha256.New, secret)
		mac.Write(label)
		mac.Write(canonical)
		var digest port.Fingerprint
		copy(digest[:], mac.Sum(nil))
		return digest
	}, nil
}
