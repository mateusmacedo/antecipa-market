package application

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

const (
	AggregateTypePartner          = "partner.Partner"
	AggregateTypePartnerAgreement = "partner.PartnerAgreement"

	AggregateTypeIntegrationProfile = "partner.IntegrationProfile"
	AggregateTypeOperationalPolicy  = "partner.OperationalPolicy"
	AggregateTypeCapability         = "partner.Capability"
	AggregateTypeFinancialProduct   = "partner.FinancialProduct"

	Destination = "partner.events"

	OperationRegisterPartner   = "partner.RegisterPartner"
	OperationRegisterAgreement = "partner.RegisterAgreement"
	OperationActivateAgreement = "partner.ActivateAgreement"

	OperationRegisterIntegrationProfile  = "partner.RegisterIntegrationProfile"
	OperationRaiseCertificateExpiryAlert = "partner.RaiseCertificateExpiryAlert"
	OperationDefineOperationalPolicy     = "partner.DefineOperationalPolicy"

	OperationChangePartnerStage = "partner.ChangePartnerStage"
	OperationDeclareCapability  = "partner.DeclareCapability"
	OperationRecordHomologation = "partner.RecordHomologation"
	OperationEnableCapability   = "partner.EnableCapability"
	OperationSuspendCapability  = "partner.SuspendCapability"
	OperationChangeRollout      = "partner.ChangeRollout"

	OperationDefineProduct = "partner.DefineProduct"
	OperationRetireProduct = "partner.RetireProduct"
)

const maxEventsPerCommand = 1

type Resources struct {
	Partners   ports.PartnerRepository
	Agreements ports.AgreementRepository

	IntegrationProfiles ports.IntegrationProfileRepository
	OperationalPolicies ports.OperationalPolicyRepository
	Capabilities        ports.CapabilityRepository
	FinancialProducts   ports.FinancialProductRepository

	Outbox   port.Outbox
	Commands port.Inbox
}

type Operation interface{ isOperation() }

type RegisterPartner struct {
	PartnerID      domain.PartnerID
	OrganizationID domain.OrganizationID
	Role           domain.PartnerRole
}

func (RegisterPartner) isOperation() {}

type RegisterAgreement struct {
	AgreementID          domain.AgreementID
	PartnerID            domain.PartnerID
	ValidFrom            kernel.Instant
	ValidUntil           kernel.Instant
	InstrumentTemplateID string
	SupersedesID         domain.AgreementID
}

func (RegisterAgreement) isOperation() {}

type ActivateAgreement struct {
	AgreementID               domain.AgreementID
	ContractEvidenceID        string
	SatisfiedCriteria         []string
	RegulatoryCheckApplicable bool
}

func (ActivateAgreement) isOperation() {}

type RegisterIntegrationProfile struct {
	ProfileID            domain.ProfileID
	PartnerID            domain.PartnerID
	Environment          domain.Environment
	ChangeKind           domain.IntegrationChangeKind
	Protocols            []string
	Endpoint             string
	SchemaVersion        string
	SecretRef            string
	CertificateRef       string
	CertificateExpiresAt kernel.Instant
	EffectiveFrom        kernel.Instant
	SupersedesID         domain.ProfileID
}

func (RegisterIntegrationProfile) isOperation() {}

type RaiseCertificateExpiryAlert struct {
	ProfileID     domain.ProfileID
	ThresholdDays int
}

func (RaiseCertificateExpiryAlert) isOperation() {}

type DefineOperationalPolicy struct {
	PolicyID           domain.PolicyID
	PartnerID          domain.PartnerID
	EffectiveFrom      kernel.Instant
	SupersedesID       domain.PolicyID
	RateLimitPerMinute int
	BusinessDays       []string
	CutoffTime         string
	ProcessingWindow   string
	SlaRef             string
	Fallback           string
	SupportContact     string
	IncidentContact    string
}

func (DefineOperationalPolicy) isOperation() {}

type ChangePartnerStage struct {
	PartnerID domain.PartnerID
	Target    domain.PartnerStatus
	Reason    string
}

func (ChangePartnerStage) isOperation() {}

type DeclareCapability struct {
	CapabilityID        domain.CapabilityID
	PartnerID           domain.PartnerID
	Operation           string
	AssetType           string
	Environment         domain.Environment
	IsWrite             bool
	StateQuerySupported bool
	RestrictionNote     string
	CallbackApplicable  bool
	SupersedesID        domain.CapabilityID
}

func (DeclareCapability) isOperation() {}

type RecordHomologation struct {
	CapabilityID domain.CapabilityID
	Results      []domain.HomologationResult
}

func (RecordHomologation) isOperation() {}

type EnableCapability struct {
	CapabilityID domain.CapabilityID
}

func (EnableCapability) isOperation() {}

type SuspendCapability struct {
	CapabilityID domain.CapabilityID
	Reason       string
	Trigger      domain.SuspensionTrigger
}

func (SuspendCapability) isOperation() {}

type ChangeRollout struct {
	CapabilityID       domain.CapabilityID
	Mode               domain.RolloutMode
	AllowlistTenantIDs []string
	Percentage         int
	ExcludedTenantIDs  []string
}

func (ChangeRollout) isOperation() {}

type DefineProduct struct {
	ProductID        domain.ProductID
	PartnerID        domain.PartnerID
	AgreementID      domain.AgreementID
	Code             string
	Name             string
	AssetTypes       []string
	Regions          []string
	Currency         string
	MinAmountMinor   int64
	MaxAmountMinor   int64
	MinTermDays      int
	MaxTermDays      int
	RequiredCheckIDs []string
	ValidFrom        kernel.Instant
	ValidUntil       kernel.Instant
	SupersedesID     domain.ProductID
}

func (DefineProduct) isOperation() {}

type RetireProduct struct {
	ProductID domain.ProductID
	Reason    domain.ProductRetireReason
}

func (RetireProduct) isOperation() {}

type Service struct {
	UoW                    port.UnitOfWork[Resources]
	Directory              ports.OrganizationDirectory
	PartnersByOrganization ports.PartnersByOrganizationReader
	AgreementsByPartner    ports.AgreementsByPartnerReader
	ProfilesByPartner      ports.IntegrationProfilesByPartnerReader
	PoliciesByPartner      ports.OperationalPoliciesByPartnerReader
	CapabilitiesByPartner  ports.CapabilitiesByPartnerReader
	ProductsByPartner      ports.FinancialProductsByPartnerReader
	PartnerReader          ports.PartnerReader
	PartnersByRole         ports.PartnersByRoleReader
	ProfilesByEnvironment  ports.IntegrationProfilesByEnvironmentReader
	Rollout                ports.RolloutBucket
	Clock                  port.Clock
	IDs                    port.IDGenerator
	Authorize              usecase.Authorize[Operation]
	Idempotency            usecase.IdempotencyPolicy
	Instrumentation        port.Instrumentation
}

type command[R any] = usecase.Command[Resources, Operation, R]

func (s Service) instrumentation() port.Instrumentation {
	if s.Instrumentation == nil {
		return port.NoInstrumentation()
	}
	return s.Instrumentation
}

func (s Service) executor() usecase.Executor[Resources, Operation] {
	return usecase.Executor[Resources, Operation]{
		UoW:             s.UoW,
		Inbox:           func(res Resources) port.Inbox { return res.Commands },
		Consumer:        CommandConsumer,
		Clock:           s.Clock,
		IDs:             s.IDs,
		MaxEvents:       maxEventsPerCommand,
		Policy:          s.Idempotency,
		Authorize:       s.Authorize,
		Instrumentation: s.Instrumentation,
	}
}

func origin(aggregateType, aggregateID string) usecase.Origin {
	return usecase.Origin{Destination: Destination, AggregateType: aggregateType, AggregateID: aggregateID}
}

var (
	loadAbsentPartner   = usecase.Absent[domain.PartnerSnapshot](domain.NewPartner)
	loadExistingPartner = usecase.Existing[domain.PartnerID](domain.FromPartnerSnapshot)

	loadAbsentAgreement   = usecase.Absent[domain.PartnerAgreementSnapshot](domain.NewPartnerAgreement)
	loadExistingAgreement = usecase.Existing[domain.AgreementID](domain.FromPartnerAgreementSnapshot)

	loadAbsentProfile   = usecase.Absent[domain.IntegrationProfileSnapshot](domain.NewIntegrationProfile)
	loadExistingProfile = usecase.Existing[domain.ProfileID](domain.FromIntegrationProfileSnapshot)

	loadAbsentPolicy = usecase.Absent[domain.OperationalPolicySnapshot](domain.NewOperationalPolicy)

	loadAbsentCapability   = usecase.Absent[domain.CapabilitySnapshot](domain.NewCapability)
	loadExistingCapability = usecase.Existing[domain.CapabilityID](domain.FromCapabilitySnapshot)

	loadAbsentProduct   = usecase.Absent[domain.FinancialProductSnapshot](domain.NewFinancialProduct)
	loadExistingProduct = usecase.Existing[domain.ProductID](domain.FromFinancialProductSnapshot)
)
