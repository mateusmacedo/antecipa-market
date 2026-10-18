package application

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

var registerPartnerCodec = usecase.OutcomeCodec[domain.RegisterPartnerResponse]{
	Encode: func(e *usecase.Encoder, r domain.RegisterPartnerResponse) { e.String(string(r.PartnerID)) },
	Decode: func(d *usecase.Decoder) domain.RegisterPartnerResponse {
		return domain.RegisterPartnerResponse{PartnerID: domain.PartnerID(d.String())}
	},
}

var registerAgreementCodec = usecase.OutcomeCodec[domain.RegisterAgreementResponse]{
	Encode: func(e *usecase.Encoder, r domain.RegisterAgreementResponse) { e.String(string(r.AgreementID)) },
	Decode: func(d *usecase.Decoder) domain.RegisterAgreementResponse {
		return domain.RegisterAgreementResponse{AgreementID: domain.AgreementID(d.String())}
	},
}

var activateAgreementCodec = usecase.OutcomeCodec[domain.ActivateAgreementResponse]{
	Encode: func(e *usecase.Encoder, r domain.ActivateAgreementResponse) { e.String(string(r.AgreementID)) },
	Decode: func(d *usecase.Decoder) domain.ActivateAgreementResponse {
		return domain.ActivateAgreementResponse{AgreementID: domain.AgreementID(d.String())}
	},
}

var registerIntegrationProfileCodec = usecase.OutcomeCodec[domain.RegisterIntegrationProfileResponse]{
	Encode: func(e *usecase.Encoder, r domain.RegisterIntegrationProfileResponse) { e.String(string(r.ProfileID)) },
	Decode: func(d *usecase.Decoder) domain.RegisterIntegrationProfileResponse {
		return domain.RegisterIntegrationProfileResponse{ProfileID: domain.ProfileID(d.String())}
	},
}

var raiseCertificateExpiryAlertCodec = usecase.OutcomeCodec[domain.RaiseCertificateExpiryAlertResponse]{
	Encode: func(e *usecase.Encoder, r domain.RaiseCertificateExpiryAlertResponse) { e.String(string(r.ProfileID)) },
	Decode: func(d *usecase.Decoder) domain.RaiseCertificateExpiryAlertResponse {
		return domain.RaiseCertificateExpiryAlertResponse{ProfileID: domain.ProfileID(d.String())}
	},
}

var defineOperationalPolicyCodec = usecase.OutcomeCodec[domain.DefineOperationalPolicyResponse]{
	Encode: func(e *usecase.Encoder, r domain.DefineOperationalPolicyResponse) { e.String(string(r.PolicyID)) },
	Decode: func(d *usecase.Decoder) domain.DefineOperationalPolicyResponse {
		return domain.DefineOperationalPolicyResponse{PolicyID: domain.PolicyID(d.String())}
	},
}

var changePartnerStageCodec = usecase.OutcomeCodec[domain.ChangePartnerStageResponse]{
	Encode: func(e *usecase.Encoder, r domain.ChangePartnerStageResponse) {
		e.String(string(r.PartnerID))
		e.String(string(r.Status))
	},
	Decode: func(d *usecase.Decoder) domain.ChangePartnerStageResponse {
		return domain.ChangePartnerStageResponse{PartnerID: domain.PartnerID(d.String()), Status: domain.PartnerStatus(d.String())}
	},
}

var declareCapabilityCodec = capabilityCodec(
	func(r domain.DeclareCapabilityResponse) domain.CapabilityID { return r.CapabilityID },
	func(id domain.CapabilityID) domain.DeclareCapabilityResponse {
		return domain.DeclareCapabilityResponse{CapabilityID: id}
	},
)

var recordHomologationCodec = usecase.OutcomeCodec[domain.RecordHomologationResponse]{
	Encode: func(e *usecase.Encoder, r domain.RecordHomologationResponse) {
		e.String(string(r.CapabilityID))
		e.String(string(r.Status))
	},
	Decode: func(d *usecase.Decoder) domain.RecordHomologationResponse {
		return domain.RecordHomologationResponse{CapabilityID: domain.CapabilityID(d.String()), Status: domain.CapabilityStatus(d.String())}
	},
}

var enableCapabilityCodec = capabilityCodec(
	func(r domain.EnableCapabilityResponse) domain.CapabilityID { return r.CapabilityID },
	func(id domain.CapabilityID) domain.EnableCapabilityResponse {
		return domain.EnableCapabilityResponse{CapabilityID: id}
	},
)

var suspendCapabilityCodec = capabilityCodec(
	func(r domain.SuspendCapabilityResponse) domain.CapabilityID { return r.CapabilityID },
	func(id domain.CapabilityID) domain.SuspendCapabilityResponse {
		return domain.SuspendCapabilityResponse{CapabilityID: id}
	},
)

var changeRolloutCodec = capabilityCodec(
	func(r domain.ChangeRolloutResponse) domain.CapabilityID { return r.CapabilityID },
	func(id domain.CapabilityID) domain.ChangeRolloutResponse {
		return domain.ChangeRolloutResponse{CapabilityID: id}
	},
)

func capabilityCodec[R any](idOf func(R) domain.CapabilityID, responseOf func(domain.CapabilityID) R) usecase.OutcomeCodec[R] {
	return usecase.OutcomeCodec[R]{
		Encode: func(e *usecase.Encoder, r R) { e.String(string(idOf(r))) },
		Decode: func(d *usecase.Decoder) R { return responseOf(domain.CapabilityID(d.String())) },
	}
}

var defineProductCodec = productCodec(
	func(r domain.DefineProductResponse) domain.ProductID { return r.ProductID },
	func(id domain.ProductID) domain.DefineProductResponse {
		return domain.DefineProductResponse{ProductID: id}
	},
)

var retireProductCodec = productCodec(
	func(r domain.RetireProductResponse) domain.ProductID { return r.ProductID },
	func(id domain.ProductID) domain.RetireProductResponse {
		return domain.RetireProductResponse{ProductID: id}
	},
)

func productCodec[R any](idOf func(R) domain.ProductID, responseOf func(domain.ProductID) R) usecase.OutcomeCodec[R] {
	return usecase.OutcomeCodec[R]{
		Encode: func(e *usecase.Encoder, r R) { e.String(string(idOf(r))) },
		Decode: func(d *usecase.Decoder) R { return responseOf(domain.ProductID(d.String())) },
	}
}
