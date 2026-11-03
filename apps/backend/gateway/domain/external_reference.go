package domain

import (
	"strings"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type ExternalSystem string

const (
	SystemRegistry   ExternalSystem = "registry"
	SystemBookkeeper ExternalSystem = "bookkeeper"
	SystemAcquirer   ExternalSystem = "acquirer"
	SystemFunder     ExternalSystem = "funder"
	SystemDepository ExternalSystem = "depository"
)

type ReferenceType string

const (
	ReferenceAsset         ReferenceType = "asset"
	ReferenceEffect        ReferenceType = "effect"
	ReferenceAuthorization ReferenceType = "authorization"
	ReferenceCommand       ReferenceType = "command"
	ReferenceSettlement    ReferenceType = "settlement"
)

// SubjectID is the id of the internal object a reference points to, which
// lives in another context (asset, effect, authorization, settlement).
type SubjectID string

// ReferenceKey is "<providerId>:<system>:<referenceType>:<reference>", the
// natural identity of an ExternalReference: one reference of a provider names
// one internal object. The provider holds no colon and the system and the type
// are enumerations, so the rest of the key is the reference, colons included.
type ReferenceKey string

func ReferenceKeyOf(provider PartnerID, system ExternalSystem, kind ReferenceType, reference string) ReferenceKey {
	return ReferenceKey(string(provider) + ":" + string(system) + ":" + string(kind) + ":" + reference)
}

// NormalizeReference is the derivation a link takes when the adapter declares
// none; the mask of a provider is the adapter's to remove (90 §2.1).
func NormalizeReference(reference string) string {
	return strings.ToUpper(strings.TrimSpace(reference))
}

// ExternalReference links the identity of an object in an external system to
// the internal object: the reference stays exactly as received, beside the
// normalized form derived from it.
type ExternalReference struct {
	key                 ReferenceKey
	providerID          PartnerID
	system              ExternalSystem
	referenceType       ReferenceType
	reference           string
	normalizedReference string
	subjectType         string
	subjectID           SubjectID
	asOf                kernel.Instant
	observedVersion     string
}

func NewExternalReference(key ReferenceKey) *ExternalReference {
	return &ExternalReference{key: key}
}

type ExternalReferenceSnapshot struct {
	Key                 ReferenceKey
	ProviderID          PartnerID
	System              ExternalSystem
	ReferenceType       ReferenceType
	Reference           string
	NormalizedReference string
	SubjectType         string
	SubjectID           SubjectID
	AsOf                kernel.Instant
	ObservedVersion     string
}

func FromExternalReferenceSnapshot(s ExternalReferenceSnapshot) *ExternalReference {
	return &ExternalReference{
		key:                 s.Key,
		providerID:          s.ProviderID,
		system:              s.System,
		referenceType:       s.ReferenceType,
		reference:           s.Reference,
		normalizedReference: s.NormalizedReference,
		subjectType:         s.SubjectType,
		subjectID:           s.SubjectID,
		asOf:                s.AsOf,
		observedVersion:     s.ObservedVersion,
	}
}

func (r *ExternalReference) Snapshot() ExternalReferenceSnapshot {
	return ExternalReferenceSnapshot{
		Key:                 r.key,
		ProviderID:          r.providerID,
		System:              r.system,
		ReferenceType:       r.referenceType,
		Reference:           r.reference,
		NormalizedReference: r.normalizedReference,
		SubjectType:         r.subjectType,
		SubjectID:           r.subjectID,
		AsOf:                r.asOf,
		ObservedVersion:     r.observedVersion,
	}
}

// Equal is plain comparison: the snapshot holds no reference type.
func (s ExternalReferenceSnapshot) Equal(other ExternalReferenceSnapshot) bool {
	return s == other
}

func (r *ExternalReference) clone() ExternalReference {
	return *r
}

// Link points the reference to an internal object once; a later link to the
// same object only renews when it was obtained, and a link to another object
// is refused, because one reference names one object.
func (r *ExternalReference) Link(cmd LinkExternalReference) (kernel.Accepted[ReferenceResponse], *kernel.Rejection) {
	return kernel.DecideOver(r, (*ExternalReference).clone, func(next *ExternalReference) (kernel.Accepted[ReferenceResponse], *kernel.Rejection) {
		if cmd.Reference == "" {
			return kernel.Refuse[ReferenceResponse](CodeExternalReferenceReferenceEmpty, "reference must not be empty")
		}
		// INVARIANT: a provider with a colon composes the key of another link,
		// so it is refused before the loaded reference is looked at.
		if strings.Contains(string(cmd.ProviderID), ":") {
			return kernel.Refuse[ReferenceResponse](CodeExternalReferenceProviderInvalid, "the provider must not hold a colon",
				kernel.Detail{Key: "providerId", Value: string(cmd.ProviderID)})
		}
		response := ReferenceResponse{ReferenceKey: next.key}
		if next.subjectID == "" {
			next.providerID = cmd.ProviderID
			next.system = cmd.System
			next.referenceType = cmd.ReferenceType
			next.reference = cmd.Reference
			next.normalizedReference = cmd.NormalizedReference
			if next.normalizedReference == "" {
				next.normalizedReference = NormalizeReference(cmd.Reference)
			}
			next.subjectType = cmd.SubjectType
			next.subjectID = cmd.SubjectID
			next.asOf = cmd.AsOf
			next.observedVersion = cmd.ObservedVersion
			return kernel.Accept(response), nil
		}
		if next.subjectType != cmd.SubjectType || next.subjectID != cmd.SubjectID {
			return kernel.Refuse[ReferenceResponse](CodeExternalReferenceAlreadyLinked, "the reference already points to another internal object",
				kernel.Detail{Key: "subjectType", Value: next.subjectType}, kernel.Detail{Key: "subjectId", Value: string(next.subjectID)})
		}
		if cmd.AsOf > next.asOf {
			next.asOf = cmd.AsOf
			next.observedVersion = cmd.ObservedVersion
		}
		return kernel.Accept(response), nil
	})
}
