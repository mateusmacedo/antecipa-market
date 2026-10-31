package application

import (
	"context"
	"errors"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// DeclareCapabilityProfile runs the kernel's ValidateExternalProfile over each
// operation before the transaction and hands the refusals to the domain as a
// fact; the version taken is answered by the identity, not by the UPR.
func (s Service) DeclareCapabilityProfile(ctx context.Context, cmd DeclareCapabilityProfile) (usecase.Outcome[domain.ProfileResponse], error) {
	key := domain.ProfileKeyOf(cmd.ConnectorKey, cmd.Version)
	invalid := invalidProfiles(cmd.Operations)
	return usecase.Execute(ctx, s.executor(), command[domain.ProfileResponse]{
		Operation:   OperationDeclareCapabilityProfile,
		Object:      string(key),
		Input:       cmd,
		Fingerprint: profileFingerprint(cmd),
		Codec:       profileCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ProfileResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Profiles, res.Outbox, origin(AggregateTypeCapabilityProfile, string(key)), key, identity,
				loadAbsentProfile,
				func(p *domain.CapabilityProfile) (kernel.Accepted[domain.ProfileResponse], *kernel.Rejection) {
					return p.Declare(domain.DeclareCapabilityProfile{
						ConnectorKey: cmd.ConnectorKey, Version: cmd.Version, EffectiveFrom: cmd.EffectiveFrom,
						Operations: cmd.Operations, InvalidProfiles: invalid,
					})
				})
			switch {
			case errors.Is(err, port.ErrAlreadyExists):
				return usecase.Rejected[domain.ProfileResponse](domain.RejectCapabilityProfileAlreadyDeclared), nil
			case err != nil:
				return outcome, fmt.Errorf("application: declare capability profile %s: %w", key, err)
			}
			return outcome, nil
		},
	})
}

func profileFingerprint(cmd DeclareCapabilityProfile) *usecase.Fingerprint {
	f := usecase.NewFingerprint(OperationDeclareCapabilityProfile).
		String(string(cmd.ConnectorKey)).Int(int64(cmd.Version)).Int(int64(cmd.EffectiveFrom)).Int(int64(len(cmd.Operations)))
	for _, o := range cmd.Operations {
		f.String(o.Operation).Int(flag(o.Supported)).String(string(o.Protocol)).Int(flag(o.StateQuerySupported)).
			String(string(o.IdempotencySupport)).Int(flag(o.PartialResultSupported)).Int(int64(len(o.AssetTypes)))
		for _, assetType := range o.AssetTypes {
			f.String(assetType)
		}
		f.Int(int64(o.MaxItemsPerRequest)).Int(int64(o.MaxRequestsPerMinute)).Int(int64(o.MaxPayloadBytes)).
			String(o.BusinessCalendar).String(o.CutoffTime).String(o.ProcessingWindow).String(string(o.EncumbranceCoverage)).
			Int(int64(o.PositionFreshnessSLA)).String(string(o.Efficacy)).Int(o.Quiescence).Int(flag(o.AuthoritativeAbsence)).
			Int(int64(o.MaxQueries)).String(o.KeyAlphabet).Int(int64(o.KeyMaxLength)).Int(int64(o.QueryInterval)).Int(int64(o.AmbiguityDeadline))
	}
	return f
}

func (s Service) FindCapabilityProfile(ctx context.Context, connector domain.ConnectorKey, version int) (domain.CapabilityProfileSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindCapabilityProfile,
		FindCapabilityProfile{ConnectorKey: connector, Version: version},
		func(ctx context.Context) (domain.CapabilityProfileSnapshot, error) {
			key := domain.ProfileKeyOf(connector, version)
			profile, _, err := s.ProfileReader.Load(ctx, key)
			if err != nil {
				return domain.CapabilityProfileSnapshot{}, fmt.Errorf("application: find capability profile %s: %w", key, err)
			}
			return profile, nil
		})
}

// FindEffectiveCapabilityProfile answers NotFound when no version is in force
// at the instant, the same answer as a connector without a profile.
func (s Service) FindEffectiveCapabilityProfile(ctx context.Context, connector domain.ConnectorKey, at kernel.Instant) (domain.CapabilityProfileSnapshot, error) {
	return usecase.Query[Operation](ctx, s.instrumentation(), s.Authorize, OperationFindEffectiveCapabilityProfile,
		FindEffectiveCapabilityProfile{ConnectorKey: connector, EffectiveAt: at},
		func(ctx context.Context) (domain.CapabilityProfileSnapshot, error) {
			profiles, err := s.ProfilesByConnector.LoadByConnector(ctx, connector)
			if err != nil && !errors.Is(err, port.ErrNotFound) {
				return domain.CapabilityProfileSnapshot{}, fmt.Errorf("application: find effective capability profile %s: %w", connector, err)
			}
			effective, found := domain.EffectiveProfile(profiles, at)
			if !found {
				return domain.CapabilityProfileSnapshot{}, fmt.Errorf("application: find effective capability profile %s: %w", connector, port.ErrNotFound)
			}
			return effective, nil
		})
}
