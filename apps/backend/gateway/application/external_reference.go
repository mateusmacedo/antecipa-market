package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// LinkExternalReference runs under the key the edge declares.
func (s Service) LinkExternalReference(ctx context.Context, cmd LinkExternalReference) (usecase.Outcome[domain.ReferenceResponse], error) {
	return s.link(ctx, cmd, cmd)
}

// LinkInteractionReference is the link a normalization finds in an
// interaction, under a key derived from the reference and the interaction, so
// a redelivery of the normalization replays it.
func (s Service) LinkInteractionReference(ctx context.Context, cmd LinkInteractionReference) (usecase.Outcome[domain.ReferenceResponse], error) {
	key, err := s.DerivedKeys.key("link", string(cmd.Link.key()), string(cmd.InteractionID))
	if err != nil {
		return usecase.Outcome[domain.ReferenceResponse]{}, fmt.Errorf("application: link external reference %s: %w", cmd.Link.key(), err)
	}
	return s.link(port.WithIdempotencyKey(ctx, key), cmd.Link, cmd)
}

func (s Service) link(ctx context.Context, cmd LinkExternalReference, input Operation) (usecase.Outcome[domain.ReferenceResponse], error) {
	key := cmd.key()
	return usecase.Execute(ctx, s.executor(), command[domain.ReferenceResponse]{
		Operation: OperationLinkExternalReference,
		Object:    string(key),
		Input:     input,
		Fingerprint: usecase.NewFingerprint(OperationLinkExternalReference).
			String(string(cmd.ProviderID)).String(string(cmd.System)).String(string(cmd.ReferenceType)).String(cmd.Reference).
			String(cmd.NormalizedReference).String(cmd.SubjectType).String(string(cmd.SubjectID)).Int(int64(cmd.AsOf)).String(cmd.ObservedVersion),
		Codec: referenceCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ReferenceResponse], error) {
			outcome, err := usecase.Decide(ctx, res.ExternalReferences, res.Outbox, origin(AggregateTypeExternalReference, string(key)), key, identity,
				loadReference,
				func(r *domain.ExternalReference) (kernel.Accepted[domain.ReferenceResponse], *kernel.Rejection) {
					return r.Link(domain.LinkExternalReference{
						ProviderID: cmd.ProviderID, System: cmd.System, ReferenceType: cmd.ReferenceType, Reference: cmd.Reference,
						NormalizedReference: cmd.NormalizedReference, SubjectType: cmd.SubjectType, SubjectID: cmd.SubjectID,
						AsOf: cmd.AsOf, ObservedVersion: cmd.ObservedVersion,
					})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: link external reference %s: %w", key, err)
			}
			return outcome, nil
		},
	})
}

func (c LinkExternalReference) key() domain.ReferenceKey {
	return domain.ReferenceKeyOf(c.ProviderID, c.System, c.ReferenceType, c.Reference)
}
