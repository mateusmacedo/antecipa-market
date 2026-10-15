package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) RegisterRepresentation(ctx context.Context, cmd RegisterRepresentation) (usecase.Outcome[domain.RegisterRepresentationResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RegisterRepresentationResponse]{
		Operation:   OperationRegisterRepresentation,
		Object:      string(cmd.RepresentationID),
		Input:       cmd,
		Fingerprint: registerRepresentationFingerprint(cmd),
		Codec:       registerRepresentationCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RegisterRepresentationResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.RegisterRepresentationResponse], error) {
				return usecase.Outcome[domain.RegisterRepresentationResponse]{}, fmt.Errorf("application: register representation %s: %w", cmd.RepresentationID, err)
			}
			status, err := organizationStatus(ctx, res.Organizations, cmd.OrganizationID)
			if err != nil {
				return fail(err)
			}
			found, err := personFound(ctx, res.Persons, cmd.PersonID)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Representations, res.Outbox, origin(AggregateTypeRepresentation, string(cmd.RepresentationID)), cmd.RepresentationID, identity,
				loadAbsentRepresentation,
				func(r *domain.Representation) (kernel.Accepted[domain.RegisterRepresentationResponse], *kernel.Rejection) {
					return r.Register(domain.RegisterRepresentation{
						OrganizationID:      cmd.OrganizationID,
						OrganizationStatus:  status,
						PersonID:            cmd.PersonID,
						PersonFound:         found,
						PowerType:           cmd.PowerType,
						SourceType:          cmd.SourceType,
						SourceEvidenceID:    cmd.SourceEvidenceID,
						ScopeCurrency:       cmd.ScopeCurrency,
						ScopeMaxAmountMinor: cmd.ScopeMaxAmountMinor,
						ScopeActTypes:       cmd.ScopeActTypes,
						ValidFrom:           cmd.ValidFrom,
						ValidUntil:          cmd.ValidUntil,
						ProposedByUserID:    cmd.ProposedByUserID,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}

func registerRepresentationFingerprint(cmd RegisterRepresentation) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationRegisterRepresentation).
		String(string(cmd.RepresentationID)).
		String(string(cmd.OrganizationID)).
		String(string(cmd.PersonID)).
		String(string(cmd.PowerType)).
		String(string(cmd.SourceType)).
		String(cmd.SourceEvidenceID).
		String(cmd.ScopeCurrency).
		Int(cmd.ScopeMaxAmountMinor).
		Int(int64(len(cmd.ScopeActTypes)))
	for _, act := range cmd.ScopeActTypes {
		f.String(act)
	}
	return f.Int(int64(cmd.ValidFrom)).
		Int(int64(cmd.ValidUntil)).
		String(cmd.ProposedByUserID)
}
