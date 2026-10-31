package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	obsusecase "github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

func Run(ctx context.Context, cfg Config) error {
	return boot.Boot(ctx, TelemetryOf(cfg), func(ctx context.Context, rt *otelboot.Runtime) error {
		return RunWith(ctx, cfg, rt)
	})
}

// RunWith is the delegate a caller that already owns the telemetry enters by,
// which is what the harnesses use.
func RunWith(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	switch cfg.Role {
	case RoleAPI:
		return serve.ServeAPI(ctx, rt, apiRole(cfg), api(cfg, rt))
	case RoleRelay:
		return runRelay(ctx, rt, cfg)
	case RoleConsumer:
		return runConsumer(ctx, cfg, rt)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

// Waits are the ceilings of INB-17 for a message and of IDM-07 for a command;
// a role that runs no command leaves Command zero.
type Waits struct {
	Message time.Duration
	Command time.Duration
}

// Bind is the composition root's half of ADR-034: one open transaction becomes
// the resource set application declares, so the inboxes, the attempt ledger,
// the state and the outbox share the same pgx.Tx (INB-07).
func Bind(waits Waits) func(tx *postgres.Tx) application.Resources {
	return func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Connectors:          provider.NewConnectorRepository(tx),
			Profiles:            provider.NewCapabilityProfileRepository(tx),
			Authorizations:      provider.NewQueryAuthorizationRepository(tx),
			ExternalCommands:    provider.NewExternalCommandRepository(tx),
			ProfilesByConnector: provider.NewCapabilityProfilesByConnectorWithin(tx),
			CommandsBySeries:    provider.NewExternalCommandsBySeriesWithin(tx),
			Ledger:              tx.AttemptLedger(),
			Outbox:              tx.Outbox(provider.Mapper{}),
			Commands:            tx.CommandInbox(application.CommandConsumer, waits.Command),
			Inbox:               tx.Inbox(ConsumerName, waits.Message),
		}
	}
}

// NewService assembles the application service over Postgres, with the waits
// the caller declares; their values are FND-08's.
func NewService(pool *pgxpool.Pool, clock ports.Clock, ids ports.IDGenerator, waits Waits) application.Service {
	reads := postgres.NewReadPool(pool)
	commands := provider.NewExternalCommandsReader(reads)
	return application.Service{
		UoW:                          unitOfWork(pool, Bind(waits)),
		ConnectorReader:              provider.NewConnectorReader(reads),
		ProfileReader:                provider.NewCapabilityProfileReader(reads),
		ProfilesByConnector:          provider.NewCapabilityProfilesByConnectorReader(reads),
		AuthorizationsByOrganization: provider.NewQueryAuthorizationsByOrganizationReader(reads),
		CommandReader:                provider.NewExternalCommandReader(reads),
		CommandsByBusinessKey:        commands,
		CommandsByStatus:             commands,
		CommandsByOutboundKey:        commands,
		Clock:                        clock,
		IDs:                          ids,
		Authorize:                    Authorization(),
		OutboundKeys:                 OutboundKeyPolicy(),
		Consumer:                     ConsumerName,
	}
}

// OutboundKeyPolicy is SHA-256 over the canonical intent (IDM-11); the
// application block imports no crypto, so the composition root fixes it.
func OutboundKeyPolicy() usecase.OutboundKeyPolicy {
	return usecase.OutboundKeyPolicy{Digest: func(canonical []byte) ports.Fingerprint { return sha256.Sum256(canonical) }}
}

func NewGatewayService(pool *pgxpool.Pool, rt *otelboot.Runtime, cfg Config) (application.Service, error) {
	policy, err := kernelapp.IdempotencyPolicy(cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	if err != nil {
		return application.Service{}, err
	}
	service := NewService(pool, idclock.SystemClock{}, idclock.NewMessageIDs("gateway"), Waits{Message: cfg.Wait, Command: cfg.Policies.IdempotencyWait})
	service.Idempotency = policy
	service.Instrumentation = obsusecase.New(rt, audit.NewLogSink(rt.LoggerProvider()), ports.SubjectOf, usecase.CategoryOf)
	return service, nil
}

// grants is what the consumer waits for besides the kernel's tables.
func grants() []postgres.Grant { return provider.Grants }

func api(cfg Config, rt *otelboot.Runtime) serve.API {
	return serve.API{
		Desc: &rpc.ServiceDesc, Commands: rpc.Commands(), Schema: provider.Schema, Grants: provider.Grants, Uniques: provider.Uniques,
		CommandConsumer: application.CommandConsumer,
		// The API migrates the kernel's attempt ledger, because
		// ResolveUnknownOutcome records its observation there.
		ExternalCommands: true,
		Register: func(server *grpc.Server, pool *pgxpool.Pool) error {
			service, err := NewGatewayService(pool, rt, cfg)
			if err != nil {
				return err
			}
			server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})
			return nil
		},
	}
}
