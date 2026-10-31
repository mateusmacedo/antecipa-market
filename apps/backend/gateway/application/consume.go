package application

import (
	"context"
	"errors"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// ConsumeCapabilityEnabled projects a CapabilityEnabled of the Partner; the
// broker effect of §6.4 is the adapter's, after the transaction (INB-08).
func (s Service) ConsumeCapabilityEnabled(ctx context.Context, cmd ConsumeCapabilityEnabled) (usecase.Disposition, error) {
	return s.consume(ctx, cmd, cmd.Receipt, cmd.Enable.ConnectorKey, func(at kernel.Instant) connectorDecision {
		return enablement(cmd.Enable, at)
	})
}

func (s Service) ConsumeCapabilitySuspended(ctx context.Context, cmd ConsumeCapabilitySuspended) (usecase.Disposition, error) {
	return s.consume(ctx, cmd, cmd.Receipt, cmd.Suspend.ConnectorKey, func(at kernel.Instant) connectorDecision {
		return suspension(cmd.Suspend, at)
	})
}

// consume walks the seven steps of FND-04 §6.3; step 5 is the only branch
// point (INB-11): R2, R3 and R4 short-circuit without writing.
func (s Service) consume(ctx context.Context, input Operation, receipt port.Receipt, key domain.ConnectorKey, decide func(kernel.Instant) connectorDecision) (usecase.Disposition, error) {
	if err := s.Authorize(ctx, input); err != nil {
		return usecase.Classify(err), err
	}
	identity := usecase.ResolveIdentity(s.Clock, s.IDs, maxEventsPerCommand)
	receipt.Consumer = s.Consumer

	var disposition usecase.Disposition
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		reception, err := res.Inbox.Register(ctx, receipt)
		if err != nil {
			return err
		}
		return reception.Match(
			func(pending port.Pending) error {
				return consumeFirst(ctx, res, key, identity, decide(identity.OccurredAt), pending, &disposition)
			},
			func() error { disposition = usecase.R2; return nil },
			func() error { disposition = usecase.R3; return nil },
			func() error { disposition = usecase.R4; return nil },
		)
	})
	if err != nil {
		return usecase.Classify(err), err
	}
	return disposition, nil
}

// consumeFirst is axis 2 of §6.4, reachable only under R1.
func consumeFirst(ctx context.Context, res Resources, key domain.ConnectorKey, identity usecase.Identity, decide connectorDecision, pending port.Pending, disposition *usecase.Disposition) error {
	connector, stored, err := loadExistingConnector(ctx, res.Connectors, key)
	if errors.Is(err, port.ErrNotFound) {
		// WHY: dropping a suspension would leave the operation open, so the
		// delivery waits for RegisterConnector until the consumer quarantines it.
		err = usecase.NewFailure(usecase.TransientDependency, true, err)
	}
	if err != nil {
		return fmt.Errorf("application: consume %s: %w", key, err)
	}

	accepted, rejection := decide(connector)
	if rejection != nil {
		*disposition = usecase.R1D2
		return pending.Complete(ctx, port.Completion{Status: port.StatusRejected, At: identity.OccurredAt, LastError: string(rejection.Code())})
	}
	if err := res.Connectors.Save(ctx, key, connector.Snapshot(), stored); err != nil {
		if errors.Is(err, port.ErrVersionConflict) {
			// MAP-07: a reread replays the decision instead of repeating one taken.
			err = usecase.NewFailure(usecase.Conflict, true, err)
		}
		return fmt.Errorf("application: consume %s: %w", key, err)
	}
	if err := usecase.Enqueue(ctx, res.Outbox, identity, origin(AggregateTypeConnector, string(key)), stored+1, accepted.Events()); err != nil {
		return fmt.Errorf("application: consume %s: enqueue: %w", key, err)
	}
	*disposition = usecase.R1D1
	return pending.Complete(ctx, port.Completion{Status: port.StatusProcessed, At: identity.OccurredAt})
}
