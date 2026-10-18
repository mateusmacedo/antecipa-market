package app

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
)

func Authorization() usecase.Authorize[application.Operation] {
	return usecase.Permitted(permissionOf)
}

func permissionOf(op application.Operation) port.Permission {
	switch op.(type) {
	case application.RegisterPartner:
		return "partner:register-partner"
	case application.RegisterAgreement:
		return "partner:register-agreement"
	case application.ActivateAgreement:
		return "partner:activate-agreement"
	case application.RegisterIntegrationProfile:
		return "partner:register-integration-profile"
	case application.RaiseCertificateExpiryAlert:
		return "partner:raise-certificate-expiry-alert"
	case application.DefineOperationalPolicy:
		return "partner:define-operational-policy"
	case application.ChangePartnerStage:
		return "partner:change-partner-stage"
	case application.DeclareCapability:
		return "partner:declare-capability"
	case application.RecordHomologation:
		return "partner:record-homologation"
	case application.EnableCapability:
		return "partner:enable-capability"
	case application.SuspendCapability:
		return "partner:suspend-capability"
	case application.ChangeRollout:
		return "partner:change-rollout"
	case application.DefineProduct:
		return "partner:define-product"
	case application.RetireProduct:
		return "partner:retire-product"
	case application.FindPartner:
		return "partner:find-partner"
	case application.FindPartnersByOrganization:
		return "partner:find-partners-by-organization"
	case application.FindAgreementsByPartner:
		return "partner:find-agreements-by-partner"
	case application.FindCapabilitiesByPartner:
		return "partner:find-capabilities-by-partner"
	case application.FindEffectiveCapability:
		return "partner:find-effective-capability"
	case application.FindEligiblePartners:
		return "partner:find-eligible-partners"
	case application.FindProductsByPartner:
		return "partner:find-products-by-partner"
	case application.FindIntegrationProfile:
		return "partner:find-integration-profile"
	case application.FindOperationalPolicy:
		return "partner:find-operational-policy"
	case application.FindCertificatesExpiring:
		return "partner:find-certificates-expiring"
	default:
		return ""
	}
}
