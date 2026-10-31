package application

import (
	"context"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// SubmissionStep is the Apply a dispatch handler hands the kernel's Dispatch,
// with the partner's code and text its adapter would have read.
func (s Service) SubmissionStep(key domain.IdempotencyKey, partnerCode, partnerMessage string) func(context.Context, Resources, usecase.ExternalStep) error {
	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	return s.applier(identity, stepDetail{
		key: key, origin: domain.OriginSubmit, source: domain.SourceSubmitResponse, partnerCode: partnerCode, partnerMessage: partnerMessage,
	})
}

func CycleOf(s domain.ExternalCommandSnapshot) ExternalCycle { return cycleOf(s) }

func LedgerOf(res Resources) port.AttemptLedger { return ledgerOf(res) }
