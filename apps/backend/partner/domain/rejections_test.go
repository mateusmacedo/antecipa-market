package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestRejectionCodesKeepTheirPublishedValues(t *testing.T) {
	codes := map[kernel.Code]string{
		domain.CodePartnerOrganizationNotFound:     "partner-management/partner/organization-not-found",
		domain.CodePartnerOrganizationKindMismatch: "partner-management/partner/organization-kind-mismatch",
		domain.CodePartnerAlreadyRegistered:        "partner-management/partner/already-registered",
		domain.CodePartnerInvalidStageTransition:   "partner-management/partner/invalid-stage-transition",
		domain.CodePartnerReasonEmpty:              "partner-management/partner/reason-empty",
		domain.CodePartnerAgreementNotActive:       "partner-management/partner/agreement-not-active",
		domain.CodePartnerNoEnabledCapability:      "partner-management/partner/no-enabled-capability",

		domain.CodePartnerAgreementValidityInvalid:               "partner-management/partner-agreement/validity-invalid",
		domain.CodePartnerAgreementSupersedesInvalid:             "partner-management/partner-agreement/supersedes-invalid",
		domain.CodePartnerAgreementNotDraft:                      "partner-management/partner-agreement/not-draft",
		domain.CodePartnerAgreementContractEvidenceMissing:       "partner-management/partner-agreement/contract-evidence-missing",
		domain.CodePartnerAgreementContractualCriteriaIncomplete: "partner-management/partner-agreement/contractual-criteria-incomplete",

		domain.CodeIntegrationProfileEndpointEmpty:            "partner-management/integration-profile/endpoint-empty",
		domain.CodeIntegrationProfileSecretRefMissing:         "partner-management/integration-profile/secret-ref-missing",
		domain.CodeIntegrationProfileCertificateExpiryInvalid: "partner-management/integration-profile/certificate-expiry-invalid",
		domain.CodeIntegrationProfileSupersedesInvalid:        "partner-management/integration-profile/supersedes-invalid",
		domain.CodeIntegrationProfileInvalidAlertThreshold:    "partner-management/integration-profile/invalid-alert-threshold",
		domain.CodeIntegrationProfileThresholdNotReached:      "partner-management/integration-profile/threshold-not-reached",
		domain.CodeIntegrationProfileAlreadyAlerted:           "partner-management/integration-profile/already-alerted",

		domain.CodeOperationalPolicyRateLimitInvalid:  "partner-management/operational-policy/rate-limit-invalid",
		domain.CodeOperationalPolicyContactsMissing:   "partner-management/operational-policy/contacts-missing",
		domain.CodeOperationalPolicySupersedesInvalid: "partner-management/operational-policy/supersedes-invalid",

		domain.CodeCapabilityOperationEmpty:                  "partner-management/capability/operation-empty",
		domain.CodeCapabilityAssetTypeEmpty:                  "partner-management/capability/asset-type-empty",
		domain.CodeCapabilityStateQueryRestrictionUndeclared: "partner-management/capability/state-query-restriction-undeclared",
		domain.CodeCapabilitySupersedesInvalid:               "partner-management/capability/supersedes-invalid",
		domain.CodeCapabilityCriteriaEmpty:                   "partner-management/capability/criteria-empty",
		domain.CodeCapabilityUnknownCriterion:                "partner-management/capability/unknown-criterion",
		domain.CodeCapabilityHomologationClosed:              "partner-management/capability/homologation-closed",
		domain.CodeCapabilityNotHomologated:                  "partner-management/capability/not-homologated",
		domain.CodeCapabilityRehomologationRequired:          "partner-management/capability/rehomologation-required",
		domain.CodeCapabilityAgreementNotActive:              "partner-management/capability/agreement-not-active",
		domain.CodeCapabilityIntegrationProfileMissing:       "partner-management/capability/integration-profile-missing",
		domain.CodeCapabilityNotEnabled:                      "partner-management/capability/not-enabled",
		domain.CodeCapabilityReasonEmpty:                     "partner-management/capability/reason-empty",
		domain.CodeCapabilityPercentageOutOfRange:            "partner-management/capability/percentage-out-of-range",
		domain.CodeCapabilityAllowlistEmpty:                  "partner-management/capability/allowlist-empty",

		domain.CodeFinancialProductAssetTypesEmpty:    "partner-management/financial-product/asset-types-empty",
		domain.CodeFinancialProductRangeInvalid:       "partner-management/financial-product/range-invalid",
		domain.CodeFinancialProductValidityInvalid:    "partner-management/financial-product/validity-invalid",
		domain.CodeFinancialProductAgreementNotActive: "partner-management/financial-product/agreement-not-active",
		domain.CodeFinancialProductSupersedesInvalid:  "partner-management/financial-product/supersedes-invalid",
		domain.CodeFinancialProductNotActive:          "partner-management/financial-product/not-active",
		domain.CodeFinancialProductNotYetExpired:      "partner-management/financial-product/not-yet-expired",
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
