package application

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// ExternalCycle is what the kernel's cycle needs of a command: the outbound
// key, the partner declaration of RES-44 and whether a state query exists.
type ExternalCycle struct {
	Key            port.OutboundKey
	Profile        port.ExternalProfile
	QueryAvailable bool
}

func cycleOf(s domain.ExternalCommandSnapshot) ExternalCycle {
	return ExternalCycle{
		Key:            port.OutboundKey(s.OutboundKey),
		Profile:        commandProfile(s.Profile),
		QueryAvailable: s.Profile.StateQuerySupported,
	}
}

func commandProfile(p domain.CommandProfile) port.ExternalProfile {
	return externalProfile(p.IdempotencySupport, p.Efficacy, p.Quiescence, p.AuthoritativeAbsence, p.MaxQueries, p.KeyAlphabet, p.KeyMaxLength)
}

func declaredProfile(o domain.OperationCapability) port.ExternalProfile {
	return externalProfile(o.IdempotencySupport, o.Efficacy, o.Quiescence, o.AuthoritativeAbsence, o.MaxQueries, o.KeyAlphabet, o.KeyMaxLength)
}

func externalProfile(idempotency domain.IdempotencySupport, efficacy domain.Efficacy, quiescence int64, authoritativeAbsence bool, maxQueries int, alphabet string, maxLength int) port.ExternalProfile {
	return port.ExternalProfile{
		KeySemantics:         keySemantics(idempotency),
		Efficacy:             externalEfficacy(efficacy),
		Quiescence:           quiescence,
		AuthoritativeAbsence: authoritativeAbsence,
		MaxQueries:           maxQueries,
		KeyAlphabet:          alphabet,
		KeyMaxLength:         maxLength,
	}
}

// An undeclared value maps to zero, which ValidateExternalProfile refuses.
func keySemantics(s domain.IdempotencySupport) port.KeySemantics {
	switch s {
	case domain.IdempotencyNative:
		return port.KeyNative
	case domain.IdempotencyKeyHeader:
		return port.KeyHeader
	case domain.IdempotencyNone:
		return port.KeyNone
	default:
		return 0
	}
}

func externalEfficacy(e domain.Efficacy) port.Efficacy {
	switch e {
	case domain.EfficacyOnSubmit:
		return port.EfficacyOnSubmit
	case domain.EfficacyByObservation:
		return port.EfficacyByObservation
	default:
		return 0
	}
}

func invalidProfiles(operations []domain.OperationCapability) []domain.InvalidProfile {
	var invalid []domain.InvalidProfile
	for _, o := range operations {
		if err := usecase.ValidateExternalProfile(declaredProfile(o)); err != nil {
			invalid = append(invalid, domain.InvalidProfile{Operation: o.Operation, Reason: err.Error()})
		}
	}
	return invalid
}

// categoryOf is the one-to-one map of the spec; a retryable class never gets
// here, because Dispatch asks for the redelivery instead.
func categoryOf(class port.ExternalClass) domain.FailureCategory {
	switch class {
	case port.ClassAccepted:
		return domain.CategoryNone
	case port.ClassFinalValidation:
		return domain.CategoryFinalValidation
	case port.ClassFinalBusiness:
		return domain.CategoryFinalBusiness
	case port.ClassFinalAuthorization:
		return domain.CategoryFinalAuthorization
	case port.ClassUnknownSent:
		return domain.CategoryUnknownSent
	case port.ClassSchemaMismatch:
		return domain.CategorySchemaMismatch
	default:
		return domain.CategoryUnknownAmbiguous
	}
}

func findingOf(f port.Finding) domain.Finding {
	switch f {
	case port.FindingApplied:
		return domain.FindingApplied
	case port.FindingAbsent:
		return domain.FindingAbsent
	case port.FindingRejected:
		return domain.FindingRejected
	case port.FindingPending:
		return domain.FindingPending
	case port.FindingInconclusive:
		return domain.FindingInconclusive
	default:
		return domain.FindingNone
	}
}

func externalFinding(f domain.Finding) port.Finding {
	switch f {
	case domain.FindingApplied:
		return port.FindingApplied
	case domain.FindingAbsent:
		return port.FindingAbsent
	case domain.FindingRejected:
		return port.FindingRejected
	case domain.FindingPending:
		return port.FindingPending
	case domain.FindingInconclusive:
		return port.FindingInconclusive
	default:
		return 0
	}
}

func stepOf(s usecase.Step) (domain.Step, bool) {
	switch s {
	case usecase.StepConfirm:
		return domain.StepConfirm, true
	case usecase.StepAwait:
		return domain.StepAwait, true
	case usecase.StepRetry:
		return domain.StepRetry, true
	case usecase.StepFail:
		return domain.StepFail, true
	case usecase.StepQuery:
		return domain.StepQuery, true
	case usecase.StepReconcile:
		return domain.StepReconcile, true
	case usecase.StepSuspend:
		return domain.StepSuspend, true
	default:
		return "", false
	}
}
