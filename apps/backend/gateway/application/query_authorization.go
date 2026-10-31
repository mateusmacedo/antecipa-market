package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func (s Service) GrantQueryAuthorization(ctx context.Context, cmd GrantQueryAuthorization) (usecase.Outcome[domain.AuthorizationResponse], error) {
	fingerprint := usecase.NewFingerprint(OperationGrantQueryAuthorization).
		String(string(cmd.ConsentID)).String(string(cmd.OrganizationID)).Int(int64(cmd.ConsentVersion)).
		Int(int64(len(cmd.AssetTypes)))
	for _, assetType := range cmd.AssetTypes {
		fingerprint.String(assetType)
	}
	fingerprint.String(cmd.PeriodFromDate).String(cmd.PeriodUntilDate)
	return usecase.Execute(ctx, s.executor(), command[domain.AuthorizationResponse]{
		Operation:   OperationGrantQueryAuthorization,
		Object:      string(cmd.ConsentID),
		Input:       cmd,
		Fingerprint: fingerprint,
		Codec:       authorizationCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.AuthorizationResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Authorizations, res.Outbox, origin(AggregateTypeQueryAuthorization, string(cmd.ConsentID)), cmd.ConsentID, identity,
				loadAuthorization,
				func(a *domain.QueryAuthorization) (kernel.Accepted[domain.AuthorizationResponse], *kernel.Rejection) {
					return a.Grant(domain.GrantQueryAuthorization{
						OrganizationID: cmd.OrganizationID, ConsentVersion: cmd.ConsentVersion, AssetTypes: cmd.AssetTypes,
						PeriodFromDate: cmd.PeriodFromDate, PeriodUntilDate: cmd.PeriodUntilDate, At: identity.OccurredAt,
					})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: grant query authorization %s: %w", cmd.ConsentID, err)
			}
			return outcome, nil
		},
	})
}

func (s Service) RevokeQueryAuthorization(ctx context.Context, cmd RevokeQueryAuthorization) (usecase.Outcome[domain.AuthorizationResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.AuthorizationResponse]{
		Operation:   OperationRevokeQueryAuthorization,
		Object:      string(cmd.ConsentID),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationRevokeQueryAuthorization).String(string(cmd.ConsentID)).Int(int64(cmd.EffectiveAt)),
		Codec:       authorizationCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.AuthorizationResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Authorizations, res.Outbox, origin(AggregateTypeQueryAuthorization, string(cmd.ConsentID)), cmd.ConsentID, identity,
				loadExistingAuthorization,
				func(a *domain.QueryAuthorization) (kernel.Accepted[domain.AuthorizationResponse], *kernel.Rejection) {
					return a.Revoke(domain.RevokeQueryAuthorization{EffectiveAt: cmd.EffectiveAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: revoke query authorization %s: %w", cmd.ConsentID, err)
			}
			return outcome, nil
		},
	})
}
