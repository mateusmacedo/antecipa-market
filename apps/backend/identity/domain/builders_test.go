package domain_test

import "testing"

// Check digits by the Serpro method, which covers the alphanumeric CNPJ
// (ADR-021).
func cnpj(base string) string {
	weights := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	digits := base
	for range 2 {
		offset := len(weights) - len(digits)
		sum := 0
		for i := range len(digits) {
			sum += int(digits[i]-'0') * weights[offset+i]
		}
		digits += string(rune('0' + checkDigit(sum%11)))
	}
	return digits
}

func checkDigit(remainder int) int {
	if remainder < 2 {
		return 0
	}
	return 11 - remainder
}

func cpf(base string) string {
	digits := base
	for range 2 {
		sum := 0
		for i := range len(digits) {
			sum += int(digits[i]-'0') * (len(digits) + 1 - i)
		}
		digit := sum * 10 % 11
		if digit == 10 {
			digit = 0
		}
		digits += string(rune('0' + digit))
	}
	return digits
}

func TestBuildersFollowTheSerproCheckDigits(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"alphanumeric CNPJ of the Serpro example", cnpj("12ABC34501DE"), "12ABC34501DE35"},
		{"numeric CNPJ", cnpj("112223330001"), "11222333000181"},
		{"alphanumeric CNPJ of another base", cnpj("A1B2C3D40001"), "A1B2C3D4000193"},
		{"CPF", cpf("123456789"), "12345678909"},
		{"CPF of another base", cpf("987654321"), "98765432100"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
