package application

import (
	"context"
	"errors"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// ErrNotAPartnerSource is a partner interaction that claims a source the
// partner never sends on its own: only a callback or a file comes in.
var ErrNotAPartnerSource = errors.New("application: a partner interaction comes in as a callback or a file")

// ReceivePartnerInteraction keeps the raw body of a callback or a file and
// records the inbound interaction (decision 19). The first receipt of a
// partner event takes the id derived from it, so the primary key serializes
// two receipts; the loser is a duplicate with its own id and raw version.
func (s Service) ReceivePartnerInteraction(ctx context.Context, cmd ReceivePartnerInteraction) (usecase.Outcome[domain.InteractionResponse], error) {
	var none usecase.Outcome[domain.InteractionResponse]
	if cmd.Source != domain.InteractionCallback && cmd.Source != domain.InteractionFile {
		return none, usecase.NewFailure(usecase.Validation, false,
			fmt.Errorf("application: %s %s: %w", OperationReceivePartnerInteraction, cmd.Source, ErrNotAPartnerSource))
	}
	if err := s.Authorize(ctx, cmd); err != nil {
		return none, err
	}
	declaration, refusal, err := s.declarationOf(ctx, cmd.ConnectorKey, cmd.Operation)
	switch {
	case err != nil:
		return none, fmt.Errorf("application: receive partner interaction %s %s: %w", cmd.ConnectorKey, cmd.Operation, err)
	case refusal != nil:
		return usecase.Rejected[domain.InteractionResponse](refusal), nil
	case len(cmd.Body) > declaration.MaxPayloadBytes:
		return usecase.Rejected[domain.InteractionResponse](domain.RejectExternalInteractionPayloadTooLarge), nil
	}
	first, err := s.inboundID(cmd)
	if err != nil {
		return none, fmt.Errorf("application: receive partner interaction %s %s: %w", cmd.ConnectorKey, cmd.PartnerEventID, err)
	}
	request, err := s.keep(ctx, cmd.ConnectorKey, first, ports.RawRequest, ports.RawBody{ContentType: cmd.ContentType, Body: cmd.Body})
	if err != nil {
		return none, fmt.Errorf("application: receive partner interaction %s: %w", first, err)
	}
	record := RecordExternalInteraction{
		InteractionID: first, ConnectorKey: cmd.ConnectorKey, Operation: cmd.Operation, Direction: domain.DirectionInbound,
		Source: cmd.Source, ContentType: cmd.ContentType, PartnerEventID: cmd.PartnerEventID, BusinessDate: cmd.BusinessDate,
		Request: request,
	}
	return usecase.Execute(ctx, s.executor(), command[domain.InteractionResponse]{
		Operation: OperationReceivePartnerInteraction,
		Object:    string(first),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationReceivePartnerInteraction).
			String(string(cmd.ConnectorKey)).String(cmd.Operation).String(string(cmd.Source)).String(cmd.PartnerEventID).
			String(cmd.BusinessDate).String(cmd.ContentType).String(request.Hash),
		Codec: interactionCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.InteractionResponse], error) {
			outcome, err := recordIn(ctx, res, identity, record)
			if cmd.PartnerEventID != "" && lostTheRace(err) {
				outcome, err = s.recordDuplicate(ctx, res, identity, record)
			}
			if err != nil {
				return outcome, fmt.Errorf("application: receive partner interaction %s: %w", first, err)
			}
			return outcome, nil
		},
	})
}

// declarationOf is the operation as the profile in force declares it. One the
// profile does not declare has no limit to bound the body by, so it fails
// closed; a suspension does not stop a receipt, which follows up what was
// already sent (13 §5).
func (s Service) declarationOf(ctx context.Context, connector domain.ConnectorKey, operation string) (domain.OperationCapability, *kernel.Rejection, error) {
	profiles, err := s.ProfilesByConnector.LoadByConnector(ctx, connector)
	if err != nil && !isNotFound(err) {
		return domain.OperationCapability{}, nil, err
	}
	effective, inForce := domain.EffectiveProfile(profiles, s.Clock.Now())
	declaration, declared := effective.Declaration(operation)
	if !inForce || !declared {
		return domain.OperationCapability{}, domain.RejectConnectorCapabilityNotEnabled, nil
	}
	return declaration, nil, nil
}

// inboundID derives the id of the first receipt of a partner event; without a
// partner event there is nothing to deduplicate on, and each receipt is minted.
func (s Service) inboundID(cmd ReceivePartnerInteraction) (domain.InteractionID, error) {
	if cmd.PartnerEventID == "" {
		return domain.InteractionID(s.IDs.NewMessageID()), nil
	}
	return s.DerivedKeys.InboundInteractionID(cmd.ConnectorKey, cmd.PartnerEventID)
}

// lostTheRace is the first receipt of the partner event already there: read by
// the load, or reported by the insert that waited for its commit.
func lostTheRace(err error) bool {
	return errors.Is(err, port.ErrAlreadyExists) || errors.Is(err, port.ErrVersionConflict)
}

// recordDuplicate records the receipt that lost the race of its partner event.
// The first is read in the same unit of work, which sees it once committed.
func (s Service) recordDuplicate(ctx context.Context, res Resources, identity usecase.Identity, record RecordExternalInteraction) (usecase.Outcome[domain.InteractionResponse], error) {
	first := record.InteractionID
	if _, _, err := res.ExternalInteractions.Load(ctx, first); err != nil {
		return usecase.Outcome[domain.InteractionResponse]{}, err
	}
	record.InteractionID, record.DuplicateOf = domain.InteractionID(s.IDs.NewMessageID()), first
	return recordIn(ctx, res, identity, record)
}

// keep writes one raw side to the store: one that is not there is a
// dependency the caller retries, a key or a body it refuses a defect.
func (s Service) keep(ctx context.Context, connector domain.ConnectorKey, id domain.InteractionID, part ports.RawPart, body ports.RawBody) (domain.RawPayload, error) {
	if s.Raw == nil {
		return domain.RawPayload{}, dependency(ports.ErrRawStoreUnavailable)
	}
	payload, err := s.Raw.Put(ctx, ports.RawKeyOf(connector, id, part), body)
	switch {
	case errors.Is(err, ports.ErrRawStoreUnavailable):
		return payload, dependency(err)
	case errors.Is(err, ports.ErrRawPayloadDefect):
		return payload, usecase.NewFailure(usecase.Unexpected, false, err)
	}
	return payload, err
}
