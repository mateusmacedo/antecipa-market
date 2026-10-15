package application_test

// A copy of the domain_test builder, since test files cannot be imported; the
// domain tests pin the Serpro algorithm (ADR-021).
func cnpj(base string) string {
	weights := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	digits := base
	for range 2 {
		offset := len(weights) - len(digits)
		sum := 0
		for i := range len(digits) {
			sum += int(digits[i]-'0') * weights[offset+i]
		}
		digit := 0
		if remainder := sum % 11; remainder >= 2 {
			digit = 11 - remainder
		}
		digits += string(rune('0' + digit))
	}
	return digits
}

var alphanumericCNPJ = cnpj("12ABC34501DE")
