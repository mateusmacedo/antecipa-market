package domain_test

import (
	"testing"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestNormalizeDocumentKeepsTheDigitsAlone(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{"formatted CPF", "123.456.789-09", "12345678909"},
		{"CPF with blanks", " 123 456 789 09 ", "12345678909"},
		{"letters never survive", "12a3B4", "1234"},
		{"no digit at all", "abc.-/", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.NormalizeDocument(tc.value); got != tc.want {
				t.Fatalf("NormalizeDocument(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestMaskDocument(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{"CPF", syntheticCPF, "***.***.*89-09"},
		{"formatted CPF", "123.456.789-09", "***.***.*89-09"},
		{"document off the CPF length", "1234567", "***4567"},
		{"document no longer than the visible tail", "123", "***"},
		{"empty document", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.MaskDocument(tc.value); got != tc.want {
				t.Fatalf("MaskDocument(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestThePersonDocumentIsAnIdentifierTypeOfItsOwn(t *testing.T) {
	if domain.TaxIDTypePersonDocument == domain.TaxIDTypeCPF || domain.TaxIDTypePersonDocument == domain.TaxIDTypeCNPJ {
		t.Fatalf("TaxIDTypePersonDocument = %q, want a type apart from cpf and cnpj", domain.TaxIDTypePersonDocument)
	}
	if got := string(domain.TaxIDTypePersonDocument); got != "person-document" {
		t.Fatalf("TaxIDTypePersonDocument = %q, want the value stored hashes depend on, %q", got, "person-document")
	}
}
