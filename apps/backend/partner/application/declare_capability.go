package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) DeclareCapability(ctx context.Context, cmd DeclareCapability) (usecase.Outcome[domain.DeclareCapabilityResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.DeclareCapabilityResponse]{
		Operation:   OperationDeclareCapability,
		Object:      string(cmd.CapabilityID),
		Input:       cmd,
		Fingerprint: declareCapabilityFingerprint(cmd),
		Codec:       declareCapabilityCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.DeclareCapabilityResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.DeclareCapabilityResponse], error) {
				return usecase.Outcome[domain.DeclareCapabilityResponse]{}, fmt.Errorf("application: declare capability %s: %w", cmd.CapabilityID, err)
			}
			head, err := s.capabilityHead(ctx, cmd)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.Capabilities, res.Outbox, origin(AggregateTypeCapability, string(cmd.CapabilityID)), cmd.CapabilityID, identity,
				loadAbsentCapability,
				func(c *domain.Capability) (kernel.Accepted[domain.DeclareCapabilityResponse], *kernel.Rejection) {
					return c.Declare(domain.DeclareCapability{
						PartnerID:           cmd.PartnerID,
						Operation:           cmd.Operation,
						AssetType:           cmd.AssetType,
						Environment:         cmd.Environment,
						IsWrite:             cmd.IsWrite,
						StateQuerySupported: cmd.StateQuerySupported,
						RestrictionNote:     cmd.RestrictionNote,
						CallbackApplicable:  cmd.CallbackApplicable,
						SupersedesID:        cmd.SupersedesID,
						Head:                head,
						At:                  identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}

func declareCapabilityFingerprint(cmd DeclareCapability) *usecase.Fingerprint {
	return usecase.NewFingerprint(OperationDeclareCapability).
		String(string(cmd.CapabilityID)).
		String(string(cmd.PartnerID)).
		String(cmd.Operation).
		String(cmd.AssetType).
		String(string(cmd.Environment)).
		Int(flag(cmd.IsWrite)).
		Int(flag(cmd.StateQuerySupported)).
		String(cmd.RestrictionNote).
		Int(flag(cmd.CallbackApplicable)).
		String(string(cmd.SupersedesID))
}
