package api

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxEvidenceLength     = 256
	maxNameLength         = 256
	maxReferenceLength    = 256
	maxTaxIDLength        = 32
	maxDocumentLength     = 32
	maxJurisdictionLength = 8
	maxCurrencyLength     = 8
	maxActTypeLength      = 64
	maxEmailLength        = 254
	maxReasonLength       = 512
	maxIdentifierLength   = 128
)

var (
	jurisdictionFormat = regexp.MustCompile(`^[A-Z]{2}$`)

	organizationKindOf      = inverse(organizationKinds)
	taxIDTypeOf             = inverse(taxIDTypes)
	externalSystemOf        = inverse(externalSystems)
	externalReferenceTypeOf = inverse(externalReferenceTypes)
	powerTypeOf             = inverse(powerTypes)
	sourceTypeOf            = inverse(sourceTypes)
	membershipRoleOf        = inverse(membershipRoles)
)

func inverse[E comparable](table map[E]string) map[string]E {
	values := make(map[string]E, len(table))
	for value, text := range table {
		values[text] = value
	}
	return values
}

func known[E comparable](values map[string]E, text *string) bool {
	if text == nil {
		return false
	}
	_, ok := values[*text]
	return ok
}

// WHY: maxLength in JSON Schema counts code points, and the schema pattern
// [^\x00-\x1F\x7F-\x9F] is exactly unicode.IsControl (category Cc).
func validText(text string, maxLength int) bool {
	return utf8.RuneCountInString(text) <= maxLength && !strings.ContainsFunc(text, unicode.IsControl)
}

func validPrintable(text string, maxLength int) bool {
	return len(text) <= maxLength && !strings.ContainsFunc(text, func(r rune) bool { return r < 0x20 || r > 0x7E })
}

func given(text *string, valid func(string) bool) bool {
	return text != nil && valid(*text)
}

func validIdentifier(text string) bool { return text != "" && validSubject(text) }

func validSubject(text string) bool {
	if len(text) > maxIdentifierLength {
		return false
	}
	for i := 0; i < len(text); i++ {
		if !identifierByte(text[i]) {
			return false
		}
	}
	return true
}

func identifierByte(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '.' || c == '_' || c == ':' || c == '-'
}

func validName(text string) bool { return validText(text, maxNameLength) }

func validTaxID(text string) bool { return validPrintable(text, maxTaxIDLength) }

type requestBankAccountChangeBody struct {
	BankAccountChangeID   *string `json:"bankAccountChangeId"`
	DestinationEvidenceID *string `json:"destinationEvidenceId"`
}

const invalidRequestBankAccountChange = "bankAccountChangeId must have 1 to 128 characters of [A-Za-z0-9._:-] and " +
	"destinationEvidenceId must be present with at most 256 characters, none of them a control character"

func validRequestBankAccountChange(b requestBankAccountChangeBody) bool {
	return given(b.BankAccountChangeID, validIdentifier) &&
		b.DestinationEvidenceID != nil && validText(*b.DestinationEvidenceID, maxEvidenceLength)
}

type registerOrganizationBody struct {
	OrganizationID *string  `json:"organizationId"`
	Kinds          []string `json:"kinds" maxItems:"4"`
	Jurisdiction   *string  `json:"jurisdiction"`
	TaxIDType      *string  `json:"taxIdType"`
	TaxID          *string  `json:"taxId"`
	LegalName      *string  `json:"legalName"`
	TradeName      string   `json:"tradeName"`
}

const invalidRegisterOrganization = "organizationId must be an identifier; kinds must hold at most 4 distinct organization kinds; " +
	"jurisdiction must have at most 8 printable ASCII characters; taxIdType must be cnpj or cpf; " +
	"taxId must have at most 32 printable ASCII characters; legalName and tradeName must have at most 256 characters, " +
	"none of them a control character"

func validRegisterOrganization(b registerOrganizationBody) bool {
	return given(b.OrganizationID, validIdentifier) && validKinds(b.Kinds) &&
		given(b.Jurisdiction, func(j string) bool { return validPrintable(j, maxJurisdictionLength) }) &&
		known(taxIDTypeOf, b.TaxIDType) && given(b.TaxID, validTaxID) &&
		given(b.LegalName, validName) && validName(b.TradeName)
}

func validKinds(kinds []string) bool {
	if kinds == nil {
		return false
	}
	seen := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		if seen[kind] || !known(organizationKindOf, &kind) {
			return false
		}
		seen[kind] = true
	}
	return true
}

type addEstablishmentBody struct {
	EstablishmentID    *string                 `json:"establishmentId"`
	TaxIDType          *string                 `json:"taxIdType"`
	TaxID              *string                 `json:"taxId"`
	TradeName          string                  `json:"tradeName"`
	Receiver           *bool                   `json:"receiver"`
	ExternalReferences []externalReferenceBody `json:"externalReferences" maxItems:"32"`
}

type externalReferenceBody struct {
	ProviderID      *string `json:"providerId"`
	System          *string `json:"system"`
	ReferenceType   *string `json:"referenceType"`
	Reference       *string `json:"reference"`
	AsOf            *int64  `json:"asOf"`
	ObservedVersion string  `json:"observedVersion"`
}

const invalidAddEstablishment = "establishmentId must be an identifier; taxIdType must be cnpj or cpf; " +
	"taxId must have at most 32 printable ASCII characters; tradeName must have at most 256 characters, none of them a control character; " +
	"receiver must be a boolean; externalReferences must hold at most 32 references, each with providerId, system, referenceType, " +
	"reference and asOf, and texts of at most 256 characters, none of them a control character"

func validAddEstablishment(b addEstablishmentBody) bool {
	for _, reference := range b.ExternalReferences {
		if !validExternalReference(reference) {
			return false
		}
	}
	return given(b.EstablishmentID, validIdentifier) && known(taxIDTypeOf, b.TaxIDType) && given(b.TaxID, validTaxID) &&
		validName(b.TradeName) && b.Receiver != nil
}

func validExternalReference(r externalReferenceBody) bool {
	reference := func(text string) bool { return validText(text, maxReferenceLength) }
	return given(r.ProviderID, reference) && known(externalSystemOf, r.System) &&
		known(externalReferenceTypeOf, r.ReferenceType) && given(r.Reference, reference) &&
		r.AsOf != nil && reference(r.ObservedVersion)
}

type findOrganizationByTaxIDBody struct {
	TaxIDType    *string `json:"taxIdType"`
	TaxID        *string `json:"taxId"`
	Jurisdiction *string `json:"jurisdiction"`
}

const invalidFindOrganizationByTaxID = "taxIdType must be cnpj or cpf; taxId must have 1 to 32 printable ASCII characters; " +
	"jurisdiction must be two uppercase letters"

func validFindOrganizationByTaxID(b findOrganizationByTaxIDBody) bool {
	return known(taxIDTypeOf, b.TaxIDType) && given(b.TaxID, func(t string) bool { return t != "" && validTaxID(t) }) &&
		given(b.Jurisdiction, jurisdictionFormat.MatchString)
}

type registerPersonBody struct {
	PersonID *string `json:"personId"`
	FullName *string `json:"fullName"`
	Document *string `json:"document"`
}

const invalidRegisterPerson = "personId must be an identifier; fullName must have at most 256 characters, none of them a control character; " +
	"document must have at most 32 printable ASCII characters"

func validRegisterPerson(b registerPersonBody) bool {
	return given(b.PersonID, validIdentifier) && given(b.FullName, validName) &&
		given(b.Document, func(d string) bool { return validPrintable(d, maxDocumentLength) })
}

type provisionUserAccountBody struct {
	Subject  *string `json:"subject"`
	PersonID *string `json:"personId"`
}

const invalidProvisionUserAccount = "subject must have at most 128 characters of [A-Za-z0-9._:-]; personId, when present, must be an identifier"

func validProvisionUserAccount(b provisionUserAccountBody) bool {
	return given(b.Subject, validSubject) && (b.PersonID == nil || validIdentifier(*b.PersonID))
}

type reasonBody struct {
	Reason *string `json:"reason"`
}

const invalidReason = "reason must be present with at most 512 characters, none of them a control character"

func validReason(b reasonBody) bool {
	return given(b.Reason, func(r string) bool { return validText(r, maxReasonLength) })
}

type registerRepresentationBody struct {
	RepresentationID    *string   `json:"representationId"`
	PersonID            *string   `json:"personId"`
	PowerType           *string   `json:"powerType"`
	SourceType          *string   `json:"sourceType"`
	SourceEvidenceID    *string   `json:"sourceEvidenceId"`
	ScopeCurrency       string    `json:"scopeCurrency"`
	ScopeMaxAmountMinor int64     `json:"scopeMaxAmountMinor"`
	ScopeActTypes       []*string `json:"scopeActTypes" maxItems:"32"`
	ValidFrom           *int64    `json:"validFrom"`
	ValidUntil          int64     `json:"validUntil"`
}

const invalidRegisterRepresentation = "representationId and personId must be identifiers; powerType and sourceType must be of the contract; " +
	"sourceEvidenceId must have at most 256 characters, none of them a control character; scopeCurrency must have at most 8 " +
	"printable ASCII characters; scopeActTypes must hold at most 32 texts of at most 64 printable ASCII characters; validFrom must be present"

func validRegisterRepresentation(b registerRepresentationBody) bool {
	for _, actType := range b.ScopeActTypes {
		if !given(actType, func(a string) bool { return validPrintable(a, maxActTypeLength) }) {
			return false
		}
	}
	return given(b.RepresentationID, validIdentifier) && given(b.PersonID, validIdentifier) &&
		known(powerTypeOf, b.PowerType) && known(sourceTypeOf, b.SourceType) &&
		given(b.SourceEvidenceID, func(e string) bool { return validText(e, maxEvidenceLength) }) &&
		validPrintable(b.ScopeCurrency, maxCurrencyLength) && b.ValidFrom != nil
}

type inviteUserBody struct {
	MembershipID        *string `json:"membershipId"`
	InviteeEmail        *string `json:"inviteeEmail"`
	Role                *string `json:"role"`
	InvitationExpiresAt *int64  `json:"invitationExpiresAt"`
}

const invalidInviteUser = "membershipId must be an identifier; inviteeEmail must have at most 254 characters, none of them a control character; " +
	"role must be operator or auditor; invitationExpiresAt must be present"

func validInviteUser(b inviteUserBody) bool {
	return given(b.MembershipID, validIdentifier) &&
		given(b.InviteeEmail, func(e string) bool { return validText(e, maxEmailLength) }) &&
		known(membershipRoleOf, b.Role) && b.InvitationExpiresAt != nil
}
