package api

import (
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

var (
	organizationKinds = map[servicev1.OrganizationKind]string{
		servicev1.OrganizationKind_ORGANIZATION_KIND_CLIENT:   "client",
		servicev1.OrganizationKind_ORGANIZATION_KIND_FUNDER:   "funder",
		servicev1.OrganizationKind_ORGANIZATION_KIND_PARTNER:  "partner",
		servicev1.OrganizationKind_ORGANIZATION_KIND_PLATFORM: "platform",
	}
	taxIDTypes = map[servicev1.TaxIdType]string{
		servicev1.TaxIdType_TAX_ID_TYPE_CNPJ: "cnpj",
		servicev1.TaxIdType_TAX_ID_TYPE_CPF:  "cpf",
	}
	organizationStatuses = map[servicev1.OrganizationStatus]string{
		servicev1.OrganizationStatus_ORGANIZATION_STATUS_ACTIVE:     "active",
		servicev1.OrganizationStatus_ORGANIZATION_STATUS_SUSPENDED:  "suspended",
		servicev1.OrganizationStatus_ORGANIZATION_STATUS_OFFBOARDED: "offboarded",
	}
	powerTypes = map[servicev1.PowerType]string{
		servicev1.PowerType_POWER_TYPE_LEGAL_REPRESENTATIVE: "legal_representative",
		servicev1.PowerType_POWER_TYPE_ATTORNEY_IN_FACT:     "attorney_in_fact",
		servicev1.PowerType_POWER_TYPE_OPERATOR:             "operator",
	}
	sourceTypes = map[servicev1.SourceType]string{
		servicev1.SourceType_SOURCE_TYPE_ARTICLES_OF_ASSOCIATION: "articles_of_association",
		servicev1.SourceType_SOURCE_TYPE_POWER_OF_ATTORNEY:       "power_of_attorney",
		servicev1.SourceType_SOURCE_TYPE_BOARD_RESOLUTION:        "board_resolution",
		servicev1.SourceType_SOURCE_TYPE_INTERNAL_DELEGATION:     "internal_delegation",
	}
	verificationStatuses = map[servicev1.RepresentationVerificationStatus]string{
		servicev1.RepresentationVerificationStatus_REPRESENTATION_VERIFICATION_STATUS_DECLARED: "declared",
	}
	bankAccountChangeStatuses = map[servicev1.BankAccountChangeStatus]string{
		servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_REQUESTED: "requested",
		servicev1.BankAccountChangeStatus_BANK_ACCOUNT_CHANGE_STATUS_APPROVED:  "approved",
	}
	externalSystems = map[servicev1.ExternalSystem]string{
		servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY:   "registry",
		servicev1.ExternalSystem_EXTERNAL_SYSTEM_BOOKKEEPER: "bookkeeper",
		servicev1.ExternalSystem_EXTERNAL_SYSTEM_ACQUIRER:   "acquirer",
		servicev1.ExternalSystem_EXTERNAL_SYSTEM_FUNDER:     "funder",
		servicev1.ExternalSystem_EXTERNAL_SYSTEM_DEPOSITORY: "depository",
	}
	externalReferenceTypes = map[servicev1.ExternalReferenceType]string{
		servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_ASSET:         "asset",
		servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_EFFECT:        "effect",
		servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_AUTHORIZATION: "authorization",
		servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_COMMAND:       "command",
		servicev1.ExternalReferenceType_EXTERNAL_REFERENCE_TYPE_SETTLEMENT:    "settlement",
	}
	membershipRoles = map[servicev1.MembershipRole]string{
		servicev1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR: "operator",
		servicev1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR:  "auditor",
	}
	membershipStatuses = map[servicev1.MembershipStatus]string{
		servicev1.MembershipStatus_MEMBERSHIP_STATUS_INVITED: "invited",
		servicev1.MembershipStatus_MEMBERSHIP_STATUS_ACTIVE:  "active",
		servicev1.MembershipStatus_MEMBERSHIP_STATUS_REVOKED: "revoked",
	}
	verificationLevels = map[servicev1.VerificationLevel]string{
		servicev1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP:     "channel_ownership",
		servicev1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED: "personal_data_checked",
	}
)

// The zero value of every contract enum is UNSPECIFIED, absent from these
// maps, so an unset or newer value fails the answer instead of shipping "".
type naming struct{ known bool }

func newNaming() *naming { return &naming{known: true} }

func name[E comparable](n *naming, names map[E]string, value E) string {
	text, ok := names[value]
	n.known = n.known && ok
	return text
}

func names[E comparable](n *naming, table map[E]string, values []E) []string {
	texts := make([]string, 0, len(values))
	for _, value := range values {
		texts = append(texts, name(n, table, value))
	}
	return texts
}

// verificationLevel is optional in the OpenAPI (openapi.yaml:1388-1428): the
// UNSPECIFIED of a membership not yet granted is its absence, not a fault.
func optionalName[E comparable](n *naming, names map[E]string, value E) string {
	var unset E
	if value == unset {
		return ""
	}
	return name(n, names, value)
}

func listOf[T, V any](items []T, convert func(T) (V, bool)) ([]V, bool) {
	views := make([]V, 0, len(items))
	for _, item := range items {
		v, representable := convert(item)
		if !representable {
			return nil, false
		}
		views = append(views, v)
	}
	return views, true
}

type organizationView struct {
	OrganizationID      string   `json:"organizationId"`
	Kinds               []string `json:"kinds"`
	Jurisdiction        string   `json:"jurisdiction"`
	TaxIDType           string   `json:"taxIdType"`
	TaxID               string   `json:"taxId"`
	TaxIDHash           string   `json:"taxIdHash"`
	TaxIDHashKeyVersion int32    `json:"taxIdHashKeyVersion"`
	TaxIDMasked         string   `json:"taxIdMasked"`
	LegalName           string   `json:"legalName"`
	TradeName           string   `json:"tradeName,omitempty"`
	Status              string   `json:"status"`
	CreatedAt           int64    `json:"createdAt"`
}

func organizationViewOf(o *servicev1.Organization) (organizationView, bool) {
	n := newNaming()
	v := organizationView{
		OrganizationID:      o.GetOrganizationId(),
		Kinds:               names(n, organizationKinds, o.GetKinds()),
		Jurisdiction:        o.GetJurisdiction(),
		TaxIDType:           name(n, taxIDTypes, o.GetTaxIdType()),
		TaxID:               o.GetTaxId(),
		TaxIDHash:           o.GetTaxIdHash(),
		TaxIDHashKeyVersion: o.GetTaxIdHashKeyVersion(),
		TaxIDMasked:         o.GetTaxIdMasked(),
		LegalName:           o.GetLegalName(),
		TradeName:           o.GetTradeName(),
		Status:              name(n, organizationStatuses, o.GetStatus()),
		CreatedAt:           o.GetCreatedAt(),
	}
	return v, o != nil && n.known
}

type personView struct {
	PersonID               string `json:"personId"`
	FullName               string `json:"fullName"`
	DocumentHash           string `json:"documentHash"`
	DocumentHashKeyVersion int32  `json:"documentHashKeyVersion"`
	DocumentMasked         string `json:"documentMasked"`
	CreatedAt              int64  `json:"createdAt"`
}

func personViewOf(p *servicev1.Person) (personView, bool) {
	return personView{
		PersonID:               p.GetPersonId(),
		FullName:               p.GetFullName(),
		DocumentHash:           p.GetDocumentHash(),
		DocumentHashKeyVersion: p.GetDocumentHashKeyVersion(),
		DocumentMasked:         p.GetDocumentMasked(),
		CreatedAt:              p.GetCreatedAt(),
	}, p != nil
}

type representationView struct {
	RepresentationID    string   `json:"representationId"`
	OrganizationID      string   `json:"organizationId"`
	PersonID            string   `json:"personId"`
	PowerType           string   `json:"powerType"`
	SourceType          string   `json:"sourceType"`
	SourceEvidenceID    string   `json:"sourceEvidenceId"`
	ScopeCurrency       string   `json:"scopeCurrency,omitempty"`
	ScopeMaxAmountMinor int64    `json:"scopeMaxAmountMinor,omitempty"`
	ScopeActTypes       []string `json:"scopeActTypes,omitempty"`
	ValidFrom           int64    `json:"validFrom"`
	ValidUntil          int64    `json:"validUntil,omitempty"`
	VerificationStatus  string   `json:"verificationStatus"`
	ProposedByUserID    string   `json:"proposedByUserId"`
}

func representationViewOf(r *servicev1.Representation) (representationView, bool) {
	n := newNaming()
	v := representationView{
		RepresentationID:    r.GetRepresentationId(),
		OrganizationID:      r.GetOrganizationId(),
		PersonID:            r.GetPersonId(),
		PowerType:           name(n, powerTypes, r.GetPowerType()),
		SourceType:          name(n, sourceTypes, r.GetSourceType()),
		SourceEvidenceID:    r.GetSourceEvidenceId(),
		ScopeCurrency:       r.GetScopeCurrency(),
		ScopeMaxAmountMinor: r.GetScopeMaxAmountMinor(),
		ScopeActTypes:       r.GetScopeActTypes(),
		ValidFrom:           r.GetValidFrom(),
		ValidUntil:          r.GetValidUntil(),
		VerificationStatus:  name(n, verificationStatuses, r.GetVerificationStatus()),
		ProposedByUserID:    r.GetProposedByUserId(),
	}
	return v, r != nil && n.known
}

type bankAccountChangeView struct {
	BankAccountChangeID   string `json:"bankAccountChangeId"`
	OrganizationID        string `json:"organizationId"`
	DestinationEvidenceID string `json:"destinationEvidenceId"`
	RequestedByUserID     string `json:"requestedByUserId"`
	ApprovedByUserID      string `json:"approvedByUserId,omitempty"`
	Status                string `json:"status"`
	RequestedAt           int64  `json:"requestedAt"`
	ApprovedAt            int64  `json:"approvedAt,omitempty"`
}

func bankAccountChangeViewOf(c *servicev1.BankAccountChange) (bankAccountChangeView, bool) {
	n := newNaming()
	v := bankAccountChangeView{
		BankAccountChangeID:   c.GetBankAccountChangeId(),
		OrganizationID:        c.GetOrganizationId(),
		DestinationEvidenceID: c.GetDestinationEvidenceId(),
		RequestedByUserID:     c.GetRequestedByUserId(),
		ApprovedByUserID:      c.GetApprovedByUserId(),
		Status:                name(n, bankAccountChangeStatuses, c.GetStatus()),
		RequestedAt:           c.GetRequestedAt(),
		ApprovedAt:            c.GetApprovedAt(),
	}
	return v, c != nil && n.known
}

type externalReferenceView struct {
	ProviderID      string `json:"providerId"`
	System          string `json:"system"`
	ReferenceType   string `json:"referenceType"`
	Reference       string `json:"reference"`
	AsOf            int64  `json:"asOf"`
	ObservedVersion string `json:"observedVersion,omitempty"`
}

func externalReferenceViewOf(r *servicev1.ExternalReference) (externalReferenceView, bool) {
	n := newNaming()
	v := externalReferenceView{
		ProviderID:      r.GetProviderId(),
		System:          name(n, externalSystems, r.GetSystem()),
		ReferenceType:   name(n, externalReferenceTypes, r.GetReferenceType()),
		Reference:       r.GetReference(),
		AsOf:            r.GetAsOf(),
		ObservedVersion: r.GetObservedVersion(),
	}
	return v, r != nil && n.known
}

type establishmentView struct {
	EstablishmentID     string                  `json:"establishmentId"`
	OrganizationID      string                  `json:"organizationId"`
	TaxIDType           string                  `json:"taxIdType"`
	TaxID               string                  `json:"taxId"`
	TaxIDHash           string                  `json:"taxIdHash"`
	TaxIDHashKeyVersion int32                   `json:"taxIdHashKeyVersion"`
	TaxIDMasked         string                  `json:"taxIdMasked"`
	TradeName           string                  `json:"tradeName,omitempty"`
	Receiver            bool                    `json:"receiver"`
	ExternalReferences  []externalReferenceView `json:"externalReferences"`
	CreatedAt           int64                   `json:"createdAt"`
}

func establishmentViewOf(e *servicev1.Establishment) (establishmentView, bool) {
	n := newNaming()
	references, referencesKnown := listOf(e.GetExternalReferences(), externalReferenceViewOf)
	v := establishmentView{
		EstablishmentID:     e.GetEstablishmentId(),
		OrganizationID:      e.GetOrganizationId(),
		TaxIDType:           name(n, taxIDTypes, e.GetTaxIdType()),
		TaxID:               e.GetTaxId(),
		TaxIDHash:           e.GetTaxIdHash(),
		TaxIDHashKeyVersion: e.GetTaxIdHashKeyVersion(),
		TaxIDMasked:         e.GetTaxIdMasked(),
		TradeName:           e.GetTradeName(),
		Receiver:            e.GetReceiver(),
		ExternalReferences:  references,
		CreatedAt:           e.GetCreatedAt(),
	}
	return v, e != nil && n.known && referencesKnown
}

type membershipView struct {
	MembershipID        string `json:"membershipId"`
	OrganizationID      string `json:"organizationId"`
	InviteeEmail        string `json:"inviteeEmail"`
	Role                string `json:"role"`
	Status              string `json:"status"`
	InvitedByUserID     string `json:"invitedByUserId"`
	InvitedAt           int64  `json:"invitedAt"`
	InvitationExpiresAt int64  `json:"invitationExpiresAt"`
	UserID              string `json:"userId,omitempty"`
	PersonID            string `json:"personId,omitempty"`
	VerificationLevel   string `json:"verificationLevel,omitempty"`
	GrantedAt           int64  `json:"grantedAt,omitempty"`
	RevokedAt           int64  `json:"revokedAt,omitempty"`
	RevocationReason    string `json:"revocationReason,omitempty"`
}

func membershipViewOf(m *servicev1.Membership) (membershipView, bool) {
	n := newNaming()
	v := membershipView{
		MembershipID:        m.GetMembershipId(),
		OrganizationID:      m.GetOrganizationId(),
		InviteeEmail:        m.GetInviteeEmail(),
		Role:                name(n, membershipRoles, m.GetRole()),
		Status:              name(n, membershipStatuses, m.GetStatus()),
		InvitedByUserID:     m.GetInvitedByUserId(),
		InvitedAt:           m.GetInvitedAt(),
		InvitationExpiresAt: m.GetInvitationExpiresAt(),
		UserID:              m.GetUserId(),
		PersonID:            m.GetPersonId(),
		VerificationLevel:   optionalName(n, verificationLevels, m.GetVerificationLevel()),
		GrantedAt:           m.GetGrantedAt(),
		RevokedAt:           m.GetRevokedAt(),
		RevocationReason:    m.GetRevocationReason(),
	}
	return v, m != nil && n.known
}

type effectivePermissionsView struct {
	Permissions []string `json:"permissions"`
}

func effectivePermissionsViewOf(resp *servicev1.FindEffectivePermissionsResponse) (any, bool) {
	return effectivePermissionsView{Permissions: append(make([]string, 0, len(resp.GetPermissions())), resp.GetPermissions()...)}, true
}
