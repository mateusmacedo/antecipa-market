package domain_test

import (
	"strings"
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestNormalizeTaxID(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  domain.TaxIDType
		value string
		want  string
	}{
		{"formatted alphanumeric CNPJ", domain.TaxIDTypeCNPJ, "12.ABC.345/01DE-35", "12ABC34501DE35"},
		{"alphanumeric CNPJ in lower case", domain.TaxIDTypeCNPJ, "12.abc.345/01de-35", "12ABC34501DE35"},
		{"CNPJ with blanks", domain.TaxIDTypeCNPJ, " 11.222.333/0001-81 ", "11222333000181"},
		{"formatted CPF", domain.TaxIDTypeCPF, "123.456.789-09", "12345678909"},
		{"CPF keeps digits alone", domain.TaxIDTypeCPF, "123a456b789c09", "12345678909"},
		{"non-ASCII letters never become ASCII", domain.TaxIDTypeCNPJ, "1ı2é3", "123"},
		{"punctuation alone", domain.TaxIDTypeCNPJ, "../-", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.NormalizeTaxID(tc.kind, tc.value); got != tc.want {
				t.Fatalf("NormalizeTaxID(%s, %q) = %q, want %q", tc.kind, tc.value, got, tc.want)
			}
		})
	}
}

func TestMaskTaxID(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  domain.TaxIDType
		value string
		want  string
	}{
		{"alphanumeric CNPJ", domain.TaxIDTypeCNPJ, alphanumericCNPJ, "**.***.***/**DE-35"},
		{"formatted alphanumeric CNPJ", domain.TaxIDTypeCNPJ, "12.abc.345/01de-35", "**.***.***/**DE-35"},
		{"numeric CNPJ", domain.TaxIDTypeCNPJ, numericCNPJ, "**.***.***/**01-81"},
		{"CPF", domain.TaxIDTypeCPF, syntheticCPF, "***.***.*89-09"},
		{"value off the canonical length", domain.TaxIDTypeCNPJ, "1234567", "***4567"},
		{"value no longer than the visible tail", domain.TaxIDTypeCPF, "123", "***"},
		{"empty value", domain.TaxIDTypeCPF, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.MaskTaxID(tc.kind, tc.value); got != tc.want {
				t.Fatalf("MaskTaxID(%s, %q) = %q, want %q", tc.kind, tc.value, got, tc.want)
			}
		})
	}
}

func TestMaskTaxIDHidesTheLeadingCharacters(t *testing.T) {
	masked := domain.MaskTaxID(domain.TaxIDTypeCNPJ, alphanumericCNPJ)

	if strings.Contains(masked, alphanumericCNPJ[:10]) || strings.ContainsAny(masked, "ABC") {
		t.Fatalf("MaskTaxID = %q reveals the leading characters of %q", masked, alphanumericCNPJ)
	}
}
