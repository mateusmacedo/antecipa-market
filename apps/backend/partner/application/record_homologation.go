package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) RecordHomologation(ctx context.Context, cmd RecordHomologation) (usecase.Outcome[domain.RecordHomologationResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RecordHomologationResponse]{
		Operation:   OperationRecordHomologation,
		Object:      string(cmd.CapabilityID),
		Input:       cmd,
		Fingerprint: recordHomologationFingerprint(cmd),
		Codec:       recordHomologationCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RecordHomologationResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Capabilities, res.Outbox, origin(AggregateTypeCapability, string(cmd.CapabilityID)), cmd.CapabilityID, identity,
				loadExistingCapability,
				func(c *domain.Capability) (kernel.Accepted[domain.RecordHomologationResponse], *kernel.Rejection) {
					return c.RecordHomologation(domain.RecordHomologation{Results: cmd.Results, At: identity.OccurredAt})
				})
			if err != nil {
				return usecase.Outcome[domain.RecordHomologationResponse]{}, fmt.Errorf("application: record homologation %s: %w", cmd.CapabilityID, err)
			}
			return outcome, nil
		},
	})
}

func recordHomologationFingerprint(cmd RecordHomologation) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationRecordHomologation).
		String(string(cmd.CapabilityID)).
		Int(int64(len(cmd.Results)))
	for _, r := range cmd.Results {
		f.String(r.CriterionID).Int(flag(r.Passed)).String(r.EvidenceID)
	}
	return f
}
