package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

type Step string

const (
	StepConfirm   Step = "confirm"
	StepAwait     Step = "await"
	StepRetry     Step = "retry"
	StepFail      Step = "fail"
	StepQuery     Step = "query"
	StepReconcile Step = "reconcile"
	StepSuspend   Step = "suspend"
)

// StepOrigin tells a submission, which counts as one, from an observation of
// the partner's state; the sweep's abandonment of an attempt is a submission.
type StepOrigin string

const (
	OriginSubmit      StepOrigin = "submit"
	OriginObservation StepOrigin = "observation"
)

type Finding string

const (
	FindingNone         Finding = ""
	FindingApplied      Finding = "applied"
	FindingAbsent       Finding = "absent"
	FindingRejected     Finding = "rejected"
	FindingPending      Finding = "pending"
	FindingInconclusive Finding = "inconclusive"
)

type ConfirmationSource string

const (
	SourceSubmitResponse ConfirmationSource = "submit_response"
	SourceStateQuery     ConfirmationSource = "state_query"
	SourceCallback       ConfirmationSource = "callback"
	SourceFile           ConfirmationSource = "file"
	SourceHuman          ConfirmationSource = "human"
)

// ApplyExternalStep is the Apply of the kernel's external cycle: the kernel
// has already recorded the outcome in its ledger and picked the step.
func (c *ExternalCommand) ApplyExternalStep(cmd ApplyExternalStep) (kernel.Accepted[StepApplied], *kernel.Rejection) {
	return kernel.DecideOver(c, (*ExternalCommand).clone, func(next *ExternalCommand) (kernel.Accepted[StepApplied], *kernel.Rejection) {
		if !next.inFlightFor(cmd.Origin) {
			return kernel.Refuse[StepApplied](CodeExternalCommandNotInFlight, "the command is not in flight for this step",
				kernel.Detail{Key: "status", Value: string(next.status)}, kernel.Detail{Key: "origin", Value: string(cmd.Origin)})
		}
		var events []kernel.DomainEvent
		if cmd.Origin == OriginSubmit {
			next.submissions++
			next.lastSubmittedAt = cmd.At
			events = append(events, ExternalCommandSubmitted{CommandHeader: next.header(), Submission: next.submissions, At: cmd.At})
		}
		wasWaiting := next.status.waits()
		next.take(cmd)
		next.settleEpisode(wasWaiting, cmd)
		if arrival := next.arrival(cmd); arrival != nil {
			events = append(events, arrival)
		}
		return kernel.Accept(StepApplied{Status: next.status}, events...), nil
	})
}

// A cancelled command still takes a submission: the cancellation raced the
// dispatch handler, and the external fact prevails over it.
func (c *ExternalCommand) inFlightFor(origin StepOrigin) bool {
	if origin == OriginSubmit {
		return c.status == StatusCreated || c.status == StatusRetryableFailure || c.status == StatusCancelled
	}
	return c.status.waits()
}

func (c *ExternalCommand) take(cmd ApplyExternalStep) {
	queryInterval := seconds(c.profile.QueryInterval)
	switch cmd.Step {
	case StepConfirm:
		c.status = StatusConfirmed
		c.confirmedAt = cmd.At
		if cmd.ExternalReferenceID != "" {
			c.externalReferenceID = cmd.ExternalReferenceID
		}
	case StepAwait:
		c.status = StatusAcknowledged
		if cmd.ExternalReferenceID != "" {
			c.externalReferenceID = cmd.ExternalReferenceID
		}
		if c.profile.StateQuerySupported {
			c.nextQueryAt = cmd.At + queryInterval
		}
	case StepRetry:
		c.status = StatusRetryableFailure
		c.failureCategory = CategoryNone
		c.resendAfter = cmd.At + kernel.Instant(c.profile.Quiescence)
	case StepFail:
		c.status = StatusFinalFailure
		c.failureCategory = CategoryFinalBusiness
		if cmd.ClassCategory.final() {
			c.failureCategory = cmd.ClassCategory
		}
	case StepQuery:
		c.status = StatusUnknownOutcome
		c.failureCategory = cmd.ClassCategory
		c.nextQueryAt = cmd.At + queryInterval
	case StepReconcile:
		c.status = StatusUnknownOutcome
		c.failureCategory = cmd.ClassCategory
		c.reconciliationRequired = true
		c.nextQueryAt = 0
	case StepSuspend:
		c.status = StatusUnknownOutcome
		c.failureCategory = CategorySchemaMismatch
		c.reconciliationRequired = !c.profile.StateQuerySupported
		c.nextQueryAt = 0
		if c.profile.StateQuerySupported {
			c.nextQueryAt = cmd.At
		}
	}
	if cmd.Source == SourceHuman && cmd.Step != StepReconcile {
		c.reconciliationRequired = false
	}
}

// settleEpisode opens an episode on entering the wait and clears everything of
// it on leaving: a reconciliation asked in one episode never passes on.
func (c *ExternalCommand) settleEpisode(wasWaiting bool, cmd ApplyExternalStep) {
	switch waits := c.status.waits(); {
	case waits && !wasWaiting:
		c.waitingSince = cmd.At
	case !waits && wasWaiting:
		c.waitingSince = 0
		c.nextQueryAt = 0
		c.ambiguityExpiredAt = 0
		c.reconciliationRequired = false
		c.acknowledgedNotified = false
		c.unknownNotified = false
	}
}

func (c *ExternalCommand) arrival(cmd ApplyExternalStep) kernel.DomainEvent {
	switch c.status {
	case StatusConfirmed:
		return ExternalCommandConfirmed{
			CommandHeader:       c.header(),
			ExternalReferenceID: c.externalReferenceID,
			ConfirmationSource:  cmd.Source,
			Partial:             cmd.Partial,
			InteractionID:       cmd.InteractionID,
			ConfirmedAt:         c.confirmedAt,
		}
	case StatusAcknowledged:
		if c.acknowledgedNotified {
			return nil
		}
		c.acknowledgedNotified = true
		return ExternalCommandAcknowledged{CommandHeader: c.header(), ExternalReferenceID: cmd.ExternalReferenceID, At: cmd.At}
	case StatusFinalFailure:
		return ExternalCommandFailed{
			CommandHeader:  c.header(),
			Category:       c.failureCategory,
			PartnerCode:    cmd.PartnerCode,
			PartnerMessage: cmd.PartnerMessage,
			InteractionID:  cmd.InteractionID,
			At:             cmd.At,
		}
	case StatusUnknownOutcome:
		if c.unknownNotified {
			return nil
		}
		c.unknownNotified = true
		return ExternalOutcomeUnknown{
			CommandHeader:           c.header(),
			FailureCategory:         c.failureCategory,
			Submissions:             c.submissions,
			LastSubmittedAt:         c.lastSubmittedAt,
			StateQuerySupported:     c.profile.StateQuerySupported,
			RequiresHumanResolution: c.reconciliationRequired,
			UnknownSince:            c.waitingSince,
		}
	default:
		return nil
	}
}
