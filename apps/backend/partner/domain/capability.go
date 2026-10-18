package domain

import (
	"slices"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type CapabilityStatus string

const (
	CapabilityStatusDeclared    CapabilityStatus = "declared"
	CapabilityStatusHomologated CapabilityStatus = "homologated"
	CapabilityStatusEnabled     CapabilityStatus = "enabled"
	CapabilityStatusSuspended   CapabilityStatus = "suspended"
)

type SuspensionTrigger string

const (
	TriggerManual         SuspensionTrigger = "manual"
	TriggerSchemaMismatch SuspensionTrigger = "schema_mismatch"
	TriggerOffboarding    SuspensionTrigger = "offboarding"
)

type CriterionResult struct {
	CriterionID string
	Passed      bool
	EvidenceID  string
	RecordedAt  kernel.Instant
}

type Suspension struct {
	Reason      string
	Trigger     SuspensionTrigger
	SuspendedAt kernel.Instant
}

type Capability struct {
	id                  CapabilityID
	partnerID           PartnerID
	operation           string
	assetType           string
	environment         Environment
	version             int
	supersedesID        CapabilityID
	isWrite             bool
	stateQuerySupported bool
	restrictionNote     string
	callbackApplicable  bool
	status              CapabilityStatus
	homologation        []CriterionResult
	rollout             Rollout
	suspension          Suspension
	enabledAt           kernel.Instant
}

func NewCapability(id CapabilityID) *Capability {
	return &Capability{id: id}
}

func FromCapabilitySnapshot(s CapabilitySnapshot) *Capability {
	return &Capability{
		id:                  s.ID,
		partnerID:           s.PartnerID,
		operation:           s.Operation,
		assetType:           s.AssetType,
		environment:         s.Environment,
		version:             s.Version,
		supersedesID:        s.SupersedesID,
		isWrite:             s.IsWrite,
		stateQuerySupported: s.StateQuerySupported,
		restrictionNote:     s.RestrictionNote,
		callbackApplicable:  s.CallbackApplicable,
		status:              s.Status,
		homologation:        slices.Clone(s.Homologation),
		rollout:             s.Rollout.clone(),
		suspension:          s.Suspension,
		enabledAt:           s.EnabledAt,
	}
}

type CapabilitySnapshot struct {
	ID                  CapabilityID
	PartnerID           PartnerID
	Operation           string
	AssetType           string
	Environment         Environment
	Version             int
	SupersedesID        CapabilityID
	IsWrite             bool
	StateQuerySupported bool
	RestrictionNote     string
	CallbackApplicable  bool
	Status              CapabilityStatus
	Homologation        []CriterionResult
	Rollout             Rollout
	Suspension          Suspension
	EnabledAt           kernel.Instant
}

func (c *Capability) Snapshot() CapabilitySnapshot {
	return CapabilitySnapshot{
		ID:                  c.id,
		PartnerID:           c.partnerID,
		Operation:           c.operation,
		AssetType:           c.assetType,
		Environment:         c.environment,
		Version:             c.version,
		SupersedesID:        c.supersedesID,
		IsWrite:             c.isWrite,
		StateQuerySupported: c.stateQuerySupported,
		RestrictionNote:     c.restrictionNote,
		CallbackApplicable:  c.callbackApplicable,
		Status:              c.status,
		Homologation:        slices.Clone(c.homologation),
		Rollout:             c.rollout.clone(),
		Suspension:          c.suspension,
		EnabledAt:           c.enabledAt,
	}
}

func (s CapabilitySnapshot) Equal(other CapabilitySnapshot) bool {
	return s.ID == other.ID &&
		s.PartnerID == other.PartnerID &&
		s.Operation == other.Operation &&
		s.AssetType == other.AssetType &&
		s.Environment == other.Environment &&
		s.Version == other.Version &&
		s.SupersedesID == other.SupersedesID &&
		s.IsWrite == other.IsWrite &&
		s.StateQuerySupported == other.StateQuerySupported &&
		s.RestrictionNote == other.RestrictionNote &&
		s.CallbackApplicable == other.CallbackApplicable &&
		s.Status == other.Status &&
		slices.Equal(s.Homologation, other.Homologation) &&
		s.Rollout.equal(other.Rollout) &&
		s.Suspension == other.Suspension &&
		s.EnabledAt == other.EnabledAt
}

func (c *Capability) clone() Capability {
	next := *c
	next.homologation = slices.Clone(c.homologation)
	next.rollout = c.rollout.clone()
	return next
}

func (c *Capability) suspendedBy(trigger SuspensionTrigger) bool {
	return c.status == CapabilityStatusSuspended && c.suspension.Trigger == trigger
}
