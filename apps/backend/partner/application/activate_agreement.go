package application

import (
	"context"
	"fmt"
	"slices"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) ActivateAgreement(ctx context.Context, cmd ActivateAgreement) (usecase.Outcome[domain.ActivateAgreementResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ActivateAgreementResponse]{
		Operation:   OperationActivateAgreement,
		Object:      string(cmd.AgreementID),
		Input:       cmd,
		Fingerprint: activateAgreementFingerprint(cmd),
		Codec:       activateAgreementCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ActivateAgreementResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Agreements, res.Outbox, origin(AggregateTypePartnerAgreement, string(cmd.AgreementID)), cmd.AgreementID, identity,
				loadExistingAgreement,
				func(a *domain.PartnerAgreement) (kernel.Accepted[domain.ActivateAgreementResponse], *kernel.Rejection) {
					return a.Activate(domain.ActivateAgreement{
						ContractEvidenceID:        cmd.ContractEvidenceID,
						SatisfiedCriteria:         cmd.SatisfiedCriteria,
						RegulatoryCheckApplicable: cmd.RegulatoryCheckApplicable,
						At:                        identity.OccurredAt,
					})
				})
			if err != nil {
				return usecase.Outcome[domain.ActivateAgreementResponse]{}, fmt.Errorf("application: activate agreement %s: %w", cmd.AgreementID, err)
			}
			return outcome, nil
		},
	})
}

func activateAgreementFingerprint(cmd ActivateAgreement) *usecase.Fingerprint {
	criteria := slices.Compact(slices.Sorted(slices.Values(cmd.SatisfiedCriteria)))
	f := usecase.NewFingerprint(OperationActivateAgreement).
		String(string(cmd.AgreementID)).
		String(cmd.ContractEvidenceID).
		Int(int64(len(criteria)))
	for _, criterion := range criteria {
		f.String(criterion)
	}
	return f.Int(flag(cmd.RegulatoryCheckApplicable))
}

func flag(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
