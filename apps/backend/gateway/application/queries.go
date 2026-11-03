package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func (s Service) FindConnector(ctx context.Context, key domain.ConnectorKey) (domain.ConnectorSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindConnector, FindConnector{ConnectorKey: key},
		func(ctx context.Context) (domain.ConnectorSnapshot, error) {
			connector, _, err := s.ConnectorReader.Load(ctx, key)
			if err != nil {
				return domain.ConnectorSnapshot{}, fmt.Errorf("application: find connector %s: %w", key, err)
			}
			return connector, nil
		})
}

func (s Service) FindExternalCommand(ctx context.Context, key domain.IdempotencyKey) (domain.ExternalCommandSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindExternalCommand, FindExternalCommand{IdempotencyKey: key},
		func(ctx context.Context) (domain.ExternalCommandSnapshot, error) {
			command, _, err := s.CommandReader.Load(ctx, key)
			if err != nil {
				return domain.ExternalCommandSnapshot{}, fmt.Errorf("application: find external command %s: %w", key, err)
			}
			return command, nil
		})
}

func (s Service) FindQueryAuthorizationByOrganization(ctx context.Context, organization domain.OrganizationID) ([]domain.QueryAuthorizationSnapshot, error) {
	return relation(ctx, s, OperationFindQueryAuthorizationByOrganization, FindQueryAuthorizationByOrganization{OrganizationID: organization},
		func(ctx context.Context) ([]domain.QueryAuthorizationSnapshot, error) {
			return s.AuthorizationsByOrganization.LoadByOrganization(ctx, organization)
		})
}

func (s Service) FindExternalCommandsByBusinessKey(ctx context.Context, connector domain.ConnectorKey, businessKey string) ([]domain.ExternalCommandSnapshot, error) {
	found, err := relation(ctx, s, OperationFindExternalCommandsByBusinessKey, FindExternalCommandsByBusinessKey{ConnectorKey: connector, BusinessKey: businessKey},
		func(ctx context.Context) ([]domain.ExternalCommandSnapshot, error) {
			return s.CommandsByBusinessKey.LoadByBusinessKey(ctx, businessKey)
		})
	return ofConnector(found, connector), err
}

// ErrStatusNotInFlight is a terminal status asked of FindExternalCommandsByStatus:
// the terminal sets grow without bound, so only the ones in flight are listed.
var ErrStatusNotInFlight = errors.New("application: the status is not one a command is in flight with")

func (s Service) FindExternalCommandsByStatus(ctx context.Context, status domain.CommandStatus, connector domain.ConnectorKey) ([]domain.ExternalCommandSnapshot, error) {
	if !status.InFlight() {
		return nil, usecase.NewFailure(usecase.Validation, false,
			fmt.Errorf("application: %s %s: %w", OperationFindExternalCommandsByStatus, status, ErrStatusNotInFlight))
	}
	found, err := relation(ctx, s, OperationFindExternalCommandsByStatus, FindExternalCommandsByStatus{Status: status, ConnectorKey: connector},
		func(ctx context.Context) ([]domain.ExternalCommandSnapshot, error) {
			return s.CommandsByStatus.LoadByStatus(ctx, status)
		})
	if connector == "" {
		return found, err
	}
	return ofConnector(found, connector), err
}

func ofConnector(commands []domain.ExternalCommandSnapshot, connector domain.ConnectorKey) []domain.ExternalCommandSnapshot {
	return slices.DeleteFunc(commands, func(c domain.ExternalCommandSnapshot) bool { return c.ConnectorKey != connector })
}

// relation is the read of a list: the caller gets the empty answer of a value
// nobody holds, while the instrumentation still gets a cross-tenant access,
// which IDN-12 records (IDN-13).
func relation[S any](ctx context.Context, s Service, operation string, input Operation, load func(context.Context) ([]S, error)) ([]S, error) {
	ctx, end := s.instrumentation().BeginOperation(ctx, operation)
	if err := s.Authorize(ctx, input); err != nil {
		end(port.AuthorizationResult(err))
		return nil, err
	}
	found, err := load(ctx)
	var access port.CrossTenantAccess
	switch {
	case errors.As(err, &access):
		end(port.Result{Outcome: port.OutcomeFailed, Err: access})
		return []S{}, nil
	case err != nil:
		failed := fmt.Errorf("application: %s: %w", operation, err)
		end(port.Result{Outcome: port.OutcomeFailed, Err: failed})
		return nil, failed
	}
	end(port.Result{Outcome: port.OutcomeAccepted})
	if found == nil {
		return []S{}, nil
	}
	return found, nil
}

func (s Service) FindExternalInteraction(ctx context.Context, id domain.InteractionID) (domain.ExternalInteractionSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindExternalInteraction, FindExternalInteraction{InteractionID: id},
		func(ctx context.Context) (domain.ExternalInteractionSnapshot, error) {
			interaction, _, err := s.InteractionReader.Load(ctx, id)
			if err != nil {
				return domain.ExternalInteractionSnapshot{}, fmt.Errorf("application: find external interaction %s: %w", id, err)
			}
			return interaction, nil
		})
}

// FindExternalInteractionByPartnerEvent answers the first reception of the
// event: a repeated reception names it in DuplicateOf, so it is the one the
// deduplication compares against.
func (s Service) FindExternalInteractionByPartnerEvent(ctx context.Context, connector domain.ConnectorKey, partnerEventID string) (domain.ExternalInteractionSnapshot, error) {
	input := FindExternalInteractionByPartnerEvent{ConnectorKey: connector, PartnerEventID: partnerEventID}
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindExternalInteractionByPartnerEvent, input,
		func(ctx context.Context) (domain.ExternalInteractionSnapshot, error) {
			found, err := s.InteractionsByPartnerEvent.LoadByPartnerEvent(ctx, partnerEventID)
			if err != nil {
				return domain.ExternalInteractionSnapshot{}, fmt.Errorf("application: find external interaction by partner event %s: %w", partnerEventID, err)
			}
			for _, interaction := range found {
				if interaction.ConnectorKey == connector && interaction.Status != domain.InteractionDuplicate {
					return interaction, nil
				}
			}
			return domain.ExternalInteractionSnapshot{}, fmt.Errorf("application: find external interaction by partner event %s: %w", partnerEventID, port.ErrNotFound)
		})
}

func (s Service) FindExternalInteractionsByCommand(ctx context.Context, command domain.IdempotencyKey) ([]domain.ExternalInteractionSnapshot, error) {
	return relation(ctx, s, OperationFindExternalInteractionsByCommand, FindExternalInteractionsByCommand{CommandID: command},
		func(ctx context.Context) ([]domain.ExternalInteractionSnapshot, error) {
			return s.InteractionsByCommand.LoadByCommand(ctx, command)
		})
}

func (s Service) FindExternalReference(ctx context.Context, key domain.ReferenceKey) (domain.ExternalReferenceSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindExternalReference, FindExternalReference{ReferenceKey: key},
		func(ctx context.Context) (domain.ExternalReferenceSnapshot, error) {
			reference, _, err := s.ReferenceReader.Load(ctx, key)
			if err != nil {
				return domain.ExternalReferenceSnapshot{}, fmt.Errorf("application: find external reference %s: %w", key, err)
			}
			return reference, nil
		})
}

func (s Service) FindExternalReferencesBySubject(ctx context.Context, subjectType string, subject domain.SubjectID) ([]domain.ExternalReferenceSnapshot, error) {
	found, err := relation(ctx, s, OperationFindExternalReferencesBySubject, FindExternalReferencesBySubject{SubjectType: subjectType, SubjectID: subject},
		func(ctx context.Context) ([]domain.ExternalReferenceSnapshot, error) {
			return s.ReferencesBySubject.LoadBySubject(ctx, subject)
		})
	return slices.DeleteFunc(found, func(r domain.ExternalReferenceSnapshot) bool { return r.SubjectType != subjectType }), err
}
