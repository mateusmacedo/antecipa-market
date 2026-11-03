package application

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
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

// InboundInteractionID is the id of the first receipt of a partner event. It
// is always a digest, because the partner's id has an alphabet of its own and
// the interaction id names a segment of the raw key; the primary key then
// serializes two receipts of one event (decision 19).
func (p DerivedKeyPolicy) InboundInteractionID(connector domain.ConnectorKey, partnerEventID string) (domain.InteractionID, error) {
	id, err := p.digest("inbound", string(connector), partnerEventID)
	return domain.InteractionID(id), err
}

// ExternalReadInteractionID is the id of the read a command key asks for: a
// replay of the key calls the partner again and writes a new version of the
// same raw keys, which stays orphan (decision 16).
func (p DerivedKeyPolicy) ExternalReadInteractionID(connector domain.ConnectorKey, commandKey string) (domain.InteractionID, error) {
	id, err := p.digest("read", string(connector), commandKey)
	return domain.InteractionID(id), err
}

// QueryInteractionID is the id of the state query the sweep makes of a command
// due at nextQueryAt: a repeated sweep derives the same id and its record replays.
func (p DerivedKeyPolicy) QueryInteractionID(command domain.IdempotencyKey, nextQueryAt kernel.Instant) (domain.InteractionID, error) {
	id, err := p.key("query", string(command), strconv.FormatInt(int64(nextQueryAt), 10))
	return domain.InteractionID(id), err
}

// key is "<step>.<parts joined by dots>" when that fits IDM-02, so the inbox
// stays readable; otherwise the digest of the parts.
func (p DerivedKeyPolicy) key(step string, parts ...string) (string, error) {
	plain := step + "." + strings.Join(parts, ".")
	if port.ValidIdempotencyKey(plain) {
		return plain, nil
	}
	return p.digest(step, parts...)
}

// digest is "<step>.<hex digest>" over the kernel's canonical encoding of the
// parts, so two parts never fold into the same key.
func (p DerivedKeyPolicy) digest(step string, parts ...string) (string, error) {
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
