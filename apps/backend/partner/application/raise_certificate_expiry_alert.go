package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) RaiseCertificateExpiryAlert(ctx context.Context, cmd RaiseCertificateExpiryAlert) (usecase.Outcome[domain.RaiseCertificateExpiryAlertResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RaiseCertificateExpiryAlertResponse]{
		Operation:   OperationRaiseCertificateExpiryAlert,
		Object:      string(cmd.ProfileID),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationRaiseCertificateExpiryAlert).String(string(cmd.ProfileID)).Int(int64(cmd.ThresholdDays)),
		Codec:       raiseCertificateExpiryAlertCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RaiseCertificateExpiryAlertResponse], error) {
			outcome, err := usecase.Decide(ctx, res.IntegrationProfiles, res.Outbox, origin(AggregateTypeIntegrationProfile, string(cmd.ProfileID)), cmd.ProfileID, identity,
				loadExistingProfile,
				func(p *domain.IntegrationProfile) (kernel.Accepted[domain.RaiseCertificateExpiryAlertResponse], *kernel.Rejection) {
					return p.RaiseExpiryAlert(domain.RaiseCertificateExpiryAlert{ThresholdDays: cmd.ThresholdDays, At: identity.OccurredAt})
				})
			if err != nil {
				return usecase.Outcome[domain.RaiseCertificateExpiryAlertResponse]{}, fmt.Errorf("application: raise certificate expiry alert %s: %w", cmd.ProfileID, err)
			}
			return outcome, nil
		},
	})
}
