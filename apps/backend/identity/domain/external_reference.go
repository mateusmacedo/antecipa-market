package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type ExternalSystem string

const (
	ExternalSystemRegistry   ExternalSystem = "registry"
	ExternalSystemBookkeeper ExternalSystem = "bookkeeper"
	ExternalSystemAcquirer   ExternalSystem = "acquirer"
	ExternalSystemFunder     ExternalSystem = "funder"
	ExternalSystemDepository ExternalSystem = "depository"
)

type ExternalReferenceType string

const (
	ExternalReferenceAsset         ExternalReferenceType = "asset"
	ExternalReferenceEffect        ExternalReferenceType = "effect"
	ExternalReferenceAuthorization ExternalReferenceType = "authorization"
	ExternalReferenceCommand       ExternalReferenceType = "command"
	ExternalReferenceSettlement    ExternalReferenceType = "settlement"
)

// ExternalReference is the identity of an object in a partner system, as the
// data model declares it (doc 90 §2.1): Reference is the partner's value
// exactly, never normalized.
type ExternalReference struct {
	ProviderID      string
	System          ExternalSystem
	ReferenceType   ExternalReferenceType
	Reference       string
	AsOf            kernel.Instant
	ObservedVersion string
}
