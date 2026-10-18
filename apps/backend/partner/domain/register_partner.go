package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (p *Partner) Register(cmd RegisterPartner) (kernel.Accepted[RegisterPartnerResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*Partner).clone, func(next *Partner) (kernel.Accepted[RegisterPartnerResponse], *kernel.Rejection) {
		organization := kernel.Detail{Key: "organization", Value: string(cmd.OrganizationID)}
		role := kernel.Detail{Key: "role", Value: string(cmd.Role)}
		switch {
		case !cmd.OrganizationFound:
			return kernel.Refuse[RegisterPartnerResponse](CodePartnerOrganizationNotFound, "organization must exist in identity", organization)
		case !cmd.Role.admits(cmd.OrganizationKinds):
			return kernel.Refuse[RegisterPartnerResponse](CodePartnerOrganizationKindMismatch, "organization kinds must admit the role", organization, role)
		case cmd.AlreadyRegistered:
			return kernel.Refuse[RegisterPartnerResponse](CodePartnerAlreadyRegistered, "organization already registered under the role", organization, role)
		}
		next.organizationID = cmd.OrganizationID
		next.role = cmd.Role
		next.status = PartnerProspect
		next.registeredAt = cmd.At
		next.stageChangedAt = cmd.At
		return kernel.Accept(
			RegisterPartnerResponse{PartnerID: next.id},
			PartnerRegistered{PartnerID: next.id, OrganizationID: cmd.OrganizationID, Role: cmd.Role, At: cmd.At},
		), nil
	})
}
