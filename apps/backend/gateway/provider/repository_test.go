//go:build integration

package provider_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

func connector(marker int) domain.ConnectorSnapshot {
	return domain.ConnectorSnapshot{
		PartnerID: "prt-1", Environment: domain.EnvironmentProduction, CredentialRef: "vault://prt-1", IntegrationVersion: marker,
		Capabilities: []domain.EnabledCapability{{CapabilityID: "cap-1", Operation: "submitEffect", AssetType: "duplicata", StateQuerySupported: true, Version: 3, At: 7}},
		Suspensions:  []domain.OperationSuspension{{Operation: "requestRelease", Cause: domain.CauseSchemaMismatch, Since: 9}},
		RegisteredAt: 1_755_432_000_000_000_001,
	}
}

func operation() domain.OperationCapability {
	return domain.OperationCapability{
		Operation: "submitEffect", Supported: true, Protocol: domain.ProtocolAsyncCallback, StateQuerySupported: true,
		IdempotencySupport: domain.IdempotencyNative, PartialResultSupported: true, AssetTypes: []string{"duplicata", "cartao"},
		MaxItemsPerRequest: 10, MaxRequestsPerMinute: 60, MaxPayloadBytes: 4096, BusinessCalendar: "BR-B3", CutoffTime: "17:00",
		ProcessingWindow: "08:00-18:00", EncumbranceCoverage: domain.CoveragePartial, PositionFreshnessSLA: 300,
		Efficacy: domain.EfficacyByObservation, Quiescence: 30_000_000_000, AuthoritativeAbsence: true, MaxQueries: 5,
		KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32, QueryInterval: 60, AmbiguityDeadline: 3600,
	}
}

func profile(connector domain.ConnectorKey, version int) domain.CapabilityProfileSnapshot {
	return domain.CapabilityProfileSnapshot{
		ConnectorKey: connector, Version: version, EffectiveFrom: kernel.Instant(version), Operations: []domain.OperationCapability{operation()},
	}
}

func authorization(organization domain.OrganizationID, version int) domain.QueryAuthorizationSnapshot {
	return domain.QueryAuthorizationSnapshot{
		OrganizationID: organization, ConsentVersion: version, Status: domain.AuthorizationGranted, AssetTypes: []string{"duplicata"},
		PeriodFromDate: "2026-01-01", PeriodUntilDate: "2026-12-31", GrantedAt: 11, RevokedAt: 0,
	}
}

func command(marker int) domain.ExternalCommandSnapshot {
	return domain.ExternalCommandSnapshot{
		TenantID: "tnt-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", AssetType: "duplicata", BusinessKey: "eff-1",
		PayloadVersion: marker, SeriesKey: "prt-1:production:submitEffect:eff-1", Payload: `{"amount":100}`, PayloadHash: "h1",
		Profile: domain.CommandProfileOf(2, operation(), true), OutboundKey: "out-" + strconv.Itoa(marker),
		Status: domain.StatusUnknownOutcome, FailureCategory: domain.CategoryUnknownSent, CreatedAt: 1, Submissions: 2, LastSubmittedAt: 3,
		WaitingSince: 4, AcknowledgedNotified: true, UnknownNotified: true, NextQueryAt: 5, ResendAfter: 6, ReconciliationRequired: true,
		AmbiguityExpiredAt: 7, ConfirmedAt: 8, ExternalReferenceID: "ext-1",
	}
}

func certify[ID ~string, S any](t *testing.T, repository func(*postgres.Tx) port.Repository[ID, S], reader func(*pgxpool.Pool) port.Reader[ID, S], state func(int) S, marker func(S) int) {
	t.Helper()
	pool := appkit.OpenPool(t)
	tb.Require(t, providerkit.Repository(func() providerkit.RepositorySubject[ID, S] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[ID, S]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[ID, S]) error) error {
				return pg.Within(ctx, pool, repository, fn)
			},
			Reader:           reader(pool),
			NewID:            func(n int) ID { return ID("kit-" + strconv.Itoa(n)) },
			NewState:         state,
			Marker:           marker,
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	}))
}

func TestTheRepositoriesConformToTheKit(t *testing.T) {
	t.Run("connectors", func(t *testing.T) {
		certify(t, provider.NewConnectorRepository, readOver(provider.NewConnectorReader), connector,
			func(s domain.ConnectorSnapshot) int { return s.IntegrationVersion })
	})
	t.Run("capability profiles", func(t *testing.T) {
		certify(t, provider.NewCapabilityProfileRepository, readOver(provider.NewCapabilityProfileReader),
			func(marker int) domain.CapabilityProfileSnapshot { return profile("prt-1:production", marker) },
			func(s domain.CapabilityProfileSnapshot) int { return s.Version })
	})
	t.Run("query authorizations", func(t *testing.T) {
		certify(t, provider.NewQueryAuthorizationRepository,
			func(pool *pgxpool.Pool) port.Reader[domain.ConsentID, domain.QueryAuthorizationSnapshot] {
				return transactionalReader[domain.ConsentID, domain.QueryAuthorizationSnapshot]{pool: pool, repository: provider.NewQueryAuthorizationRepository}
			},
			func(marker int) domain.QueryAuthorizationSnapshot { return authorization("org-1", marker) },
			func(s domain.QueryAuthorizationSnapshot) int { return s.ConsentVersion })
	})
	t.Run("external commands", func(t *testing.T) {
		certify(t, provider.NewExternalCommandRepository, readOver(provider.NewExternalCommandReader), command,
			func(s domain.ExternalCommandSnapshot) int { return s.PayloadVersion })
	})
}

func readOver[ID ~string, S any](reader func(postgres.ReadPool) port.Reader[ID, S]) func(*pgxpool.Pool) port.Reader[ID, S] {
	return func(pool *pgxpool.Pool) port.Reader[ID, S] { return reader(postgres.NewReadPool(pool)) }
}

// transactionalReader reads through the repository: the context reads an
// authorization only by organization, so it publishes no reader by consent.
type transactionalReader[ID ~string, S any] struct {
	pool       *pgxpool.Pool
	repository func(*postgres.Tx) port.Repository[ID, S]
}

func (r transactionalReader[ID, S]) Load(ctx context.Context, id ID) (S, port.Version, error) {
	var (
		state   S
		version port.Version
	)
	err := pg.Within(ctx, r.pool, r.repository, func(ctx context.Context, repo port.Repository[ID, S]) error {
		var err error
		state, version, err = repo.Load(ctx, id)
		return err
	})
	return state, version, err
}

func TestEachAggregateRoundTripsItsWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t)
	reads := postgres.NewReadPool(pool)

	wantConnector := connector(1)
	wantConnector.Key = "prt-1:production"
	pg.Seed(t, ctx, pool, provider.NewConnectorRepository, wantConnector.Key, wantConnector)
	if got, _, err := provider.NewConnectorReader(reads).Load(ctx, wantConnector.Key); err != nil || !got.Equal(wantConnector) {
		t.Fatalf("connector = %+v, %v; want %+v", got, err, wantConnector)
	}

	wantProfile := profile("prt-1:production", 2)
	wantProfile.Key = domain.ProfileKeyOf("prt-1:production", 2)
	pg.Seed(t, ctx, pool, provider.NewCapabilityProfileRepository, wantProfile.Key, wantProfile)
	if got, _, err := provider.NewCapabilityProfileReader(reads).Load(ctx, wantProfile.Key); err != nil || !got.Equal(wantProfile) {
		t.Fatalf("profile = %+v, %v; want %+v", got, err, wantProfile)
	}

	wantCommand := command(1)
	wantCommand.Key = "idem-1"
	pg.Seed(t, ctx, pool, provider.NewExternalCommandRepository, wantCommand.Key, wantCommand)
	if got, _, err := provider.NewExternalCommandReader(reads).Load(ctx, wantCommand.Key); err != nil || !got.Equal(wantCommand) {
		t.Fatalf("command = %+v, %v; want %+v", got, err, wantCommand)
	}

	wantAuthorization := authorization("org-1", 1)
	wantAuthorization.ConsentID = "cns-1"
	pg.Seed(t, ctx, pool, provider.NewQueryAuthorizationRepository, wantAuthorization.ConsentID, wantAuthorization)
	found, err := provider.NewQueryAuthorizationsByOrganizationReader(reads).LoadByOrganization(ctx, "org-1")
	if err != nil || len(found) != 1 || !found[0].Equal(wantAuthorization) {
		t.Fatalf("authorizations = %+v, %v; want %+v", found, err, wantAuthorization)
	}
}
