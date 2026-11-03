package application

import (
	"context"
	"errors"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// receive registers one delivery of an internal channel in the message inbox
// of that channel and, under R1 only, runs the command it carries in the same
// unit of work, so the inbox entry and the command commit together (INB-07).
// The command answers the refusal it recorded, or none.
func (s Service) receive(ctx context.Context, inbox func(Resources) port.Inbox, receipt port.Receipt, consumer string,
	run func(context.Context, executor) (string, error)) (usecase.Disposition, error) {
	receipt.Consumer = consumer
	var disposition usecase.Disposition
	err := s.UoW.Within(ctx, func(ctx context.Context, res Resources) error {
		reception, err := inbox(res).Register(ctx, receipt)
		if err != nil {
			return err
		}
		return reception.Match(
			func(pending port.Pending) error {
				refusal, err := run(ctx, s.executorWithin(res))
				if err != nil {
					return err
				}
				completion := port.Completion{Status: port.StatusProcessed, At: s.Clock.Now()}
				disposition = usecase.R1D1
				if refusal != "" {
					completion.Status, completion.LastError, disposition = port.StatusRejected, refusal, usecase.R1D2
				}
				return pending.Complete(ctx, completion)
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

// refusalOf is the code a command recorded as its refusal, empty when it
// accepted.
func refusalOf[R any](outcome usecase.Outcome[R]) string {
	if rejection, refused := outcome.Rejection(); refused {
		return string(rejection.Code())
	}
	return ""
}

// dependency is a way to a partner or to the raw store that is not there: the
// delivery is redelivered and, when that runs out, quarantined.
func dependency(err error) error {
	var failure *usecase.Failure
	if errors.As(err, &failure) {
		return err
	}
	return usecase.NewFailure(usecase.TransientDependency, true, err)
}
