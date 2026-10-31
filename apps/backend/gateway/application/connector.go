package application

import (
	"context"
	"fmt"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

func (s Service) RegisterConnector(ctx context.Context, cmd RegisterConnector) (usecase.Outcome[domain.ConnectorResponse], error) {
	key := domain.ConnectorKeyOf(cmd.PartnerID, cmd.Environment)
	return usecase.Execute(ctx, s.executor(), command[domain.ConnectorResponse]{
		Operation: OperationRegisterConnector,
		Object:    string(key),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationRegisterConnector).
			String(string(cmd.PartnerID)).String(string(cmd.Environment)).String(cmd.CredentialRef).Int(int64(cmd.IntegrationVersion)),
		Codec: connectorCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ConnectorResponse], error) {
			outcome, err := usecase.Decide(ctx, res.Connectors, res.Outbox, origin(AggregateTypeConnector, string(key)), key, identity,
				loadConnector,
				func(c *domain.Connector) (kernel.Accepted[domain.ConnectorResponse], *kernel.Rejection) {
					return c.Register(domain.RegisterConnector{
						PartnerID: cmd.PartnerID, Environment: cmd.Environment, CredentialRef: cmd.CredentialRef,
						IntegrationVersion: cmd.IntegrationVersion, At: identity.OccurredAt,
					})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: register connector %s: %w", key, err)
			}
			return outcome, nil
		},
	})
}

func (s Service) SuspendOperation(ctx context.Context, cmd SuspendOperation) (usecase.Outcome[domain.ConnectorResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ConnectorResponse]{
		Operation: OperationSuspendOperation,
		Object:    string(cmd.ConnectorKey),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationSuspendOperation).
			String(string(cmd.ConnectorKey)).String(cmd.Operation).String(cmd.AssetType).String(string(cmd.Cause)).
			String(string(cmd.CapabilityID)).Int(int64(cmd.Version)),
		Codec: connectorCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ConnectorResponse], error) {
			outcome, err := decideConnector(ctx, res, identity, cmd.ConnectorKey, suspension(cmd, identity.OccurredAt))
			if err != nil {
				return outcome, fmt.Errorf("application: suspend operation %s: %w", cmd.ConnectorKey, err)
			}
			return outcome, nil
		},
	})
}

func (s Service) EnableOperation(ctx context.Context, cmd EnableOperation) (usecase.Outcome[domain.ConnectorResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.ConnectorResponse]{
		Operation: OperationEnableOperation,
		Object:    string(cmd.ConnectorKey),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationEnableOperation).
			String(string(cmd.ConnectorKey)).String(string(cmd.CapabilityID)).String(cmd.Operation).String(cmd.AssetType).
			Int(flag(cmd.StateQuerySupported)).Int(int64(cmd.Version)),
		Codec: connectorCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.ConnectorResponse], error) {
			outcome, err := decideConnector(ctx, res, identity, cmd.ConnectorKey, enablement(cmd, identity.OccurredAt))
			if err != nil {
				return outcome, fmt.Errorf("application: enable operation %s: %w", cmd.ConnectorKey, err)
			}
			return outcome, nil
		},
	})
}

type connectorDecision = func(*domain.Connector) (kernel.Accepted[domain.ConnectorResponse], *kernel.Rejection)

func decideConnector(ctx context.Context, res Resources, identity usecase.Identity, key domain.ConnectorKey, decide connectorDecision) (usecase.Outcome[domain.ConnectorResponse], error) {
	return usecase.Decide(ctx, res.Connectors, res.Outbox, origin(AggregateTypeConnector, string(key)), key, identity, loadExistingConnector, decide)
}

func suspension(cmd SuspendOperation, at kernel.Instant) connectorDecision {
	return func(c *domain.Connector) (kernel.Accepted[domain.ConnectorResponse], *kernel.Rejection) {
		return c.SuspendOperation(domain.SuspendOperation{
			Operation: cmd.Operation, AssetType: cmd.AssetType, Cause: cmd.Cause, CapabilityID: cmd.CapabilityID, Version: cmd.Version, At: at,
		})
	}
}

func enablement(cmd EnableOperation, at kernel.Instant) connectorDecision {
	return func(c *domain.Connector) (kernel.Accepted[domain.ConnectorResponse], *kernel.Rejection) {
		return c.EnableOperation(domain.EnableOperation{
			CapabilityID: cmd.CapabilityID, Operation: cmd.Operation, AssetType: cmd.AssetType,
			StateQuerySupported: cmd.StateQuerySupported, Version: cmd.Version, At: at,
		})
	}
}

func flag(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
