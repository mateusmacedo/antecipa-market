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

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
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
		return runAPI(ctx, cfg, rt)
	case RoleRelay:
		return serve.RunRelay(ctx, rt, relayRole(cfg), serve.Relay{Component: "identity", Channel: IdentityChannel(cfg), Scope: reflect.TypeFor[Config]().PkgPath()})
	default:
		return fmt.Errorf("%w: %q", ErrUnknownRole, cfg.Role)
	}
}

// runAPI serves the reads from a pool of their own: a command of spec note ††
// reads while it holds its transaction, and admission bounds each (method,
// tenant), so no size of a shared pool keeps a connection free for the read.
func runAPI(ctx context.Context, cfg Config, rt *otelboot.Runtime) error {
	if rt == nil {
		return serve.ErrNoRuntime
	}
	reads, err := postgres.NewPool(ctx, cfg.DSN, rt.Tracer(), postgres.WithMeterProvider(rt.MeterProvider()))
	if err != nil {
		return err
	}
	defer reads.Close()
	return serve.ServeAPI(ctx, rt, apiRole(cfg), api(cfg, rt, reads))
}

type Waits struct {
	Command time.Duration
}

func bind(waits Waits) func(*postgres.Tx) application.Resources {
	return func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Organizations:   provider.NewOrganizationRepository(tx),
			Establishments:  provider.NewEstablishmentRepository(tx),
			Persons:         provider.NewPersonRepository(tx),
			Representations: provider.NewRepresentationRepository(tx),
			UserAccounts:    provider.NewUserAccountRepository(tx),
			Memberships:     provider.NewMembershipRepository(tx),
			Outbox:          tx.Outbox(provider.Mapper{}),
			Commands:        tx.CommandInbox(application.CommandConsumer, waits.Command),

			BankAccountChanges: provider.NewBankAccountChangeRepository(tx),
		}
	}
}

func NewService(transactions, readPool *pgxpool.Pool, protector ports.IdentifierProtector, clock port.Clock, ids port.IDGenerator, waits Waits) application.Service {
	reads := postgres.NewReadPool(readPool)
	return application.Service{
		UoW:         postgres.NewUnitOfWork(transactions, bind(waits)),
		Reader:      provider.NewOrganizationReader(reads),
		TaxIDReader: provider.NewOrganizationsByTaxIDReader(reads),
		Protector:   protector,
		Clock:       clock,
		IDs:         ids,
		Authorize:   Authorization(),

		EstablishmentTaxIDReader:      provider.NewEstablishmentsByTaxIDReader(reads),
		EstablishmentsByOrganization:  provider.NewEstablishmentsByOrganizationReader(reads),
		PersonReader:                  provider.NewPersonReader(reads),
		DocumentReader:                provider.NewPersonsByDocumentReader(reads),
		RepresentationReader:          provider.NewRepresentationReader(reads),
		RepresentationsByOrganization: provider.NewRepresentationsByOrganizationReader(reads),
		UserAccountReader:             provider.NewUserAccountReader(reads),
		MembershipsByUser:             provider.NewMembershipsByUserReader(reads),
		MembershipsByOrganization:     provider.NewMembershipsByOrganizationReader(reads),

		BankAccountChangeReader:          provider.NewBankAccountChangeReader(reads),
		BankAccountChangesByOrganization: provider.NewBankAccountChangesByOrganizationReader(reads),
	}
}

// IdempotencyPolicy is the kernel policy with its digest keyed by the identifier
// key: a fingerprint holds fields in clear, the document of RegisterPerson
// among them, and the inbox keeps the digest (spec, Idempotência).
func IdempotencyPolicy(cfg Config) (usecase.IdempotencyPolicy, error) {
	policy, err := kernelapp.IdempotencyPolicy(cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	if err != nil {
		return usecase.IdempotencyPolicy{}, err
	}
	digest, err := provider.NewIdempotencyDigest([]byte(cfg.IdentifierKey.value))
	if err != nil {
		return usecase.IdempotencyPolicy{}, fmt.Errorf("%w: %s: %w", ErrInvalidVariable, envIdentifierKey, err)
	}
	policy.Digest = digest
	return policy, nil
}

func NewIdentityService(transactions, reads *pgxpool.Pool, rt *otelboot.Runtime, cfg Config) (application.Service, error) {
	policy, err := IdempotencyPolicy(cfg)
	if err != nil {
		return application.Service{}, err
	}
	protector, err := cfg.identifierProtector()
	if err != nil {
		return application.Service{}, err
	}
	service := NewService(transactions, reads, protector, idclock.SystemClock{}, idclock.NewMessageIDs("identity"), Waits{Command: cfg.Policies.IdempotencyWait})
	service.Idempotency = policy
	service.Instrumentation = obsusecase.New(rt,
		audit.NewLogSink(rt.LoggerProvider()),
		port.SubjectOf, usecase.CategoryOf,
		application.OperationFindOrganization, application.OperationFindOrganizationByTaxID,
		application.OperationFindEstablishmentsByOrganization, application.OperationFindPerson,
		application.OperationFindRepresentation, application.OperationFindRepresentationsByOrganization,
		application.OperationFindMembershipsByUser, application.OperationFindMembershipsByOrganization,
		application.OperationFindEffectivePermissions,
		application.OperationFindBankAccountChange, application.OperationFindBankAccountChangesByOrganization)
	return service, nil
}

func api(cfg Config, rt *otelboot.Runtime, reads *pgxpool.Pool) serve.API {
	return serve.API{
		Desc: &rpc.ServiceDesc, Commands: rpc.Commands(), Schema: provider.Schema, CommandConsumer: application.CommandConsumer,
		Register: func(server *grpc.Server, pool *pgxpool.Pool) error {
			service, err := NewIdentityService(pool, reads, rt, cfg)
			if err != nil {
				return err
			}
			server.RegisterService(&rpc.ServiceDesc, rpc.Server{Service: service})
			return nil
		},
	}
}
