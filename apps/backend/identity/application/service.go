package application

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

const (
	AggregateTypeOrganization   = "identity.Organization"
	AggregateTypeEstablishment  = "identity.Establishment"
	AggregateTypePerson         = "identity.Person"
	AggregateTypeRepresentation = "identity.Representation"
	AggregateTypeUserAccount    = "identity.UserAccount"
	AggregateTypeMembership     = "identity.Membership"

	AggregateTypeBankAccountChange = "identity.BankAccountChange"

	Destination = "identity.events"

	OperationRegisterOrganization    = "identity.RegisterOrganization"
	OperationFindOrganization        = "identity.FindOrganization"
	OperationFindOrganizationByTaxID = "identity.FindOrganizationByTaxId"

	OperationAddEstablishment                  = "identity.AddEstablishment"
	OperationRegisterPerson                    = "identity.RegisterPerson"
	OperationRegisterRepresentation            = "identity.RegisterRepresentation"
	OperationFindEstablishmentsByOrganization  = "identity.FindEstablishmentsByOrganization"
	OperationFindPerson                        = "identity.FindPerson"
	OperationFindRepresentation                = "identity.FindRepresentation"
	OperationFindRepresentationsByOrganization = "identity.FindRepresentationsByOrganization"

	OperationProvisionUserAccount          = "identity.ProvisionUserAccount"
	OperationSuspendUser                   = "identity.SuspendUser"
	OperationInviteUser                    = "identity.InviteUser"
	OperationGrantMembership               = "identity.GrantMembership"
	OperationRevokeMembership              = "identity.RevokeMembership"
	OperationFindMembershipsByUser         = "identity.FindMembershipsByUser"
	OperationFindMembershipsByOrganization = "identity.FindMembershipsByOrganization"
	OperationFindEffectivePermissions      = "identity.FindEffectivePermissions"

	OperationRequestBankAccountChange             = "identity.RequestBankAccountChange"
	OperationApproveBankAccountChange             = "identity.ApproveBankAccountChange"
	OperationFindBankAccountChange                = "identity.FindBankAccountChange"
	OperationFindBankAccountChangesByOrganization = "identity.FindBankAccountChangesByOrganization"
)

const maxEventsPerCommand = 1

type Resources struct {
	Organizations   ports.OrganizationRepository
	Establishments  ports.EstablishmentRepository
	Persons         ports.PersonRepository
	Representations ports.RepresentationRepository
	UserAccounts    ports.UserAccountRepository
	Memberships     ports.MembershipRepository
	Outbox          port.Outbox
	Commands        port.Inbox

	BankAccountChanges ports.BankAccountChangeRepository
}

type Operation interface{ isOperation() }

type RegisterOrganization struct {
	OrganizationID domain.OrganizationID
	Kinds          []domain.OrganizationKind
	Jurisdiction   string
	TaxIDType      domain.TaxIDType
	TaxID          string
	LegalName      string
	TradeName      string
}

func (RegisterOrganization) isOperation() {}

type FindOrganization struct {
	Organization domain.OrganizationID
}

func (FindOrganization) isOperation() {}

// FindOrganizationByTaxID carries the type with the value because the type is
// part of the protected hash: the same digits as CPF and as CNPJ hash apart.
type FindOrganizationByTaxID struct {
	TaxIDType    domain.TaxIDType
	TaxID        string
	Jurisdiction string
}

func (FindOrganizationByTaxID) isOperation() {}

type AddEstablishment struct {
	EstablishmentID    domain.EstablishmentID
	OrganizationID     domain.OrganizationID
	TaxIDType          domain.TaxIDType
	TaxID              string
	TradeName          string
	Receiver           bool
	ExternalReferences []domain.ExternalReference
}

func (AddEstablishment) isOperation() {}

type RegisterPerson struct {
	PersonID domain.PersonID
	FullName string
	Document string
}

func (RegisterPerson) isOperation() {}

type RegisterRepresentation struct {
	RepresentationID    domain.RepresentationID
	OrganizationID      domain.OrganizationID
	PersonID            domain.PersonID
	PowerType           domain.PowerType
	SourceType          domain.SourceType
	SourceEvidenceID    string
	ScopeCurrency       string
	ScopeMaxAmountMinor int64
	ScopeActTypes       []string
	ValidFrom           kernel.Instant
	ValidUntil          kernel.Instant
	ProposedByUserID    string
}

func (RegisterRepresentation) isOperation() {}

type FindEstablishmentsByOrganization struct {
	Organization domain.OrganizationID
}

func (FindEstablishmentsByOrganization) isOperation() {}

type FindPerson struct {
	Person domain.PersonID
}

func (FindPerson) isOperation() {}

type FindRepresentation struct {
	Representation domain.RepresentationID
}

func (FindRepresentation) isOperation() {}

// A zero ValidAt is the absent optional filter of the spec: valid_at is a plain
// proto3 int64 (identity_service.proto), so the wire cannot tell zero from unset.
type FindRepresentationsByOrganization struct {
	Organization domain.OrganizationID
	ValidAt      kernel.Instant
}

func (FindRepresentationsByOrganization) isOperation() {}

type ProvisionUserAccount struct {
	Subject  domain.UserID
	PersonID domain.PersonID
}

func (ProvisionUserAccount) isOperation() {}

type SuspendUser struct {
	UserID domain.UserID
	Reason string
}

func (SuspendUser) isOperation() {}

type InviteUser struct {
	MembershipID        domain.MembershipID
	OrganizationID      domain.OrganizationID
	InviteeEmail        string
	Role                domain.Role
	InvitedByUserID     string
	InvitationExpiresAt kernel.Instant
}

func (InviteUser) isOperation() {}

type GrantMembership struct {
	MembershipID      domain.MembershipID
	UserID            domain.UserID
	PersonID          domain.PersonID
	ChannelConfirmed  bool
	VerificationLevel domain.VerificationLevel
}

func (GrantMembership) isOperation() {}

type RevokeMembership struct {
	MembershipID domain.MembershipID
	Reason       string
}

func (RevokeMembership) isOperation() {}

type FindMembershipsByUser struct {
	User domain.UserID
}

func (FindMembershipsByUser) isOperation() {}

type FindMembershipsByOrganization struct {
	Organization domain.OrganizationID
}

func (FindMembershipsByOrganization) isOperation() {}

type FindEffectivePermissions struct {
	User domain.UserID
}

func (FindEffectivePermissions) isOperation() {}

type RequestBankAccountChange struct {
	BankAccountChangeID   domain.BankAccountChangeID
	OrganizationID        domain.OrganizationID
	DestinationEvidenceID string
	RequestedByUserID     string
}

func (RequestBankAccountChange) isOperation() {}

type ApproveBankAccountChange struct {
	BankAccountChangeID domain.BankAccountChangeID
	ApprovedByUserID    string
}

func (ApproveBankAccountChange) isOperation() {}

type FindBankAccountChange struct {
	BankAccountChange domain.BankAccountChangeID
}

func (FindBankAccountChange) isOperation() {}

type FindBankAccountChangesByOrganization struct {
	Organization domain.OrganizationID
}

func (FindBankAccountChangesByOrganization) isOperation() {}

type Service struct {
	UoW         port.UnitOfWork[Resources]
	Reader      ports.OrganizationReader
	TaxIDReader ports.OrganizationsByTaxIDReader
	Protector   ports.IdentifierProtector
	Clock       port.Clock
	IDs         port.IDGenerator
	Authorize   usecase.Authorize[Operation]
	Idempotency usecase.IdempotencyPolicy

	EstablishmentTaxIDReader      ports.EstablishmentsByTaxIDReader
	EstablishmentsByOrganization  ports.EstablishmentsByOrganizationReader
	PersonReader                  ports.PersonReader
	DocumentReader                ports.PersonsByDocumentReader
	RepresentationReader          ports.RepresentationReader
	RepresentationsByOrganization ports.RepresentationsByOrganizationReader
	UserAccountReader             ports.UserAccountReader
	MembershipsByUser             ports.MembershipsByUserReader
	MembershipsByOrganization     ports.MembershipsByOrganizationReader

	BankAccountChangeReader          ports.BankAccountChangeReader
	BankAccountChangesByOrganization ports.BankAccountChangesByOrganizationReader

	Instrumentation port.Instrumentation
}

type command[R any] = usecase.Command[Resources, Operation, R]

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

func (s Service) instrumentation() port.Instrumentation {
	if s.Instrumentation == nil {
		return port.NoInstrumentation()
	}
	return s.Instrumentation
}

func origin(aggregateType, aggregateID string) usecase.Origin {
	return usecase.Origin{Destination: Destination, AggregateType: aggregateType, AggregateID: aggregateID}
}

var (
	loadAbsentOrganization   = usecase.Absent[domain.OrganizationSnapshot](domain.NewOrganization)
	loadAbsentEstablishment  = usecase.Absent[domain.EstablishmentSnapshot](domain.NewEstablishment)
	loadAbsentPerson         = usecase.Absent[domain.PersonSnapshot](domain.NewPerson)
	loadAbsentRepresentation = usecase.Absent[domain.RepresentationSnapshot](domain.NewRepresentation)

	loadUserAccount         = usecase.OrNew(domain.NewUserAccount, domain.FromUserAccountSnapshot)
	loadExistingUserAccount = usecase.Existing[domain.UserID](domain.FromUserAccountSnapshot)
	loadAbsentMembership    = usecase.Absent[domain.MembershipSnapshot](domain.NewMembership)
	loadExistingMembership  = usecase.Existing[domain.MembershipID](domain.FromMembershipSnapshot)

	loadAbsentBankAccountChange   = usecase.Absent[domain.BankAccountChangeSnapshot](domain.NewBankAccountChange)
	loadExistingBankAccountChange = usecase.Existing[domain.BankAccountChangeID](domain.FromBankAccountChangeSnapshot)
)
