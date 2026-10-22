package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type (
	PartnerID      string
	OrganizationID string
	AgreementID    string
)

type RegisterPartner struct {
	OrganizationID    OrganizationID
	Role              PartnerRole
	OrganizationFound bool
	OrganizationKinds []OrganizationKind
	At                kernel.Instant
}

type RegisterPartnerResponse struct {
	PartnerID PartnerID
}

type PartnerRegistered struct {
	PartnerID      PartnerID
	OrganizationID OrganizationID
	Role           PartnerRole
	At             kernel.Instant
}

func (PartnerRegistered) EventName() string {
	return "partner-management.partner.registered"
}

var _ kernel.DomainEvent = PartnerRegistered{}

type ChangePartnerStage struct {
	Target                PartnerStatus
	Reason                string
	HasEffectiveAgreement bool
	HasEnabledCapability  bool
	At                    kernel.Instant
}

type ChangePartnerStageResponse struct {
	PartnerID PartnerID
	Status    PartnerStatus
}

type PartnerOffboardingStarted struct {
	PartnerID      PartnerID
	OrganizationID OrganizationID
	Reason         string
	At             kernel.Instant
}

func (PartnerOffboardingStarted) EventName() string {
	return "partner-management.partner.offboarding-started"
}

var _ kernel.DomainEvent = PartnerOffboardingStarted{}

type AgreementHead struct {
	ID      AgreementID
	Version int
}

type RegisterAgreement struct {
	PartnerID            PartnerID
	ValidFrom            kernel.Instant
	ValidUntil           kernel.Instant
	InstrumentTemplateID string
	SupersedesID         AgreementID
	Head                 AgreementHead
	At                   kernel.Instant
}

type RegisterAgreementResponse struct {
	AgreementID AgreementID
}

type ActivateAgreement struct {
	ContractEvidenceID        string
	SatisfiedCriteria         []string
	RegulatoryCheckApplicable bool
	At                        kernel.Instant
}

type ActivateAgreementResponse struct {
	AgreementID AgreementID
}

type PartnerAgreementActivated struct {
	AgreementID AgreementID
	PartnerID   PartnerID
	Version     int
	ValidFrom   kernel.Instant
	ValidUntil  kernel.Instant
	At          kernel.Instant
}

func (PartnerAgreementActivated) EventName() string {
	return "partner-management.partner-agreement.activated"
}

var _ kernel.DomainEvent = PartnerAgreementActivated{}

type (
	ProfileID string
	PolicyID  string
)

type ProfileHead struct {
	ID      ProfileID
	Version int
}

type RegisterIntegrationProfile struct {
	PartnerID            PartnerID
	Environment          Environment
	ChangeKind           IntegrationChangeKind
	Protocols            []string
	Endpoint             string
	SchemaVersion        string
	SecretRef            string
	CertificateRef       string
	CertificateExpiresAt kernel.Instant
	EffectiveFrom        kernel.Instant
	SupersedesID         ProfileID
	Head                 ProfileHead
	At                   kernel.Instant
}

type RegisterIntegrationProfileResponse struct {
	ProfileID ProfileID
}

type RaiseCertificateExpiryAlert struct {
	ThresholdDays int
	At            kernel.Instant
}

type RaiseCertificateExpiryAlertResponse struct {
	ProfileID ProfileID
}

type PartnerCertificateExpiring struct {
	ProfileID            ProfileID
	PartnerID            PartnerID
	Environment          Environment
	CertificateExpiresAt kernel.Instant
	ThresholdDays        int
	At                   kernel.Instant
}

func (PartnerCertificateExpiring) EventName() string {
	return "partner-management.integration-profile.certificate-expiring"
}

var _ kernel.DomainEvent = PartnerCertificateExpiring{}

type PolicyHead struct {
	ID      PolicyID
	Version int
}

type DefineOperationalPolicy struct {
	PartnerID          PartnerID
	EffectiveFrom      kernel.Instant
	SupersedesID       PolicyID
	Head               PolicyHead
	RateLimitPerMinute int
	BusinessDays       []string
	CutoffTime         string
	ProcessingWindow   string
	SlaRef             string
	Fallback           string
	SupportContact     string
	IncidentContact    string
	At                 kernel.Instant
}

type DefineOperationalPolicyResponse struct {
	PolicyID PolicyID
}

type CapabilityID string

type CapabilityHead struct {
	ID      CapabilityID
	Version int
}

type DeclareCapability struct {
	PartnerID           PartnerID
	Operation           string
	AssetType           string
	Environment         Environment
	IsWrite             bool
	StateQuerySupported bool
	RestrictionNote     string
	CallbackApplicable  bool
	SupersedesID        CapabilityID
	Head                CapabilityHead
	At                  kernel.Instant
}

type DeclareCapabilityResponse struct {
	CapabilityID CapabilityID
}

type HomologationResult struct {
	CriterionID string
	Passed      bool
	EvidenceID  string
}

type RecordHomologation struct {
	Results []HomologationResult
	At      kernel.Instant
}

type RecordHomologationResponse struct {
	CapabilityID CapabilityID
	Status       CapabilityStatus
}

type EnableCapability struct {
	HasEffectiveAgreement bool
	HasEffectiveProfile   bool
	At                    kernel.Instant
}

type EnableCapabilityResponse struct {
	CapabilityID CapabilityID
}

type CapabilityEnabled struct {
	CapabilityID        CapabilityID
	PartnerID           PartnerID
	Operation           string
	AssetType           string
	Environment         Environment
	Version             int
	RolloutMode         RolloutMode
	StateQuerySupported bool
	RestrictionNote     string
	At                  kernel.Instant
}

func (CapabilityEnabled) EventName() string {
	return "partner-management.capability.enabled"
}

var _ kernel.DomainEvent = CapabilityEnabled{}

type SuspendCapability struct {
	Reason  string
	Trigger SuspensionTrigger
	At      kernel.Instant
}

type SuspendCapabilityResponse struct {
	CapabilityID CapabilityID
}

type CapabilitySuspended struct {
	CapabilityID CapabilityID
	PartnerID    PartnerID
	Operation    string
	AssetType    string
	Environment  Environment
	Version      int
	Reason       string
	Trigger      SuspensionTrigger
	At           kernel.Instant
}

func (CapabilitySuspended) EventName() string {
	return "partner-management.capability.suspended"
}

var _ kernel.DomainEvent = CapabilitySuspended{}

type ChangeRollout struct {
	Mode               RolloutMode
	AllowlistTenantIDs []string
	Percentage         int
	ExcludedTenantIDs  []string
	At                 kernel.Instant
}

type ChangeRolloutResponse struct {
	CapabilityID CapabilityID
}

type CapabilityRolloutChanged struct {
	CapabilityID CapabilityID
	PartnerID    PartnerID
	Operation    string
	AssetType    string
	Environment  Environment
	Version      int
	Mode         RolloutMode
	Percentage   int
	At           kernel.Instant
}

func (CapabilityRolloutChanged) EventName() string {
	return "partner-management.capability.rollout-changed"
}

var _ kernel.DomainEvent = CapabilityRolloutChanged{}

type ProductID string

type ProductHead struct {
	ID      ProductID
	Version int
}

type DefineProduct struct {
	PartnerID             PartnerID
	AgreementID           AgreementID
	Code                  string
	Name                  string
	AssetTypes            []string
	Regions               []string
	Currency              string
	MinAmountMinor        int64
	MaxAmountMinor        int64
	MinTermDays           int
	MaxTermDays           int
	RequiredCheckIDs      []string
	ValidFrom             kernel.Instant
	ValidUntil            kernel.Instant
	SupersedesID          ProductID
	Head                  ProductHead
	HasEffectiveAgreement bool
	At                    kernel.Instant
}

type DefineProductResponse struct {
	ProductID ProductID
}

type PartnerProductDefined struct {
	ProductID      ProductID
	PartnerID      PartnerID
	AgreementID    AgreementID
	Code           string
	Version        int
	ValidFrom      kernel.Instant
	ValidUntil     kernel.Instant
	AssetTypes     []string
	Regions        []string
	Currency       string
	MinAmountMinor int64
	MaxAmountMinor int64
	MinTermDays    int
	MaxTermDays    int
	At             kernel.Instant
}

func (PartnerProductDefined) EventName() string {
	return "partner-management.financial-product.defined"
}

var _ kernel.DomainEvent = PartnerProductDefined{}

type PartnerProductChanged struct {
	ProductID      ProductID
	PartnerID      PartnerID
	AgreementID    AgreementID
	Code           string
	Version        int
	SupersedesID   ProductID
	ValidFrom      kernel.Instant
	ValidUntil     kernel.Instant
	AssetTypes     []string
	Regions        []string
	Currency       string
	MinAmountMinor int64
	MaxAmountMinor int64
	MinTermDays    int
	MaxTermDays    int
	At             kernel.Instant
}

func (PartnerProductChanged) EventName() string {
	return "partner-management.financial-product.changed"
}

var _ kernel.DomainEvent = PartnerProductChanged{}

type RetireProduct struct {
	Reason ProductRetireReason
	At     kernel.Instant
}

type RetireProductResponse struct {
	ProductID ProductID
}

type PartnerProductExpired struct {
	ProductID  ProductID
	PartnerID  PartnerID
	Version    int
	ValidUntil kernel.Instant
	At         kernel.Instant
}

func (PartnerProductExpired) EventName() string {
	return "partner-management.financial-product.expired"
}

var _ kernel.DomainEvent = PartnerProductExpired{}

type PartnerProductRetired struct {
	ProductID ProductID
	PartnerID PartnerID
	Version   int
	At        kernel.Instant
}

func (PartnerProductRetired) EventName() string {
	return "partner-management.financial-product.retired"
}

var _ kernel.DomainEvent = PartnerProductRetired{}
