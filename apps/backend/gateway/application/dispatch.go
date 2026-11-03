package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// DispatchExternalCommand is the handler of the dispatch intent (decision
// 14). The command and its connector are read before any partner call: a
// command out of created and retryable_failure is acknowledged with no send,
// and one whose operation is not allowed is held, because a suspension lasts
// until the rehomologation and would run the redelivery into the quarantine.
func (s Service) DispatchExternalCommand(ctx context.Context, cmd DispatchExternalCommand) (usecase.Disposition, error) {
	if err := s.Authorize(ctx, cmd); err != nil {
		return usecase.Classify(err), err
	}
	command, err := s.intended(ctx, cmd.IdempotencyKey)
	if err != nil {
		return usecase.Classify(err), fmt.Errorf("application: dispatch external command %s: %w", cmd.IdempotencyKey, err)
	}
	// WHY: a discarded intent writes nothing, not even the message inbox: a
	// redelivery of a message that was held must still dispatch once the
	// operation is allowed, and the ledger already absorbs a settled one.
	if command.Status != domain.StatusCreated && command.Status != domain.StatusRetryableFailure {
		return usecase.R1D2, nil
	}
	connector, _, err := s.ConnectorReader.Load(ctx, command.ConnectorKey)
	if err != nil {
		return usecase.Classify(err), fmt.Errorf("application: dispatch external command %s: connector: %w", cmd.IdempotencyKey, err)
	}
	// NOTE: an operation no longer enabled is held like a suspended one, the
	// fail-closed reading of 13 §4; the sweep releases both on the same gate.
	if gate, _ := connector.Gate(command.Operation, command.AssetType); gate != domain.GateAllowed {
		return s.hold(ctx, cmd)
	}
	return s.dispatch(ctx, cmd.Receipt, command)
}

// intended reads the command an intent names. The intent is written in the
// unit of work of the command, so a command not found yet is a read that has
// not caught up, redelivered until the consumer quarantines it.
func (s Service) intended(ctx context.Context, key domain.IdempotencyKey) (domain.ExternalCommandSnapshot, error) {
	command, _, err := s.CommandReader.Load(ctx, key)
	if errors.Is(err, port.ErrNotFound) {
		return command, usecase.NewFailure(usecase.TransientDependency, true, err)
	}
	return command, err
}

func (s Service) hold(ctx context.Context, cmd DispatchExternalCommand) (usecase.Disposition, error) {
	outcome, err := s.HoldDispatch(ctx, HoldDispatch{IdempotencyKey: cmd.IdempotencyKey, MessageID: cmd.Receipt.MessageID})
	if err != nil {
		return usecase.Classify(err), fmt.Errorf("application: dispatch external command %s: %w", cmd.IdempotencyKey, err)
	}
	if _, refused := outcome.Rejection(); refused {
		return usecase.R1D2, nil
	}
	return usecase.R1D1, nil
}

// dispatch calls the partner through the kernel's Dispatch, with a collector of
// its own that the adapter fills and the applier reads (decision 17).
func (s Service) dispatch(ctx context.Context, receipt port.Receipt, command domain.ExternalCommandSnapshot) (usecase.Disposition, error) {
	collector := &ports.CaptureCollector{}
	adapter, err := s.partners().Command(command.ConnectorKey, command.Operation, collector)
	if err != nil {
		failed := dependency(err)
		return usecase.Classify(failed), fmt.Errorf("application: dispatch external command %s: %w", command.Key, failed)
	}
	partner := &submission{ExternalCommand: adapter}
	cycle := cycleOf(command)
	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	receipt.Consumer = DispatchConsumer
	disposition, err := usecase.Dispatch(ctx, usecase.ExternalDispatch[Resources]{
		UoW:             s.UoW,
		Store:           s.Attempts,
		Ledger:          ledgerOf,
		Inbox:           dispatchInboxOf,
		Receipt:         receipt,
		Command:         partner,
		Key:             cycle.Key,
		Payload:         []byte(command.Payload),
		Profile:         cycle.Profile,
		QueryAvailable:  cycle.QueryAvailable,
		StaleAfter:      s.Budget.StaleAfter,
		Clock:           s.Clock,
		SettleContext:   s.Budget.SettleContext,
		Instrumentation: s.instrumentation(),
		Apply: s.applier(identity, stepDetail{
			key: command.Key, origin: domain.OriginSubmit, source: domain.SourceSubmitResponse,
			capture: &submissionCapture{collector: collector, connector: command.ConnectorKey, operation: command.Operation},
		}),
	})
	s.settleCapture(ctx, command, cycle, partner, collector)
	if err != nil {
		return disposition, fmt.Errorf("application: dispatch external command %s: %w", command.Key, err)
	}
	return disposition, nil
}

// submission keeps what the adapter's Submit returned, which Dispatch does not
// hand back when the class is retryable.
type submission struct {
	port.ExternalCommand
	class   port.ExternalClass
	finding port.Finding
}

func (p *submission) Submit(ctx context.Context, key port.OutboundKey, payload []byte) (port.ExternalClass, port.Observation, error) {
	class, observation, err := p.ExternalCommand.Submit(ctx, key, payload)
	p.class, p.finding = class, observation.Finding
	return class, observation, err
}

// settleCapture is the part of the capture the applier never reaches: a lost
// response is telemetry only (decision 17), and a retryable class ends the
// attempt before any step, so its interaction is recorded here, before the
// failure goes back for the redelivery. Losing that record is acceptable: the
// attempt stays in the ledger and the redelivery follows.
func (s Service) settleCapture(ctx context.Context, command domain.ExternalCommandSnapshot, cycle ExternalCycle, partner *submission, collector *ports.CaptureCollector) {
	if lost := collector.LostResponse(); lost != nil {
		s.captureFailed(ctx, command.Key, "the response of a submission was not kept", lost)
	}
	if !partner.class.Retryable() {
		return
	}
	capture, captured := collector.Captured()
	if !captured {
		return
	}
	step, _ := stepOf(usecase.NextStep(usecase.StepInput{
		Class: partner.class, Finding: partner.finding, Profile: cycle.Profile, QueryAvailable: cycle.QueryAvailable,
	}))
	ctx, cancel := s.Budget.SettleContext(context.WithoutCancel(ctx))
	defer cancel()
	outcome, err := s.RecordExternalInteraction(ctx, RecordExternalInteraction{
		InteractionID: capture.InteractionID, ConnectorKey: command.ConnectorKey, Operation: command.Operation,
		Direction: domain.DirectionOutbound, Source: domain.InteractionSubmitResponse, ContentType: capture.ContentType,
		CommandID: command.Key, Request: capture.Request, Response: capture.Response, CapturedStep: step,
	})
	switch refusal := refusalOf(outcome); {
	case err != nil:
		s.captureFailed(ctx, command.Key, "the interaction of a retryable submission was not recorded", err)
	case refusal != "":
		s.captureFailed(ctx, command.Key, "the interaction of a retryable submission was refused", errors.New(refusal))
	}
}

// captureFailed reports a capture that did not reach its record through an
// operation of its own that fails, so the metric counts it while the outcome
// of the submission stands.
func (s Service) captureFailed(ctx context.Context, key domain.IdempotencyKey, message string, cause error) {
	_, end := s.instrumentation().BeginOperation(ctx, OperationCaptureResponse)
	end(port.Result{Outcome: port.OutcomeFailed, Err: usecase.NewFailure(usecase.TransientDependency, true, cause)})
	s.logger().LogAttrs(ctx, slog.LevelWarn, message, slog.String("idempotency_key", string(key)), slog.String("error", cause.Error()))
}
