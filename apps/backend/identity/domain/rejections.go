package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

const (
	CodeOrganizationKindsEmpty          kernel.Code = "identity-organization/organization/kinds-empty"
	CodeOrganizationJurisdictionInvalid kernel.Code = "identity-organization/organization/jurisdiction-invalid"
	CodeOrganizationTaxIDEmpty          kernel.Code = "identity-organization/organization/tax-id-empty"
	CodeOrganizationLegalNameEmpty      kernel.Code = "identity-organization/organization/legal-name-empty"
	CodeOrganizationDuplicateTaxID      kernel.Code = "identity-organization/organization/duplicate-tax-id"

	CodeEstablishmentOrganizationNotActive kernel.Code = "identity-organization/establishment/organization-not-active"
	CodeEstablishmentTaxIDEmpty            kernel.Code = "identity-organization/establishment/tax-id-empty"
	CodeEstablishmentDuplicateTaxID        kernel.Code = "identity-organization/establishment/duplicate-tax-id"

	CodePersonFullNameEmpty     kernel.Code = "identity-organization/person/full-name-empty"
	CodePersonDocumentEmpty     kernel.Code = "identity-organization/person/document-empty"
	CodePersonDuplicateDocument kernel.Code = "identity-organization/person/duplicate-document"

	CodeRepresentationSourceEvidenceMissing kernel.Code = "identity-organization/representation/source-evidence-missing"
	CodeRepresentationValidityInvalid       kernel.Code = "identity-organization/representation/validity-invalid"
	CodeRepresentationScopeInvalid          kernel.Code = "identity-organization/representation/scope-invalid"
	CodeRepresentationOrganizationNotActive kernel.Code = "identity-organization/representation/organization-not-active"
	CodeRepresentationPersonNotFound        kernel.Code = "identity-organization/representation/person-not-found"

	CodeUserAccountSubjectEmpty kernel.Code = "identity-organization/user-account/subject-empty"
	CodeUserAccountNotActive    kernel.Code = "identity-organization/user-account/not-active"
	CodeUserAccountReasonEmpty  kernel.Code = "identity-organization/user-account/reason-empty"

	CodeMembershipInviteeEmailEmpty        kernel.Code = "identity-organization/membership/invitee-email-empty"
	CodeMembershipInvitationExpiryInvalid  kernel.Code = "identity-organization/membership/invitation-expiry-invalid"
	CodeMembershipOrganizationNotActive    kernel.Code = "identity-organization/membership/organization-not-active"
	CodeMembershipRoleKindMismatch         kernel.Code = "identity-organization/membership/role-kind-mismatch"
	CodeMembershipNotInvited               kernel.Code = "identity-organization/membership/not-invited"
	CodeMembershipInvitationExpired        kernel.Code = "identity-organization/membership/invitation-expired"
	CodeMembershipChannelNotConfirmed      kernel.Code = "identity-organization/membership/channel-not-confirmed"
	CodeMembershipVerificationInsufficient kernel.Code = "identity-organization/membership/verification-insufficient"
	CodeMembershipUserNotActive            kernel.Code = "identity-organization/membership/user-not-active"
	CodeMembershipNotActive                kernel.Code = "identity-organization/membership/not-active"

	CodeBankAccountChangeDestinationEvidenceMissing kernel.Code = "identity-organization/bank-account-change/destination-evidence-missing"
	CodeBankAccountChangeOrganizationNotActive      kernel.Code = "identity-organization/bank-account-change/organization-not-active"
	CodeBankAccountChangeNotRequested               kernel.Code = "identity-organization/bank-account-change/not-requested"
	CodeBankAccountChangeSameActorApproval          kernel.Code = "identity-organization/bank-account-change/same-actor-approval"
)

// The duplicate rejections are static: the unique constraint raises them from
// the Save, where no command value is at hand to detail (ADR-066).
var (
	RejectOrganizationDuplicateTaxID  = kernel.Reject(CodeOrganizationDuplicateTaxID, "tax id already registered in the jurisdiction")
	RejectEstablishmentDuplicateTaxID = kernel.Reject(CodeEstablishmentDuplicateTaxID, "tax id already registered in the organization")
	RejectPersonDuplicateDocument     = kernel.Reject(CodePersonDuplicateDocument, "document already registered")
)
