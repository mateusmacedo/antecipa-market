package application

import (
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const (
	OperationFindPartner                = "partner.FindPartner"
	OperationFindPartnersByOrganization = "partner.FindPartnersByOrganization"
	OperationFindAgreementsByPartner    = "partner.FindAgreementsByPartner"
	OperationFindCapabilitiesByPartner  = "partner.FindCapabilitiesByPartner"
	OperationFindEffectiveCapability    = "partner.FindEffectiveCapability"
	OperationFindEligiblePartners       = "partner.FindEligiblePartners"
	OperationFindProductsByPartner      = "partner.FindProductsByPartner"
	OperationFindIntegrationProfile     = "partner.FindIntegrationProfile"
	OperationFindOperationalPolicy      = "partner.FindOperationalPolicy"
	OperationFindCertificatesExpiring   = "partner.FindCertificatesExpiring"
)

type FindPartner struct {
	Partner domain.PartnerID
}

func (FindPartner) isOperation() {}

type FindPartnersByOrganization struct {
	Organization domain.OrganizationID
}

func (FindPartnersByOrganization) isOperation() {}

type FindAgreementsByPartner struct {
	Partner domain.PartnerID
}

func (FindAgreementsByPartner) isOperation() {}

type FindCapabilitiesByPartner struct {
	Partner     domain.PartnerID
	Environment domain.Environment
	Status      domain.CapabilityStatus
}

func (FindCapabilitiesByPartner) isOperation() {}

// FindEffectiveCapability takes TenantID as the tenant the rollout evaluates:
// the configuration lives in a single platform tenant, the one of the caller.
type FindEffectiveCapability struct {
	Partner     domain.PartnerID
	Operation   string
	AssetType   string
	Environment domain.Environment
	TenantID    string
	AsOf        kernel.Instant
	ProductID   domain.ProductID
	AmountMinor *int64
	TermDays    *int
}

func (FindEffectiveCapability) isOperation() {}

type FindEligiblePartners struct {
	AssetType   string
	Operation   string
	Environment domain.Environment
	TenantID    string
	AmountMinor int64
	TermDays    int
	Region      string
	AsOf        kernel.Instant
}

func (FindEligiblePartners) isOperation() {}

type FindProductsByPartner struct {
	Partner domain.PartnerID
	AsOf    kernel.Instant
}

func (FindProductsByPartner) isOperation() {}

type FindIntegrationProfile struct {
	Partner     domain.PartnerID
	Environment domain.Environment
	AsOf        kernel.Instant
}

func (FindIntegrationProfile) isOperation() {}

type FindOperationalPolicy struct {
	Partner domain.PartnerID
	AsOf    kernel.Instant
}

func (FindOperationalPolicy) isOperation() {}

type FindCertificatesExpiring struct {
	Environment   domain.Environment
	ExpiresBefore kernel.Instant
}

func (FindCertificatesExpiring) isOperation() {}
