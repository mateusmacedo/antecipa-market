package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (a *PartnerAgreement) Register(cmd RegisterAgreement) (kernel.Accepted[RegisterAgreementResponse], *kernel.Rejection) {
	return kernel.DecideOver(a, (*PartnerAgreement).clone, func(next *PartnerAgreement) (kernel.Accepted[RegisterAgreementResponse], *kernel.Rejection) {
		switch {
		case cmd.ValidUntil != 0 && cmd.ValidUntil <= cmd.ValidFrom:
			return kernel.Refuse[RegisterAgreementResponse](CodePartnerAgreementValidityInvalid, "valid until must be after valid from")
		case cmd.SupersedesID != cmd.Head.ID:
			return kernel.Refuse[RegisterAgreementResponse](CodePartnerAgreementSupersedesInvalid,
				"supersedes must name the last version of the series of the partner, and a series holds one first version",
				kernel.Detail{Key: "supersedes", Value: string(cmd.SupersedesID)},
			)
		}
		next.partnerID = cmd.PartnerID
		next.version = cmd.Head.Version + 1
		next.supersedesID = cmd.SupersedesID
		next.status = AgreementDraft
		next.validFrom = cmd.ValidFrom
		next.validUntil = cmd.ValidUntil
		next.instrumentTemplateID = cmd.InstrumentTemplateID
		return kernel.Accept(RegisterAgreementResponse{AgreementID: next.id}), nil
	})
}
