package application

var RegisterOrganizationCodec = registerOrganizationCodec

var (
	AddEstablishmentCodec       = addEstablishmentCodec
	RegisterPersonCodec         = registerPersonCodec
	RegisterRepresentationCodec = registerRepresentationCodec
)

var (
	ProvisionUserAccountCodec = provisionUserAccountCodec
	SuspendUserCodec          = suspendUserCodec
	InviteUserCodec           = inviteUserCodec
	GrantMembershipCodec      = grantMembershipCodec
	RevokeMembershipCodec     = revokeMembershipCodec
)

var (
	RequestBankAccountChangeCodec = requestBankAccountChangeCodec
	ApproveBankAccountChangeCodec = approveBankAccountChangeCodec
)
