package domain

type TaxIDType string

const (
	TaxIDTypeCNPJ TaxIDType = "cnpj"
	TaxIDTypeCPF  TaxIDType = "cpf"
)

const maskVisibleTail = 4

// NormalizeTaxID keeps the ASCII letters and digits of a CNPJ, upper-cased,
// because the CNPJ has been alphanumeric since July 2026 (ADR-021), and only the
// digits of a CPF. Stored hashes depend on this form: changing it orphans them.
func NormalizeTaxID(kind TaxIDType, value string) string {
	out := make([]byte, 0, len(value))
	for i := range len(value) {
		c := value[i]
		switch {
		case c >= '0' && c <= '9':
			out = append(out, c)
		case kind == TaxIDTypeCPF, kind == TaxIDTypePersonDocument:
		case c >= 'A' && c <= 'Z':
			out = append(out, c)
		case c >= 'a' && c <= 'z':
			out = append(out, c-'a'+'A')
		}
	}
	return string(out)
}

// The mask leaves the context in events, so a value no longer than the visible
// tail is masked whole instead of shown.
func MaskTaxID(kind TaxIDType, value string) string {
	normalized := NormalizeTaxID(kind, value)
	masked := []byte(normalized)
	for i := range max(len(masked)-maskVisibleTail, 0) {
		masked[i] = '*'
	}
	if len(masked) <= maskVisibleTail {
		for i := range masked {
			masked[i] = '*'
		}
	}
	switch {
	case kind == TaxIDTypeCNPJ && len(masked) == 14:
		return layout(masked, "__.___.___/____-__")
	case (kind == TaxIDTypeCPF || kind == TaxIDTypePersonDocument) && len(masked) == 11:
		return layout(masked, "___.___.___-__")
	default:
		return string(masked)
	}
}

func layout(chars []byte, pattern string) string {
	out := []byte(pattern)
	next := 0
	for i := range out {
		if out[i] == '_' {
			out[i] = chars[next]
			next++
		}
	}
	return string(out)
}
