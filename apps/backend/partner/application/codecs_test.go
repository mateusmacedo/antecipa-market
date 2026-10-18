package application_test

import (
	"bytes"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestTheResponsesKeepTheirStoredFormat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stored []byte
		golden []byte
	}{
		{
			"register partner",
			usecase.EncodeOutcome(application.RegisterPartnerCodec, usecase.Accepted(domain.RegisterPartnerResponse{PartnerID: "prt-100"})),
			[]byte{0x01, 'a', 0x07, 'p', 'r', 't', '-', '1', '0', '0'},
		},
		{
			"register agreement",
			usecase.EncodeOutcome(application.RegisterAgreementCodec, usecase.Accepted(domain.RegisterAgreementResponse{AgreementID: "agr-100"})),
			[]byte{0x01, 'a', 0x07, 'a', 'g', 'r', '-', '1', '0', '0'},
		},
		{
			"activate agreement",
			usecase.EncodeOutcome(application.ActivateAgreementCodec, usecase.Accepted(domain.ActivateAgreementResponse{AgreementID: "agr-100"})),
			[]byte{0x01, 'a', 0x07, 'a', 'g', 'r', '-', '1', '0', '0'},
		},
		{
			"register integration profile",
			usecase.EncodeOutcome(application.RegisterIntegrationProfileCodec, usecase.Accepted(domain.RegisterIntegrationProfileResponse{ProfileID: "ipf-100"})),
			[]byte{0x01, 'a', 0x07, 'i', 'p', 'f', '-', '1', '0', '0'},
		},
		{
			"raise certificate expiry alert",
			usecase.EncodeOutcome(application.RaiseCertificateExpiryAlertCodec, usecase.Accepted(domain.RaiseCertificateExpiryAlertResponse{ProfileID: "ipf-100"})),
			[]byte{0x01, 'a', 0x07, 'i', 'p', 'f', '-', '1', '0', '0'},
		},
		{
			"define operational policy",
			usecase.EncodeOutcome(application.DefineOperationalPolicyCodec, usecase.Accepted(domain.DefineOperationalPolicyResponse{PolicyID: "pol-100"})),
			[]byte{0x01, 'a', 0x07, 'p', 'o', 'l', '-', '1', '0', '0'},
		},
		{
			"change partner stage",
			usecase.EncodeOutcome(application.ChangePartnerStageCodec, usecase.Accepted(domain.ChangePartnerStageResponse{PartnerID: "prt-100", Status: domain.PartnerActive})),
			[]byte{0x01, 'a', 0x07, 'p', 'r', 't', '-', '1', '0', '0', 0x06, 'a', 'c', 't', 'i', 'v', 'e'},
		},
		{
			"declare capability",
			usecase.EncodeOutcome(application.DeclareCapabilityCodec, usecase.Accepted(domain.DeclareCapabilityResponse{CapabilityID: "cap-100"})),
			[]byte{0x01, 'a', 0x07, 'c', 'a', 'p', '-', '1', '0', '0'},
		},
		{
			"record homologation",
			usecase.EncodeOutcome(application.RecordHomologationCodec, usecase.Accepted(domain.RecordHomologationResponse{CapabilityID: "cap-100", Status: domain.CapabilityStatusDeclared})),
			[]byte{0x01, 'a', 0x07, 'c', 'a', 'p', '-', '1', '0', '0', 0x08, 'd', 'e', 'c', 'l', 'a', 'r', 'e', 'd'},
		},
		{
			"enable capability",
			usecase.EncodeOutcome(application.EnableCapabilityCodec, usecase.Accepted(domain.EnableCapabilityResponse{CapabilityID: "cap-100"})),
			[]byte{0x01, 'a', 0x07, 'c', 'a', 'p', '-', '1', '0', '0'},
		},
		{
			"suspend capability",
			usecase.EncodeOutcome(application.SuspendCapabilityCodec, usecase.Accepted(domain.SuspendCapabilityResponse{CapabilityID: "cap-100"})),
			[]byte{0x01, 'a', 0x07, 'c', 'a', 'p', '-', '1', '0', '0'},
		},
		{
			"change rollout",
			usecase.EncodeOutcome(application.ChangeRolloutCodec, usecase.Accepted(domain.ChangeRolloutResponse{CapabilityID: "cap-100"})),
			[]byte{0x01, 'a', 0x07, 'c', 'a', 'p', '-', '1', '0', '0'},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !bytes.Equal(tc.stored, tc.golden) {
				t.Fatalf("stored = %x, want %x: an entry written by the previous release must still decode", tc.stored, tc.golden)
			}
		})
	}
}

func TestTheResponsesRoundTrip(t *testing.T) {
	partner, errPartner := usecase.DecodeOutcome(application.RegisterPartnerCodec,
		usecase.EncodeOutcome(application.RegisterPartnerCodec, usecase.Accepted(domain.RegisterPartnerResponse{PartnerID: "prt-9"})))
	registered, errRegistered := usecase.DecodeOutcome(application.RegisterAgreementCodec,
		usecase.EncodeOutcome(application.RegisterAgreementCodec, usecase.Accepted(domain.RegisterAgreementResponse{AgreementID: "agr-9"})))
	activated, errActivated := usecase.DecodeOutcome(application.ActivateAgreementCodec,
		usecase.EncodeOutcome(application.ActivateAgreementCodec, usecase.Accepted(domain.ActivateAgreementResponse{AgreementID: "agr-9"})))

	if errPartner != nil || partner.Response().PartnerID != "prt-9" ||
		errRegistered != nil || registered.Response().AgreementID != "agr-9" ||
		errActivated != nil || activated.Response().AgreementID != "agr-9" {
		t.Fatalf("decoded = %+v %v, %+v %v, %+v %v; want each response back",
			partner.Response(), errPartner, registered.Response(), errRegistered, activated.Response(), errActivated)
	}
}

func TestTheProfileAndPolicyResponsesRoundTrip(t *testing.T) {
	profile, errProfile := usecase.DecodeOutcome(application.RegisterIntegrationProfileCodec,
		usecase.EncodeOutcome(application.RegisterIntegrationProfileCodec, usecase.Accepted(domain.RegisterIntegrationProfileResponse{ProfileID: "ipf-9"})))
	alert, errAlert := usecase.DecodeOutcome(application.RaiseCertificateExpiryAlertCodec,
		usecase.EncodeOutcome(application.RaiseCertificateExpiryAlertCodec, usecase.Accepted(domain.RaiseCertificateExpiryAlertResponse{ProfileID: "ipf-9"})))
	policy, errPolicy := usecase.DecodeOutcome(application.DefineOperationalPolicyCodec,
		usecase.EncodeOutcome(application.DefineOperationalPolicyCodec, usecase.Accepted(domain.DefineOperationalPolicyResponse{PolicyID: "pol-9"})))

	if errProfile != nil || profile.Response().ProfileID != "ipf-9" ||
		errAlert != nil || alert.Response().ProfileID != "ipf-9" ||
		errPolicy != nil || policy.Response().PolicyID != "pol-9" {
		t.Fatalf("decoded = %+v %v, %+v %v, %+v %v; want each response back",
			profile.Response(), errProfile, alert.Response(), errAlert, policy.Response(), errPolicy)
	}
}

func TestTheStageAndCapabilityResponsesRoundTrip(t *testing.T) {
	stage, errStage := usecase.DecodeOutcome(application.ChangePartnerStageCodec,
		usecase.EncodeOutcome(application.ChangePartnerStageCodec, usecase.Accepted(domain.ChangePartnerStageResponse{PartnerID: "prt-9", Status: domain.PartnerSuspended})))
	homologation, errHomologation := usecase.DecodeOutcome(application.RecordHomologationCodec,
		usecase.EncodeOutcome(application.RecordHomologationCodec, usecase.Accepted(domain.RecordHomologationResponse{CapabilityID: "cap-9", Status: domain.CapabilityStatusHomologated})))
	declared, errDeclared := usecase.DecodeOutcome(application.DeclareCapabilityCodec,
		usecase.EncodeOutcome(application.DeclareCapabilityCodec, usecase.Accepted(domain.DeclareCapabilityResponse{CapabilityID: "cap-9"})))
	enabled, errEnabled := usecase.DecodeOutcome(application.EnableCapabilityCodec,
		usecase.EncodeOutcome(application.EnableCapabilityCodec, usecase.Accepted(domain.EnableCapabilityResponse{CapabilityID: "cap-9"})))
	suspended, errSuspended := usecase.DecodeOutcome(application.SuspendCapabilityCodec,
		usecase.EncodeOutcome(application.SuspendCapabilityCodec, usecase.Accepted(domain.SuspendCapabilityResponse{CapabilityID: "cap-9"})))
	rollout, errRollout := usecase.DecodeOutcome(application.ChangeRolloutCodec,
		usecase.EncodeOutcome(application.ChangeRolloutCodec, usecase.Accepted(domain.ChangeRolloutResponse{CapabilityID: "cap-9"})))

	if errStage != nil || stage.Response() != (domain.ChangePartnerStageResponse{PartnerID: "prt-9", Status: domain.PartnerSuspended}) ||
		errHomologation != nil || homologation.Response() != (domain.RecordHomologationResponse{CapabilityID: "cap-9", Status: domain.CapabilityStatusHomologated}) ||
		errDeclared != nil || declared.Response().CapabilityID != "cap-9" ||
		errEnabled != nil || enabled.Response().CapabilityID != "cap-9" ||
		errSuspended != nil || suspended.Response().CapabilityID != "cap-9" ||
		errRollout != nil || rollout.Response().CapabilityID != "cap-9" {
		t.Fatalf("decoded = %+v %+v %+v %+v %+v %+v; want each response back",
			stage.Response(), homologation.Response(), declared.Response(), enabled.Response(), suspended.Response(), rollout.Response())
	}
}
