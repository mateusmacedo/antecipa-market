package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	organizationID = domain.OrganizationID("org-100")
	jurisdiction   = "BR"
	taxIDHash      = "4a455609da3a3d6d689b65257f38532e3868396abc39d3b92311b18489f169dc"
	legalName      = "Recebiveis Sinteticos Ltda"
	tradeName      = "Sintetica"
	at             = kernel.Instant(1_755_432_000_000_000_000)
)

var (
	alphanumericCNPJ = cnpj("12ABC34501DE")
	numericCNPJ      = cnpj("112223330001")
	syntheticCPF     = cpf("123456789")
)

func registerCommand() domain.RegisterOrganization {
	return domain.RegisterOrganization{
		Kinds:               domain.KindsOf(domain.KindClient),
		Jurisdiction:        jurisdiction,
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               alphanumericCNPJ,
		TaxIDHash:           taxIDHash,
		TaxIDHashKeyVersion: 1,
		LegalName:           legalName,
		TradeName:           tradeName,
		At:                  at,
	}
}

func newRegisteredOrganization(t *testing.T) *domain.Organization {
	t.Helper()
	o := domain.NewOrganization(organizationID)
	if _, rej := o.Register(registerCommand()); rej != nil {
		t.Fatalf("setup: Register rejected: %v", rej)
	}
	return o
}

func requireOrganizationUnchanged(t *testing.T, before, after domain.OrganizationSnapshot) {
	t.Helper()
	if !before.Equal(after) {
		t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
	}
}
