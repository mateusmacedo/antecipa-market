package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

// The rejection codes are published to the callers of the context: a rename in
// Go must not reach the values on the wire.
func TestRejectionCodesKeepTheirPublishedValues(t *testing.T) {
	codes := map[kernel.Code]string{
		domain.CodeOrganizationKindsEmpty:              "identity-organization/organization/kinds-empty",
		domain.CodeOrganizationJurisdictionInvalid:     "identity-organization/organization/jurisdiction-invalid",
		domain.CodeOrganizationTaxIDEmpty:              "identity-organization/organization/tax-id-empty",
		domain.CodeOrganizationLegalNameEmpty:          "identity-organization/organization/legal-name-empty",
		domain.CodeOrganizationDuplicateTaxID:          "identity-organization/organization/duplicate-tax-id",
		domain.CodeEstablishmentOrganizationNotActive:  "identity-organization/establishment/organization-not-active",
		domain.CodeEstablishmentTaxIDEmpty:             "identity-organization/establishment/tax-id-empty",
		domain.CodeEstablishmentDuplicateTaxID:         "identity-organization/establishment/duplicate-tax-id",
		domain.CodePersonFullNameEmpty:                 "identity-organization/person/full-name-empty",
		domain.CodePersonDocumentEmpty:                 "identity-organization/person/document-empty",
		domain.CodePersonDuplicateDocument:             "identity-organization/person/duplicate-document",
		domain.CodeRepresentationSourceEvidenceMissing: "identity-organization/representation/source-evidence-missing",
		domain.CodeRepresentationValidityInvalid:       "identity-organization/representation/validity-invalid",
		domain.CodeRepresentationScopeInvalid:          "identity-organization/representation/scope-invalid",
		domain.CodeRepresentationOrganizationNotActive: "identity-organization/representation/organization-not-active",
		domain.CodeRepresentationPersonNotFound:        "identity-organization/representation/person-not-found",

		domain.CodeUserAccountSubjectEmpty:            "identity-organization/user-account/subject-empty",
		domain.CodeUserAccountNotActive:               "identity-organization/user-account/not-active",
		domain.CodeUserAccountReasonEmpty:             "identity-organization/user-account/reason-empty",
		domain.CodeMembershipInviteeEmailEmpty:        "identity-organization/membership/invitee-email-empty",
		domain.CodeMembershipInvitationExpiryInvalid:  "identity-organization/membership/invitation-expiry-invalid",
		domain.CodeMembershipOrganizationNotActive:    "identity-organization/membership/organization-not-active",
		domain.CodeMembershipRoleKindMismatch:         "identity-organization/membership/role-kind-mismatch",
		domain.CodeMembershipNotInvited:               "identity-organization/membership/not-invited",
		domain.CodeMembershipInvitationExpired:        "identity-organization/membership/invitation-expired",
		domain.CodeMembershipChannelNotConfirmed:      "identity-organization/membership/channel-not-confirmed",
		domain.CodeMembershipVerificationInsufficient: "identity-organization/membership/verification-insufficient",
		domain.CodeMembershipUserNotActive:            "identity-organization/membership/user-not-active",
		domain.CodeMembershipNotActive:                "identity-organization/membership/not-active",

		domain.CodeBankAccountChangeDestinationEvidenceMissing: "identity-organization/bank-account-change/destination-evidence-missing",
		domain.CodeBankAccountChangeOrganizationNotActive:      "identity-organization/bank-account-change/organization-not-active",
		domain.CodeBankAccountChangeNotRequested:               "identity-organization/bank-account-change/not-requested",
		domain.CodeBankAccountChangeSameActorApproval:          "identity-organization/bank-account-change/same-actor-approval",
	}
	for code, want := range codes {
		if string(code) != want {
			t.Errorf("code = %q, want the published value %q", code, want)
		}
		if !code.Valid() {
			t.Errorf("code %q is not a valid context/aggregate/reason code", code)
		}
	}
}
