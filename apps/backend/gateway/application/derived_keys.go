package application

import (
	"errors"
	"fmt"
	"strings"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrNoKeyDigest is a derived key that does not fit IDM-02 with no digest to
// shorten it: the composition root left DerivedKeyPolicy empty.
var ErrNoKeyDigest = errors.New("application: a derived key does not fit IDM-02 and the policy has no digest")

// DerivedKeyPolicy fixes the digest that shortens the command key an internal
// step derives from identities of unbounded length; the application block
// imports no crypto, so the composition root fixes it, like OutboundKeyPolicy.
type DerivedKeyPolicy struct {
	Digest func(canonical []byte) port.Fingerprint
}

// key is "<step>.<parts joined by dots>" when that fits IDM-02, so the inbox
// stays readable; otherwise "<step>.<hex digest of the parts>", over the
// kernel's canonical encoding, so two parts never fold into the same key.
func (p DerivedKeyPolicy) key(step string, parts ...string) (string, error) {
	plain := step + "." + strings.Join(parts, ".")
	if port.ValidIdempotencyKey(plain) {
		return plain, nil
	}
	if p.Digest == nil {
		return "", fmt.Errorf("%w: %s", ErrNoKeyDigest, step)
	}
	canonical := usecase.NewFingerprint(step)
	for _, part := range parts {
		canonical.String(part)
	}
	digest := p.Digest(canonical.Canonical())
	return fmt.Sprintf("%s.%x", step, digest[:]), nil
}
