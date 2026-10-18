package domain

import "slices"

type PartnerRole string

const (
	RoleFunder                PartnerRole = "funder"
	RoleBookkeeper            PartnerRole = "bookkeeper"
	RoleRegistry              PartnerRole = "registry"
	RoleDepository            PartnerRole = "depository"
	RoleDataProvider          PartnerRole = "data_provider"
	RoleSettlementInstitution PartnerRole = "settlement_institution"
)

type OrganizationKind string

const (
	KindFunder  OrganizationKind = "funder"
	KindPartner OrganizationKind = "partner"
)

var requiredKind = map[PartnerRole]OrganizationKind{
	RoleFunder:                KindFunder,
	RoleBookkeeper:            KindPartner,
	RoleRegistry:              KindPartner,
	RoleDepository:            KindPartner,
	RoleDataProvider:          KindPartner,
	RoleSettlementInstitution: KindPartner,
}

func (r PartnerRole) admits(kinds []OrganizationKind) bool {
	kind, known := requiredKind[r]
	return known && slices.Contains(kinds, kind)
}
