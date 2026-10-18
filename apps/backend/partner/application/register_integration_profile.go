package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func (s Service) RegisterIntegrationProfile(ctx context.Context, cmd RegisterIntegrationProfile) (usecase.Outcome[domain.RegisterIntegrationProfileResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.RegisterIntegrationProfileResponse]{
		Operation:   OperationRegisterIntegrationProfile,
		Object:      string(cmd.ProfileID),
		Input:       cmd,
		Fingerprint: registerIntegrationProfileFingerprint(cmd),
		Codec:       registerIntegrationProfileCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.RegisterIntegrationProfileResponse], error) {
			fail := func(err error) (usecase.Outcome[domain.RegisterIntegrationProfileResponse], error) {
				return usecase.Outcome[domain.RegisterIntegrationProfileResponse]{}, fmt.Errorf("application: register integration profile %s: %w", cmd.ProfileID, err)
			}
			head, err := s.profileHead(ctx, cmd.PartnerID, cmd.Environment)
			if err != nil {
				return fail(err)
			}
			outcome, err := usecase.Decide(ctx, res.IntegrationProfiles, res.Outbox, origin(AggregateTypeIntegrationProfile, string(cmd.ProfileID)), cmd.ProfileID, identity,
				loadAbsentProfile,
				func(p *domain.IntegrationProfile) (kernel.Accepted[domain.RegisterIntegrationProfileResponse], *kernel.Rejection) {
					return p.Register(domain.RegisterIntegrationProfile{
						PartnerID:            cmd.PartnerID,
						Environment:          cmd.Environment,
						ChangeKind:           cmd.ChangeKind,
						Protocols:            cmd.Protocols,
						Endpoint:             cmd.Endpoint,
						SchemaVersion:        cmd.SchemaVersion,
						SecretRef:            cmd.SecretRef,
						CertificateRef:       cmd.CertificateRef,
						CertificateExpiresAt: cmd.CertificateExpiresAt,
						EffectiveFrom:        cmd.EffectiveFrom,
						SupersedesID:         cmd.SupersedesID,
						Head:                 head,
						At:                   identity.OccurredAt,
					})
				})
			if err != nil {
				return fail(err)
			}
			return outcome, nil
		},
	})
}

func registerIntegrationProfileFingerprint(cmd RegisterIntegrationProfile) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationRegisterIntegrationProfile).
		String(string(cmd.ProfileID)).
		String(string(cmd.PartnerID)).
		String(string(cmd.Environment)).
		String(string(cmd.ChangeKind))
	fingerprintList(f, cmd.Protocols)
	return f.
		String(cmd.Endpoint).
		String(cmd.SchemaVersion).
		String(cmd.SecretRef).
		String(cmd.CertificateRef).
		Int(int64(cmd.CertificateExpiresAt)).
		Int(int64(cmd.EffectiveFrom)).
		String(string(cmd.SupersedesID))
}

func fingerprintList(f *usecase.Fingerprint, items []string) {
	f.Int(int64(len(items)))
	for _, item := range items {
		f.String(item)
	}
}
