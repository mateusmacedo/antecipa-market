package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

// identifierScheme leads the HMAC input so a future scheme can never produce
// the hash of this one; it is not the key version.
const identifierScheme = "v1"

const minIdentifierKeyBytes = 32

var (
	ErrIdentifierKeyTooShort       = errors.New("provider: the identifier key has fewer than 32 bytes")
	ErrIdentifierKeyVersionInvalid = errors.New("provider: the identifier key version must be positive")
	ErrIdentifierTypeUnsupported   = errors.New("provider: unsupported identifier type")
	ErrIdentifierTenantMissing     = errors.New("provider: an identifier is protected under a tenant")
)

// IdentifierProtector realizes ports.IdentifierProtector by HMAC-SHA256 under
// the secret key of the identity configuration. The key cannot
// rotate: without the identifier in clear, stored hashes cannot be recomputed.
type IdentifierProtector struct {
	key        []byte
	keyVersion int
}

func NewIdentifierProtector(key []byte, keyVersion int) (*IdentifierProtector, error) {
	if len(key) < minIdentifierKeyBytes {
		return nil, ErrIdentifierKeyTooShort
	}
	if keyVersion < 1 {
		return nil, ErrIdentifierKeyVersionInvalid
	}
	return &IdentifierProtector{key: append([]byte(nil), key...), keyVersion: keyVersion}, nil
}

func (p *IdentifierProtector) Protect(_ context.Context, tenant port.TenantID, kind domain.TaxIDType, value string) (ports.ProtectedIdentifier, error) {
	switch kind {
	case domain.TaxIDTypeCNPJ, domain.TaxIDTypeCPF, domain.TaxIDTypePersonDocument:
	default:
		return ports.ProtectedIdentifier{}, ErrIdentifierTypeUnsupported
	}
	if tenant == "" {
		return ports.ProtectedIdentifier{}, ErrIdentifierTenantMissing
	}
	mac := hmac.New(sha256.New, p.key)
	for _, field := range []string{identifierScheme, string(tenant), string(kind), domain.NormalizeTaxID(kind, value)} {
		mac.Write(binary.BigEndian.AppendUint32(nil, uint32(len(field))))
		mac.Write([]byte(field))
	}
	return ports.ProtectedIdentifier{Hash: hex.EncodeToString(mac.Sum(nil)), KeyVersion: p.keyVersion}, nil
}

// String and GoString keep the key out of any formatted output, logs included.
func (p *IdentifierProtector) String() string {
	return "provider.IdentifierProtector{key: redacted}"
}

func (p *IdentifierProtector) GoString() string { return p.String() }
