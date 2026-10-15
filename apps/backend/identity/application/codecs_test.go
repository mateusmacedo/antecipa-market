package application_test

import (
	"bytes"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestTheRegisterResponseKeepsItsStoredFormat(t *testing.T) {
	stored := usecase.EncodeOutcome(application.RegisterOrganizationCodec,
		usecase.Accepted(domain.RegisterOrganizationResponse{OrganizationID: "org-100"}))

	if golden := []byte{0x01, 'a', 0x07, 'o', 'r', 'g', '-', '1', '0', '0'}; !bytes.Equal(stored, golden) {
		t.Fatalf("stored = %x, want %x: an entry written by the previous release must still decode", stored, golden)
	}
}

func TestTheRegisterResponseRoundTrips(t *testing.T) {
	want := domain.RegisterOrganizationResponse{OrganizationID: "org-9"}

	got, err := usecase.DecodeOutcome(application.RegisterOrganizationCodec,
		usecase.EncodeOutcome(application.RegisterOrganizationCodec, usecase.Accepted(want)))

	if err != nil || got.Response() != want {
		t.Fatalf("decoded = (%+v, %v), want %+v", got.Response(), err, want)
	}
}

func TestTheResponsesOfTheNewCommandsKeepTheirStoredFormat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stored []byte
		golden []byte
	}{
		{
			"add establishment",
			usecase.EncodeOutcome(application.AddEstablishmentCodec, usecase.Accepted(domain.AddEstablishmentResponse{EstablishmentID: "est-100"})),
			[]byte{0x01, 'a', 0x07, 'e', 's', 't', '-', '1', '0', '0'},
		},
		{
			"register person",
			usecase.EncodeOutcome(application.RegisterPersonCodec, usecase.Accepted(domain.RegisterPersonResponse{PersonID: "per-100"})),
			[]byte{0x01, 'a', 0x07, 'p', 'e', 'r', '-', '1', '0', '0'},
		},
		{
			"register representation",
			usecase.EncodeOutcome(application.RegisterRepresentationCodec, usecase.Accepted(domain.RegisterRepresentationResponse{RepresentationID: "rep-100"})),
			[]byte{0x01, 'a', 0x07, 'r', 'e', 'p', '-', '1', '0', '0'},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !bytes.Equal(tc.stored, tc.golden) {
				t.Fatalf("stored = %x, want %x", tc.stored, tc.golden)
			}
		})
	}
}

func TestTheResponsesOfTheNewCommandsRoundTrip(t *testing.T) {
	establishment, errEstablishment := usecase.DecodeOutcome(application.AddEstablishmentCodec,
		usecase.EncodeOutcome(application.AddEstablishmentCodec, usecase.Accepted(domain.AddEstablishmentResponse{EstablishmentID: "est-9"})))
	person, errPerson := usecase.DecodeOutcome(application.RegisterPersonCodec,
		usecase.EncodeOutcome(application.RegisterPersonCodec, usecase.Accepted(domain.RegisterPersonResponse{PersonID: "per-9"})))
	representation, errRepresentation := usecase.DecodeOutcome(application.RegisterRepresentationCodec,
		usecase.EncodeOutcome(application.RegisterRepresentationCodec, usecase.Accepted(domain.RegisterRepresentationResponse{RepresentationID: "rep-9"})))

	if errEstablishment != nil || establishment.Response().EstablishmentID != "est-9" {
		t.Fatalf("establishment = (%+v, %v), want est-9", establishment.Response(), errEstablishment)
	}
	if errPerson != nil || person.Response().PersonID != "per-9" {
		t.Fatalf("person = (%+v, %v), want per-9", person.Response(), errPerson)
	}
	if errRepresentation != nil || representation.Response().RepresentationID != "rep-9" {
		t.Fatalf("representation = (%+v, %v), want rep-9", representation.Response(), errRepresentation)
	}
}

func TestTheResponsesOfTheAccessCommandsKeepTheirStoredFormat(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stored []byte
		golden []byte
	}{
		{
			"provision user account",
			usecase.EncodeOutcome(application.ProvisionUserAccountCodec, usecase.Accepted(domain.ProvisionUserAccountResponse{UserID: "sub-100"})),
			[]byte{0x01, 'a', 0x07, 's', 'u', 'b', '-', '1', '0', '0'},
		},
		{
			"suspend user",
			usecase.EncodeOutcome(application.SuspendUserCodec, usecase.Accepted(domain.SuspendUserResponse{UserID: "sub-100"})),
			[]byte{0x01, 'a', 0x07, 's', 'u', 'b', '-', '1', '0', '0'},
		},
		{
			"invite user",
			usecase.EncodeOutcome(application.InviteUserCodec, usecase.Accepted(domain.InviteUserResponse{MembershipID: "mem-100"})),
			[]byte{0x01, 'a', 0x07, 'm', 'e', 'm', '-', '1', '0', '0'},
		},
		{
			"grant membership",
			usecase.EncodeOutcome(application.GrantMembershipCodec, usecase.Accepted(domain.GrantMembershipResponse{MembershipID: "mem-100"})),
			[]byte{0x01, 'a', 0x07, 'm', 'e', 'm', '-', '1', '0', '0'},
		},
		{
			"revoke membership",
			usecase.EncodeOutcome(application.RevokeMembershipCodec, usecase.Accepted(domain.RevokeMembershipResponse{MembershipID: "mem-100"})),
			[]byte{0x01, 'a', 0x07, 'm', 'e', 'm', '-', '1', '0', '0'},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !bytes.Equal(tc.stored, tc.golden) {
				t.Fatalf("stored = %x, want %x", tc.stored, tc.golden)
			}
		})
	}
}

func TestTheResponsesOfTheAccessCommandsRoundTrip(t *testing.T) {
	provisioned, errProvision := usecase.DecodeOutcome(application.ProvisionUserAccountCodec,
		usecase.EncodeOutcome(application.ProvisionUserAccountCodec, usecase.Accepted(domain.ProvisionUserAccountResponse{UserID: "sub-9"})))
	suspended, errSuspend := usecase.DecodeOutcome(application.SuspendUserCodec,
		usecase.EncodeOutcome(application.SuspendUserCodec, usecase.Accepted(domain.SuspendUserResponse{UserID: "sub-9"})))
	invited, errInvite := usecase.DecodeOutcome(application.InviteUserCodec,
		usecase.EncodeOutcome(application.InviteUserCodec, usecase.Accepted(domain.InviteUserResponse{MembershipID: "mem-9"})))
	granted, errGrant := usecase.DecodeOutcome(application.GrantMembershipCodec,
		usecase.EncodeOutcome(application.GrantMembershipCodec, usecase.Accepted(domain.GrantMembershipResponse{MembershipID: "mem-9"})))
	revoked, errRevoke := usecase.DecodeOutcome(application.RevokeMembershipCodec,
		usecase.EncodeOutcome(application.RevokeMembershipCodec, usecase.Accepted(domain.RevokeMembershipResponse{MembershipID: "mem-9"})))

	for name, ok := range map[string]bool{
		"provision user account": errProvision == nil && provisioned.Response().UserID == "sub-9",
		"suspend user":           errSuspend == nil && suspended.Response().UserID == "sub-9",
		"invite user":            errInvite == nil && invited.Response().MembershipID == "mem-9",
		"grant membership":       errGrant == nil && granted.Response().MembershipID == "mem-9",
		"revoke membership":      errRevoke == nil && revoked.Response().MembershipID == "mem-9",
	} {
		if !ok {
			t.Errorf("%s did not round trip", name)
		}
	}
}

func TestTheResponsesOfTheBankAccountChangeCommandsKeepTheirStoredFormat(t *testing.T) {
	golden := []byte{0x01, 'a', 0x07, 'b', 'a', 'c', '-', '1', '0', '0'}
	for name, stored := range map[string][]byte{
		"request bank account change": usecase.EncodeOutcome(application.RequestBankAccountChangeCodec,
			usecase.Accepted(domain.RequestBankAccountChangeResponse{BankAccountChangeID: "bac-100"})),
		"approve bank account change": usecase.EncodeOutcome(application.ApproveBankAccountChangeCodec,
			usecase.Accepted(domain.ApproveBankAccountChangeResponse{BankAccountChangeID: "bac-100"})),
	} {
		if !bytes.Equal(stored, golden) {
			t.Errorf("%s stored = %x, want %x", name, stored, golden)
		}
	}
}

func TestTheResponsesOfTheBankAccountChangeCommandsRoundTrip(t *testing.T) {
	requested, errRequest := usecase.DecodeOutcome(application.RequestBankAccountChangeCodec,
		usecase.EncodeOutcome(application.RequestBankAccountChangeCodec, usecase.Accepted(domain.RequestBankAccountChangeResponse{BankAccountChangeID: "bac-9"})))
	approved, errApprove := usecase.DecodeOutcome(application.ApproveBankAccountChangeCodec,
		usecase.EncodeOutcome(application.ApproveBankAccountChangeCodec, usecase.Accepted(domain.ApproveBankAccountChangeResponse{BankAccountChangeID: "bac-9"})))

	if errRequest != nil || requested.Response().BankAccountChangeID != "bac-9" {
		t.Fatalf("request = (%+v, %v), want bac-9", requested.Response(), errRequest)
	}
	if errApprove != nil || approved.Response().BankAccountChangeID != "bac-9" {
		t.Fatalf("approve = (%+v, %v), want bac-9", approved.Response(), errApprove)
	}
}
