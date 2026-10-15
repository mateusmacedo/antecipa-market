package provider_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

// The vectors were computed apart from this package, by HMAC-SHA256 over
// "v1", tenant, type and normalized value, each prefixed by its length as a
// big-endian uint32. A stored hash is never recomputed, so they must not move.
var (
	testKey      = []byte("0123456789abcdef0123456789abcdef")
	otherTestKey = []byte("fedcba9876543210fedcba9876543210")
)

var _ ports.IdentifierProtector = (*provider.IdentifierProtector)(nil)

func newProtector(t *testing.T, key []byte, version int) *provider.IdentifierProtector {
	t.Helper()
	p, err := provider.NewIdentifierProtector(key, version)
	if err != nil {
		t.Fatalf("NewIdentifierProtector() = %v", err)
	}
	return p
}

func protect(t *testing.T, p *provider.IdentifierProtector, tenant port.TenantID, kind domain.TaxIDType, value string) ports.ProtectedIdentifier {
	t.Helper()
	protected, err := p.Protect(context.Background(), tenant, kind, value)
	if err != nil {
		t.Fatalf("Protect(%s, %s) = %v", tenant, kind, err)
	}
	return protected
}

func TestProtectMatchesTheFixedVectors(t *testing.T) {
	p := newProtector(t, testKey, 1)
	for _, tc := range []struct {
		name   string
		tenant port.TenantID
		kind   domain.TaxIDType
		value  string
		want   string
	}{
		{"alphanumeric CNPJ", "acme", domain.TaxIDTypeCNPJ, "12ABC34501DE35", "4a455609da3a3d6d689b65257f38532e3868396abc39d3b92311b18489f169dc"},
		{"alphanumeric CNPJ as typed by a person", "acme", domain.TaxIDTypeCNPJ, "12.abc.345/01de-35", "4a455609da3a3d6d689b65257f38532e3868396abc39d3b92311b18489f169dc"},
		{"numeric CNPJ", "acme", domain.TaxIDTypeCNPJ, "11.222.333/0001-81", "2ad689d9d2bdfa27268b09d5f0bd86e01cc7545bfb72a11b4c28450f3098b053"},
		{"CPF", "acme", domain.TaxIDTypeCPF, "123.456.789-09", "accdef5fa5a63d40218f9b3fb5abdfc9eeb4036cbdbe46cff36bff2ec9462a3d"},
		{"alphanumeric CNPJ in another tenant", "globex", domain.TaxIDTypeCNPJ, "12ABC34501DE35", "d9e740ecc0099261fc6176681ef8f152dd08514d121084cef8a2de3d6a2f44d3"},
		{"CPF digits hashed as a CNPJ", "acme", domain.TaxIDTypeCNPJ, "12345678909", "6ea39ed985968f506a1d6272b3863874ded7f8bee4a20730da40d4b0afe8a333"},
		{"person document", "acme", domain.TaxIDTypePersonDocument, "123.456.789-09", "9b07f96fee3b329855d08503cfee6269d19587470dfa1a60c4c99cb463d6ec9e"},
		{"person document with a letter typed in", "acme", domain.TaxIDTypePersonDocument, "123.456.789-09x", "9b07f96fee3b329855d08503cfee6269d19587470dfa1a60c4c99cb463d6ec9e"},
		{"person document in another tenant", "globex", domain.TaxIDTypePersonDocument, "12345678909", "234a1d900371cce0bc2762fd4d654da7ab0325f3c7ae0046c68632c2dcbce5f5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := protect(t, p, tc.tenant, tc.kind, tc.value); got != (ports.ProtectedIdentifier{Hash: tc.want, KeyVersion: 1}) {
				t.Fatalf("Protect() = %+v, want hash %s at key version 1", got, tc.want)
			}
		})
	}
}

func TestProtectSeparatesTenantsAndTypes(t *testing.T) {
	p := newProtector(t, testKey, 1)

	acme := protect(t, p, "acme", domain.TaxIDTypeCPF, "12345678909")
	globex := protect(t, p, "globex", domain.TaxIDTypeCPF, "12345678909")
	asCNPJ := protect(t, p, "acme", domain.TaxIDTypeCNPJ, "12345678909")

	if acme.Hash == globex.Hash {
		t.Fatal("the same tax id hashes alike in two tenants")
	}
	if acme.Hash == asCNPJ.Hash {
		t.Fatal("the same digits hash alike as CPF and as CNPJ")
	}
	if asPerson := protect(t, p, "acme", domain.TaxIDTypePersonDocument, "12345678909"); acme.Hash == asPerson.Hash {
		t.Fatal("the same digits hash alike as the CPF of an organization and as a person document")
	}
}

// The length prefixes keep the field boundaries: without them "acme"+"cnpj"
// and "ac"+"mecnpj" would feed the same bytes to the HMAC.
func TestProtectKeepsTheFieldBoundaries(t *testing.T) {
	p := newProtector(t, testKey, 1)

	if protect(t, p, "acme", domain.TaxIDTypeCNPJ, "1").Hash == protect(t, p, "acmecnpj", domain.TaxIDTypeCNPJ, "1").Hash {
		t.Fatal("tenants of different lengths collide")
	}
}

func TestProtectAnswersTheKeyVersionAndDependsOnTheKey(t *testing.T) {
	first := protect(t, newProtector(t, testKey, 1), "acme", domain.TaxIDTypeCNPJ, "12ABC34501DE35")
	second := protect(t, newProtector(t, otherTestKey, 2), "acme", domain.TaxIDTypeCNPJ, "12ABC34501DE35")

	if second != (ports.ProtectedIdentifier{Hash: "86ca406930607eff0555a9374a3477ac53483a3955fec32cc8bd1e3c9bfdb0ea", KeyVersion: 2}) {
		t.Fatalf("Protect() = %+v, want the vector of the second key at version 2", second)
	}
	if first.Hash == second.Hash {
		t.Fatal("two keys produce the same hash")
	}
}

func TestNewIdentifierProtectorRefusesAWeakConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		key     []byte
		version int
		want    error
	}{
		{"key of 31 bytes", testKey[:31], 1, provider.ErrIdentifierKeyTooShort},
		{"no key", nil, 1, provider.ErrIdentifierKeyTooShort},
		{"key version zero", testKey, 0, provider.ErrIdentifierKeyVersionInvalid},
		{"negative key version", testKey, -1, provider.ErrIdentifierKeyVersionInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := provider.NewIdentifierProtector(tc.key, tc.version); !errors.Is(err, tc.want) {
				t.Fatalf("NewIdentifierProtector() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestProtectRefusesAnUnsupportedTypeOrAMissingTenant(t *testing.T) {
	p := newProtector(t, testKey, 1)

	if _, err := p.Protect(context.Background(), "acme", "passport", "X1"); !errors.Is(err, provider.ErrIdentifierTypeUnsupported) {
		t.Fatalf("Protect(passport) = %v, want ErrIdentifierTypeUnsupported", err)
	}
	if _, err := p.Protect(context.Background(), "", domain.TaxIDTypeCPF, "12345678909"); !errors.Is(err, provider.ErrIdentifierTenantMissing) {
		t.Fatalf("Protect(no tenant) = %v, want ErrIdentifierTenantMissing", err)
	}
}

func TestTheProtectorKeepsItsOwnCopyOfTheKey(t *testing.T) {
	key := append([]byte(nil), testKey...)
	p := newProtector(t, key, 1)
	key[0] ^= 0xff

	if got := protect(t, p, "acme", domain.TaxIDTypeCNPJ, "12ABC34501DE35").Hash; got != "4a455609da3a3d6d689b65257f38532e3868396abc39d3b92311b18489f169dc" {
		t.Fatalf("Protect() = %s: a change to the caller's slice reached the key", got)
	}
}

func TestTheKeyNeverAppearsWhenTheProtectorIsPrinted(t *testing.T) {
	p := newProtector(t, testKey, 1)

	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		if printed := fmt.Sprintf(format, p); strings.Contains(printed, string(testKey)) || strings.Contains(printed, fmt.Sprintf("%d", testKey[:4])) {
			t.Fatalf("Sprintf(%q) = %q carries the key", format, printed)
		}
	}
}
