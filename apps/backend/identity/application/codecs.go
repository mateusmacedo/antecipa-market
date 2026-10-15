package application

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

var registerOrganizationCodec = usecase.OutcomeCodec[domain.RegisterOrganizationResponse]{
	Encode: func(e *usecase.Encoder, r domain.RegisterOrganizationResponse) { e.String(string(r.OrganizationID)) },
	Decode: func(d *usecase.Decoder) domain.RegisterOrganizationResponse {
		return domain.RegisterOrganizationResponse{OrganizationID: domain.OrganizationID(d.String())}
	},
}

var addEstablishmentCodec = usecase.OutcomeCodec[domain.AddEstablishmentResponse]{
	Encode: func(e *usecase.Encoder, r domain.AddEstablishmentResponse) { e.String(string(r.EstablishmentID)) },
	Decode: func(d *usecase.Decoder) domain.AddEstablishmentResponse {
		return domain.AddEstablishmentResponse{EstablishmentID: domain.EstablishmentID(d.String())}
	},
}

var registerPersonCodec = usecase.OutcomeCodec[domain.RegisterPersonResponse]{
	Encode: func(e *usecase.Encoder, r domain.RegisterPersonResponse) { e.String(string(r.PersonID)) },
	Decode: func(d *usecase.Decoder) domain.RegisterPersonResponse {
		return domain.RegisterPersonResponse{PersonID: domain.PersonID(d.String())}
	},
}

var registerRepresentationCodec = usecase.OutcomeCodec[domain.RegisterRepresentationResponse]{
	Encode: func(e *usecase.Encoder, r domain.RegisterRepresentationResponse) {
		e.String(string(r.RepresentationID))
	},
	Decode: func(d *usecase.Decoder) domain.RegisterRepresentationResponse {
		return domain.RegisterRepresentationResponse{RepresentationID: domain.RepresentationID(d.String())}
	},
}

var provisionUserAccountCodec = usecase.OutcomeCodec[domain.ProvisionUserAccountResponse]{
	Encode: func(e *usecase.Encoder, r domain.ProvisionUserAccountResponse) { e.String(string(r.UserID)) },
	Decode: func(d *usecase.Decoder) domain.ProvisionUserAccountResponse {
		return domain.ProvisionUserAccountResponse{UserID: domain.UserID(d.String())}
	},
}

var suspendUserCodec = usecase.OutcomeCodec[domain.SuspendUserResponse]{
	Encode: func(e *usecase.Encoder, r domain.SuspendUserResponse) { e.String(string(r.UserID)) },
	Decode: func(d *usecase.Decoder) domain.SuspendUserResponse {
		return domain.SuspendUserResponse{UserID: domain.UserID(d.String())}
	},
}

var inviteUserCodec = usecase.OutcomeCodec[domain.InviteUserResponse]{
	Encode: func(e *usecase.Encoder, r domain.InviteUserResponse) { e.String(string(r.MembershipID)) },
	Decode: func(d *usecase.Decoder) domain.InviteUserResponse {
		return domain.InviteUserResponse{MembershipID: domain.MembershipID(d.String())}
	},
}

var grantMembershipCodec = usecase.OutcomeCodec[domain.GrantMembershipResponse]{
	Encode: func(e *usecase.Encoder, r domain.GrantMembershipResponse) { e.String(string(r.MembershipID)) },
	Decode: func(d *usecase.Decoder) domain.GrantMembershipResponse {
		return domain.GrantMembershipResponse{MembershipID: domain.MembershipID(d.String())}
	},
}

var revokeMembershipCodec = usecase.OutcomeCodec[domain.RevokeMembershipResponse]{
	Encode: func(e *usecase.Encoder, r domain.RevokeMembershipResponse) { e.String(string(r.MembershipID)) },
	Decode: func(d *usecase.Decoder) domain.RevokeMembershipResponse {
		return domain.RevokeMembershipResponse{MembershipID: domain.MembershipID(d.String())}
	},
}

var requestBankAccountChangeCodec = usecase.OutcomeCodec[domain.RequestBankAccountChangeResponse]{
	Encode: func(e *usecase.Encoder, r domain.RequestBankAccountChangeResponse) {
		e.String(string(r.BankAccountChangeID))
	},
	Decode: func(d *usecase.Decoder) domain.RequestBankAccountChangeResponse {
		return domain.RequestBankAccountChangeResponse{BankAccountChangeID: domain.BankAccountChangeID(d.String())}
	},
}

var approveBankAccountChangeCodec = usecase.OutcomeCodec[domain.ApproveBankAccountChangeResponse]{
	Encode: func(e *usecase.Encoder, r domain.ApproveBankAccountChangeResponse) {
		e.String(string(r.BankAccountChangeID))
	},
	Decode: func(d *usecase.Decoder) domain.ApproveBankAccountChangeResponse {
		return domain.ApproveBankAccountChangeResponse{BankAccountChangeID: domain.BankAccountChangeID(d.String())}
	},
}
