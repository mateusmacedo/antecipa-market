package domain

import kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

// Cancel is possible only before anything was sent; undoing an effect already
// sent is another external command, never a transition (09 §7).
func (c *ExternalCommand) Cancel(CancelExternalCommand) (kernel.Accepted[CommandResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*ExternalCommand).clone, func(next *ExternalCommand) (kernel.Accepted[CommandResponse], *kernel.Rejection) {
		if next.status != StatusCreated {
			return kernel.Refuse[CommandResponse](CodeExternalCommandAlreadySent, "only a command never sent can be cancelled",
				kernel.Detail{Key: "status", Value: string(next.status)})
		}
		next.status = StatusCancelled
		return kernel.Accept(CommandResponse{IdempotencyKey: next.key}), nil
	})
}

// RequestResend asks the same dispatch again: same intent and generation, so
// the kernel derives the same outbound key (RES-42).
func (c *ExternalCommand) RequestResend(cmd RequestResend) (kernel.Accepted[CommandResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*ExternalCommand).clone, func(next *ExternalCommand) (kernel.Accepted[CommandResponse], *kernel.Rejection) {
		if next.status != StatusRetryableFailure {
			return kernel.Refuse[CommandResponse](CodeExternalCommandNotResendable, "only a retryable failure is resent",
				kernel.Detail{Key: "status", Value: string(next.status)})
		}
		if next.resendAfter == 0 || next.resendAfter > cmd.At {
			return kernel.Refuse[CommandResponse](CodeExternalCommandQuiescenceNotElapsed, "the quiescence before the resend has not elapsed")
		}
		next.resendAfter = 0
		return kernel.Accept(CommandResponse{IdempotencyKey: next.key}, next.dispatchRequested(cmd.At)), nil
	})
}

// ExpireAmbiguity escalates a wait past its deadline once per episode; the
// Backoffice opens the case from the event (00 §7.2.5).
func (c *ExternalCommand) ExpireAmbiguity(cmd ExpireAmbiguity) (kernel.Accepted[CommandResponse], *kernel.Rejection) {
	return kernel.DecideOver(c, (*ExternalCommand).clone, func(next *ExternalCommand) (kernel.Accepted[CommandResponse], *kernel.Rejection) {
		if !next.status.waits() {
			return kernel.Refuse[CommandResponse](CodeExternalCommandNotWaiting, "the command is not waiting for an outcome",
				kernel.Detail{Key: "status", Value: string(next.status)})
		}
		if next.waitingSince+seconds(next.profile.AmbiguityDeadline) > cmd.At {
			return kernel.Refuse[CommandResponse](CodeExternalCommandAmbiguityDeadlineNotReached, "the ambiguity deadline has not been reached")
		}
		response := CommandResponse{IdempotencyKey: next.key}
		if next.ambiguityExpiredAt != 0 {
			return kernel.Accept(response), nil
		}
		next.ambiguityExpiredAt = cmd.At
		next.reconciliationRequired = true
		return kernel.Accept(response, ExternalCommandAmbiguityExpired{
			CommandHeader:       next.header(),
			PartnerID:           cmd.PartnerID,
			Status:              next.status,
			WaitingSince:        next.waitingSince,
			AmbiguityDeadline:   next.profile.AmbiguityDeadline,
			StateQuerySupported: next.profile.StateQuerySupported,
			At:                  next.ambiguityExpiredAt,
		}), nil
	})
}

// ResolveUnknownOutcome decides only whether a human resolution may reach the
// kernel, and as which finding; the transition is ApplyExternalStep's, with
// the step the kernel picks from the finding (RES-43).
func (c *ExternalCommand) ResolveUnknownOutcome(cmd ResolveUnknownOutcome) (kernel.Accepted[ResolutionAccepted], *kernel.Rejection) {
	return kernel.DecideOver(c, (*ExternalCommand).clone, func(next *ExternalCommand) (kernel.Accepted[ResolutionAccepted], *kernel.Rejection) {
		if !next.status.waits() {
			return kernel.Refuse[ResolutionAccepted](CodeExternalCommandNotWaiting, "the command is not waiting for an outcome",
				kernel.Detail{Key: "status", Value: string(next.status)})
		}
		if cmd.Justification == "" {
			return kernel.Refuse[ResolutionAccepted](CodeExternalCommandJustificationMissing, "a human resolution states its justification")
		}
		if cmd.QueryInteractionID == "" {
			return kernel.Refuse[ResolutionAccepted](CodeExternalCommandQueryEvidenceMissing,
				"a human resolution rests on a recorded query or evidence")
		}
		if cmd.Partial && !next.profile.PartialResultSupported {
			return kernel.Refuse[ResolutionAccepted](CodeExternalCommandPartialResultNotSupported,
				"the partner declares no partial result for this operation")
		}
		return kernel.Accept(ResolutionAccepted{Finding: cmd.Resolution.finding()}), nil
	})
}

func (r Resolution) finding() Finding {
	switch r {
	case ResolutionConfirmed:
		return FindingApplied
	case ResolutionFinalFailure:
		return FindingRejected
	default:
		return FindingAbsent
	}
}
