package api

import (
	"net/http"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

func (h handlers) findOrganization() http.HandlerFunc {
	return endpoint(fromPath("organizationId", findOrganizationRequest), h.identity.FindOrganization, view(organizationAnswer))
}

func findOrganizationRequest(id string) *servicev1.FindOrganizationRequest {
	return &servicev1.FindOrganizationRequest{OrganizationId: id}
}

func organizationAnswer(resp *servicev1.FindOrganizationResponse) (any, bool) {
	return organizationViewOf(resp.GetOrganization())
}

func (h handlers) findOrganizationByTaxID() http.HandlerFunc {
	return endpoint(
		fromBody("FindOrganizationByTaxIdRequest", invalidFindOrganizationByTaxID, validFindOrganizationByTaxID, findOrganizationByTaxIDRequest),
		h.identity.FindOrganizationByTaxId,
		view(organizationByTaxIDAnswer))
}

func findOrganizationByTaxIDRequest(b findOrganizationByTaxIDBody) *servicev1.FindOrganizationByTaxIdRequest {
	return &servicev1.FindOrganizationByTaxIdRequest{TaxIdType: taxIDTypeOf[*b.TaxIDType], TaxId: *b.TaxID, Jurisdiction: *b.Jurisdiction}
}

func organizationByTaxIDAnswer(resp *servicev1.FindOrganizationByTaxIdResponse) (any, bool) {
	return organizationViewOf(resp.GetOrganization())
}

func (h handlers) findPerson() http.HandlerFunc {
	return endpoint(fromPath("personId", findPersonRequest), h.identity.FindPerson, view(personAnswer))
}

func findPersonRequest(id string) *servicev1.FindPersonRequest {
	return &servicev1.FindPersonRequest{PersonId: id}
}

func personAnswer(resp *servicev1.FindPersonResponse) (any, bool) {
	return personViewOf(resp.GetPerson())
}

func (h handlers) findRepresentation() http.HandlerFunc {
	return endpoint(fromPath("representationId", findRepresentationRequest), h.identity.FindRepresentation, view(representationAnswer))
}

func findRepresentationRequest(id string) *servicev1.FindRepresentationRequest {
	return &servicev1.FindRepresentationRequest{RepresentationId: id}
}

func representationAnswer(resp *servicev1.FindRepresentationResponse) (any, bool) {
	return representationViewOf(resp.GetRepresentation())
}

func (h handlers) findBankAccountChange() http.HandlerFunc {
	return endpoint(fromPath("bankAccountChangeId", findBankAccountChangeRequest), h.identity.FindBankAccountChange, view(bankAccountChangeAnswer))
}

func findBankAccountChangeRequest(id string) *servicev1.FindBankAccountChangeRequest {
	return &servicev1.FindBankAccountChangeRequest{BankAccountChangeId: id}
}

func bankAccountChangeAnswer(resp *servicev1.FindBankAccountChangeResponse) (any, bool) {
	return bankAccountChangeViewOf(resp.GetBankAccountChange())
}

func (h handlers) findEstablishmentsByOrganization() http.HandlerFunc {
	return endpoint(fromPath("organizationId", findEstablishmentsByOrganizationRequest), h.identity.FindEstablishmentsByOrganization, view(establishmentsAnswer))
}

func findEstablishmentsByOrganizationRequest(id string) *servicev1.FindEstablishmentsByOrganizationRequest {
	return &servicev1.FindEstablishmentsByOrganizationRequest{OrganizationId: id}
}

func establishmentsAnswer(resp *servicev1.FindEstablishmentsByOrganizationResponse) (any, bool) {
	return listOf(resp.GetEstablishments(), establishmentViewOf)
}

func (h handlers) findRepresentationsByOrganization() http.HandlerFunc {
	return endpoint(fromPathAndInstant("organizationId", "validAt", findRepresentationsByOrganizationRequest), h.identity.FindRepresentationsByOrganization, view(representationsAnswer))
}

func findRepresentationsByOrganizationRequest(id string, validAt int64) *servicev1.FindRepresentationsByOrganizationRequest {
	return &servicev1.FindRepresentationsByOrganizationRequest{OrganizationId: id, ValidAt: validAt}
}

func representationsAnswer(resp *servicev1.FindRepresentationsByOrganizationResponse) (any, bool) {
	return listOf(resp.GetRepresentations(), representationViewOf)
}

func (h handlers) findMembershipsByOrganization() http.HandlerFunc {
	return endpoint(fromPath("organizationId", findMembershipsByOrganizationRequest), h.identity.FindMembershipsByOrganization, view(membershipsByOrganizationAnswer))
}

func findMembershipsByOrganizationRequest(id string) *servicev1.FindMembershipsByOrganizationRequest {
	return &servicev1.FindMembershipsByOrganizationRequest{OrganizationId: id}
}

func membershipsByOrganizationAnswer(resp *servicev1.FindMembershipsByOrganizationResponse) (any, bool) {
	return listOf(resp.GetMemberships(), membershipViewOf)
}

func (h handlers) findBankAccountChangesByOrganization() http.HandlerFunc {
	return endpoint(fromPath("organizationId", findBankAccountChangesByOrganizationRequest), h.identity.FindBankAccountChangesByOrganization, view(bankAccountChangesAnswer))
}

func findBankAccountChangesByOrganizationRequest(id string) *servicev1.FindBankAccountChangesByOrganizationRequest {
	return &servicev1.FindBankAccountChangesByOrganizationRequest{OrganizationId: id}
}

func bankAccountChangesAnswer(resp *servicev1.FindBankAccountChangesByOrganizationResponse) (any, bool) {
	return listOf(resp.GetBankAccountChanges(), bankAccountChangeViewOf)
}

func (h handlers) findMembershipsByUser() http.HandlerFunc {
	return endpoint(fromPath("userId", findMembershipsByUserRequest), h.identity.FindMembershipsByUser, view(membershipsByUserAnswer))
}

func findMembershipsByUserRequest(id string) *servicev1.FindMembershipsByUserRequest {
	return &servicev1.FindMembershipsByUserRequest{UserId: id}
}

func membershipsByUserAnswer(resp *servicev1.FindMembershipsByUserResponse) (any, bool) {
	return listOf(resp.GetMemberships(), membershipViewOf)
}

func (h handlers) findEffectivePermissions() http.HandlerFunc {
	return endpoint(fromPath("userId", findEffectivePermissionsRequest), h.identity.FindEffectivePermissions, view(effectivePermissionsViewOf))
}

func findEffectivePermissionsRequest(id string) *servicev1.FindEffectivePermissionsRequest {
	return &servicev1.FindEffectivePermissionsRequest{UserId: id}
}

func (h handlers) requestBankAccountChange() http.HandlerFunc {
	return endpoint(
		acting(fromPathAndBody("organizationId", "RequestBankAccountChangeRequest", invalidRequestBankAccountChange,
			validRequestBankAccountChange, requestBankAccountChangeRequest),
			func(req *servicev1.RequestBankAccountChangeRequest, actor string) { req.RequestedByUserId = actor }),
		h.identity.RequestBankAccountChange,
		outcome(http.StatusCreated, requestedBankAccountChangeOf))
}

func requestBankAccountChangeRequest(organizationID string, b requestBankAccountChangeBody) *servicev1.RequestBankAccountChangeRequest {
	return &servicev1.RequestBankAccountChangeRequest{
		BankAccountChangeId:   *b.BankAccountChangeID,
		OrganizationId:        organizationID,
		DestinationEvidenceId: *b.DestinationEvidenceID,
	}
}

func (h handlers) registerOrganization() http.HandlerFunc {
	return endpoint(
		fromBody("RegisterOrganizationRequest", invalidRegisterOrganization, validRegisterOrganization, registerOrganizationRequest),
		h.identity.RegisterOrganization,
		outcome(http.StatusCreated, registeredOrganizationOf))
}

func registerOrganizationRequest(b registerOrganizationBody) *servicev1.RegisterOrganizationRequest {
	kinds := make([]servicev1.OrganizationKind, 0, len(b.Kinds))
	for _, kind := range b.Kinds {
		kinds = append(kinds, organizationKindOf[kind])
	}
	return &servicev1.RegisterOrganizationRequest{
		OrganizationId: *b.OrganizationID,
		Kinds:          kinds,
		Jurisdiction:   *b.Jurisdiction,
		TaxIdType:      taxIDTypeOf[*b.TaxIDType],
		TaxId:          *b.TaxID,
		LegalName:      *b.LegalName,
		TradeName:      b.TradeName,
	}
}

func (h handlers) addEstablishment() http.HandlerFunc {
	return endpoint(
		fromPathAndBody("organizationId", "AddEstablishmentRequest", invalidAddEstablishment, validAddEstablishment, addEstablishmentRequest),
		h.identity.AddEstablishment,
		outcome(http.StatusCreated, addedEstablishmentOf))
}

func addEstablishmentRequest(organizationID string, b addEstablishmentBody) *servicev1.AddEstablishmentRequest {
	references := make([]*servicev1.ExternalReference, 0, len(b.ExternalReferences))
	for _, r := range b.ExternalReferences {
		references = append(references, &servicev1.ExternalReference{
			ProviderId:      *r.ProviderID,
			System:          externalSystemOf[*r.System],
			ReferenceType:   externalReferenceTypeOf[*r.ReferenceType],
			Reference:       *r.Reference,
			AsOf:            *r.AsOf,
			ObservedVersion: r.ObservedVersion,
		})
	}
	return &servicev1.AddEstablishmentRequest{
		EstablishmentId:    *b.EstablishmentID,
		OrganizationId:     organizationID,
		TaxIdType:          taxIDTypeOf[*b.TaxIDType],
		TaxId:              *b.TaxID,
		TradeName:          b.TradeName,
		Receiver:           *b.Receiver,
		ExternalReferences: references,
	}
}

func (h handlers) registerPerson() http.HandlerFunc {
	return endpoint(
		fromBody("RegisterPersonRequest", invalidRegisterPerson, validRegisterPerson, registerPersonRequest),
		h.identity.RegisterPerson,
		outcome(http.StatusCreated, registeredPersonOf))
}

func registerPersonRequest(b registerPersonBody) *servicev1.RegisterPersonRequest {
	return &servicev1.RegisterPersonRequest{PersonId: *b.PersonID, FullName: *b.FullName, Document: *b.Document}
}

func (h handlers) provisionUserAccount() http.HandlerFunc {
	return endpoint(
		fromBody("ProvisionUserAccountRequest", invalidProvisionUserAccount, validProvisionUserAccount, provisionUserAccountRequest),
		h.identity.ProvisionUserAccount,
		outcome(http.StatusCreated, provisionedUserAccountOf))
}

func provisionUserAccountRequest(b provisionUserAccountBody) *servicev1.ProvisionUserAccountRequest {
	req := &servicev1.ProvisionUserAccountRequest{Subject: *b.Subject}
	if b.PersonID != nil {
		req.PersonId = *b.PersonID
	}
	return req
}

func (h handlers) registerRepresentation() http.HandlerFunc {
	return endpoint(
		acting(fromPathAndBody("organizationId", "RegisterRepresentationRequest", invalidRegisterRepresentation,
			validRegisterRepresentation, registerRepresentationRequest),
			func(req *servicev1.RegisterRepresentationRequest, actor string) { req.ProposedByUserId = actor }),
		h.identity.RegisterRepresentation,
		outcome(http.StatusCreated, registeredRepresentationOf))
}

func registerRepresentationRequest(organizationID string, b registerRepresentationBody) *servicev1.RegisterRepresentationRequest {
	var actTypes []string
	for _, actType := range b.ScopeActTypes {
		actTypes = append(actTypes, *actType)
	}
	return &servicev1.RegisterRepresentationRequest{
		RepresentationId:    *b.RepresentationID,
		OrganizationId:      organizationID,
		PersonId:            *b.PersonID,
		PowerType:           powerTypeOf[*b.PowerType],
		SourceType:          sourceTypeOf[*b.SourceType],
		SourceEvidenceId:    *b.SourceEvidenceID,
		ScopeCurrency:       b.ScopeCurrency,
		ScopeMaxAmountMinor: b.ScopeMaxAmountMinor,
		ScopeActTypes:       actTypes,
		ValidFrom:           *b.ValidFrom,
		ValidUntil:          b.ValidUntil,
	}
}

func (h handlers) inviteUser() http.HandlerFunc {
	return endpoint(
		acting(fromPathAndBody("organizationId", "InviteUserRequest", invalidInviteUser, validInviteUser, inviteUserRequest),
			func(req *servicev1.InviteUserRequest, actor string) { req.InvitedByUserId = actor }),
		h.identity.InviteUser,
		outcome(http.StatusCreated, invitedUserOf))
}

func inviteUserRequest(organizationID string, b inviteUserBody) *servicev1.InviteUserRequest {
	return &servicev1.InviteUserRequest{
		MembershipId:        *b.MembershipID,
		OrganizationId:      organizationID,
		InviteeEmail:        *b.InviteeEmail,
		Role:                membershipRoleOf[*b.Role],
		InvitationExpiresAt: *b.InvitationExpiresAt,
	}
}

func (h handlers) suspendUser() http.HandlerFunc {
	return endpoint(
		fromPathAndBody("userId", "SuspendUserRequest", invalidReason, validReason, suspendUserRequest),
		h.identity.SuspendUser,
		outcome(http.StatusOK, suspendedUserOf))
}

func suspendUserRequest(userID string, b reasonBody) *servicev1.SuspendUserRequest {
	return &servicev1.SuspendUserRequest{UserId: userID, Reason: *b.Reason}
}

func (h handlers) revokeMembership() http.HandlerFunc {
	return endpoint(
		fromPathAndBody("membershipId", "RevokeMembershipRequest", invalidReason, validReason, revokeMembershipRequest),
		h.identity.RevokeMembership,
		outcome(http.StatusOK, revokedMembershipOf))
}

func revokeMembershipRequest(membershipID string, b reasonBody) *servicev1.RevokeMembershipRequest {
	return &servicev1.RevokeMembershipRequest{MembershipId: membershipID, Reason: *b.Reason}
}

func (h handlers) approveBankAccountChange() http.HandlerFunc {
	return endpoint(
		acting(fromPathWithoutBody("bankAccountChangeId", approveBankAccountChangeRequest),
			func(req *servicev1.ApproveBankAccountChangeRequest, actor string) { req.ApprovedByUserId = actor }),
		h.identity.ApproveBankAccountChange,
		outcome(http.StatusOK, approvedBankAccountChangeOf))
}

func approveBankAccountChangeRequest(bankAccountChangeID string) *servicev1.ApproveBankAccountChangeRequest {
	return &servicev1.ApproveBankAccountChangeRequest{BankAccountChangeId: bankAccountChangeID}
}
