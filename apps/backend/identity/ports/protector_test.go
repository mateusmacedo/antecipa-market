package ports_test

import (
	"context"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

type protectorFunc func(context.Context, port.TenantID, domain.TaxIDType, string) (ports.ProtectedIdentifier, error)

func (f protectorFunc) Protect(ctx context.Context, tenant port.TenantID, kind domain.TaxIDType, value string) (ports.ProtectedIdentifier, error) {
	return f(ctx, tenant, kind, value)
}

func TestProtectReceivesTenantTypeAndValueAndAnswersHashAndKeyVersion(t *testing.T) {
	type call struct {
		tenant port.TenantID
		kind   domain.TaxIDType
		value  string
	}
	var got call
	var protector ports.IdentifierProtector = protectorFunc(
		func(_ context.Context, tenant port.TenantID, kind domain.TaxIDType, value string) (ports.ProtectedIdentifier, error) {
			got = call{tenant, kind, value}
			return ports.ProtectedIdentifier{Hash: "h-1", KeyVersion: 1}, nil
		})

	protected, err := protector.Protect(context.Background(), "acme", domain.TaxIDTypeCNPJ, "12ABC34501DE35")

	if err != nil {
		t.Fatalf("Protect returned %v, want no error", err)
	}
	if want := (call{"acme", domain.TaxIDTypeCNPJ, "12ABC34501DE35"}); got != want {
		t.Fatalf("Protect received %+v, want %+v: the tenant takes part in the hash", got, want)
	}
	if want := (ports.ProtectedIdentifier{Hash: "h-1", KeyVersion: 1}); protected != want {
		t.Fatalf("Protect = %+v, want %+v", protected, want)
	}
}
