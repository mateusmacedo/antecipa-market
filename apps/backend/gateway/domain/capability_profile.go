package domain

import (
	"slices"
	"strconv"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

type Protocol string

const (
	ProtocolSyncHTTP      Protocol = "sync_http"
	ProtocolAsyncCallback Protocol = "async_callback"
	ProtocolFileBatch     Protocol = "file_batch"
	ProtocolPolling       Protocol = "polling"
)

// IdempotencySupport is the partner's KeySemantics of RES-44: whether it
// deduplicates on the outbound key, which decides if a resend is verifiable.
type IdempotencySupport string

const (
	IdempotencyNative    IdempotencySupport = "native"
	IdempotencyKeyHeader IdempotencySupport = "key_header"
	IdempotencyNone      IdempotencySupport = "none"
)

type EncumbranceCoverage string

const (
	CoverageComplete    EncumbranceCoverage = "complete"
	CoveragePartial     EncumbranceCoverage = "partial"
	CoverageUnavailable EncumbranceCoverage = "unavailable"
)

type Efficacy string

const (
	EfficacyOnSubmit      Efficacy = "on_submit"
	EfficacyByObservation Efficacy = "by_observation"
)

// OperationCapability is one operation of a connector as the partner declares
// it (04 §2.1). Quiescence is in nanoseconds; QueryInterval, AmbiguityDeadline
// and PositionFreshnessSLA in seconds, the last zero when the source declares none.
type OperationCapability struct {
	Operation              string
	Supported              bool
	Protocol               Protocol
	StateQuerySupported    bool
	IdempotencySupport     IdempotencySupport
	PartialResultSupported bool
	AssetTypes             []string
	MaxItemsPerRequest     int
	MaxRequestsPerMinute   int
	MaxPayloadBytes        int
	BusinessCalendar       string
	CutoffTime             string
	ProcessingWindow       string
	EncumbranceCoverage    EncumbranceCoverage
	PositionFreshnessSLA   int
	Efficacy               Efficacy
	Quiescence             int64
	AuthoritativeAbsence   bool
	MaxQueries             int
	KeyAlphabet            string
	KeyMaxLength           int
	QueryInterval          int
	AmbiguityDeadline      int
}

func (o OperationCapability) Equal(other OperationCapability) bool {
	return o.Operation == other.Operation &&
		o.Supported == other.Supported &&
		o.Protocol == other.Protocol &&
		o.StateQuerySupported == other.StateQuerySupported &&
		o.IdempotencySupport == other.IdempotencySupport &&
		o.PartialResultSupported == other.PartialResultSupported &&
		slices.Equal(o.AssetTypes, other.AssetTypes) &&
		o.MaxItemsPerRequest == other.MaxItemsPerRequest &&
		o.MaxRequestsPerMinute == other.MaxRequestsPerMinute &&
		o.MaxPayloadBytes == other.MaxPayloadBytes &&
		o.BusinessCalendar == other.BusinessCalendar &&
		o.CutoffTime == other.CutoffTime &&
		o.ProcessingWindow == other.ProcessingWindow &&
		o.EncumbranceCoverage == other.EncumbranceCoverage &&
		o.PositionFreshnessSLA == other.PositionFreshnessSLA &&
		o.Efficacy == other.Efficacy &&
		o.Quiescence == other.Quiescence &&
		o.AuthoritativeAbsence == other.AuthoritativeAbsence &&
		o.MaxQueries == other.MaxQueries &&
		o.KeyAlphabet == other.KeyAlphabet &&
		o.KeyMaxLength == other.KeyMaxLength &&
		o.QueryInterval == other.QueryInterval &&
		o.AmbiguityDeadline == other.AmbiguityDeadline
}

func (o OperationCapability) clone() OperationCapability {
	o.AssetTypes = slices.Clone(o.AssetTypes)
	return o
}

func (o OperationCapability) servesAsset(assetType string) bool {
	return len(o.AssetTypes) == 0 || slices.Contains(o.AssetTypes, assetType)
}

// InvalidProfile is an operation whose partner declaration the kernel's
// ValidateExternalProfile refused; the use case runs it, the domain decides.
type InvalidProfile struct {
	Operation string
	Reason    string
}

// CapabilityProfile is immutable once declared: a change of capability is a
// new version, never an update.
type CapabilityProfile struct {
	key           ProfileKey
	connectorKey  ConnectorKey
	version       int
	effectiveFrom kernel.Instant
	operations    []OperationCapability
}

func NewCapabilityProfile(key ProfileKey) *CapabilityProfile {
	return &CapabilityProfile{key: key}
}

func FromCapabilityProfileSnapshot(s CapabilityProfileSnapshot) *CapabilityProfile {
	return &CapabilityProfile{
		key:           s.Key,
		connectorKey:  s.ConnectorKey,
		version:       s.Version,
		effectiveFrom: s.EffectiveFrom,
		operations:    cloneOperations(s.Operations),
	}
}

type CapabilityProfileSnapshot struct {
	Key           ProfileKey
	ConnectorKey  ConnectorKey
	Version       int
	EffectiveFrom kernel.Instant
	Operations    []OperationCapability
}

func (p *CapabilityProfile) Snapshot() CapabilityProfileSnapshot {
	return CapabilityProfileSnapshot{
		Key:           p.key,
		ConnectorKey:  p.connectorKey,
		Version:       p.version,
		EffectiveFrom: p.effectiveFrom,
		Operations:    cloneOperations(p.operations),
	}
}

func (s CapabilityProfileSnapshot) Equal(other CapabilityProfileSnapshot) bool {
	return s.Key == other.Key &&
		s.ConnectorKey == other.ConnectorKey &&
		s.Version == other.Version &&
		s.EffectiveFrom == other.EffectiveFrom &&
		slices.EqualFunc(s.Operations, other.Operations, OperationCapability.Equal)
}

func (p *CapabilityProfile) clone() CapabilityProfile {
	return CapabilityProfile{
		key:           p.key,
		connectorKey:  p.connectorKey,
		version:       p.version,
		effectiveFrom: p.effectiveFrom,
		operations:    cloneOperations(p.operations),
	}
}

func cloneOperations(operations []OperationCapability) []OperationCapability {
	if operations == nil {
		return nil
	}
	out := make([]OperationCapability, 0, len(operations))
	for _, o := range operations {
		out = append(out, o.clone())
	}
	return out
}

// Supports answers supports(operation, assetType) of CreateExternalCommand
// with the declaration the command will copy.
func (s CapabilityProfileSnapshot) Supports(operation, assetType string) (OperationCapability, bool) {
	for _, o := range s.Operations {
		if o.Operation == operation {
			return o.clone(), o.Supported && o.servesAsset(assetType)
		}
	}
	return OperationCapability{}, false
}

// EffectiveProfile is the version in force at an instant: the highest version
// whose effectiveFrom has already arrived.
func EffectiveProfile(profiles []CapabilityProfileSnapshot, at kernel.Instant) (CapabilityProfileSnapshot, bool) {
	var (
		effective CapabilityProfileSnapshot
		found     bool
	)
	for _, p := range profiles {
		if p.EffectiveFrom <= at && (!found || p.Version > effective.Version) {
			effective, found = p, true
		}
	}
	return effective, found
}

func (p *CapabilityProfile) Declare(cmd DeclareCapabilityProfile) (kernel.Accepted[ProfileResponse], *kernel.Rejection) {
	return kernel.DecideOver(p, (*CapabilityProfile).clone, func(next *CapabilityProfile) (kernel.Accepted[ProfileResponse], *kernel.Rejection) {
		if len(cmd.Operations) == 0 {
			return kernel.Refuse[ProfileResponse](CodeCapabilityProfileOperationsEmpty, "a capability profile declares at least one operation")
		}
		if len(cmd.InvalidProfiles) > 0 {
			invalid := cmd.InvalidProfiles[0]
			return kernel.Refuse[ProfileResponse](CodeCapabilityProfileExternalProfileInvalid,
				"the partner declaration of the operation is not one a command can run under",
				kernel.Detail{Key: "operation", Value: invalid.Operation},
				kernel.Detail{Key: "reason", Value: invalid.Reason})
		}
		for _, o := range cmd.Operations {
			if rejection := refuseWriteDeclaration(o); rejection != nil {
				return kernel.Accepted[ProfileResponse]{}, rejection
			}
		}
		next.connectorKey = cmd.ConnectorKey
		next.version = cmd.Version
		next.effectiveFrom = cmd.EffectiveFrom
		next.operations = cloneOperations(cmd.Operations)
		return kernel.Accept(ProfileResponse{ProfileKey: next.key}), nil
	})
}

func refuseWriteDeclaration(o OperationCapability) *kernel.Rejection {
	if !IsWriteOperation(o.Operation) {
		return nil
	}
	operation := kernel.Detail{Key: "operation", Value: o.Operation}
	if o.AmbiguityDeadline <= 0 {
		_, rejection := kernel.Refuse[ProfileResponse](CodeCapabilityProfileAmbiguityDeadlineMissing,
			"a write operation declares an ambiguity deadline greater than zero", operation,
			kernel.Detail{Key: "ambiguityDeadline", Value: strconv.Itoa(o.AmbiguityDeadline)})
		return rejection
	}
	if o.StateQuerySupported && o.QueryInterval <= 0 {
		_, rejection := kernel.Refuse[ProfileResponse](CodeCapabilityProfileQueryIntervalMissing,
			"a write operation with state query declares a query interval greater than zero", operation,
			kernel.Detail{Key: "queryInterval", Value: strconv.Itoa(o.QueryInterval)})
		return rejection
	}
	return nil
}
