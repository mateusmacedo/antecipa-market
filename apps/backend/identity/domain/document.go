package domain

// TaxIDTypePersonDocument is the type a person's document is protected under.
// It is a type of its own so the digits of a person never hash alike with the
// CPF of an organization; stored hashes depend on its value.
const TaxIDTypePersonDocument TaxIDType = "person-document"

func NormalizeDocument(value string) string {
	return NormalizeTaxID(TaxIDTypePersonDocument, value)
}

func MaskDocument(value string) string {
	return MaskTaxID(TaxIDTypePersonDocument, value)
}
