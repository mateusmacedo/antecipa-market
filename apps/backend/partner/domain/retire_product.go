package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

func (p *FinancialProduct) Retire(cmd RetireProduct) (kernel.Accepted[RetireProductResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*FinancialProduct).clone, func(next *FinancialProduct) (kernel.Accepted[RetireProductResponse], *kernel.Rejection) {
		expired := cmd.Reason == RetireExpired
		switch {
		case next.status != ProductActive:
			return kernel.Refuse[RetireProductResponse](CodeFinancialProductNotActive, "only an active product is retired")
		case expired && (next.validUntil == 0 || next.validUntil > cmd.At):
			return kernel.Refuse[RetireProductResponse](CodeFinancialProductNotYetExpired, "the validity of the product has not ended yet")
		}
		next.status = ProductRetired
		response := RetireProductResponse{ProductID: next.id}
		if expired {
			return kernel.Accept(response, PartnerProductExpired{
				ProductID: next.id, PartnerID: next.partnerID, Version: next.version, ValidUntil: next.validUntil, At: cmd.At,
			}), nil
		}
		return kernel.Accept(response, PartnerProductRetired{
			ProductID: next.id, PartnerID: next.partnerID, Version: next.version, At: cmd.At,
		}), nil
	})
}
