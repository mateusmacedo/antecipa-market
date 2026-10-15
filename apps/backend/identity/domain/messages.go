package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type OrganizationID string

type OrganizationStatus int

const (
	OrganizationNew OrganizationStatus = iota
	OrganizationActive
	OrganizationSuspended
	OrganizationOffboarded
)

// RegisterOrganization carries the protected tax id as the caller derived it:
// the domain holds no key, so TaxIDHash and TaxIDHashKeyVersion arrive ready,
// and TaxIDAlreadyRegistered is the tenant uniqueness the use case looked up.
type RegisterOrganization struct {
	Kinds                  OrganizationKinds
	Jurisdiction           string
	TaxIDType              TaxIDType
	TaxID                  string
	TaxIDHash              string
	TaxIDHashKeyVersion    int
	TaxIDAlreadyRegistered bool
	LegalName              string
	TradeName              string
	At                     kernel.Instant
}

type RegisterOrganizationResponse struct {
	OrganizationID OrganizationID
}

type OrganizationRegistered struct {
	OrganizationID OrganizationID
	Kinds          OrganizationKinds
	Jurisdiction   string
	TaxIDType      TaxIDType
	TaxIDHash      string
	TaxIDMasked    string
	At             kernel.Instant
}

func (OrganizationRegistered) EventName() string {
	return "identity-organization.organization.registered"
}

var _ kernel.DomainEvent = OrganizationRegistered{}

type EstablishmentID string

// AddEstablishment carries the state of the organization the use case loaded
// as a value (spec note †): OrganizationNew stands for an organization that
// was not found, so it refuses like any status other than active.
type AddEstablishment struct {
	OrganizationID         OrganizationID
	OrganizationStatus     OrganizationStatus
	TaxIDType              TaxIDType
	TaxID                  string
	TaxIDHash              string
	TaxIDHashKeyVersion    int
	TaxIDAlreadyRegistered bool
	TradeName              string
	Receiver               bool
	ExternalReferences     []ExternalReference
	At                     kernel.Instant
}

type AddEstablishmentResponse struct {
	EstablishmentID EstablishmentID
}

type EstablishmentAdded struct {
	EstablishmentID EstablishmentID
	OrganizationID  OrganizationID
	TaxIDType       TaxIDType
	TaxIDHash       string
	Receiver        bool
	At              kernel.Instant
}

func (EstablishmentAdded) EventName() string {
	return "identity-organization.establishment.added"
}

var _ kernel.DomainEvent = EstablishmentAdded{}

type PersonID string

// RegisterPerson carries the document only to be checked and masked: the
// person keeps its hash and mask, never the value (spec note ‡‡).
type RegisterPerson struct {
	FullName                  string
	Document                  string
	DocumentHash              string
	DocumentHashKeyVersion    int
	DocumentAlreadyRegistered bool
	At                        kernel.Instant
}

type RegisterPersonResponse struct {
	PersonID PersonID
}

type RepresentationID string

type RepresentationStatus int

const (
	RepresentationNew RepresentationStatus = iota
	RepresentationDeclared
)

type PowerType string

const (
	PowerLegalRepresentative PowerType = "legal_representative"
	PowerAttorneyInFact      PowerType = "attorney_in_fact"
	PowerOperator            PowerType = "operator"
)

type SourceType string

const (
	SourceArticlesOfAssociation SourceType = "articles_of_association"
	SourcePowerOfAttorney       SourceType = "power_of_attorney"
	SourceBoardResolution       SourceType = "board_resolution"
	SourceInternalDelegation    SourceType = "internal_delegation"
)

// RegisterRepresentation carries the organization status and the existence of
// the person the use case loaded (spec note †). A zero ValidUntil is an
// indefinite validity, and a zero amount with no currency is no monetary scope.
type RegisterRepresentation struct {
	OrganizationID      OrganizationID
	OrganizationStatus  OrganizationStatus
	PersonID            PersonID
	PersonFound         bool
	PowerType           PowerType
	SourceType          SourceType
	SourceEvidenceID    string
	ScopeCurrency       string
	ScopeMaxAmountMinor int64
	ScopeActTypes       []string
	ValidFrom           kernel.Instant
	ValidUntil          kernel.Instant
	ProposedByUserID    string
}

type RegisterRepresentationResponse struct {
	RepresentationID RepresentationID
}

// UserID is the subject claim of the realm (ADR-016), the natural identity of a
// user account; the kernel scopes it by tenant, so one subject has an account
// in each tenant it acts in.
type UserID string

type UserAccountStatus int

const (
	UserAccountNew UserAccountStatus = iota
	UserAccountActive
	UserAccountSuspended
)

type ProvisionUserAccount struct {
	PersonID PersonID
	At       kernel.Instant
}

type ProvisionUserAccountResponse struct {
	UserID UserID
}

type SuspendUser struct {
	Reason string
	At     kernel.Instant
}

type SuspendUserResponse struct {
	UserID UserID
}

type UserSuspended struct {
	UserID UserID
	Reason string
	At     kernel.Instant
}

func (UserSuspended) EventName() string {
	return "identity-organization.user-account.suspended"
}

var _ kernel.DomainEvent = UserSuspended{}

type MembershipID string

type MembershipStatus int

const (
	MembershipStatusNew MembershipStatus = iota
	MembershipStatusInvited
	MembershipStatusActive
	MembershipStatusRevoked
)

// InviteUser carries the status and the kinds of the organization the use case
// loaded (spec note †): OrganizationNew stands for one that was not found.
type InviteUser struct {
	OrganizationID      OrganizationID
	OrganizationStatus  OrganizationStatus
	OrganizationKinds   OrganizationKinds
	InviteeEmail        string
	Role                Role
	InvitedByUserID     string
	InvitationExpiresAt kernel.Instant
	At                  kernel.Instant
}

type InviteUserResponse struct {
	MembershipID MembershipID
}

// GrantMembership carries the status of the user account the use case loaded
// (spec note †): UserAccountNew stands for one that was not found.
type GrantMembership struct {
	UserID            UserID
	UserStatus        UserAccountStatus
	PersonID          PersonID
	ChannelConfirmed  bool
	VerificationLevel VerificationLevel
	At                kernel.Instant
}

type GrantMembershipResponse struct {
	MembershipID MembershipID
}

type MembershipGranted struct {
	MembershipID      MembershipID
	OrganizationID    OrganizationID
	UserID            UserID
	Role              Role
	VerificationLevel VerificationLevel
	At                kernel.Instant
}

func (MembershipGranted) EventName() string {
	return "identity-organization.membership.granted"
}

var _ kernel.DomainEvent = MembershipGranted{}

type RevokeMembership struct {
	Reason string
	At     kernel.Instant
}

type RevokeMembershipResponse struct {
	MembershipID MembershipID
}

type MembershipRevoked struct {
	MembershipID   MembershipID
	OrganizationID OrganizationID
	UserID         UserID
	Role           Role
	Reason         string
	At             kernel.Instant
}

func (MembershipRevoked) EventName() string {
	return "identity-organization.membership.revoked"
}

var _ kernel.DomainEvent = MembershipRevoked{}

type BankAccountChangeID string

type BankAccountChangeStatus int

const (
	BankAccountChangeStatusNew BankAccountChangeStatus = iota
	BankAccountChangeStatusRequested
	BankAccountChangeStatusApproved
)

// RequestBankAccountChange carries the status of the organization the use case
// loaded (spec note †): OrganizationNew stands for one that was not found. The
// bank data itself never reaches this context, only the id of its evidence.
type RequestBankAccountChange struct {
	OrganizationID        OrganizationID
	OrganizationStatus    OrganizationStatus
	DestinationEvidenceID string
	RequestedByUserID     string
	At                    kernel.Instant
}

type RequestBankAccountChangeResponse struct {
	BankAccountChangeID BankAccountChangeID
}

type BankAccountChangeRequested struct {
	BankAccountChangeID   BankAccountChangeID
	OrganizationID        OrganizationID
	DestinationEvidenceID string
	RequestedByUserID     string
	At                    kernel.Instant
}

func (BankAccountChangeRequested) EventName() string {
	return "identity-organization.bank-account-change.requested"
}

var _ kernel.DomainEvent = BankAccountChangeRequested{}

type ApproveBankAccountChange struct {
	ApprovedByUserID string
	At               kernel.Instant
}

type ApproveBankAccountChangeResponse struct {
	BankAccountChangeID BankAccountChangeID
}

type BankAccountChangeApproved struct {
	BankAccountChangeID BankAccountChangeID
	OrganizationID      OrganizationID
	ApprovedByUserID    string
	At                  kernel.Instant
}

func (BankAccountChangeApproved) EventName() string {
	return "identity-organization.bank-account-change.approved"
}

var _ kernel.DomainEvent = BankAccountChangeApproved{}
