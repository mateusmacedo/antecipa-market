package application

var (
	RegisterPartnerCodec   = registerPartnerCodec
	RegisterAgreementCodec = registerAgreementCodec
	ActivateAgreementCodec = activateAgreementCodec

	RegisterIntegrationProfileCodec  = registerIntegrationProfileCodec
	RaiseCertificateExpiryAlertCodec = raiseCertificateExpiryAlertCodec
	DefineOperationalPolicyCodec     = defineOperationalPolicyCodec

	ChangePartnerStageCodec = changePartnerStageCodec
	DeclareCapabilityCodec  = declareCapabilityCodec
	RecordHomologationCodec = recordHomologationCodec
	EnableCapabilityCodec   = enableCapabilityCodec
	SuspendCapabilityCodec  = suspendCapabilityCodec
	ChangeRolloutCodec      = changeRolloutCodec

	DefineProductCodec = defineProductCodec
	RetireProductCodec = retireProductCodec
)
