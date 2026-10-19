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
