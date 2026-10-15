package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func (s Service) RegisterPerson(ctx context.Context, cmd RegisterPerson) (usecase.Outcome[domain.RegisterPersonResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RegisterPersonResponse]{
		Operation: OperationRegisterPerson,
		Object:    string(cmd.PersonID),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationRegisterPerson).
			String(string(cmd.PersonID)).
			String(cmd.FullName).
			String(cmd.Document),
		Codec: registerPersonCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RegisterPersonResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.RegisterPersonResponse], error) {
				return usecase.Outcome[domain.RegisterPersonResponse]{}, fmt.Errorf("application: register person %s: %w", cmd.PersonID, err)
			}
			protected, err := s.protectTaxID(ctx, domain.TaxIDTypePersonDocument, cmd.Document)
			if err != nil {
				return fail(err)
			}
			registered, err := s.documentRegistered(ctx, protected.Hash)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Persons, res.Outbox, origin(AggregateTypePerson, string(cmd.PersonID)), cmd.PersonID, identity,
				loadAbsentPerson,
				func(p *domain.Person) (kernel.Accepted[domain.RegisterPersonResponse], *kernel.Rejection) {
					return p.Register(domain.RegisterPerson{
						FullName:                  cmd.FullName,
						Document:                  cmd.Document,
						DocumentHash:              protected.Hash,
						DocumentHashKeyVersion:    protected.KeyVersion,
						DocumentAlreadyRegistered: registered,
						At:                        identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}
