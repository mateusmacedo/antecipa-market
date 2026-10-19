package api

import (
	"encoding/json"

	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/service/v1"
)

type reference struct {
	member string
	id     string
}

func (r reference) MarshalJSON() ([]byte, error) {
	member, err := json.Marshal(r.member)
	if err != nil {
		return nil, err
	}
	id, err := json.Marshal(r.id)
	if err != nil {
		return nil, err
	}
	encoded := make([]byte, 0, len(member)+len(id)+3)
	encoded = append(append(append(append(append(encoded, '{'), member...), ':'), id...), '}')
	return encoded, nil
}

func registeredOrganizationOf(resp *servicev1.RegisterOrganizationResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.RegisterOrganizationResponse_Registered:
		return reference{"organizationId", result.Registered.GetOrganizationId()}, nil
	case *servicev1.RegisterOrganizationResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func addedEstablishmentOf(resp *servicev1.AddEstablishmentResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.AddEstablishmentResponse_Added:
		return reference{"establishmentId", result.Added.GetEstablishmentId()}, nil
	case *servicev1.AddEstablishmentResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func registeredPersonOf(resp *servicev1.RegisterPersonResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.RegisterPersonResponse_Registered:
		return reference{"personId", result.Registered.GetPersonId()}, nil
	case *servicev1.RegisterPersonResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func provisionedUserAccountOf(resp *servicev1.ProvisionUserAccountResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.ProvisionUserAccountResponse_Provisioned:
		return reference{"userId", result.Provisioned.GetUserId()}, nil
	case *servicev1.ProvisionUserAccountResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func suspendedUserOf(resp *servicev1.SuspendUserResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.SuspendUserResponse_Suspended:
		return reference{"userId", result.Suspended.GetUserId()}, nil
	case *servicev1.SuspendUserResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func registeredRepresentationOf(resp *servicev1.RegisterRepresentationResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.RegisterRepresentationResponse_Registered:
		return reference{"representationId", result.Registered.GetRepresentationId()}, nil
	case *servicev1.RegisterRepresentationResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func invitedUserOf(resp *servicev1.InviteUserResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.InviteUserResponse_Invited:
		return reference{"membershipId", result.Invited.GetMembershipId()}, nil
	case *servicev1.InviteUserResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func revokedMembershipOf(resp *servicev1.RevokeMembershipResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.RevokeMembershipResponse_Revoked:
		return reference{"membershipId", result.Revoked.GetMembershipId()}, nil
	case *servicev1.RevokeMembershipResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func requestedBankAccountChangeOf(resp *servicev1.RequestBankAccountChangeResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.RequestBankAccountChangeResponse_Requested:
		return reference{"bankAccountChangeId", result.Requested.GetBankAccountChangeId()}, nil
	case *servicev1.RequestBankAccountChangeResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}

func approvedBankAccountChangeOf(resp *servicev1.ApproveBankAccountChangeResponse) (reference, refusal) {
	switch result := resp.GetResult().(type) {
	case *servicev1.ApproveBankAccountChangeResponse_Approved:
		return reference{"bankAccountChangeId", result.Approved.GetBankAccountChangeId()}, nil
	case *servicev1.ApproveBankAccountChangeResponse_Rejection:
		return reference{}, result.Rejection
	default:
		return reference{}, nil
	}
}
