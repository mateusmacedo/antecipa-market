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

// outboundGeneration never advances: correcting a command after a final
// refusal is a new payload version, hence a new command (IDM-11).
const outboundGeneration = 1

func (s Service) CreateExternalCommand(ctx context.Context, cmd CreateExternalCommand) (usecase.Outcome[domain.CommandStatusResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.CommandStatusResponse]{
		Operation: OperationCreateExternalCommand,
		Object:    string(cmd.IdempotencyKey),
		Input:     cmd,
		Fingerprint: usecase.NewFingerprint(OperationCreateExternalCommand).
			String(string(cmd.IdempotencyKey)).String(string(cmd.TenantID)).String(string(cmd.ConnectorKey)).
			String(cmd.Operation).String(cmd.AssetType).String(cmd.BusinessKey).Int(int64(cmd.PayloadVersion)).
			String(cmd.Payload).String(cmd.PayloadHash),
		Codec: commandStatusCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.CommandStatusResponse], error) {
			facts, err := s.creationFacts(ctx, res, cmd, identity.OccurredAt)
			if err != nil {
				return usecase.Outcome[domain.CommandStatusResponse]{}, fmt.Errorf("application: create external command %s: %w", cmd.IdempotencyKey, err)
			}
			outcome, err := usecase.Decide(ctx, res.ExternalCommands, res.Outbox, dispatchOrigin(cmd.IdempotencyKey), cmd.IdempotencyKey, identity,
				loadCommand,
				func(c *domain.ExternalCommand) (kernel.Accepted[domain.CommandStatusResponse], *kernel.Rejection) {
					return c.Create(domain.CreateExternalCommand{
						TenantID: cmd.TenantID, ConnectorKey: cmd.ConnectorKey, Operation: cmd.Operation, AssetType: cmd.AssetType,
						BusinessKey: cmd.BusinessKey, PayloadVersion: cmd.PayloadVersion, Payload: cmd.Payload, PayloadHash: cmd.PayloadHash,
						Profile: facts.profile, OutboundKey: string(facts.outboundKey), CapabilityGate: facts.gate, SeriesOpen: facts.seriesOpen,
						At: identity.OccurredAt,
					})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: create external command %s: %w", cmd.IdempotencyKey, err)
			}
			return outcome, nil
		},
	})
}

type creationFacts struct {
	gate        domain.CapabilityGate
	profile     domain.CommandProfile
	outboundKey port.OutboundKey
	seriesOpen  bool
}

// creationFacts reads the profile in force and the series under their guards,
// in that order on every command, and calls no partner: a refusal here leaves
// nothing to send (13 §4, fail-closed).
func (s Service) creationFacts(ctx context.Context, res Resources, cmd CreateExternalCommand, at kernel.Instant) (creationFacts, error) {
	facts := creationFacts{gate: domain.GateNotEnabled, seriesOpen: true}
	profiles, err := res.ProfilesByConnector.LoadByConnector(ctx, cmd.ConnectorKey)
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		return facts, err
	}
	effective, inForce := domain.EffectiveProfile(profiles, at)
	declaration, supported := effective.Supports(cmd.Operation, cmd.AssetType)

	connector, _, err := res.Connectors.Load(ctx, cmd.ConnectorKey)
	switch {
	case errors.Is(err, port.ErrNotFound):
	case err != nil:
		return facts, err
	case inForce && supported && domain.IsWriteOperation(cmd.Operation):
		gate, capability := connector.Gate(cmd.Operation, cmd.AssetType)
		facts.gate = gate
		facts.profile = domain.CommandProfileOf(effective.Version, declaration, capability.StateQuerySupported)
	}
	if facts.gate == domain.GateAllowed {
		if facts.outboundKey, err = s.outboundKey(ctx, cmd, facts.profile); err != nil {
			return facts, err
		}
	}

	series, err := res.CommandsBySeries.LoadBySeries(ctx, domain.SeriesKeyOf(cmd.ConnectorKey, cmd.Operation, cmd.BusinessKey))
	if err != nil && !errors.Is(err, port.ErrNotFound) {
		return facts, err
	}
	for _, other := range series {
		if other.Key != cmd.IdempotencyKey && !other.KeepsSeriesOpen(cmd.PayloadVersion) {
			facts.seriesOpen = false
		}
	}
	return facts, nil
}

func (s Service) outboundKey(ctx context.Context, cmd CreateExternalCommand, profile domain.CommandProfile) (port.OutboundKey, error) {
	execution, err := port.RequireExecutionContext(ctx)
	if err != nil {
		return "", err
	}
	tenant, _ := execution.Tenant()
	return usecase.DeriveOutboundKey(s.OutboundKeys, commandProfile(profile), usecase.OutboundIntent{
		Tenant:      string(tenant),
		Destination: string(cmd.ConnectorKey),
		Operation:   cmd.Operation,
		IntentID:    string(cmd.IdempotencyKey),
		Generation:  outboundGeneration,
	})
}

func (s Service) CancelExternalCommand(ctx context.Context, cmd CancelExternalCommand) (usecase.Outcome[domain.CommandResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.CommandResponse]{
		Operation:   OperationCancelExternalCommand,
		Object:      string(cmd.IdempotencyKey),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationCancelExternalCommand).String(string(cmd.IdempotencyKey)),
		Codec:       commandCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.CommandResponse], error) {
			outcome, err := usecase.Decide(ctx, res.ExternalCommands, res.Outbox, origin(AggregateTypeExternalCommand, string(cmd.IdempotencyKey)), cmd.IdempotencyKey, identity,
				loadExistingCommand,
				func(c *domain.ExternalCommand) (kernel.Accepted[domain.CommandResponse], *kernel.Rejection) {
					return c.Cancel(domain.CancelExternalCommand{At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: cancel external command %s: %w", cmd.IdempotencyKey, err)
			}
			return outcome, nil
		},
	})
}

// RequestResend is called by SweepExternalCommands, one command per unit of
// work, once the quiescence the profile declares has elapsed (RES-42).
func (s Service) RequestResend(ctx context.Context, cmd RequestResend) (usecase.Outcome[domain.CommandResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.CommandResponse]{
		Operation:   OperationRequestResend,
		Object:      string(cmd.IdempotencyKey),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationRequestResend).String(string(cmd.IdempotencyKey)),
		Codec:       commandCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.CommandResponse], error) {
			outcome, err := usecase.Decide(ctx, res.ExternalCommands, res.Outbox, dispatchOrigin(cmd.IdempotencyKey), cmd.IdempotencyKey, identity,
				loadExistingCommand,
				func(c *domain.ExternalCommand) (kernel.Accepted[domain.CommandResponse], *kernel.Rejection) {
					return c.RequestResend(domain.RequestResend{At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: request resend %s: %w", cmd.IdempotencyKey, err)
			}
			return outcome, nil
		},
	})
}

// ExpireAmbiguity is called by SweepExternalCommands; the connector is read in
// the unit of work for the partner the escalation event carries.
func (s Service) ExpireAmbiguity(ctx context.Context, cmd ExpireAmbiguity) (usecase.Outcome[domain.CommandResponse], error) {
	return usecase.Execute(ctx, s.executor(), command[domain.CommandResponse]{
		Operation:   OperationExpireAmbiguity,
		Object:      string(cmd.IdempotencyKey),
		Input:       cmd,
		Fingerprint: usecase.NewFingerprint(OperationExpireAmbiguity).String(string(cmd.IdempotencyKey)),
		Codec:       commandCodec,
		Run: func(ctx context.Context, res Resources, identity usecase.Identity) (usecase.Outcome[domain.CommandResponse], error) {
			partner, err := partnerOf(ctx, res, cmd.IdempotencyKey)
			if err != nil {
				return usecase.Outcome[domain.CommandResponse]{}, fmt.Errorf("application: expire ambiguity %s: %w", cmd.IdempotencyKey, err)
			}
			outcome, err := usecase.Decide(ctx, res.ExternalCommands, res.Outbox, origin(AggregateTypeExternalCommand, string(cmd.IdempotencyKey)), cmd.IdempotencyKey, identity,
				loadExistingCommand,
				func(c *domain.ExternalCommand) (kernel.Accepted[domain.CommandResponse], *kernel.Rejection) {
					return c.ExpireAmbiguity(domain.ExpireAmbiguity{PartnerID: partner, At: identity.OccurredAt})
				})
			if err != nil {
				return outcome, fmt.Errorf("application: expire ambiguity %s: %w", cmd.IdempotencyKey, err)
			}
			return outcome, nil
		},
	})
}

func partnerOf(ctx context.Context, res Resources, key domain.IdempotencyKey) (domain.PartnerID, error) {
	command, _, err := res.ExternalCommands.Load(ctx, key)
	if err != nil {
		return "", err
	}
	connector, _, err := res.Connectors.Load(ctx, command.ConnectorKey)
	if err != nil {
		return "", err
	}
	return connector.PartnerID, nil
}
