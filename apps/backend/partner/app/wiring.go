package app

import (
	"context"
	"fmt"
	"reflect"
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
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/grpc"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
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
		return runAPI(ctx, cfg, rt, productionDirectory())
	case RoleRelay:
		return serve.RunRelay(ctx, rt, relayRole(cfg), serve.Relay{Component: "partner", Channel: PartnerChannel(cfg), Scope: reflect.TypeFor[Config]().PkgPath()})
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

func productionDirectory() ports.OrganizationDirectory {
	return provider.UnconfiguredOrganizationDirectory{}
}

func runAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime, directory ports.OrganizationDirectory) error {
	if rt == nil {
		return serve.ErrNoRuntime
	}
	reads, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer reads.Close()
	return serve.ServeAPI(ctx, rt, apiRole(cfg), api(cfg, rt, reads, directory))
}

type Waits struct {
	Command time.Duration
}

func bind(waits Waits) func(*postgres.Tx) application.Resources {
	return func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Partners:   provider.NewPartnerRepository(tx),
			Agreements: provider.NewAgreementRepository(tx),

			IntegrationProfiles: provider.NewIntegrationProfileRepository(tx),
			OperationalPolicies: provider.NewOperationalPolicyRepository(tx),
			Capabilities:        provider.NewCapabilityRepository(tx),
			FinancialProducts:   provider.NewFinancialProductRepository(tx),

			AgreementsByPartner:   provider.NewAgreementsByPartnerWithin(tx),
			ProfilesByPartner:     provider.NewIntegrationProfilesByPartnerWithin(tx),
			CapabilitiesByPartner: provider.NewCapabilitiesByPartnerWithin(tx),

			Outbox:   tx.Outbox(provider.Mapper{}),
			Commands: tx.CommandInbox(application.CommandConsumer, waits.Command),
		}
	}
}

func NewService(transactions, readPool *pgxpool.Pool, directory ports.OrganizationDirectory, rollout ports.RolloutBucket, clock port.Clock, ids port.IDGenerator, waits Waits) application.Service {
	reads := postgres.NewReadPool(readPool)
	return application.Service{
		UoW:                    unitOfWork(transactions, bind(waits)),
		Directory:              directory,
		PartnersByOrganization: provider.NewPartnersByOrganizationReader(reads),
		AgreementsByPartner:    provider.NewAgreementsByPartnerReader(reads),
		ProfilesByPartner:      provider.NewIntegrationProfilesByPartnerReader(reads),
		PoliciesByPartner:      provider.NewOperationalPoliciesByPartnerReader(reads),
		CapabilitiesByPartner:  provider.NewCapabilitiesByPartnerReader(reads),
		ProductsByPartner:      provider.NewFinancialProductsByPartnerReader(reads),
		PartnerReader:          provider.NewPartnerReader(reads),
		PartnersByRole:         provider.NewPartnersByRoleReader(reads),
		ProfilesByEnvironment:  provider.NewIntegrationProfilesByEnvironmentReader(reads),
		Rollout:                rollout,
		Clock:                  clock,
		IDs:                    ids,
		Authorize:              Authorization(),
	}
}

func NewPartnerService(transactions, reads *pgxpool.Pool, rt *otelboot.Runtime, cfg Config, directory ports.OrganizationDirectory) (application.Service, error) {
	policy, err := kernelapp.IdempotencyPolicy(cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	if err != nil {
		return application.Service{}, err
	}
	service := NewService(transactions, reads, directory, provider.RolloutBucket{}, idclock.SystemClock{}, idclock.NewMessageIDs("partner"), Waits{Command: cfg.Policies.IdempotencyWait})
	service.Idempotency = policy
	service.Instrumentation = obsusecase.New(rt, audit.NewLogSink(rt.LoggerProvider()), port.SubjectOf, usecase.CategoryOf)
	return service, nil
}

func api(cfg Config, rt *otelboot.Runtime, reads *pgxpool.Pool, directory ports.OrganizationDirectory) serve.API {
	return serve.API{
		Desc: &rpc.ServiceDesc, Commands: rpc.Commands(), Schema: provider.Schema, Grants: provider.Grants, Uniques: provider.Uniques, CommandConsumer: application.CommandConsumer,
		Register: func(server *grpc.Server, pool *pgxpool.Pool) error {
			service, err := NewPartnerService(pool, reads, rt, cfg, directory)
			if err != nil {
				return err
			}
			server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})
			return nil
		},
	}
}
